package pluginmgr

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/chentianyu/celestia/internal/coreapi"
	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/pluginapi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

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
	if err := m.ensureCoreAPI(); err != nil {
		return err
	}

	addr, port, err := allocateAddr()
	if err != nil {
		return err
	}
	m.mu.RLock()
	coreAddr := m.coreAddr
	m.mu.RUnlock()
	processCtx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(processCtx, record.BinaryPath)
	cmd.Env = append(os.Environ(),
		"CELESTIA_PLUGIN_PORT="+port,
		coreapi.EnvCoreAddr+"="+coreAddr,
	)
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
	m.stopProcess(pluginID)
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
			// Stop the process only — do NOT persist a disabled status so that
			// the next startup's Reconcile will re-enable the plugin correctly.
			m.stopProcess(record.PluginID)
		}
	}
	m.stopCoreAPI()
	return nil
}

// stopProcess tears down the in-memory runtime for a plugin (stream, gRPC
// connection, OS process) without touching the persisted status record.
func (m *Manager) stopProcess(pluginID string) {
	m.mu.RLock()
	runtime := m.runtimes[pluginID]
	m.mu.RUnlock()
	if runtime == nil {
		return
	}
	runtime.stoppedByManager = true
	if runtime.streamCancel != nil {
		runtime.streamCancel()
	}
	if runtime.client != nil {
		stopCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
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

func (m *Manager) IsRunning(pluginID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	runtime := m.runtimes[pluginID]
	return runtime != nil && runtime.running
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
