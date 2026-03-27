package app

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/pluginruntime"
	"github.com/chentianyu/celestia/plugins/xiaomi/internal/cloud"
	"github.com/chentianyu/celestia/plugins/xiaomi/internal/mapper"
	"github.com/chentianyu/celestia/plugins/xiaomi/internal/spec"
	"github.com/google/uuid"
)

type AccountConfig struct {
	Name         string   `json:"name,omitempty"`
	Region       string   `json:"region"`
	Username     string   `json:"username,omitempty"`
	Password     string   `json:"password,omitempty"`
	VerifyURL    string   `json:"verify_url,omitempty"`
	VerifyTicket string   `json:"verify_ticket,omitempty"`
	ClientID     string   `json:"client_id,omitempty"`
	RedirectURL  string   `json:"redirect_url,omitempty"`
	AccessToken  string   `json:"access_token,omitempty"`
	RefreshToken string   `json:"refresh_token,omitempty"`
	AuthCode     string   `json:"auth_code,omitempty"`
	DeviceID     string   `json:"device_id,omitempty"`
	ServiceToken string   `json:"service_token,omitempty"`
	SSecurity    string   `json:"ssecurity,omitempty"`
	UserID       string   `json:"user_id,omitempty"`
	CUserID      string   `json:"cuser_id,omitempty"`
	Locale       string   `json:"locale,omitempty"`
	Timezone     string   `json:"timezone,omitempty"`
	ExpiresAt    string   `json:"expires_at,omitempty"`
	HomeIDs      []string `json:"home_ids,omitempty"`
}

type Config struct {
	Accounts            []AccountConfig `json:"accounts"`
	PollIntervalSeconds int             `json:"poll_interval_seconds"`
}

type accountRuntime struct {
	cfg      cloud.AccountConfig
	client   *cloud.Client
	specs    map[string]spec.Instance
	lastErr  error
	lastSync time.Time
}

type deviceRuntime struct {
	accountName string
	account     *accountRuntime
	raw         cloud.DeviceRecord
	device      models.Device
	mapping     *mapper.DeviceMapping
}

type Plugin struct {
	mu         sync.RWMutex
	config     Config
	accounts   map[string]*accountRuntime
	devices    map[string]models.Device
	states     map[string]models.DeviceStateSnapshot
	runtimes   map[string]*deviceRuntime
	events     chan models.Event
	cancel     context.CancelFunc
	started    bool
	lastError  string
	lastSyncAt time.Time
}

func New() *Plugin {
	return &Plugin{
		accounts: map[string]*accountRuntime{},
		devices:  map[string]models.Device{},
		states:   map[string]models.DeviceStateSnapshot{},
		runtimes: map[string]*deviceRuntime{},
		events:   make(chan models.Event, 128),
	}
}

func (p *Plugin) Manifest() models.PluginManifest {
	return models.PluginManifest{
		ID:      "xiaomi",
		Name:    "Xiaomi MIoT Plugin",
		Version: "1.0.0",
		Vendor:  "xiaomi",
		Capabilities: []string{
			"discover",
			"state",
			"command",
			"events",
			"oauth",
			"account_password_login",
			"real_cloud",
			"multi_account",
			"multi_region",
			"service_token_session",
			"aquarium_control",
			"speaker_voice_push",
		},
		ConfigSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"poll_interval_seconds": map[string]any{
					"type":    "number",
					"default": 30,
				},
				"accounts": map[string]any{
					"type":        "array",
					"description": "Real Xiaomi cloud accounts. Prefer username/password or service_token/ssecurity/user_id. OAuth auth_code/refresh_token flows remain optional.",
				},
			},
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
	_, _, err := parseConfig(cfg, nil)
	return err
}

func (p *Plugin) Setup(_ context.Context, cfg map[string]any) error {
	p.mu.RLock()
	existing := p.accounts
	p.mu.RUnlock()
	config, runtimes, err := parseConfig(cfg, existing)
	if err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config = config
	p.accounts = runtimes
	p.devices = map[string]models.Device{}
	p.states = map[string]models.DeviceStateSnapshot{}
	p.runtimes = map[string]*deviceRuntime{}
	p.lastError = ""
	p.lastSyncAt = time.Time{}
	return nil
}

func (p *Plugin) Start(_ context.Context) error {
	p.mu.Lock()
	if p.started {
		p.mu.Unlock()
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	interval := time.Duration(max(p.config.PollIntervalSeconds, 15)) * time.Second
	p.cancel = cancel
	p.started = true
	p.mu.Unlock()

	if err := p.refreshAll(ctx, false); err != nil {
		p.mu.Lock()
		p.lastError = err.Error()
		p.mu.Unlock()
	}

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := p.refreshAll(ctx, true); err != nil {
					p.mu.Lock()
					p.lastError = err.Error()
					p.mu.Unlock()
				}
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
	message := "xiaomi cloud sync active"
	if !p.started {
		status = models.HealthStateStopped
		message = "plugin idle"
	} else if p.lastError != "" {
		status = models.HealthStateDegraded
		message = p.lastError
	}
	return pluginruntime.NewHealth("xiaomi", "1.0.0", status, message)
}

func (p *Plugin) DiscoverDevices(ctx context.Context) ([]models.Device, []models.DeviceStateSnapshot, error) {
	if err := p.refreshAll(ctx, false); err != nil {
		return nil, nil, err
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return cloneViews(p.devices, p.states), cloneStates(p.states), nil
}

func (p *Plugin) ListDevices(ctx context.Context) ([]models.Device, error) {
	devices, _, err := p.DiscoverDevices(ctx)
	return devices, err
}

func (p *Plugin) GetDeviceState(ctx context.Context, deviceID string) (models.DeviceStateSnapshot, error) {
	if err := p.refreshSingle(ctx, deviceID, false); err != nil {
		p.mu.RLock()
		state, ok := p.states[deviceID]
		p.mu.RUnlock()
		if ok {
			return state, nil
		}
		return models.DeviceStateSnapshot{}, err
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	state, ok := p.states[deviceID]
	if !ok {
		return models.DeviceStateSnapshot{}, errors.New("device not found")
	}
	return state, nil
}

func (p *Plugin) ExecuteCommand(ctx context.Context, req models.CommandRequest) (models.CommandResponse, error) {
	runtime, ok := p.runtime(req.DeviceID)
	if !ok {
		return models.CommandResponse{}, errors.New("device not found")
	}

	switch req.Action {
	case "turn_on", "power_on":
		if err := p.setPower(ctx, runtime, true); err != nil {
			return models.CommandResponse{}, err
		}
	case "turn_off", "power_off":
		if err := p.setPower(ctx, runtime, false); err != nil {
			return models.CommandResponse{}, err
		}
	case "set_power":
		if err := p.setPower(ctx, runtime, boolParam(req.Params, "on", true)); err != nil {
			return models.CommandResponse{}, err
		}
	case "set_toggle":
		if err := p.setToggle(ctx, runtime, stringParam(req.Params["toggle_id"]), boolParam(req.Params, "on", true)); err != nil {
			return models.CommandResponse{}, err
		}
	case "set_brightness":
		if err := p.setPropertyCommand(ctx, runtime, runtime.mapping.Brightness, req.Params["value"]); err != nil {
			return models.CommandResponse{}, err
		}
	case "set_color_temp":
		if err := p.setPropertyCommand(ctx, runtime, runtime.mapping.ColorTemp, req.Params["value"]); err != nil {
			return models.CommandResponse{}, err
		}
	case "set_target_temperature":
		if err := p.setPropertyCommand(ctx, runtime, runtime.mapping.TargetTemperature, req.Params["value"]); err != nil {
			return models.CommandResponse{}, err
		}
	case "set_mode":
		if err := p.setPropertyCommand(ctx, runtime, runtime.mapping.Mode, req.Params["value"]); err != nil {
			return models.CommandResponse{}, err
		}
	case "set_fan_speed":
		if err := p.setPropertyCommand(ctx, runtime, runtime.mapping.FanSpeed, req.Params["value"]); err != nil {
			return models.CommandResponse{}, err
		}
	case "set_pump_power":
		if err := p.setPropertyCommand(ctx, runtime, runtime.mapping.PumpPower, req.Params["on"]); err != nil {
			return models.CommandResponse{}, err
		}
	case "set_pump_level":
		if err := p.setPropertyCommand(ctx, runtime, runtime.mapping.PumpLevel, req.Params["value"]); err != nil {
			return models.CommandResponse{}, err
		}
	case "set_light_power":
		if err := p.setPropertyCommand(ctx, runtime, runtime.mapping.LightPower, req.Params["on"]); err != nil {
			return models.CommandResponse{}, err
		}
	case "set_light_brightness":
		if err := p.setPropertyCommand(ctx, runtime, runtime.mapping.LightBrightness, req.Params["value"]); err != nil {
			return models.CommandResponse{}, err
		}
	case "set_light_mode":
		if err := p.setPropertyCommand(ctx, runtime, runtime.mapping.LightMode, req.Params["value"]); err != nil {
			return models.CommandResponse{}, err
		}
	case "set_volume":
		if err := p.setPropertyCommand(ctx, runtime, runtime.mapping.Volume, req.Params["value"]); err != nil {
			return models.CommandResponse{}, err
		}
	case "set_mute":
		if err := p.setPropertyCommand(ctx, runtime, runtime.mapping.Mute, req.Params["on"]); err != nil {
			return models.CommandResponse{}, err
		}
	case "push_voice_message":
		if err := p.pushVoiceMessage(ctx, runtime, req.Params); err != nil {
			return models.CommandResponse{}, err
		}
	default:
		return models.CommandResponse{Accepted: false, Message: "action not supported"}, nil
	}

	if err := p.refreshSingle(ctx, req.DeviceID, true); err != nil {
		p.mu.Lock()
		p.lastError = err.Error()
		p.mu.Unlock()
	}

	return models.CommandResponse{
		Accepted: true,
		JobID:    uuid.NewString(),
		Message:  "command accepted",
	}, nil
}

func (p *Plugin) Events() <-chan models.Event {
	return p.events
}
