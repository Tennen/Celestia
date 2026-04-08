package pluginmgr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/chentianyu/celestia/internal/coreapi"
	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/pluginapi"
	"google.golang.org/protobuf/types/known/structpb"
)

func (m *Manager) persistPluginConfig(ctx context.Context, pluginID string, config map[string]any) (models.PluginInstallRecord, error) {
	record, ok, err := m.store.GetPluginRecord(ctx, pluginID)
	if err != nil {
		return models.PluginInstallRecord{}, err
	}
	if !ok {
		return models.PluginInstallRecord{}, errors.New("plugin not installed")
	}
	cloned, err := cloneConfig(config)
	if err != nil {
		return models.PluginInstallRecord{}, err
	}
	record.Config = cloned
	record.UpdatedAt = time.Now().UTC()
	if err := m.store.UpsertPluginRecord(ctx, record); err != nil {
		return models.PluginInstallRecord{}, err
	}
	m.mu.RLock()
	runtime := m.runtimes[pluginID]
	m.mu.RUnlock()
	if runtime != nil {
		runtime.record.Config = cloned
		runtime.record.UpdatedAt = record.UpdatedAt
	}
	return record, nil
}

func cloneConfig(config map[string]any) (map[string]any, error) {
	raw, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *configServiceServer) PersistPluginConfig(ctx context.Context, in *structpb.Struct) (*structpb.Struct, error) {
	var req coreapi.PersistPluginConfigRequest
	if err := pluginapi.DecodeStruct(in, &req); err != nil {
		return nil, err
	}
	if req.PluginID == "" {
		return nil, errors.New("plugin_id is required")
	}
	if _, err := s.manager.persistPluginConfig(ctx, req.PluginID, req.Config); err != nil {
		return nil, err
	}
	return pluginapi.EncodeStruct(coreapi.PersistPluginConfigResponse{OK: true})
}

func (m *Manager) Catalog() []models.CatalogPlugin {
	out := make([]models.CatalogPlugin, 0, len(m.catalog))
	for _, item := range m.catalog {
		out = append(out, item)
	}
	return out
}

func (m *Manager) Reconcile(ctx context.Context) error {
	records, err := m.store.ListPluginRecords(ctx)
	if err != nil {
		return err
	}
	for _, record := range records {
		if record.Status == models.PluginStatusEnabled {
			if err := m.Enable(ctx, record.PluginID); err != nil {
				m.setRuntimeError(record.PluginID, err)
			}
		}
	}
	return nil
}

func (m *Manager) Install(ctx context.Context, req InstallRequest) (models.PluginInstallRecord, error) {
	item, ok := m.catalog[req.PluginID]
	if !ok {
		return models.PluginInstallRecord{}, fmt.Errorf("unknown plugin %q", req.PluginID)
	}
	now := time.Now().UTC()
	record := models.PluginInstallRecord{
		PluginID:         item.ID,
		Version:          item.Manifest.Version,
		Status:           models.PluginStatusInstalled,
		BinaryPath:       chooseBinaryPath(req.BinaryPath, item.BinaryName),
		Config:           req.Config,
		InstalledAt:      now,
		UpdatedAt:        now,
		LastHealthStatus: models.HealthStateStopped,
		Metadata:         req.Metadata,
	}
	if existing, found, err := m.store.GetPluginRecord(ctx, req.PluginID); err != nil {
		return models.PluginInstallRecord{}, err
	} else if found {
		record.InstalledAt = existing.InstalledAt
		record.Status = existing.Status
		if req.BinaryPath == "" {
			record.BinaryPath = existing.BinaryPath
		}
	}
	if err := m.store.UpsertPluginRecord(ctx, record); err != nil {
		return models.PluginInstallRecord{}, err
	}
	return record, nil
}

func (m *Manager) UpdateConfig(ctx context.Context, pluginID string, config map[string]any) (models.PluginInstallRecord, error) {
	record, ok, err := m.store.GetPluginRecord(ctx, pluginID)
	if err != nil {
		return models.PluginInstallRecord{}, err
	}
	if !ok {
		return models.PluginInstallRecord{}, errors.New("plugin not installed")
	}

	m.mu.RLock()
	runtime := m.runtimes[pluginID]
	running := runtime != nil && runtime.running && runtime.client != nil
	m.mu.RUnlock()
	restartRequired := hikvisionRestartRequiredForConfig(pluginID, record.Config, config)

	if running {
		payload, err := pluginapi.EncodeStruct(config)
		if err != nil {
			return models.PluginInstallRecord{}, err
		}
		validation, err := runtime.client.ValidateConfig(ctx, payload)
		if err != nil {
			return models.PluginInstallRecord{}, err
		}
		validMap := validation.AsMap()
		if valid, ok := validMap["valid"].(bool); ok && !valid {
			return models.PluginInstallRecord{}, fmt.Errorf("plugin config invalid: %v", validMap["error"])
		}
		if !restartRequired {
			if _, err := runtime.client.Setup(ctx, payload); err != nil {
				runtime.lastError = err.Error()
				runtime.health.Status = models.HealthStateDegraded
				runtime.health.Message = err.Error()
				runtime.health.CheckedAt = time.Now().UTC()
				return models.PluginInstallRecord{}, err
			}
		}
	}

	record, err = m.persistPluginConfig(ctx, pluginID, config)
	if err != nil {
		return models.PluginInstallRecord{}, err
	}

	if running && restartRequired {
		m.stopProcess(pluginID)
		if err := m.Enable(ctx, pluginID); err != nil {
			runtime.lastError = err.Error()
			runtime.health.Status = models.HealthStateDegraded
			runtime.health.Message = err.Error()
			runtime.health.CheckedAt = time.Now().UTC()
			return models.PluginInstallRecord{}, err
		}
		updated, ok, err := m.store.GetPluginRecord(ctx, pluginID)
		if err != nil {
			return models.PluginInstallRecord{}, err
		}
		if ok {
			return updated, nil
		}
		return record, nil
	}

	if running {
		runtime.record.PluginID = record.PluginID
		runtime.record.Version = record.Version
		runtime.record.Status = record.Status
		runtime.record.BinaryPath = record.BinaryPath
		runtime.record.ConfigRef = record.ConfigRef
		runtime.record.InstalledAt = record.InstalledAt
		runtime.record.Metadata = record.Metadata
		runtime.record.LastHeartbeatAt = record.LastHeartbeatAt
		runtime.record.LastHealthStatus = record.LastHealthStatus
		if err := m.syncDevices(ctx, runtime); err != nil {
			runtime.lastError = err.Error()
			runtime.health.Status = models.HealthStateDegraded
			runtime.health.Message = err.Error()
			runtime.health.CheckedAt = time.Now().UTC()
			runtime.record.LastHealthStatus = models.HealthStateDegraded
		} else {
			runtime.lastError = ""
			runtime.health.Status = models.HealthStateHealthy
			runtime.health.Message = "plugin running"
			runtime.health.CheckedAt = time.Now().UTC()
			runtime.record.LastHealthStatus = models.HealthStateHealthy
		}
		runtime.record.UpdatedAt = time.Now().UTC()
		record = runtime.record
		if err := m.store.UpsertPluginRecord(ctx, record); err != nil {
			return models.PluginInstallRecord{}, err
		}
	}
	return record, nil
}

func (m *Manager) ListRuntimeViews(ctx context.Context) ([]models.PluginRuntimeView, error) {
	records, err := m.store.ListPluginRecords(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]models.PluginRuntimeView, 0, len(records))
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, record := range records {
		view := models.PluginRuntimeView{
			Record: record,
			Health: models.PluginHealth{
				PluginID:  record.PluginID,
				Status:    record.LastHealthStatus,
				CheckedAt: record.UpdatedAt,
			},
		}
		if runtime := m.runtimes[record.PluginID]; runtime != nil {
			view.Manifest = runtime.manifest
			view.Health = runtime.health
			view.Running = runtime.running
			view.LastError = runtime.lastError
			view.RecentLogs = runtime.logs.Snapshot()
			view.ProcessPID = runtime.pid
			view.ListenAddr = runtime.addr
		}
		views = append(views, view)
	}
	return views, nil
}

func (m *Manager) GetLogs(pluginID string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if runtime := m.runtimes[pluginID]; runtime != nil && runtime.logs != nil {
		return runtime.logs.Snapshot()
	}
	return nil
}
