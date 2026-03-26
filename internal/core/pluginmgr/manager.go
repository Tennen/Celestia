package pluginmgr

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/chentianyu/celestia/internal/core/eventbus"
	"github.com/chentianyu/celestia/internal/core/registry"
	"github.com/chentianyu/celestia/internal/core/state"
	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/pluginapi"
	"github.com/chentianyu/celestia/internal/storage"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Manager struct {
	store    storage.Store
	registry *registry.Service
	state    *state.Service
	bus      *eventbus.Bus

	mu       sync.RWMutex
	runtimes map[string]*managedPlugin
	catalog  map[string]models.CatalogPlugin
}

type managedPlugin struct {
	record           models.PluginInstallRecord
	manifest         *models.PluginManifest
	health           models.PluginHealth
	running          bool
	lastError        string
	addr             string
	pid              int
	logs             *logBuffer
	stoppedByManager bool

	cmd          *exec.Cmd
	conn         *grpc.ClientConn
	client       pluginapi.PluginClient
	cancel       context.CancelFunc
	streamCancel context.CancelFunc
}

type InstallRequest struct {
	PluginID   string         `json:"plugin_id"`
	BinaryPath string         `json:"binary_path,omitempty"`
	Config     map[string]any `json:"config,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

func New(store storage.Store, registry *registry.Service, state *state.Service, bus *eventbus.Bus) *Manager {
	catalog := map[string]models.CatalogPlugin{}
	for _, item := range BuiltinCatalog() {
		catalog[item.ID] = item
	}
	return &Manager{
		store:    store,
		registry: registry,
		state:    state,
		bus:      bus,
		runtimes: make(map[string]*managedPlugin),
		catalog:  catalog,
	}
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
	wasEnabled := record.Status == models.PluginStatusEnabled
	record.Config = config
	record.UpdatedAt = time.Now().UTC()
	if err := m.store.UpsertPluginRecord(ctx, record); err != nil {
		return models.PluginInstallRecord{}, err
	}
	if wasEnabled {
		m.mu.RLock()
		runtime := m.runtimes[pluginID]
		running := runtime != nil && runtime.running
		m.mu.RUnlock()
		if running {
			if err := m.Disable(ctx, pluginID); err != nil {
				return models.PluginInstallRecord{}, err
			}
		}
		if err := m.Enable(ctx, pluginID); err != nil {
			refreshed, ok, refreshErr := m.store.GetPluginRecord(ctx, pluginID)
			if refreshErr == nil && ok {
				return refreshed, err
			}
			return models.PluginInstallRecord{}, err
		}
		refreshed, ok, err := m.store.GetPluginRecord(ctx, pluginID)
		if err != nil {
			return models.PluginInstallRecord{}, err
		}
		if ok {
			return refreshed, nil
		}
	}
	return record, nil
}

func (m *Manager) Enable(ctx context.Context, pluginID string) error {
	record, ok, err := m.store.GetPluginRecord(ctx, pluginID)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("plugin not installed")
	}

	m.mu.RLock()
	existing := m.runtimes[pluginID]
	m.mu.RUnlock()
	if existing != nil && existing.running {
		return nil
	}

	addr, port, err := allocateAddr()
	if err != nil {
		return err
	}
	processCtx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(processCtx, record.BinaryPath)
	cmd.Env = append(os.Environ(), "CELESTIA_PLUGIN_PORT="+port)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return err
	}
	runtime := &managedPlugin{
		record:  record,
		health:  models.PluginHealth{PluginID: pluginID, Status: models.HealthStateUnknown, CheckedAt: time.Now().UTC()},
		addr:    addr,
		logs:    newLogBuffer(200),
		cmd:     cmd,
		cancel:  cancel,
		running: false,
	}
	m.mu.Lock()
	m.runtimes[pluginID] = runtime
	m.mu.Unlock()

	if err := cmd.Start(); err != nil {
		cancel()
		m.setRuntimeError(pluginID, err)
		return err
	}
	runtime.pid = cmd.Process.Pid
	go consumeLogs(runtime.logs, pluginID, "stdout", stdout)
	go consumeLogs(runtime.logs, pluginID, "stderr", stderr)
	go m.watchExit(pluginID, runtime)

	conn, client, manifest, err := m.connectAndSetup(ctx, runtime)
	if err != nil {
		runtime.lastError = err.Error()
		cancel()
		if runtime.cmd.Process != nil {
			_ = runtime.cmd.Process.Kill()
		}
		return err
	}
	runtime.conn = conn
	runtime.client = client
	runtime.manifest = &manifest
	runtime.running = true
	runtime.lastError = ""
	runtime.health = models.PluginHealth{
		PluginID:   pluginID,
		Status:     models.HealthStateHealthy,
		Message:    "plugin running",
		CheckedAt:  time.Now().UTC(),
		Manifest:   manifest.Version,
		ProcessPID: runtime.pid,
	}

	record.Status = models.PluginStatusEnabled
	record.Version = manifest.Version
	record.UpdatedAt = time.Now().UTC()
	record.LastHealthStatus = models.HealthStateHealthy
	now := time.Now().UTC()
	record.LastHeartbeatAt = &now
	runtime.record = record
	if err := m.store.UpsertPluginRecord(ctx, record); err != nil {
		return err
	}

	if err := m.syncDevices(ctx, runtime); err != nil {
		runtime.lastError = err.Error()
	}
	streamCtx, streamCancel := context.WithCancel(context.Background())
	runtime.streamCancel = streamCancel
	go m.consumeEvents(streamCtx, runtime)
	go m.healthLoop(runtime)
	m.publishLifecycle(runtime.record.PluginID, "enabled")
	return nil
}

func (m *Manager) Disable(ctx context.Context, pluginID string) error {
	record, ok, err := m.store.GetPluginRecord(ctx, pluginID)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("plugin not installed")
	}
	m.mu.RLock()
	runtime := m.runtimes[pluginID]
	m.mu.RUnlock()
	if runtime != nil {
		runtime.stoppedByManager = true
		if runtime.streamCancel != nil {
			runtime.streamCancel()
		}
		if runtime.client != nil {
			stopCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			_, _ = runtime.client.Stop(stopCtx, &emptypb.Empty{})
			cancel()
		}
		if runtime.cancel != nil {
			runtime.cancel()
		}
		if runtime.conn != nil {
			_ = runtime.conn.Close()
		}
		if runtime.cmd != nil && runtime.cmd.Process != nil {
			_ = runtime.cmd.Process.Signal(syscall.SIGTERM)
		}
		runtime.running = false
		runtime.health = models.PluginHealth{
			PluginID:   pluginID,
			Status:     models.HealthStateStopped,
			Message:    "plugin stopped",
			CheckedAt:  time.Now().UTC(),
			ProcessPID: runtime.pid,
		}
	}
	record.Status = models.PluginStatusDisabled
	record.UpdatedAt = time.Now().UTC()
	record.LastHealthStatus = models.HealthStateStopped
	if err := m.store.UpsertPluginRecord(ctx, record); err != nil {
		return err
	}
	m.publishLifecycle(pluginID, "disabled")
	return nil
}

func (m *Manager) Shutdown(ctx context.Context) error {
	records, err := m.store.ListPluginRecords(ctx)
	if err != nil {
		return err
	}
	for _, record := range records {
		if record.Status == models.PluginStatusEnabled {
			if err := m.Disable(ctx, record.PluginID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Manager) Uninstall(ctx context.Context, pluginID string) error {
	if err := m.Disable(ctx, pluginID); err != nil && !errors.Is(err, context.Canceled) {
		// Continue uninstall even if the plugin process is already gone.
	}
	if err := m.registry.DeleteByPlugin(ctx, pluginID); err != nil {
		return err
	}
	if err := m.store.DeletePluginRecord(ctx, pluginID); err != nil {
		return err
	}
	m.mu.Lock()
	delete(m.runtimes, pluginID)
	m.mu.Unlock()
	m.publishLifecycle(pluginID, "uninstalled")
	return nil
}

func (m *Manager) Discover(ctx context.Context, pluginID string) error {
	m.mu.RLock()
	runtime := m.runtimes[pluginID]
	m.mu.RUnlock()
	if runtime == nil || !runtime.running {
		return errors.New("plugin not running")
	}
	return m.syncDevices(ctx, runtime)
}

func (m *Manager) ExecuteCommand(ctx context.Context, device models.Device, req models.CommandRequest) (models.CommandResponse, error) {
	m.mu.RLock()
	runtime := m.runtimes[device.PluginID]
	m.mu.RUnlock()
	if runtime == nil || !runtime.running || runtime.client == nil {
		return models.CommandResponse{}, errors.New("plugin is not running")
	}
	payload, err := pluginapi.EncodeStruct(req)
	if err != nil {
		return models.CommandResponse{}, err
	}
	respStruct, err := runtime.client.ExecuteCommand(ctx, payload)
	if err != nil {
		return models.CommandResponse{}, err
	}
	var resp models.CommandResponse
	if err := pluginapi.DecodeStruct(respStruct, &resp); err != nil {
		return models.CommandResponse{}, err
	}
	return resp, nil
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

func (m *Manager) connectAndSetup(ctx context.Context, runtime *managedPlugin) (*grpc.ClientConn, pluginapi.PluginClient, models.PluginManifest, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var (
		conn   *grpc.ClientConn
		client pluginapi.PluginClient
		err    error
	)
	for {
		conn, err = grpc.DialContext(ctx, runtime.addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		)
		if err == nil {
			client = pluginapi.NewPluginClient(conn)
			break
		}
		if ctx.Err() != nil {
			return nil, nil, models.PluginManifest{}, err
		}
		time.Sleep(250 * time.Millisecond)
	}
	manifestStruct, err := client.GetManifest(ctx, &emptypb.Empty{})
	if err != nil {
		_ = conn.Close()
		return nil, nil, models.PluginManifest{}, err
	}
	var manifest models.PluginManifest
	if err := pluginapi.DecodeStruct(manifestStruct, &manifest); err != nil {
		_ = conn.Close()
		return nil, nil, models.PluginManifest{}, err
	}
	payload, err := pluginapi.EncodeStruct(runtime.record.Config)
	if err != nil {
		_ = conn.Close()
		return nil, nil, models.PluginManifest{}, err
	}
	validation, err := client.ValidateConfig(ctx, payload)
	if err != nil {
		_ = conn.Close()
		return nil, nil, models.PluginManifest{}, err
	}
	validMap := validation.AsMap()
	if valid, ok := validMap["valid"].(bool); ok && !valid {
		_ = conn.Close()
		return nil, nil, models.PluginManifest{}, fmt.Errorf("plugin config invalid: %v", validMap["error"])
	}
	if _, err := client.Setup(ctx, payload); err != nil {
		_ = conn.Close()
		return nil, nil, models.PluginManifest{}, err
	}
	if _, err := client.Start(ctx, &emptypb.Empty{}); err != nil {
		_ = conn.Close()
		return nil, nil, models.PluginManifest{}, err
	}
	return conn, client, manifest, nil
}

func (m *Manager) syncDevices(ctx context.Context, runtime *managedPlugin) error {
	list, err := runtime.client.DiscoverDevices(ctx, &emptypb.Empty{})
	if err != nil {
		return err
	}
	var devices []models.Device
	if err := pluginapi.DecodeList(list, &devices); err != nil {
		return err
	}
	if err := m.registry.Upsert(ctx, devices); err != nil {
		return err
	}
	for _, device := range devices {
		event := models.Event{
			ID:       uuid.NewString(),
			Type:     models.EventDeviceDiscovered,
			PluginID: device.PluginID,
			DeviceID: device.ID,
			TS:       time.Now().UTC(),
			Payload: map[string]any{
				"device": device,
			},
		}
		_ = m.store.AppendEvent(context.Background(), event)
		m.bus.Publish(event)
		statePayload, err := pluginapi.EncodeStruct(map[string]any{"device_id": device.ID})
		if err != nil {
			return err
		}
		stateStruct, err := runtime.client.GetDeviceState(ctx, statePayload)
		if err != nil {
			return err
		}
		var snapshot models.DeviceStateSnapshot
		if err := pluginapi.DecodeStruct(stateStruct, &snapshot); err != nil {
			return err
		}
		if snapshot.TS.IsZero() {
			snapshot.TS = time.Now().UTC()
		}
		if snapshot.PluginID == "" {
			snapshot.PluginID = device.PluginID
		}
		if err := m.state.Upsert(ctx, []models.DeviceStateSnapshot{snapshot}); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) consumeEvents(ctx context.Context, runtime *managedPlugin) {
	stream, err := runtime.client.StreamEvents(ctx, &emptypb.Empty{})
	if err != nil {
		runtime.lastError = err.Error()
		return
	}
	for {
		msg, err := stream.Recv()
		if err != nil {
			if ctx.Err() == nil {
				runtime.lastError = err.Error()
			}
			return
		}
		var event models.Event
		if err := pluginapi.DecodeStruct(msg, &event); err != nil {
			runtime.logs.Append("stream decode error: " + err.Error())
			continue
		}
		if event.ID == "" {
			event.ID = uuid.NewString()
		}
		if event.TS.IsZero() {
			event.TS = time.Now().UTC()
		}
		if event.PluginID == "" {
			event.PluginID = runtime.record.PluginID
		}
		if err := m.store.AppendEvent(context.Background(), event); err != nil {
			runtime.logs.Append("persist event error: " + err.Error())
		}
		if statePayload, ok := event.Payload["state"].(map[string]any); ok && event.DeviceID != "" {
			_ = m.state.Upsert(context.Background(), []models.DeviceStateSnapshot{{
				DeviceID: event.DeviceID,
				PluginID: event.PluginID,
				TS:       event.TS,
				State:    statePayload,
			}})
		}
		if healthValue, ok := event.Payload["health_status"].(string); ok {
			runtime.health.Status = models.HealthState(healthValue)
			runtime.health.CheckedAt = time.Now().UTC()
			runtime.health.Message, _ = event.Payload["message"].(string)
		}
		m.bus.Publish(event)
	}
}

func (m *Manager) healthLoop(runtime *managedPlugin) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if !runtime.running || runtime.client == nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		resp, err := runtime.client.HealthCheck(ctx, &emptypb.Empty{})
		cancel()
		if err != nil {
			runtime.lastError = err.Error()
			runtime.health.Status = models.HealthStateUnhealthy
			runtime.health.Message = err.Error()
			continue
		}
		var health models.PluginHealth
		if err := pluginapi.DecodeStruct(resp, &health); err != nil {
			runtime.lastError = err.Error()
			continue
		}
		health.ProcessPID = runtime.pid
		runtime.health = health
		now := time.Now().UTC()
		runtime.record.LastHealthStatus = health.Status
		runtime.record.LastHeartbeatAt = &now
		runtime.record.UpdatedAt = now
		_ = m.store.UpsertPluginRecord(context.Background(), runtime.record)
	}
}

func (m *Manager) watchExit(pluginID string, runtime *managedPlugin) {
	err := runtime.cmd.Wait()
	if runtime.conn != nil {
		_ = runtime.conn.Close()
	}
	runtime.running = false
	if err != nil && !errors.Is(err, context.Canceled) {
		runtime.lastError = err.Error()
		runtime.health = models.PluginHealth{
			PluginID:   pluginID,
			Status:     models.HealthStateUnhealthy,
			Message:    err.Error(),
			CheckedAt:  time.Now().UTC(),
			ProcessPID: runtime.pid,
		}
		now := time.Now().UTC()
		runtime.record.LastHealthStatus = models.HealthStateUnhealthy
		runtime.record.UpdatedAt = now
		runtime.record.LastHeartbeatAt = &now
		_ = m.store.UpsertPluginRecord(context.Background(), runtime.record)
		m.publishLifecycle(pluginID, "crashed")
		if !runtime.stoppedByManager && runtime.record.Status == models.PluginStatusEnabled {
			go func() {
				time.Sleep(3 * time.Second)
				if restartErr := m.Enable(context.Background(), pluginID); restartErr != nil {
					runtime.logs.Append("restart failed: " + restartErr.Error())
				}
			}()
		}
	}
}

func (m *Manager) setRuntimeError(pluginID string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	runtime := m.runtimes[pluginID]
	if runtime == nil {
		runtime = &managedPlugin{logs: newLogBuffer(200)}
		m.runtimes[pluginID] = runtime
	}
	runtime.lastError = err.Error()
}

func (m *Manager) publishLifecycle(pluginID, state string) {
	event := models.Event{
		ID:       uuid.NewString(),
		Type:     models.EventPluginLifecycleState,
		PluginID: pluginID,
		TS:       time.Now().UTC(),
		Payload: map[string]any{
			"state": state,
		},
	}
	_ = m.store.AppendEvent(context.Background(), event)
	m.bus.Publish(event)
}

func chooseBinaryPath(path, binaryName string) string {
	if path != "" {
		return path
	}
	dir := os.Getenv("CELESTIA_PLUGIN_BIN_DIR")
	if dir == "" {
		if exe, err := os.Executable(); err == nil {
			dir = filepath.Dir(exe)
		} else {
			dir = "./bin"
		}
	}
	return filepath.Join(dir, binaryName)
}

func allocateAddr() (string, string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", "", err
	}
	defer listener.Close()
	addr := listener.Addr().String()
	_, port, err := net.SplitHostPort(addr)
	return addr, port, err
}

func consumeLogs(buffer *logBuffer, pluginID, stream string, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		buffer.Append(fmt.Sprintf("[%s][%s] %s", pluginID, stream, scanner.Text()))
	}
}
