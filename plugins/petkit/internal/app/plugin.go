package app

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/pluginruntime"
	"github.com/chentianyu/celestia/internal/pluginutil"
	"github.com/chentianyu/celestia/plugins/petkit/internal/mock"
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
		ID:           "petkit",
		Name:         "Petkit Plugin",
		Version:      "0.1.0",
		Vendor:       "petkit",
		Capabilities: []string{"discover", "state", "command", "events", "relay", "media_reserved"},
		DeviceKinds:  []models.DeviceKind{models.DeviceKindPetFeeder, models.DeviceKindPetLitterBox, models.DeviceKindPetFountain},
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
			config.Accounts = append(config.Accounts, mock.Account{Name: pluginutil.String(entry["name"], "pet-parent")})
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

	ticker := time.NewTicker(18 * time.Second)
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
	message := "relay and cloud transports active"
	if !p.started {
		status = models.HealthStateStopped
		message = "plugin idle"
	}
	return pluginruntime.NewHealth("petkit", "0.1.0", status, message)
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
	switch req.Action {
	case "feed_once":
		if device.Kind != models.DeviceKindPetFeeder {
			return models.CommandResponse{Accepted: false, Message: "feed_once only supported on feeder"}, nil
		}
		portions := pluginutil.Int(req.Params["portions"], 1)
		snapshot.State["food_level"] = pluginutil.Max(0, pluginutil.Int(snapshot.State["food_level"], 80)-portions)
		snapshot.State["last_feed_portions"] = portions
		p.emitLocked(models.Event{
			ID:       uuid.NewString(),
			Type:     models.EventDeviceOccurred,
			PluginID: "petkit",
			DeviceID: req.DeviceID,
			TS:       time.Now().UTC(),
			Payload: map[string]any{
				"event":    "feed_once",
				"portions": portions,
			},
		})
	case "clean_now":
		if device.Kind != models.DeviceKindPetLitterBox {
			return models.CommandResponse{Accepted: false, Message: "clean_now only supported on litter box"}, nil
		}
		snapshot.State["status"] = "cleaning"
	case "pause":
		if device.Kind != models.DeviceKindPetLitterBox {
			return models.CommandResponse{Accepted: false, Message: "pause only supported on litter box"}, nil
		}
		snapshot.State["status"] = "paused"
	case "resume":
		if device.Kind != models.DeviceKindPetLitterBox {
			return models.CommandResponse{Accepted: false, Message: "resume only supported on litter box"}, nil
		}
		snapshot.State["status"] = "idle"
	default:
		return models.CommandResponse{Accepted: false, Message: "capability not supported for this model"}, nil
	}
	snapshot.TS = time.Now().UTC()
	p.states[req.DeviceID] = snapshot
	p.emitLocked(models.Event{
		ID:       uuid.NewString(),
		Type:     models.EventDeviceStateChanged,
		PluginID: "petkit",
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
	for id, device := range p.devices {
		snapshot := p.states[id]
		switch device.Kind {
		case models.DeviceKindPetFeeder:
			snapshot.State["food_level"] = pluginutil.Max(0, pluginutil.Int(snapshot.State["food_level"], 80)-1)
			if pluginutil.Int(snapshot.State["food_level"], 80) < 20 {
				p.emitLocked(models.Event{
					ID:       uuid.NewString(),
					Type:     models.EventDeviceOccurred,
					PluginID: "petkit",
					DeviceID: id,
					TS:       now,
					Payload:  map[string]any{"event": "low_food", "state": snapshot.State},
				})
			}
		case models.DeviceKindPetLitterBox:
			if pluginutil.String(snapshot.State["status"], "idle") == "cleaning" {
				snapshot.State["status"] = "idle"
				snapshot.State["last_usage"] = now.Format(time.RFC3339)
			}
			snapshot.State["waste_level"] = pluginutil.Min(100, pluginutil.Int(snapshot.State["waste_level"], 20)+2)
		case models.DeviceKindPetFountain:
			snapshot.State["water_level"] = pluginutil.Max(0, pluginutil.Int(snapshot.State["water_level"], 60)-1)
			snapshot.State["filter_life"] = pluginutil.Max(0, pluginutil.Int(snapshot.State["filter_life"], 80)-1)
		}
		snapshot.TS = now
		p.states[id] = snapshot
		p.emitLocked(models.Event{
			ID:       uuid.NewString(),
			Type:     models.EventDeviceStateChanged,
			PluginID: "petkit",
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
