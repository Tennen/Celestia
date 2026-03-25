package app

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/pluginruntime"
	"github.com/chentianyu/celestia/internal/pluginutil"
	"github.com/chentianyu/celestia/plugins/haier/internal/mock"
	"github.com/google/uuid"
)

type Config struct {
	Accounts []mock.Account `json:"accounts"`
}

type Plugin struct {
	mu      sync.RWMutex
	config  Config
	devices map[string]models.Device
	states  map[string]models.DeviceStateSnapshot
	events  chan models.Event
	cancel  context.CancelFunc
	started bool
}

func New() *Plugin {
	return &Plugin{
		devices: map[string]models.Device{},
		states:  map[string]models.DeviceStateSnapshot{},
		events:  make(chan models.Event, 128),
	}
}

func (p *Plugin) Manifest() models.PluginManifest {
	return models.PluginManifest{
		ID:           "haier",
		Name:         "Haier Washer Plugin",
		Version:      "0.1.0",
		Vendor:       "haier",
		Capabilities: []string{"discover", "state", "command", "events", "capability_matrix"},
		DeviceKinds:  []models.DeviceKind{models.DeviceKindWasher},
	}
}

func (p *Plugin) ValidateConfig(_ context.Context, _ map[string]any) error {
	return nil
}

func (p *Plugin) Setup(_ context.Context, cfg map[string]any) error {
	config := Config{}
	if rawAccounts, ok := cfg["accounts"].([]any); ok && len(rawAccounts) > 0 {
		config.Accounts = make([]mock.Account, 0, len(rawAccounts))
		for _, item := range rawAccounts {
			entry, _ := item.(map[string]any)
			config.Accounts = append(config.Accounts, mock.Account{Name: pluginutil.String(entry["name"], "hon-home")})
		}
	}
	if len(config.Accounts) == 0 {
		config.Accounts = mock.DefaultAccounts()
	}
	devices, states := mock.SeedDevices(config.Accounts)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config = config
	p.devices = map[string]models.Device{}
	p.states = map[string]models.DeviceStateSnapshot{}
	for _, device := range devices {
		p.devices[device.ID] = device
	}
	for id, state := range states {
		p.states[id] = state
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
	p.mu.Unlock()

	ticker := time.NewTicker(15 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				p.advance()
			}
		}
	}()
	return nil
}

func (p *Plugin) Stop(_ context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cancel != nil {
		p.cancel()
	}
	p.started = false
	return nil
}

func (p *Plugin) HealthCheck(_ context.Context) models.PluginHealth {
	p.mu.RLock()
	defer p.mu.RUnlock()
	status := models.HealthStateHealthy
	message := "washer sessions healthy"
	if !p.started {
		status = models.HealthStateStopped
		message = "plugin idle"
	}
	return pluginruntime.NewHealth("haier", "0.1.0", status, message)
}

func (p *Plugin) DiscoverDevices(_ context.Context) ([]models.Device, []models.DeviceStateSnapshot, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	devices := make([]models.Device, 0, len(p.devices))
	states := make([]models.DeviceStateSnapshot, 0, len(p.states))
	for _, device := range p.devices {
		devices = append(devices, device)
		states = append(states, p.states[device.ID])
	}
	return devices, states, nil
}

func (p *Plugin) ListDevices(ctx context.Context) ([]models.Device, error) {
	devices, _, err := p.DiscoverDevices(ctx)
	return devices, err
}

func (p *Plugin) GetDeviceState(_ context.Context, deviceID string) (models.DeviceStateSnapshot, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	state, ok := p.states[deviceID]
	if !ok {
		return models.DeviceStateSnapshot{}, errors.New("device not found")
	}
	return state, nil
}

func (p *Plugin) ExecuteCommand(_ context.Context, req models.CommandRequest) (models.CommandResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	device, ok := p.devices[req.DeviceID]
	if !ok {
		return models.CommandResponse{}, errors.New("device not found")
	}
	snapshot := p.states[req.DeviceID]
	if !supports(device, req.Action) && !isMappedAction(req.Action) {
		return models.CommandResponse{Accepted: false, Message: "action unsupported by model"}, nil
	}
	switch req.Action {
	case "start":
		snapshot.State["machine_status"] = "running"
		snapshot.State["phase"] = "wash"
		snapshot.State["remaining_minutes"] = 45
	case "stop":
		snapshot.State["machine_status"] = "idle"
		snapshot.State["phase"] = "ready"
		snapshot.State["remaining_minutes"] = 0
	case "pause":
		snapshot.State["machine_status"] = "paused"
	case "resume":
		snapshot.State["machine_status"] = "running"
	case "set_delay_time":
		if !supports(device, "delay_time") {
			return models.CommandResponse{Accepted: false, Message: "delay_time unsupported"}, nil
		}
		snapshot.State["delay_time"] = pluginutil.Int(req.Params["minutes"], 0)
	case "set_temp_level":
		if !supports(device, "temp_level") {
			return models.CommandResponse{Accepted: false, Message: "temp_level unsupported"}, nil
		}
		snapshot.State["temperature"] = pluginutil.Int(req.Params["value"], 40)
	case "set_spin_speed":
		if !supports(device, "spin_speed") {
			return models.CommandResponse{Accepted: false, Message: "spin_speed unsupported"}, nil
		}
		snapshot.State["spin_speed"] = pluginutil.Int(req.Params["value"], 1000)
	case "set_prewash":
		if !supports(device, "prewash") {
			return models.CommandResponse{Accepted: false, Message: "prewash unsupported"}, nil
		}
		snapshot.State["prewash"] = pluginutil.Bool(req.Params["enabled"], false)
	default:
		return models.CommandResponse{Accepted: false, Message: "action unsupported by model"}, nil
	}
	snapshot.TS = time.Now().UTC()
	p.states[req.DeviceID] = snapshot
	p.emitLocked(models.Event{
		ID:       uuid.NewString(),
		Type:     models.EventDeviceStateChanged,
		PluginID: "haier",
		DeviceID: req.DeviceID,
		TS:       snapshot.TS,
		Payload: map[string]any{
			"state": snapshot.State,
		},
	})
	return models.CommandResponse{Accepted: true, JobID: uuid.NewString(), Message: "command accepted"}, nil
}

func (p *Plugin) Events() <-chan models.Event {
	return p.events
}

func (p *Plugin) advance() {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now().UTC()
	for id, snapshot := range p.states {
		status := pluginutil.String(snapshot.State["machine_status"], "idle")
		if status == "running" {
			remaining := pluginutil.Max(0, pluginutil.Int(snapshot.State["remaining_minutes"], 0)-5)
			snapshot.State["remaining_minutes"] = remaining
			if remaining == 0 {
				snapshot.State["machine_status"] = "idle"
				snapshot.State["phase"] = "done"
				p.emitLocked(models.Event{
					ID:       uuid.NewString(),
					Type:     models.EventDeviceOccurred,
					PluginID: "haier",
					DeviceID: id,
					TS:       now,
					Payload: map[string]any{
						"event": "program_complete",
						"state": snapshot.State,
					},
				})
			} else if remaining < 20 {
				snapshot.State["phase"] = "rinse"
			}
		}
		snapshot.TS = now
		p.states[id] = snapshot
		p.emitLocked(models.Event{
			ID:       uuid.NewString(),
			Type:     models.EventDeviceStateChanged,
			PluginID: "haier",
			DeviceID: id,
			TS:       now,
			Payload: map[string]any{
				"state": snapshot.State,
			},
		})
	}
}

func (p *Plugin) emitLocked(event models.Event) {
	select {
	case p.events <- event:
	default:
	}
}

func supports(device models.Device, capability string) bool {
	return slices.Contains(device.Capabilities, capability)
}

func isMappedAction(action string) bool {
	return slices.Contains([]string{"start", "stop", "pause", "resume", "set_delay_time", "set_temp_level", "set_spin_speed", "set_prewash"}, action)
}
