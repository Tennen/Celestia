package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/pluginruntime"
	"github.com/chentianyu/celestia/internal/xiaomi/oauth"
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

func (p *Plugin) setPower(ctx context.Context, runtime *deviceRuntime, on bool) error {
	if runtime.mapping.Power != nil {
		return p.setPropertyCommand(ctx, runtime, runtime.mapping.Power, on)
	}
	if runtime.mapping.Kind == models.DeviceKindAquarium {
		var errs []string
		for _, ref := range []*mapper.PropertyRef{runtime.mapping.PumpPower, runtime.mapping.LightPower} {
			if ref == nil {
				continue
			}
			if err := p.setPropertyCommand(ctx, runtime, ref, on); err != nil {
				errs = append(errs, err.Error())
			}
		}
		if len(errs) > 0 {
			return errors.New(strings.Join(errs, "; "))
		}
		return nil
	}
	return errors.New("power unsupported")
}

func (p *Plugin) setToggle(ctx context.Context, runtime *deviceRuntime, toggleID string, on bool) error {
	for _, item := range runtime.mapping.ToggleChannels {
		if item.ID == toggleID && item.Ref != nil {
			return p.setPropertyCommand(ctx, runtime, item.Ref, on)
		}
	}
	if toggleID == "power" || toggleID == "" {
		return p.setPower(ctx, runtime, on)
	}
	return fmt.Errorf("toggle %q unsupported", toggleID)
}

func (p *Plugin) setPropertyCommand(ctx context.Context, runtime *deviceRuntime, ref *mapper.PropertyRef, raw any) error {
	if ref == nil {
		return errors.New("capability unsupported")
	}
	value, err := encodePropertyValue(ref.Property, raw)
	if err != nil {
		return err
	}
	results, err := runtime.account.client.SetProps(ctx, []map[string]any{{
		"did":   runtime.raw.DID,
		"siid":  ref.ServiceIID,
		"piid":  ref.Property.IID,
		"value": value,
	}})
	if err != nil {
		return err
	}
	for _, result := range results {
		if code := intParam(result["code"]); code != 0 {
			return fmt.Errorf("xiaomi command rejected: code=%d", code)
		}
	}
	return nil
}

func (p *Plugin) pushVoiceMessage(ctx context.Context, runtime *deviceRuntime, params map[string]any) error {
	message := strings.TrimSpace(stringParam(params["message"]))
	if message == "" {
		return errors.New("message is required")
	}
	if volume, ok := params["volume"]; ok && runtime.mapping.Volume != nil {
		if err := p.setPropertyCommand(ctx, runtime, runtime.mapping.Volume, volume); err != nil {
			return err
		}
	}
	switch {
	case runtime.mapping.NotifyAction != nil:
		inputs := buildActionInputs(runtime.mapping.NotifyAction, message, params)
		result, err := runtime.account.client.Action(ctx, runtime.raw.DID, runtime.mapping.NotifyAction.ServiceIID, runtime.mapping.NotifyAction.Action.IID, inputs)
		if err != nil {
			return err
		}
		if code := intParam(result["code"]); code != 0 {
			return fmt.Errorf("xiaomi notify action rejected: code=%d", code)
		}
	case runtime.mapping.Text != nil:
		if err := p.setPropertyCommand(ctx, runtime, runtime.mapping.Text, message); err != nil {
			return err
		}
	default:
		return errors.New("voice_push unsupported")
	}
	p.emit(models.Event{
		ID:       uuid.NewString(),
		Type:     models.EventDeviceOccurred,
		PluginID: "xiaomi",
		DeviceID: runtime.device.ID,
		TS:       time.Now().UTC(),
		Payload: map[string]any{
			"event":   "speaker.text_sent",
			"message": message,
		},
	})
	return nil
}

func buildActionInputs(action *mapper.ActionRef, message string, params map[string]any) []any {
	inputs := make([]any, 0, len(action.Inputs))
	usedMessage := false
	for _, input := range action.Inputs {
		switch input.Format {
		case "string":
			if !usedMessage {
				inputs = append(inputs, message)
				usedMessage = true
			} else {
				inputs = append(inputs, message)
			}
		case "bool":
			inputs = append(inputs, boolParam(params, "on", false))
		default:
			if _, ok := params["volume"]; ok {
				inputs = append(inputs, intParam(params["volume"]))
			} else {
				inputs = append(inputs, 0)
			}
		}
	}
	return inputs
}

func (p *Plugin) refreshAll(ctx context.Context, emitEvents bool) error {
	accounts := p.accountList()
	nextDevices := map[string]models.Device{}
	nextStates := map[string]models.DeviceStateSnapshot{}
	nextRuntimes := map[string]*deviceRuntime{}
	var errs []string

	for _, account := range accounts {
		devices, err := p.refreshAccount(ctx, account)
		account.lastSync = time.Now().UTC()
		account.lastErr = err
		p.syncAccountSessionConfig(account.cfg.Name, account.client)
		if err != nil {
			errs = append(errs, err.Error())
			if len(devices) == 0 {
				continue
			}
		}
		for _, runtime := range devices {
			nextDevices[runtime.device.ID] = runtime.device
			nextRuntimes[runtime.device.ID] = runtime
			state, err := p.readState(ctx, runtime)
			if err != nil {
				errs = append(errs, err.Error())
				continue
			}
			nextStates[state.DeviceID] = state
		}
	}

	if len(nextDevices) == 0 && len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}

	var previous map[string]models.DeviceStateSnapshot
	p.mu.Lock()
	previous = cloneStateMap(p.states)
	p.devices = nextDevices
	p.states = nextStates
	p.runtimes = nextRuntimes
	p.lastSyncAt = time.Now().UTC()
	if len(errs) > 0 {
		p.lastError = strings.Join(errs, "; ")
	} else {
		p.lastError = ""
	}
	p.mu.Unlock()

	if emitEvents {
		for deviceID, state := range nextStates {
			prev, ok := previous[deviceID]
			if ok && reflect.DeepEqual(prev.State, state.State) {
				continue
			}
			p.emit(models.Event{
				ID:       uuid.NewString(),
				Type:     models.EventDeviceStateChanged,
				PluginID: "xiaomi",
				DeviceID: deviceID,
				TS:       state.TS,
				Payload: map[string]any{
					"source": "cloud_http",
					"state":  state.State,
				},
			})
		}
	}
	return nil
}

func (p *Plugin) refreshSingle(ctx context.Context, deviceID string, emitEvent bool) error {
	runtime, ok := p.runtime(deviceID)
	if !ok {
		return errors.New("device not found")
	}
	state, err := p.readState(ctx, runtime)
	if err != nil {
		return err
	}
	p.syncAccountSessionConfig(runtime.accountName, runtime.account.client)
	var previous models.DeviceStateSnapshot
	var hadPrev bool
	p.mu.Lock()
	previous, hadPrev = p.states[deviceID]
	p.states[deviceID] = state
	p.lastSyncAt = time.Now().UTC()
	p.mu.Unlock()
	if emitEvent && (!hadPrev || !reflect.DeepEqual(previous.State, state.State)) {
		p.emit(models.Event{
			ID:       uuid.NewString(),
			Type:     models.EventDeviceStateChanged,
			PluginID: "xiaomi",
			DeviceID: deviceID,
			TS:       state.TS,
			Payload: map[string]any{
				"source": "cloud_http",
				"state":  state.State,
			},
		})
	}
	return nil
}

func (p *Plugin) refreshAccount(ctx context.Context, account *accountRuntime) ([]*deviceRuntime, error) {
	rawDevices, err := account.client.ListDevices(ctx, account.cfg.HomeIDs)
	if err != nil {
		return nil, fmt.Errorf("xiaomi account %s: %w", account.cfg.Name, err)
	}
	out := make([]*deviceRuntime, 0, len(rawDevices))
	var errs []string
	for _, raw := range rawDevices {
		instance, err := account.specInstance(ctx, raw.URN)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s spec %s: %v", account.cfg.Name, raw.URN, err))
			continue
		}
		device, mapping, err := mapper.Build(raw, instance, account.cfg.Name)
		if err != nil {
			continue
		}
		out = append(out, &deviceRuntime{
			accountName: account.cfg.Name,
			account:     account,
			raw:         raw,
			device:      *device,
			mapping:     mapping,
		})
	}
	if len(errs) > 0 {
		return out, errors.New(strings.Join(errs, "; "))
	}
	return out, nil
}

func (p *Plugin) readState(ctx context.Context, runtime *deviceRuntime) (models.DeviceStateSnapshot, error) {
	refs := propertyRefs(runtime.mapping)
	params := make([]map[string]any, 0, len(refs))
	for _, item := range refs {
		params = append(params, map[string]any{
			"did":  runtime.raw.DID,
			"siid": item.ref.ServiceIID,
			"piid": item.ref.Property.IID,
		})
	}
	state := map[string]any{}
	if len(params) > 0 {
		results, err := runtime.account.client.GetProps(ctx, params)
		if err != nil {
			return models.DeviceStateSnapshot{}, err
		}
		indexed := map[string]any{}
		for _, result := range results {
			key := fmt.Sprintf("%d.%d", intParam(result["siid"]), intParam(result["piid"]))
			indexed[key] = result["value"]
		}
		for _, item := range refs {
			key := fmt.Sprintf("%d.%d", item.ref.ServiceIID, item.ref.Property.IID)
			value, ok := indexed[key]
			if !ok {
				continue
			}
			state[item.name] = decodePropertyValue(item.ref.Property, item.name, value)
		}
	}
	return models.DeviceStateSnapshot{
		DeviceID: runtime.device.ID,
		PluginID: runtime.device.PluginID,
		TS:       time.Now().UTC(),
		State:    state,
	}, nil
}

func (p *Plugin) runtime(deviceID string) (*deviceRuntime, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	runtime, ok := p.runtimes[deviceID]
	return runtime, ok
}

func (p *Plugin) accountList() []*accountRuntime {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]*accountRuntime, 0, len(p.accounts))
	for _, runtime := range p.accounts {
		out = append(out, runtime)
	}
	return out
}

func (r *accountRuntime) specInstance(ctx context.Context, urn string) (spec.Instance, error) {
	if item, ok := r.specs[urn]; ok {
		return item, nil
	}
	instance, err := r.client.SpecInstance(ctx, urn)
	if err != nil {
		return spec.Instance{}, err
	}
	r.specs[urn] = instance
	return instance, nil
}

func (p *Plugin) emit(event models.Event) {
	select {
	case p.events <- event:
	default:
	}
}

type namedPropertyRef struct {
	name string
	ref  *mapper.PropertyRef
}

func propertyRefs(mapping *mapper.DeviceMapping) []namedPropertyRef {
	var refs []namedPropertyRef
	appendRef := func(name string, ref *mapper.PropertyRef) {
		if ref == nil || !stateReadable(ref) {
			return
		}
		refs = append(refs, namedPropertyRef{name: name, ref: ref})
	}
	appendRef("power", mapping.Power)
	appendRef("brightness", mapping.Brightness)
	appendRef("color_temp", mapping.ColorTemp)
	appendRef("target_temperature", mapping.TargetTemperature)
	appendRef("mode", mapping.Mode)
	appendRef("fan_speed", mapping.FanSpeed)
	appendRef("temperature", mapping.Temperature)
	appendRef("humidity", mapping.Humidity)
	appendRef("pump_power", mapping.PumpPower)
	appendRef("light_power", mapping.LightPower)
	appendRef("light_brightness", mapping.LightBrightness)
	appendRef("light_mode", mapping.LightMode)
	appendRef("water_temperature", mapping.WaterTemperature)
	appendRef("filter_life", mapping.FilterLife)
	appendRef("volume", mapping.Volume)
	appendRef("mute", mapping.Mute)
	for _, toggle := range mapping.ToggleChannels {
		appendRef(toggle.StateKey, toggle.Ref)
	}
	return refs
}

func stateReadable(ref *mapper.PropertyRef) bool {
	if ref == nil {
		return false
	}
	return ref.Property.Readable() || ref.Property.Notifiable()
}

func (p *Plugin) syncAccountSessionConfig(accountName string, client *cloud.Client) {
	serviceToken, ssecurity, userID, cuserID, ok := client.CurrentLegacySession()
	if !ok {
		return
	}

	var snapshot Config
	changed := false

	p.mu.Lock()
	for idx := range p.config.Accounts {
		account := p.config.Accounts[idx]
		if account.Name != accountName {
			continue
		}
		if account.ServiceToken != serviceToken {
			account.ServiceToken = serviceToken
			changed = true
		}
		if account.SSecurity != ssecurity {
			account.SSecurity = ssecurity
			changed = true
		}
		if account.UserID != userID {
			account.UserID = userID
			changed = true
		}
		if account.CUserID != cuserID {
			account.CUserID = cuserID
			changed = true
		}
		if account.VerifyURL != "" {
			account.VerifyURL = ""
			changed = true
		}
		if account.VerifyTicket != "" {
			account.VerifyTicket = ""
			changed = true
		}
		if changed {
			p.config.Accounts[idx] = account
			snapshot = p.config
		}
		break
	}
	p.mu.Unlock()

	if !changed {
		return
	}
	payload, err := configMap(snapshot)
	if err != nil {
		return
	}
	p.emit(models.Event{
		ID:       uuid.NewString(),
		Type:     models.EventPluginConfigUpdated,
		PluginID: "xiaomi",
		TS:       time.Now().UTC(),
		Payload: map[string]any{
			"config": payload,
		},
	})
}

func configMap(cfg Config) (map[string]any, error) {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func parseConfig(cfg map[string]any, existing map[string]*accountRuntime) (Config, map[string]*accountRuntime, error) {
	config := Config{PollIntervalSeconds: 30}
	if raw, ok := cfg["poll_interval_seconds"].(float64); ok && int(raw) > 0 {
		config.PollIntervalSeconds = int(raw)
	}
	accountsRaw, ok := cfg["accounts"].([]any)
	if !ok || len(accountsRaw) == 0 {
		return Config{}, nil, errors.New("accounts is required")
	}
	runtimes := make(map[string]*accountRuntime, len(accountsRaw))
	for idx, raw := range accountsRaw {
		entry, _ := raw.(map[string]any)
		account, cloudCfg, err := parseAccount(entry, idx)
		if err != nil {
			return Config{}, nil, err
		}
		config.Accounts = append(config.Accounts, account)
		if prev := existing[account.Name]; prev != nil && canReuseAccountRuntime(prev.cfg, cloudCfg) {
			prev.cfg = cloudCfg
			prev.client.UpdateConfig(cloudCfg)
			runtimes[account.Name] = prev
			continue
		}
		runtimes[account.Name] = &accountRuntime{
			cfg:    cloudCfg,
			client: cloud.NewClient(cloudCfg, nil),
			specs:  map[string]spec.Instance{},
		}
	}
	return config, runtimes, nil
}

func canReuseAccountRuntime(current, next cloud.AccountConfig) bool {
	return current.Name == next.Name &&
		current.Region == next.Region &&
		current.Username == next.Username &&
		current.DeviceID == next.DeviceID
}

func parseAccount(entry map[string]any, idx int) (AccountConfig, cloud.AccountConfig, error) {
	account := AccountConfig{
		Name:         stringParam(entry["name"]),
		Region:       oauth.NormalizeRegion(stringParam(entry["region"])),
		Username:     stringParam(entry["username"]),
		Password:     stringParam(entry["password"]),
		VerifyURL:    stringParam(entry["verify_url"]),
		VerifyTicket: stringParam(entry["verify_ticket"]),
		ClientID:     stringParam(entry["client_id"]),
		RedirectURL:  stringParam(entry["redirect_url"]),
		AccessToken:  stringParam(entry["access_token"]),
		RefreshToken: stringParam(entry["refresh_token"]),
		AuthCode:     stringParam(entry["auth_code"]),
		DeviceID:     stringParam(entry["device_id"]),
		ServiceToken: stringParam(entry["service_token"]),
		SSecurity:    stringParam(entry["ssecurity"]),
		UserID:       stringParam(entry["user_id"]),
		CUserID:      stringParam(entry["cuser_id"]),
		Locale:       stringParam(entry["locale"]),
		Timezone:     stringParam(entry["timezone"]),
		ExpiresAt:    stringParam(entry["expires_at"]),
	}
	if account.Name == "" {
		account.Name = fmt.Sprintf("xiaomi-%d", idx+1)
	}
	if !slices.Contains([]string{"cn", "de", "i2", "ru", "sg", "us"}, account.Region) {
		return AccountConfig{}, cloud.AccountConfig{}, fmt.Errorf("unsupported Xiaomi region %q", account.Region)
	}
	hasPasswordLogin := account.Username != "" || account.Password != ""
	if hasPasswordLogin && (account.Username == "" || account.Password == "") {
		return AccountConfig{}, cloud.AccountConfig{}, fmt.Errorf("xiaomi account %q requires both username and password", account.Name)
	}
	hasVerification := account.VerifyURL != "" || account.VerifyTicket != ""
	if hasVerification && (account.VerifyURL == "" || account.VerifyTicket == "") {
		return AccountConfig{}, cloud.AccountConfig{}, fmt.Errorf("xiaomi account %q requires verify_url and verify_ticket together", account.Name)
	}
	if hasVerification && !hasPasswordLogin {
		return AccountConfig{}, cloud.AccountConfig{}, fmt.Errorf("xiaomi account %q requires username/password when verify_ticket is provided", account.Name)
	}
	hasLegacySession := account.ServiceToken != "" || account.SSecurity != "" || account.UserID != ""
	if hasLegacySession && (account.ServiceToken == "" || account.SSecurity == "" || account.UserID == "") {
		return AccountConfig{}, cloud.AccountConfig{}, fmt.Errorf("xiaomi account %q requires service_token, ssecurity, and user_id together", account.Name)
	}
	hasOAuthSession := account.AccessToken != "" || account.RefreshToken != "" || account.AuthCode != ""
	if (account.RefreshToken != "" || account.AuthCode != "") && (account.ClientID == "" || account.RedirectURL == "") {
		return AccountConfig{}, cloud.AccountConfig{}, fmt.Errorf("xiaomi account %q requires client_id and redirect_url for refresh_token/auth_code flows", account.Name)
	}
	if account.AuthCode != "" && account.DeviceID == "" {
		return AccountConfig{}, cloud.AccountConfig{}, fmt.Errorf("xiaomi account %q requires device_id when auth_code is provided", account.Name)
	}
	if !hasPasswordLogin && !hasLegacySession && !hasOAuthSession {
		return AccountConfig{}, cloud.AccountConfig{}, fmt.Errorf("xiaomi account %q requires username/password, service_token/ssecurity/user_id, or OAuth token fields", account.Name)
	}
	if account.DeviceID == "" && hasPasswordLogin {
		account.DeviceID = stableDeviceID(account.Name, account.Username, account.Region)
	}
	if rawHomeIDs, ok := entry["home_ids"].([]any); ok {
		for _, item := range rawHomeIDs {
			value := stringParam(item)
			if value != "" {
				account.HomeIDs = append(account.HomeIDs, value)
			}
		}
	}
	var expiresAt time.Time
	if strings.TrimSpace(account.ExpiresAt) != "" {
		parsed, err := time.Parse(time.RFC3339, account.ExpiresAt)
		if err != nil {
			return AccountConfig{}, cloud.AccountConfig{}, fmt.Errorf("xiaomi account %q expires_at must be RFC3339", account.Name)
		}
		expiresAt = parsed.UTC()
	}
	cloudCfg := cloud.AccountConfig{
		Name:         account.Name,
		Region:       account.Region,
		Username:     account.Username,
		Password:     account.Password,
		VerifyURL:    account.VerifyURL,
		VerifyTicket: account.VerifyTicket,
		ClientID:     account.ClientID,
		RedirectURL:  account.RedirectURL,
		AccessToken:  account.AccessToken,
		RefreshToken: account.RefreshToken,
		AuthCode:     account.AuthCode,
		DeviceID:     account.DeviceID,
		ServiceToken: account.ServiceToken,
		SSecurity:    account.SSecurity,
		UserID:       account.UserID,
		CUserID:      account.CUserID,
		HomeIDs:      account.HomeIDs,
		Locale:       account.Locale,
		Timezone:     account.Timezone,
		ExpiresAt:    expiresAt,
	}
	return account, cloudCfg, nil
}

func stableDeviceID(parts ...string) string {
	joined := strings.ToUpper(strings.Join(parts, "|"))
	if joined == "" {
		return "CELESTIA00000000"
	}
	replacer := strings.NewReplacer("|", "", "@", "", ".", "", "-", "")
	joined = replacer.Replace(joined)
	if len(joined) >= 16 {
		return joined[:16]
	}
	for len(joined) < 16 {
		joined += "0"
	}
	return joined
}

func encodePropertyValue(prop spec.Property, raw any) (any, error) {
	switch prop.Format {
	case "bool":
		return boolParam(raw, "", false), nil
	case "string":
		return stringParam(raw), nil
	default:
		if len(prop.ValueList) > 0 {
			switch typed := raw.(type) {
			case string:
				if value, ok := prop.EnumValue(typed); ok {
					return value, nil
				}
				return nil, fmt.Errorf("unsupported enum value %q", typed)
			default:
				return intParam(raw), nil
			}
		}
		number := floatParam(raw)
		if min, max, _, ok := prop.RangeBounds(); ok {
			if number < min {
				number = min
			}
			if number > max {
				number = max
			}
		}
		if strings.HasPrefix(prop.Format, "uint") || strings.HasPrefix(prop.Format, "int") {
			return int(number), nil
		}
		return number, nil
	}
}

func decodePropertyValue(prop spec.Property, key string, raw any) any {
	if len(prop.ValueList) > 0 {
		if desc, ok := prop.EnumDescription(intParam(raw)); ok {
			return desc
		}
	}
	switch key {
	case "brightness", "color_temp", "target_temperature", "light_brightness", "filter_life", "volume":
		return intParam(raw)
	case "temperature", "water_temperature":
		return floatParam(raw)
	default:
		switch prop.Format {
		case "bool":
			return boolParam(raw, "", false)
		case "string":
			return stringParam(raw)
		default:
			if strings.HasPrefix(prop.Format, "uint") || strings.HasPrefix(prop.Format, "int") {
				return intParam(raw)
			}
			return raw
		}
	}
}

func cloneViews(devices map[string]models.Device, _ map[string]models.DeviceStateSnapshot) []models.Device {
	out := make([]models.Device, 0, len(devices))
	for _, device := range devices {
		out = append(out, device)
	}
	return out
}

func cloneStates(states map[string]models.DeviceStateSnapshot) []models.DeviceStateSnapshot {
	out := make([]models.DeviceStateSnapshot, 0, len(states))
	for _, state := range states {
		out = append(out, state)
	}
	return out
}

func cloneStateMap(in map[string]models.DeviceStateSnapshot) map[string]models.DeviceStateSnapshot {
	out := make(map[string]models.DeviceStateSnapshot, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func stringParam(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}

func boolParam(value any, key string, fallback bool) bool {
	if key != "" {
		if typed, ok := value.(map[string]any); ok {
			value = typed[key]
		}
	}
	switch typed := value.(type) {
	case bool:
		return typed
	default:
		return fallback
	}
}

func intParam(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func floatParam(value any) float64 {
	switch typed := value.(type) {
	case int:
		return float64(typed)
	case int32:
		return float64(typed)
	case int64:
		return float64(typed)
	case float64:
		return typed
	default:
		return 0
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
