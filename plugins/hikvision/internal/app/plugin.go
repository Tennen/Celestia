package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/pluginruntime"
	lanclient "github.com/chentianyu/celestia/plugins/hikvision/internal/client"
	ezvizcloud "github.com/chentianyu/celestia/plugins/hikvision/internal/cloud"
	"github.com/google/uuid"
)

const (
	pluginID      = "hikvision"
	pluginVersion = "0.3.0"
)

func New() *Plugin {
	return &Plugin{
		entries:     map[string]*entryRuntime{},
		deviceIndex: map[string]string{},
		events:      make(chan models.Event, 256),
	}
}

func (p *Plugin) Manifest() models.PluginManifest {
	return models.PluginManifest{
		ID:      pluginID,
		Name:    "Hikvision EZVIZ Plugin",
		Version: pluginVersion,
		Vendor:  "hikvision",
		Capabilities: []string{
			"discover",
			"state",
			"command",
			"events",
			"real_lan_sdk",
			"real_cloud",
			"ptz",
			"playback",
			"recordings",
			"stream",
		},
		DeviceKinds: []models.DeviceKind{models.DeviceKindCameraLike},
	}
}

func (p *Plugin) ValidateConfig(_ context.Context, cfg map[string]any) error {
	_, err := parseConfig(cfg)
	return err
}

func (p *Plugin) Setup(_ context.Context, cfg map[string]any) error {
	parsed, err := parseConfig(cfg)
	if err != nil {
		return err
	}

	entries := make(map[string]*entryRuntime, len(parsed.Entries))
	deviceIndex := make(map[string]string, len(parsed.Entries))
	for _, item := range parsed.Entries {
		controlBlocked := p.controlBlockedReason(parsed, item, nil, "")
		device := buildDevice(parsed.Mode, item, nil, controlBlocked)
		state := initialState(parsed, item, controlBlocked)
		entry := &entryRuntime{
			Config:         item,
			Device:         device,
			LastState:      state,
			Connected:      false,
			LastError:      stringParam(state.State, "last_error"),
			ControlBlocked: controlBlocked,
		}
		if parsed.Mode == RuntimeModeLAN {
			entry.Client = lanclient.NewCameraClient()
		}
		entries[item.EntryID] = entry
		deviceIndex[item.DeviceID] = item.EntryID
	}

	var cloudSession *ezvizcloud.Session
	if parsed.Mode == RuntimeModeCloud && parsed.Cloud.HasAuth() {
		cloudSession = ezvizcloud.NewSession(parsed.Cloud, nil)
	}

	previous := p.entryRuntimes()
	p.mu.Lock()
	p.config = parsed
	p.entries = entries
	p.deviceIndex = deviceIndex
	p.cloud = cloudSession
	p.lastError = ""
	p.lastSyncAt = time.Time{}
	p.mu.Unlock()

	for _, runtime := range previous {
		if runtime.Client == nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = runtime.Client.Disconnect(ctx)
		cancel()
	}
	return nil
}

func (p *Plugin) Start(_ context.Context) error {
	p.mu.Lock()
	if p.started {
		p.mu.Unlock()
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.started = true
	interval := time.Duration(p.config.PollIntervalSeconds) * time.Second
	p.mu.Unlock()

	if err := p.refreshAll(ctx); err != nil {
		p.setLastError(err.Error())
	}

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := p.refreshAll(ctx); err != nil {
					p.setLastError(err.Error())
				} else {
					p.setLastError("")
				}
			}
		}
	}()
	return nil
}

func (p *Plugin) Stop(_ context.Context) error {
	p.mu.Lock()
	if p.cancel != nil {
		p.cancel()
	}
	p.cancel = nil
	p.started = false
	p.mu.Unlock()

	for _, runtime := range p.entryRuntimes() {
		if runtime.Client != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = runtime.Client.Disconnect(ctx)
			cancel()
		}
		state := initialState(p.config, runtime.Config, runtime.ControlBlocked)
		p.applyState(runtime.Config.EntryID, state, false)
	}
	p.setLastError("")
	return nil
}

func (p *Plugin) HealthCheck(_ context.Context) models.PluginHealth {
	p.mu.RLock()
	started := p.started
	lastError := p.lastError
	lastSync := p.lastSyncAt
	mode := p.config.Mode
	p.mu.RUnlock()

	status := models.HealthStateHealthy
	message := "hikvision sync active"
	if mode == RuntimeModeCloud {
		message = "ezviz cloud sync active"
	}
	if !started {
		status = models.HealthStateStopped
		message = "plugin idle"
	} else if strings.TrimSpace(lastError) != "" {
		status = models.HealthStateDegraded
		message = lastError
	} else if lastSync.IsZero() {
		status = models.HealthStateDegraded
		message = "waiting for first sync"
	}
	return pluginruntime.NewHealth(pluginID, pluginVersion, status, message)
}

func (p *Plugin) DiscoverDevices(ctx context.Context) ([]models.Device, []models.DeviceStateSnapshot, error) {
	if err := p.refreshAll(ctx); err != nil {
		p.setLastError(err.Error())
	} else {
		p.setLastError("")
	}
	devices, states := p.snapshot()
	if len(devices) == 0 {
		return nil, nil, errors.New("no configured cameras")
	}
	return devices, states, nil
}

func (p *Plugin) ListDevices(context.Context) ([]models.Device, error) {
	devices, _ := p.snapshot()
	return devices, nil
}

func (p *Plugin) GetDeviceState(ctx context.Context, deviceID string) (models.DeviceStateSnapshot, error) {
	entryID, ok := p.runtimeByDeviceID(deviceID)
	if !ok {
		return models.DeviceStateSnapshot{}, errors.New("device not found")
	}
	if p.config.Mode == RuntimeModeCloud {
		if err := p.refreshAll(ctx); err != nil {
			p.setLastError(err.Error())
		}
	} else if err := p.refreshLocalEntry(ctx, entryID); err != nil {
		p.setLastError(err.Error())
	}
	p.mu.RLock()
	runtime := p.entries[entryID]
	defer p.mu.RUnlock()
	if runtime == nil {
		return models.DeviceStateSnapshot{}, errors.New("device not found")
	}
	return cloneSnapshot(runtime.LastState), nil
}

func (p *Plugin) ExecuteCommand(ctx context.Context, req models.CommandRequest) (models.CommandResponse, error) {
	entryID, ok := p.runtimeByDeviceID(req.DeviceID)
	if !ok {
		return models.CommandResponse{}, errors.New("device not found")
	}
	p.mu.RLock()
	runtime := p.entries[entryID]
	p.mu.RUnlock()
	if runtime == nil {
		return models.CommandResponse{}, errors.New("device not found")
	}
	resultPayload, message, err := p.executeCommand(ctx, runtime, req)
	if err != nil {
		p.emitEvent(models.Event{
			ID:       uuid.NewString(),
			Type:     models.EventDeviceCommandFailed,
			PluginID: pluginID,
			DeviceID: req.DeviceID,
			TS:       time.Now().UTC(),
			Payload: map[string]any{
				"action": req.Action,
				"error":  err.Error(),
			},
		})
		return models.CommandResponse{}, err
	}
	if err := p.refreshAfterCommand(ctx, entryID); err != nil {
		p.setLastError(err.Error())
	}

	eventPayload := map[string]any{
		"action": req.Action,
		"params": cloneMap(req.Params),
	}
	if len(resultPayload) > 0 {
		eventPayload["result"] = cloneMap(resultPayload)
	}
	p.emitEvent(models.Event{
		ID:       uuid.NewString(),
		Type:     models.EventDeviceCommandAccept,
		PluginID: pluginID,
		DeviceID: req.DeviceID,
		TS:       time.Now().UTC(),
		Payload:  eventPayload,
	})

	resp := models.CommandResponse{
		Accepted: true,
		JobID:    uuid.NewString(),
		Message:  message,
	}
	if len(resultPayload) > 0 {
		resp.Payload = resultPayload
	}
	return resp, nil
}

func (p *Plugin) Events() <-chan models.Event {
	return p.events
}

func (p *Plugin) refreshAfterCommand(ctx context.Context, entryID string) error {
	if p.config.Mode == RuntimeModeCloud {
		return p.refreshAll(ctx)
	}
	return p.refreshLocalEntry(ctx, entryID)
}

func (p *Plugin) refreshAll(ctx context.Context) error {
	switch p.config.Mode {
	case RuntimeModeLAN:
		return p.refreshAllLocal(ctx)
	case RuntimeModeCloud:
		return p.refreshAllCloud(ctx)
	default:
		return fmt.Errorf("unsupported hikvision mode %q", p.config.Mode)
	}
}

func (p *Plugin) applyState(entryID string, snapshot models.DeviceStateSnapshot, hasError bool) {
	current := cloneSnapshot(snapshot)
	p.mu.Lock()
	runtime := p.entries[entryID]
	if runtime == nil {
		p.mu.Unlock()
		return
	}
	previousState := cloneMap(runtime.LastState.State)
	runtime.LastState = current
	runtime.Connected = boolParam(current.State, "connected")
	runtime.Device.Online = runtime.Connected
	if hasError {
		runtime.LastError = stringParam(current.State, "last_error")
	} else {
		runtime.LastError = ""
	}
	p.lastSyncAt = time.Now().UTC()
	changed := stateChanged(previousState, current.State)
	p.mu.Unlock()

	if changed {
		p.emitEvent(models.Event{
			ID:       uuid.NewString(),
			Type:     models.EventDeviceStateChanged,
			PluginID: pluginID,
			DeviceID: runtime.Device.ID,
			TS:       current.TS,
			Payload: map[string]any{
				"state": cloneMap(current.State),
			},
		})
	}
}

func (p *Plugin) updateRuntimeDevice(entryID string, device models.Device, cloudInfo *ezvizcloud.DeviceInfo, controlBlocked string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	runtime := p.entries[entryID]
	if runtime == nil {
		return
	}
	runtime.Device = device
	runtime.CloudDevice = cloudInfo
	runtime.ControlBlocked = controlBlocked
}

func (p *Plugin) emitEvent(event models.Event) {
	select {
	case p.events <- event:
	default:
	}
}

func (p *Plugin) entryRuntimes() []*entryRuntime {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]*entryRuntime, 0, len(p.entries))
	for _, runtime := range p.entries {
		out = append(out, runtime)
	}
	return out
}

func (p *Plugin) entryIDs() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	ids := make([]string, 0, len(p.entries))
	for entryID := range p.entries {
		ids = append(ids, entryID)
	}
	sort.Strings(ids)
	return ids
}

func (p *Plugin) runtimeByDeviceID(deviceID string) (string, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	entryID, ok := p.deviceIndex[deviceID]
	if !ok {
		return "", false
	}
	runtime := p.entries[entryID]
	if runtime == nil {
		return "", false
	}
	return entryID, true
}

func (p *Plugin) snapshot() ([]models.Device, []models.DeviceStateSnapshot) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	devices := make([]models.Device, 0, len(p.entries))
	states := make([]models.DeviceStateSnapshot, 0, len(p.entries))
	for _, runtime := range p.entries {
		devices = append(devices, runtime.Device)
		states = append(states, cloneSnapshot(runtime.LastState))
	}
	sort.SliceStable(devices, func(i, j int) bool {
		return devices[i].ID < devices[j].ID
	})
	sort.SliceStable(states, func(i, j int) bool {
		return states[i].DeviceID < states[j].DeviceID
	})
	return devices, states
}

func (p *Plugin) setLastError(value string) {
	p.mu.Lock()
	p.lastError = strings.TrimSpace(value)
	p.mu.Unlock()
}

func initialState(cfg Config, entry EntryConfig, controlBlocked string) models.DeviceStateSnapshot {
	switch cfg.Mode {
	case RuntimeModeLAN:
		return buildLocalState(entry, lanclient.CameraStatus{Connected: false}, "not connected")
	case RuntimeModeCloud:
		return buildCloudState(entry, nil, cfg.Cloud.HasAuth(), controlBlocked, "")
	default:
		return models.DeviceStateSnapshot{}
	}
}

func (p *Plugin) controlBlockedReason(cfg Config, entry EntryConfig, info *ezvizcloud.DeviceInfo, lastError string) string {
	if cfg.Mode != RuntimeModeCloud {
		return ""
	}
	if lastError != "" {
		return lastError
	}
	if entry.DeviceSerial == "" {
		return "configure device_serial to enable Ezviz PTZ control"
	}
	if !cfg.Cloud.HasAuth() {
		return "configure cloud.username and cloud.password to enable Ezviz PTZ control"
	}
	if info != nil && !info.PTZSupported() {
		return "PTZ is not exposed by the Ezviz cloud for this camera"
	}
	return ""
}
