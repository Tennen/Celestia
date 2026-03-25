package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/pluginruntime"
	"github.com/chentianyu/celestia/plugins/xiaomi/internal/mock"
	"github.com/google/uuid"
)

type Config struct {
	Accounts            []mock.Account `json:"accounts"`
	PollIntervalSeconds int            `json:"poll_interval_seconds"`
}

type Plugin struct {
	mu      sync.RWMutex
	config  Config
	devices map[string]models.Device
	states  map[string]models.DeviceStateSnapshot
	events  chan models.Event
	cancel  context.CancelFunc
	started bool
	tick    int
}

func New() *Plugin {
	return &Plugin{
		events:  make(chan models.Event, 128),
		devices: map[string]models.Device{},
		states:  map[string]models.DeviceStateSnapshot{},
	}
}

func (p *Plugin) Manifest() models.PluginManifest {
	return models.PluginManifest{
		ID:           "xiaomi",
		Name:         "Xiaomi MIoT Plugin",
		Version:      "0.1.0",
		Vendor:       "xiaomi",
		Capabilities: []string{"discover", "state", "command", "events", "oauth", "multi_account", "multi_region", "aquarium_control", "speaker_voice_push"},
		ConfigSchema: map[string]any{
			"type": "object",
		},
		DeviceKinds: []models.DeviceKind{
			models.DeviceKindLight,
			models.DeviceKindSwitch,
			models.DeviceKindSensor,
			models.DeviceKindClimate,
			models.DeviceKindAquarium,
			models.DeviceKindSpeaker,
		},
	}
}

func (p *Plugin) ValidateConfig(_ context.Context, cfg map[string]any) error {
	if accounts, ok := cfg["accounts"].([]any); ok {
		for _, item := range accounts {
			entry, _ := item.(map[string]any)
			region := strings.ToLower(stringValue(entry["region"], "cn"))
			if !slices.Contains([]string{"cn", "eu", "in", "ru", "sg", "us"}, region) {
				return fmt.Errorf("unsupported Xiaomi region %q", region)
			}
		}
	}
	return nil
}

func (p *Plugin) Setup(_ context.Context, cfg map[string]any) error {
	config := Config{PollIntervalSeconds: 20}
	if value, ok := cfg["poll_interval_seconds"].(float64); ok && int(value) > 0 {
		config.PollIntervalSeconds = int(value)
	}
	if rawAccounts, ok := cfg["accounts"].([]any); ok && len(rawAccounts) > 0 {
		config.Accounts = make([]mock.Account, 0, len(rawAccounts))
		for _, item := range rawAccounts {
			entry, _ := item.(map[string]any)
			account := mock.Account{
				Name:   stringValue(entry["name"], "demo-cn"),
				Region: stringValue(entry["region"], "cn"),
			}
			config.Accounts = append(config.Accounts, account)
		}
	}
	if len(config.Accounts) == 0 {
		config.Accounts = mock.DefaultAccounts()
	}
	for _, account := range config.Accounts {
		if !slices.Contains([]string{"cn", "eu", "in", "ru", "sg", "us"}, strings.ToLower(account.Region)) {
			return fmt.Errorf("unsupported Xiaomi region %q", account.Region)
		}
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
	interval := time.Duration(max(p.config.PollIntervalSeconds, 8)) * time.Second
	p.started = true
	p.mu.Unlock()

	ticker := time.NewTicker(interval)
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
	message := "mqtt subscription active"
	if !p.started {
		status = models.HealthStateStopped
		message = "plugin idle"
	}
	return pluginruntime.NewHealth("xiaomi", "0.1.0", status, message)
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
		return models.DeviceStateSnapshot{}, errors.New("device state not found")
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
	case "turn_on", "power_on":
		snapshot.State["power"] = true
		if device.Kind == models.DeviceKindAquarium {
			snapshot.State["pump_power"] = true
			snapshot.State["light_power"] = true
		}
	case "turn_off", "power_off":
		snapshot.State["power"] = false
		if device.Kind == models.DeviceKindAquarium {
			snapshot.State["pump_power"] = false
			snapshot.State["light_power"] = false
		}
	case "set_power":
		snapshot.State["power"] = boolValue(req.Params["on"], true)
		if device.Kind == models.DeviceKindAquarium {
			snapshot.State["pump_power"] = boolValue(req.Params["on"], true)
			snapshot.State["light_power"] = boolValue(req.Params["on"], true)
		}
	case "set_brightness":
		if !supports(device, "brightness") {
			return models.CommandResponse{Accepted: false, Message: "brightness unsupported"}, nil
		}
		snapshot.State["brightness"] = intValue(req.Params["value"], 50)
	case "set_color_temp":
		if !supports(device, "color_temp") {
			return models.CommandResponse{Accepted: false, Message: "color_temp unsupported"}, nil
		}
		snapshot.State["color_temp"] = intValue(req.Params["value"], 4000)
	case "set_target_temperature":
		if !supports(device, "target_temperature") {
			return models.CommandResponse{Accepted: false, Message: "target_temperature unsupported"}, nil
		}
		snapshot.State["target_temperature"] = intValue(req.Params["value"], 25)
	case "set_mode":
		if !supports(device, "mode") {
			return models.CommandResponse{Accepted: false, Message: "mode unsupported"}, nil
		}
		snapshot.State["mode"] = stringValue(req.Params["value"], "cool")
	case "set_fan_speed":
		if !supports(device, "fan_speed") {
			return models.CommandResponse{Accepted: false, Message: "fan_speed unsupported"}, nil
		}
		snapshot.State["fan_speed"] = stringValue(req.Params["value"], "auto")
	case "set_pump_power":
		if !supports(device, "pump_power") {
			return models.CommandResponse{Accepted: false, Message: "pump_power unsupported"}, nil
		}
		snapshot.State["pump_power"] = boolValue(req.Params["on"], true)
	case "set_light_power":
		if !supports(device, "light_power") {
			return models.CommandResponse{Accepted: false, Message: "light_power unsupported"}, nil
		}
		snapshot.State["light_power"] = boolValue(req.Params["on"], true)
	case "set_light_brightness":
		if !supports(device, "light_brightness") {
			return models.CommandResponse{Accepted: false, Message: "light_brightness unsupported"}, nil
		}
		snapshot.State["light_brightness"] = intValue(req.Params["value"], 60)
	case "set_light_mode":
		if !supports(device, "light_mode") {
			return models.CommandResponse{Accepted: false, Message: "light_mode unsupported"}, nil
		}
		snapshot.State["light_mode"] = stringValue(req.Params["value"], "day")
	case "set_volume":
		if !supports(device, "volume") {
			return models.CommandResponse{Accepted: false, Message: "volume unsupported"}, nil
		}
		snapshot.State["volume"] = intValue(req.Params["value"], 45)
	case "set_mute":
		if !supports(device, "mute") {
			return models.CommandResponse{Accepted: false, Message: "mute unsupported"}, nil
		}
		snapshot.State["mute"] = boolValue(req.Params["on"], false)
	case "push_voice_message":
		if !supports(device, "voice_push") {
			return models.CommandResponse{Accepted: false, Message: "voice_push unsupported"}, nil
		}
		message := strings.TrimSpace(stringValue(req.Params["message"], ""))
		if message == "" {
			return models.CommandResponse{Accepted: false, Message: "message is required"}, nil
		}
		if volume, ok := req.Params["volume"]; ok {
			snapshot.State["volume"] = intValue(volume, intValue(snapshot.State["volume"], 45))
		}
		snapshot.State["last_message"] = message
		snapshot.State["last_message_at"] = time.Now().UTC().Format(time.RFC3339)
		snapshot.State["delivery_status"] = "sent"
		p.emitLocked(models.Event{
			ID:       uuid.NewString(),
			Type:     models.EventDeviceOccurred,
			PluginID: "xiaomi",
			DeviceID: req.DeviceID,
			TS:       time.Now().UTC(),
			Payload: map[string]any{
				"event":   "voice_message.sent",
				"message": message,
				"state":   snapshot.State,
			},
		})
	default:
		return models.CommandResponse{Accepted: false, Message: "action not supported"}, nil
	}
	snapshot.TS = time.Now().UTC()
	p.states[req.DeviceID] = snapshot
	p.emitLocked(models.Event{
		ID:       uuid.NewString(),
		Type:     models.EventDeviceCommandAccept,
		PluginID: "xiaomi",
		DeviceID: req.DeviceID,
		TS:       time.Now().UTC(),
		Payload: map[string]any{
			"action": req.Action,
			"state":  snapshot.State,
		},
	})
	p.emitLocked(models.Event{
		ID:       uuid.NewString(),
		Type:     models.EventDeviceStateChanged,
		PluginID: "xiaomi",
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
	p.tick++
	for id, device := range p.devices {
		snapshot := p.states[id]
		switch device.Kind {
		case models.DeviceKindSensor:
			snapshot.State["temperature"] = 23.5 + float64((p.tick%6))/10
			snapshot.State["humidity"] = 46 + (p.tick % 8)
		case models.DeviceKindClimate:
			if power, _ := snapshot.State["power"].(bool); power {
				snapshot.State["target_temperature"] = 24 + (p.tick % 2)
			}
		case models.DeviceKindAquarium:
			snapshot.State["water_temperature"] = 25.0 + float64((p.tick%5))/10
			if p.tick%12 == 0 {
				snapshot.State["filter_life"] = max(0, intValue(snapshot.State["filter_life"], 92)-1)
			}
		case models.DeviceKindSpeaker:
			if stringValue(snapshot.State["delivery_status"], "idle") == "sent" && p.tick%2 == 0 {
				snapshot.State["delivery_status"] = "idle"
			}
		}
		snapshot.TS = time.Now().UTC()
		p.states[id] = snapshot
		if device.Kind == models.DeviceKindSensor || device.Kind == models.DeviceKindClimate || device.Kind == models.DeviceKindAquarium || device.Kind == models.DeviceKindSpeaker {
			p.emitLocked(models.Event{
				ID:       uuid.NewString(),
				Type:     models.EventDeviceStateChanged,
				PluginID: "xiaomi",
				DeviceID: id,
				TS:       snapshot.TS,
				Payload: map[string]any{
					"source": "cloud_mqtt",
					"state":  snapshot.State,
				},
			})
		}
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

func boolValue(value any, fallback bool) bool {
	if raw, ok := value.(bool); ok {
		return raw
	}
	return fallback
}

func intValue(value any, fallback int) int {
	if raw, ok := value.(float64); ok {
		return int(raw)
	}
	if raw, ok := value.(int); ok {
		return raw
	}
	return fallback
}

func stringValue(value any, fallback string) string {
	if raw, ok := value.(string); ok && raw != "" {
		return raw
	}
	return fallback
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
