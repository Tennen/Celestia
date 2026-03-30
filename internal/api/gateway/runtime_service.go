package gateway

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/core/control"
	"github.com/chentianyu/celestia/internal/core/pluginmgr"
	runtimepkg "github.com/chentianyu/celestia/internal/core/runtime"
	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/storage"
	"github.com/google/uuid"
)

type RuntimeService struct {
	runtime *runtimepkg.Runtime
}

func NewRuntimeService(runtime *runtimepkg.Runtime) *RuntimeService {
	return &RuntimeService{runtime: runtime}
}

func (s *RuntimeService) Health(_ context.Context) (HealthStatus, error) {
	return HealthStatus{
		Status: "ok",
		Time:   time.Now().UTC(),
	}, nil
}

func (s *RuntimeService) Dashboard(ctx context.Context) (models.DashboardSummary, error) {
	summary, err := s.runtime.Dashboard(ctx)
	if err != nil {
		return models.DashboardSummary{}, statusError(http.StatusInternalServerError, err)
	}
	return summary, nil
}

func (s *RuntimeService) ListCatalogPlugins(_ context.Context) ([]models.CatalogPlugin, error) {
	return s.runtime.PluginMgr.Catalog(), nil
}

func (s *RuntimeService) ListPlugins(ctx context.Context) ([]models.PluginRuntimeView, error) {
	views, err := s.runtime.PluginMgr.ListRuntimeViews(ctx)
	if err != nil {
		return nil, statusError(http.StatusInternalServerError, err)
	}
	return views, nil
}

func (s *RuntimeService) InstallPlugin(ctx context.Context, req InstallPluginRequest) (models.PluginInstallRecord, error) {
	record, err := s.runtime.PluginMgr.Install(ctx, req.toPluginInstallRequest())
	if err != nil {
		return models.PluginInstallRecord{}, statusError(http.StatusBadRequest, err)
	}
	return record, nil
}

func (s *RuntimeService) UpdatePluginConfig(ctx context.Context, req UpdatePluginConfigRequest) (models.PluginInstallRecord, error) {
	record, err := s.runtime.PluginMgr.UpdateConfig(ctx, req.PluginID, req.Config)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not installed") {
			status = http.StatusNotFound
		}
		return models.PluginInstallRecord{}, statusError(status, err)
	}
	return record, nil
}

func (s *RuntimeService) EnablePlugin(ctx context.Context, pluginID string) error {
	if err := s.runtime.PluginMgr.Enable(ctx, pluginID); err != nil {
		return statusError(http.StatusBadRequest, err)
	}
	return nil
}

func (s *RuntimeService) DisablePlugin(ctx context.Context, pluginID string) error {
	if err := s.runtime.PluginMgr.Disable(ctx, pluginID); err != nil {
		return statusError(http.StatusBadRequest, err)
	}
	return nil
}

func (s *RuntimeService) DiscoverPlugin(ctx context.Context, pluginID string) error {
	if err := s.runtime.PluginMgr.Discover(ctx, pluginID); err != nil {
		return statusError(http.StatusBadRequest, err)
	}
	return nil
}

func (s *RuntimeService) DeletePlugin(ctx context.Context, pluginID string) error {
	if err := s.runtime.PluginMgr.Uninstall(ctx, pluginID); err != nil {
		return statusError(http.StatusBadRequest, err)
	}
	return nil
}

func (s *RuntimeService) GetPluginLogs(_ context.Context, pluginID string) (PluginLogsView, error) {
	return PluginLogsView{
		PluginID: pluginID,
		Logs:     s.runtime.PluginMgr.GetLogs(pluginID),
	}, nil
}

func (s *RuntimeService) ListDevices(ctx context.Context, filter DeviceFilter) ([]models.DeviceView, error) {
	devices, err := s.runtime.Registry.List(ctx, storage.DeviceFilter{
		PluginID: filter.PluginID,
		Kind:     filter.Kind,
		Query:    filter.Query,
	})
	if err != nil {
		return nil, statusError(http.StatusInternalServerError, err)
	}
	states, err := s.runtime.State.List(ctx, storage.StateFilter{})
	if err != nil {
		return nil, statusError(http.StatusInternalServerError, err)
	}
	stateMap := map[string]models.DeviceStateSnapshot{}
	for _, item := range states {
		stateMap[item.DeviceID] = item
	}
	out := make([]models.DeviceView, 0, len(devices))
	for _, device := range devices {
		view, err := s.deviceView(ctx, device, stateMap[device.ID])
		if err != nil {
			return nil, statusError(http.StatusInternalServerError, err)
		}
		out = append(out, view)
	}
	return out, nil
}

func (s *RuntimeService) GetDevice(ctx context.Context, deviceID string) (models.DeviceView, error) {
	device, ok, err := s.runtime.Registry.Get(ctx, deviceID)
	if err != nil {
		return models.DeviceView{}, statusError(http.StatusInternalServerError, err)
	}
	if !ok {
		return models.DeviceView{}, statusError(http.StatusNotFound, errors.New("device not found"))
	}
	state, _, err := s.runtime.State.Get(ctx, device.ID)
	if err != nil {
		return models.DeviceView{}, statusError(http.StatusInternalServerError, err)
	}
	view, err := s.deviceView(ctx, device, state)
	if err != nil {
		return models.DeviceView{}, statusError(http.StatusInternalServerError, err)
	}
	return view, nil
}

func (s *RuntimeService) UpdateDevicePreference(ctx context.Context, req UpdateDevicePreferenceRequest) (models.DevicePreference, error) {
	device, ok, err := s.runtime.Registry.Get(ctx, req.DeviceID)
	if err != nil {
		return models.DevicePreference{}, statusError(http.StatusInternalServerError, err)
	}
	if !ok {
		return models.DevicePreference{}, statusError(http.StatusNotFound, errors.New("device not found"))
	}

	pref := models.DevicePreference{
		DeviceID:  device.ID,
		Alias:     strings.TrimSpace(req.Alias),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.runtime.Store.UpsertDevicePreference(ctx, pref); err != nil {
		return models.DevicePreference{}, statusError(http.StatusInternalServerError, err)
	}
	return pref, nil
}

func (s *RuntimeService) UpdateControlPreference(ctx context.Context, req UpdateControlPreferenceRequest) (models.DeviceControlPreference, error) {
	device, ok, err := s.runtime.Registry.Get(ctx, req.DeviceID)
	if err != nil {
		return models.DeviceControlPreference{}, statusError(http.StatusInternalServerError, err)
	}
	if !ok {
		return models.DeviceControlPreference{}, statusError(http.StatusNotFound, errors.New("device not found"))
	}
	state, _, err := s.runtime.State.Get(ctx, device.ID)
	if err != nil {
		return models.DeviceControlPreference{}, statusError(http.StatusInternalServerError, err)
	}
	view, err := s.deviceView(ctx, device, state)
	if err != nil {
		return models.DeviceControlPreference{}, statusError(http.StatusInternalServerError, err)
	}

	controlID := strings.TrimSpace(req.ControlID)
	if !hasControl(view.Controls, controlID) {
		return models.DeviceControlPreference{}, statusError(http.StatusNotFound, errors.New("control not found"))
	}

	visible := true
	if req.Visible != nil {
		visible = *req.Visible
	}
	pref := models.DeviceControlPreference{
		DeviceID:  device.ID,
		ControlID: controlID,
		Alias:     strings.TrimSpace(req.Alias),
		Visible:   visible,
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.runtime.Store.UpsertDeviceControlPreference(ctx, pref); err != nil {
		return models.DeviceControlPreference{}, statusError(http.StatusInternalServerError, err)
	}
	return pref, nil
}

func (s *RuntimeService) SendDeviceCommand(ctx context.Context, req DeviceCommandRequest) (CommandExecutionResult, error) {
	device, ok, err := s.runtime.Registry.Get(ctx, req.DeviceID)
	if err != nil {
		return CommandExecutionResult{}, statusError(http.StatusInternalServerError, err)
	}
	if !ok {
		return CommandExecutionResult{}, statusError(http.StatusNotFound, errors.New("device not found"))
	}
	return s.executeDeviceCommand(ctx, actorOrDefault(req.Actor), device, models.CommandRequest{
		DeviceID:  device.ID,
		Action:    req.Action,
		Params:    req.Params,
		RequestID: uuid.NewString(),
	})
}

func (s *RuntimeService) ToggleControl(ctx context.Context, req ToggleControlRequest) (CommandExecutionResult, error) {
	device, state, controlID, err := s.resolveControlTarget(ctx, req.CompoundControlID)
	if err != nil {
		return CommandExecutionResult{}, err
	}
	commandReq, err := s.runtime.Controls.ResolveToggle(device, state, controlID, req.On)
	if err != nil {
		return CommandExecutionResult{}, statusError(http.StatusBadRequest, err)
	}
	commandReq.RequestID = uuid.NewString()
	return s.executeDeviceCommand(ctx, actorOrDefault(req.Actor), device, commandReq)
}

func (s *RuntimeService) RunActionControl(ctx context.Context, req ActionControlRequest) (CommandExecutionResult, error) {
	device, state, controlID, err := s.resolveControlTarget(ctx, req.CompoundControlID)
	if err != nil {
		return CommandExecutionResult{}, err
	}
	commandReq, err := s.runtime.Controls.ResolveAction(device, state, controlID)
	if err != nil {
		return CommandExecutionResult{}, statusError(http.StatusBadRequest, err)
	}
	commandReq.RequestID = uuid.NewString()
	return s.executeDeviceCommand(ctx, actorOrDefault(req.Actor), device, commandReq)
}

func (s *RuntimeService) ListEvents(ctx context.Context, filter EventFilter) ([]models.Event, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	items, err := s.runtime.Store.ListEvents(ctx, storage.EventFilter{
		PluginID: filter.PluginID,
		DeviceID: filter.DeviceID,
		Limit:    limit,
	})
	if err != nil {
		return nil, statusError(http.StatusInternalServerError, err)
	}
	return items, nil
}

func (s *RuntimeService) ListAudits(ctx context.Context, filter AuditFilter) ([]models.AuditRecord, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	items, err := s.runtime.Audit.List(ctx, storage.AuditFilter{
		DeviceID: filter.DeviceID,
		Limit:    limit,
	})
	if err != nil {
		return nil, statusError(http.StatusInternalServerError, err)
	}
	return items, nil
}

func (s *RuntimeService) resolveControlTarget(ctx context.Context, compoundControlID string) (models.Device, models.DeviceStateSnapshot, string, error) {
	deviceID, controlID, err := control.ParseCompoundControlID(compoundControlID)
	if err != nil {
		return models.Device{}, models.DeviceStateSnapshot{}, "", statusError(http.StatusBadRequest, err)
	}
	device, ok, err := s.runtime.Registry.Get(ctx, deviceID)
	if err != nil {
		return models.Device{}, models.DeviceStateSnapshot{}, "", statusError(http.StatusInternalServerError, err)
	}
	if !ok {
		return models.Device{}, models.DeviceStateSnapshot{}, "", statusError(http.StatusNotFound, errors.New("device not found"))
	}
	state, _, err := s.runtime.State.Get(ctx, device.ID)
	if err != nil {
		return models.Device{}, models.DeviceStateSnapshot{}, "", statusError(http.StatusInternalServerError, err)
	}
	return device, state, controlID, nil
}

func (s *RuntimeService) executeDeviceCommand(ctx context.Context, actor string, device models.Device, req models.CommandRequest) (CommandExecutionResult, error) {
	decision := s.runtime.Policy.Evaluate(actor, req.Action)
	auditRecord := models.AuditRecord{
		ID:        uuid.NewString(),
		Actor:     actor,
		DeviceID:  device.ID,
		Action:    req.Action,
		Params:    req.Params,
		Allowed:   decision.Allowed,
		RiskLevel: decision.RiskLevel,
		CreatedAt: time.Now().UTC(),
	}
	if !decision.Allowed {
		auditRecord.Result = "denied"
		_ = s.runtime.Audit.Append(ctx, auditRecord)
		return CommandExecutionResult{}, &StatusError{StatusCode: http.StatusForbidden, Err: &PolicyDeniedError{Decision: decision}}
	}
	resp, err := s.runtime.PluginMgr.ExecuteCommand(ctx, device, req)
	if err != nil {
		auditRecord.Result = "failed"
		_ = s.runtime.Audit.Append(ctx, auditRecord)
		return CommandExecutionResult{}, statusError(http.StatusBadGateway, err)
	}
	auditRecord.Result = "accepted"
	if err := s.runtime.Audit.Append(ctx, auditRecord); err != nil {
		return CommandExecutionResult{}, statusError(http.StatusInternalServerError, err)
	}
	return CommandExecutionResult{
		Decision: decision,
		Result:   resp,
	}, nil
}

func (s *RuntimeService) deviceView(ctx context.Context, device models.Device, state models.DeviceStateSnapshot) (models.DeviceView, error) {
	view := s.runtime.Controls.BuildView(device, state)
	devicePref, _, err := s.runtime.Store.GetDevicePreference(ctx, device.ID)
	if err != nil {
		return models.DeviceView{}, err
	}
	view.Device = applyDevicePreference(view.Device, devicePref)
	prefs, err := s.runtime.Store.ListDeviceControlPreferences(ctx, device.ID)
	if err != nil {
		return models.DeviceView{}, err
	}
	return s.runtime.Controls.ApplyPreferences(view, prefs), nil
}

func applyDevicePreference(device models.Device, pref models.DevicePreference) models.Device {
	alias := strings.TrimSpace(pref.Alias)
	if alias == "" {
		return device
	}
	device.DefaultName = device.Name
	device.Alias = alias
	device.Name = alias
	return device
}

func hasControl(controls []models.DeviceControl, controlID string) bool {
	for _, control := range controls {
		if control.ID == controlID {
			return true
		}
	}
	return false
}

func actorOrDefault(actor string) string {
	if trimmed := strings.TrimSpace(actor); trimmed != "" {
		return trimmed
	}
	return "admin"
}

func (r InstallPluginRequest) toPluginInstallRequest() pluginmgr.InstallRequest {
	return pluginmgr.InstallRequest{
		PluginID:   r.PluginID,
		BinaryPath: r.BinaryPath,
		Config:     r.Config,
		Metadata:   r.Metadata,
	}
}
