package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chentianyu/celestia/internal/coreapi"
	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/pluginruntime"
	"github.com/google/uuid"
)

type AccountConfig struct {
	Name             string `json:"name,omitempty"`
	Username         string `json:"username"`
	Password         string `json:"password"`
	Region           string `json:"region"`
	Timezone         string `json:"timezone,omitempty"`
	SessionID        string `json:"session_id,omitempty"`
	SessionUserID    string `json:"session_user_id,omitempty"`
	SessionCreatedAt string `json:"session_created_at,omitempty"`
	SessionExpiresAt string `json:"session_expires_at,omitempty"`
	SessionBaseURL   string `json:"session_base_url,omitempty"`
}

type CompatConfig struct {
	PassportBaseURL string `json:"passport_base_url,omitempty"`
	ChinaBaseURL    string `json:"china_base_url,omitempty"`
	APIVersion      string `json:"api_version,omitempty"`
	ClientHeader    string `json:"client_header,omitempty"`
	UserAgent       string `json:"user_agent,omitempty"`
	Locale          string `json:"locale,omitempty"`
	AcceptLanguage  string `json:"accept_language,omitempty"`
	Platform        string `json:"platform,omitempty"`
	OSVersion       string `json:"os_version,omitempty"`
	ModelName       string `json:"model_name,omitempty"`
	PhoneBrand      string `json:"phone_brand,omitempty"`
	Source          string `json:"source,omitempty"`
	HourMode        string `json:"hour_mode,omitempty"`
}

type Config struct {
	Accounts            []AccountConfig `json:"accounts"`
	PollIntervalSeconds int             `json:"poll_interval_seconds"`
	Compat              CompatConfig    `json:"compat,omitempty"`
}

type deviceSnapshot struct {
	AccountName string
	Client      *Client
	Info        petkitDeviceInfo
	Device      models.Device
	State       models.DeviceStateSnapshot
	Detail      map[string]any
}

type accountRuntime struct {
	cfg      AccountConfig
	client   *Client
	devices  map[string]deviceSnapshot
	lastErr  error
	lastSync time.Time
}

type Plugin struct {
	mu       sync.RWMutex
	config   Config
	runtimes map[string]*accountRuntime
	devices  map[string]models.Device
	states   map[string]models.DeviceStateSnapshot
	events   chan models.Event
	cancel   context.CancelFunc
	started  bool
}

func New() *Plugin {
	return &Plugin{
		runtimes: map[string]*accountRuntime{},
		devices:  map[string]models.Device{},
		states:   map[string]models.DeviceStateSnapshot{},
		events:   make(chan models.Event, 128),
	}
}

func (p *Plugin) Manifest() models.PluginManifest {
	return models.PluginManifest{
		ID:      "petkit",
		Name:    "Petkit Plugin",
		Version: "1.0.0",
		Vendor:  "petkit",
		Capabilities: []string{
			"discover",
			"state",
			"command",
			"events",
			"cloud_login",
			"cloud_session",
			"feeder_control",
			"litter_control",
			"fountain_ble_relay",
		},
		ConfigSchema: map[string]any{
			"type": "object",
			"default": map[string]any{
				"accounts": []map[string]any{
					{
						"name":     "primary",
						"username": "<petkit-username>",
						"password": "<petkit-password>",
						"region":   "US",
						"timezone": "Asia/Shanghai",
					},
				},
				"poll_interval_seconds": 30,
				"compat": map[string]any{
					"passport_base_url": "https://passport.petkt.com/",
					"china_base_url":    "https://api.petkit.cn/6/",
					"api_version":       "13.2.1",
					"client_header":     "android(16.1;23127PN0CG)",
					"user_agent":        "okhttp/3.14.9",
					"locale":            "en-US",
					"accept_language":   "en-US;q=1, it-US;q=0.9",
					"platform":          "android",
					"os_version":        "16.1",
					"model_name":        "23127PN0CG",
					"phone_brand":       "Xiaomi",
					"source":            "app.petkit-android",
					"hour_mode":         "24",
				},
			},
			"properties": map[string]any{
				"poll_interval_seconds": map[string]any{
					"type":    "number",
					"default": 30,
				},
				"accounts": map[string]any{
					"type":        "array",
					"description": "Petkit cloud accounts with username, password, region and timezone.",
				},
				"compat": map[string]any{
					"type":        "object",
					"description": "Optional Petkit cloud compatibility overrides. Leave empty to use the built-in defaults that track the current upstream app behavior.",
				},
			},
		},
		DeviceKinds: []models.DeviceKind{
			models.DeviceKindPetFeeder,
			models.DeviceKindPetLitterBox,
			models.DeviceKindPetFountain,
		},
	}
}

func (p *Plugin) ValidateConfig(_ context.Context, cfg map[string]any) error {
	_, err := parseConfig(cfg)
	return err
}

func (p *Plugin) Setup(_ context.Context, cfg map[string]any) error {
	config, err := parseConfig(cfg)
	if err != nil {
		return err
	}
	runtimes := make(map[string]*accountRuntime, len(config.Accounts))
	for _, account := range config.Accounts {
		runtimes[accountKey(account)] = &accountRuntime{
			cfg:     account,
			client:  NewClient(account, config.Compat),
			devices: map[string]deviceSnapshot{},
		}
	}

	p.mu.Lock()
	p.config = config
	p.runtimes = runtimes
	p.devices = map[string]models.Device{}
	p.states = map[string]models.DeviceStateSnapshot{}
	p.mu.Unlock()
	return nil
}

func (p *Plugin) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.started {
		p.mu.Unlock()
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	interval := time.Duration(max(p.config.PollIntervalSeconds, 30)) * time.Second
	p.cancel = cancel
	p.started = true
	p.mu.Unlock()

	go p.pollLoop(runCtx, interval)
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
	message := "petkit cloud polling active"
	if !p.started {
		status = models.HealthStateStopped
		message = "plugin idle"
		return pluginruntime.NewHealth("petkit", "1.0.0", status, message)
	}
	var failed int
	var total int
	for _, runtime := range p.runtimes {
		total++
		if runtime.lastErr != nil {
			failed++
		}
	}
	if total > 0 && failed == total {
		status = models.HealthStateUnhealthy
		message = "all Petkit accounts are failing to sync"
	} else if failed > 0 {
		status = models.HealthStateDegraded
		message = "some Petkit accounts are failing to sync"
	}
	return pluginruntime.NewHealth("petkit", "1.0.0", status, message)
}

func (p *Plugin) DiscoverDevices(ctx context.Context) ([]models.Device, []models.DeviceStateSnapshot, error) {
	if err := p.refreshAll(ctx); err != nil {
		return nil, nil, err
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	devices, states := cloneDeviceViews(p.devices, p.states)
	return devices, states, nil
}

func (p *Plugin) ListDevices(ctx context.Context) ([]models.Device, error) {
	devices, _, err := p.DiscoverDevices(ctx)
	return devices, err
}

func (p *Plugin) GetDeviceState(ctx context.Context, deviceID string) (models.DeviceStateSnapshot, error) {
	if err := p.refreshDeviceIfNeeded(ctx, deviceID); err != nil {
		// Fall through to cached state if available, but keep the error if not.
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
	snapshot, ok := p.snapshotForDevice(req.DeviceID)
	if !ok {
		return models.CommandResponse{}, errors.New("device not found")
	}
	if err := snapshot.Client.ExecuteCommand(ctx, snapshot, req); err != nil {
		return models.CommandResponse{}, err
	}
	if err := p.syncAccountSession(snapshot.AccountName, snapshot.Client); err != nil {
		return models.CommandResponse{}, err
	}

	if refreshed, err := snapshot.Client.RefreshDevice(ctx, snapshot); err == nil {
		p.applySnapshot(snapshot.AccountName, refreshed)
	} else {
		p.setRuntimeError(snapshot.AccountName, err)
	}

	response := models.CommandResponse{
		Accepted: true,
		JobID:    uuid.NewString(),
		Message:  "command accepted",
	}
	p.emitLocked(models.Event{
		ID:       uuid.NewString(),
		Type:     models.EventDeviceCommandAccept,
		PluginID: "petkit",
		DeviceID: req.DeviceID,
		TS:       time.Now().UTC(),
		Payload: map[string]any{
			"action": req.Action,
		},
	})
	return response, nil
}

func (p *Plugin) Events() <-chan models.Event {
	return p.events
}

func (p *Plugin) pollLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := p.refreshAll(ctx); err != nil {
			p.setLastGlobalError(err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (p *Plugin) refreshAll(ctx context.Context) error {
	p.mu.RLock()
	runtimes := make([]*accountRuntime, 0, len(p.runtimes))
	for _, runtime := range p.runtimes {
		runtimes = append(runtimes, runtime)
	}
	p.mu.RUnlock()

	var firstErr error
	updatedAny := false
	for _, runtime := range runtimes {
		snapshots, err := runtime.client.Sync(ctx)
		if err != nil {
			runtime.lastErr = err
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		runtime.lastErr = nil
		runtime.lastSync = time.Now().UTC()
		if err := p.syncAccountSession(runtime.cfg.Name, runtime.client); err != nil {
			runtime.lastErr = err
			if firstErr == nil {
				firstErr = err
			}
		}
		p.applyAccountSnapshots(runtime.cfg, snapshots)
		updatedAny = true
	}
	if updatedAny {
		return nil
	}
	return firstErr
}

func (p *Plugin) refreshDeviceIfNeeded(ctx context.Context, deviceID string) error {
	accountName, ok := p.accountForDevice(deviceID)
	if !ok {
		return errors.New("device not found")
	}
	p.mu.RLock()
	runtime := p.runtimes[accountName]
	p.mu.RUnlock()
	if runtime == nil {
		return errors.New("account runtime not found")
	}
	snapshot, err := runtime.client.RefreshDeviceByID(ctx, deviceID)
	if err != nil {
		return err
	}
	if err := p.syncAccountSession(runtime.cfg.Name, runtime.client); err != nil {
		return err
	}
	p.applyAccountSnapshots(runtime.cfg, []deviceSnapshot{snapshot})
	return nil
}

func (p *Plugin) applyAccountSnapshots(cfg AccountConfig, snapshots []deviceSnapshot) {
	sort.SliceStable(snapshots, func(i, j int) bool {
		return snapshots[i].Device.ID < snapshots[j].Device.ID
	})

	p.mu.Lock()
	defer p.mu.Unlock()

	runtime := p.runtimes[accountKey(cfg)]
	if runtime == nil {
		runtime = &accountRuntime{cfg: cfg, client: NewClient(cfg, p.config.Compat), devices: map[string]deviceSnapshot{}}
		p.runtimes[accountKey(cfg)] = runtime
	}

	next := make(map[string]deviceSnapshot, len(snapshots))
	for _, snapshot := range snapshots {
		next[snapshot.Device.ID] = snapshot
		if old, ok := p.states[snapshot.Device.ID]; !ok {
			p.emitLocked(models.Event{
				ID:       uuid.NewString(),
				Type:     models.EventDeviceDiscovered,
				PluginID: "petkit",
				DeviceID: snapshot.Device.ID,
				TS:       snapshot.State.TS,
				Payload: map[string]any{
					"device": snapshot.Device,
				},
			})
		} else if !stateEqual(old.State, snapshot.State.State) {
			p.emitLocked(models.Event{
				ID:       uuid.NewString(),
				Type:     models.EventDeviceStateChanged,
				PluginID: "petkit",
				DeviceID: snapshot.Device.ID,
				TS:       snapshot.State.TS,
				Payload: map[string]any{
					"state": snapshot.State.State,
				},
			})
		}
		p.devices[snapshot.Device.ID] = snapshot.Device
		p.states[snapshot.Device.ID] = snapshot.State
	}
	runtime.devices = next
}

func (p *Plugin) applySnapshot(accountName string, snapshot deviceSnapshot) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if old, ok := p.states[snapshot.Device.ID]; !ok {
		p.emitLocked(models.Event{
			ID:       uuid.NewString(),
			Type:     models.EventDeviceDiscovered,
			PluginID: "petkit",
			DeviceID: snapshot.Device.ID,
			TS:       snapshot.State.TS,
		})
	} else if !stateEqual(old.State, snapshot.State.State) {
		p.emitLocked(models.Event{
			ID:       uuid.NewString(),
			Type:     models.EventDeviceStateChanged,
			PluginID: "petkit",
			DeviceID: snapshot.Device.ID,
			TS:       snapshot.State.TS,
			Payload: map[string]any{
				"state": snapshot.State.State,
			},
		})
	}
	p.devices[snapshot.Device.ID] = snapshot.Device
	p.states[snapshot.Device.ID] = snapshot.State
	if runtime := p.runtimes[accountName]; runtime != nil {
		runtime.devices[snapshot.Device.ID] = snapshot
		runtime.lastSync = time.Now().UTC()
	}
}

func (p *Plugin) snapshotForDevice(deviceID string) (deviceSnapshot, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, runtime := range p.runtimes {
		if snapshot, ok := runtime.devices[deviceID]; ok {
			return snapshot, true
		}
	}
	return deviceSnapshot{}, false
}

func (p *Plugin) accountForDevice(deviceID string) (string, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for key, runtime := range p.runtimes {
		if _, ok := runtime.devices[deviceID]; ok {
			return key, true
		}
	}
	return "", false
}

func (p *Plugin) setRuntimeError(accountName string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if runtime := p.runtimes[accountName]; runtime != nil {
		runtime.lastErr = err
	}
}

func (p *Plugin) setLastGlobalError(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, runtime := range p.runtimes {
		runtime.lastErr = err
	}
}

func (p *Plugin) emitLocked(event models.Event) {
	select {
	case p.events <- event:
	default:
	}
}

func cloneDeviceViews(devices map[string]models.Device, states map[string]models.DeviceStateSnapshot) ([]models.Device, []models.DeviceStateSnapshot) {
	deviceIDs := make([]string, 0, len(devices))
	for id := range devices {
		deviceIDs = append(deviceIDs, id)
	}
	sort.Strings(deviceIDs)
	outDevices := make([]models.Device, 0, len(deviceIDs))
	outStates := make([]models.DeviceStateSnapshot, 0, len(deviceIDs))
	for _, id := range deviceIDs {
		outDevices = append(outDevices, devices[id])
		outStates = append(outStates, states[id])
	}
	return outDevices, outStates
}

func parseConfig(cfg map[string]any) (Config, error) {
	config := Config{
		PollIntervalSeconds: 30,
		Compat:              defaultCompatConfig(),
	}
	if raw, ok := cfg["poll_interval_seconds"].(float64); ok && int(raw) > 0 {
		config.PollIntervalSeconds = int(raw)
	}
	if compat, ok := cfg["compat"].(map[string]any); ok {
		config.Compat = parseCompatConfig(compat)
	}
	rawAccounts, ok := cfg["accounts"].([]any)
	if !ok || len(rawAccounts) == 0 {
		return Config{}, errors.New("accounts is required")
	}
	for i, raw := range rawAccounts {
		entry, _ := raw.(map[string]any)
		account := AccountConfig{
			Name:             stringValue(entry["name"], ""),
			Username:         strings.TrimSpace(stringValue(entry["username"], "")),
			Password:         strings.TrimSpace(stringValue(entry["password"], "")),
			Region:           strings.ToLower(strings.TrimSpace(stringValue(entry["region"], ""))),
			Timezone:         strings.TrimSpace(stringValue(entry["timezone"], "")),
			SessionID:        strings.TrimSpace(stringValue(entry["session_id"], "")),
			SessionUserID:    strings.TrimSpace(stringValue(entry["session_user_id"], "")),
			SessionCreatedAt: strings.TrimSpace(stringValue(entry["session_created_at"], "")),
			SessionExpiresAt: strings.TrimSpace(stringValue(entry["session_expires_at"], "")),
			SessionBaseURL:   strings.TrimSpace(stringValue(entry["session_base_url"], "")),
		}
		if account.Username == "" {
			return Config{}, fmt.Errorf("accounts[%d].username is required", i)
		}
		if account.Password == "" {
			return Config{}, fmt.Errorf("accounts[%d].password is required", i)
		}
		if account.Region == "" {
			return Config{}, fmt.Errorf("accounts[%d].region is required", i)
		}
		if account.Timezone == "" {
			if account.Region == "cn" {
				account.Timezone = "Asia/Shanghai"
			} else {
				account.Timezone = "UTC"
			}
		}
		if account.Name == "" {
			account.Name = account.Username
		}
		config.Accounts = append(config.Accounts, account)
	}
	return config, nil
}

func defaultCompatConfig() CompatConfig {
	return CompatConfig{
		PassportBaseURL: "https://passport.petkt.com/",
		ChinaBaseURL:    "https://api.petkit.cn/6/",
		APIVersion:      "13.2.1",
		ClientHeader:    "android(16.1;23127PN0CG)",
		UserAgent:       "okhttp/3.14.9",
		Locale:          "en-US",
		AcceptLanguage:  "en-US;q=1, it-US;q=0.9",
		Platform:        "android",
		OSVersion:       "16.1",
		ModelName:       "23127PN0CG",
		PhoneBrand:      "Xiaomi",
		Source:          "app.petkit-android",
		HourMode:        "24",
	}
}

func parseCompatConfig(raw map[string]any) CompatConfig {
	compat := defaultCompatConfig()
	if value := strings.TrimSpace(stringValue(raw["passport_base_url"], "")); value != "" {
		compat.PassportBaseURL = value
	}
	if value := strings.TrimSpace(stringValue(raw["china_base_url"], "")); value != "" {
		compat.ChinaBaseURL = value
	}
	if value := strings.TrimSpace(stringValue(raw["api_version"], "")); value != "" {
		compat.APIVersion = value
	}
	if value := strings.TrimSpace(stringValue(raw["client_header"], "")); value != "" {
		compat.ClientHeader = value
	}
	if value := strings.TrimSpace(stringValue(raw["user_agent"], "")); value != "" {
		compat.UserAgent = value
	}
	if value := strings.TrimSpace(stringValue(raw["locale"], "")); value != "" {
		compat.Locale = value
	}
	if value := strings.TrimSpace(stringValue(raw["accept_language"], "")); value != "" {
		compat.AcceptLanguage = value
	}
	if value := strings.TrimSpace(stringValue(raw["platform"], "")); value != "" {
		compat.Platform = value
	}
	if value := strings.TrimSpace(stringValue(raw["os_version"], "")); value != "" {
		compat.OSVersion = value
	}
	if value := strings.TrimSpace(stringValue(raw["model_name"], "")); value != "" {
		compat.ModelName = value
	}
	if value := strings.TrimSpace(stringValue(raw["phone_brand"], "")); value != "" {
		compat.PhoneBrand = value
	}
	if value := strings.TrimSpace(stringValue(raw["source"], "")); value != "" {
		compat.Source = value
	}
	if value := strings.TrimSpace(stringValue(raw["hour_mode"], "")); value != "" {
		compat.HourMode = value
	}
	return compat
}

func accountKey(cfg AccountConfig) string {
	return accountKeyString(cfg.Name + "|" + cfg.Username + "|" + cfg.Region)
}

func (p *Plugin) syncAccountSession(accountName string, client *Client) error {
	baseURL, session, ok := client.CurrentSession()
	if !ok {
		return nil
	}

	var snapshot Config
	changed := false

	p.mu.Lock()
	for idx := range p.config.Accounts {
		account := p.config.Accounts[idx]
		if account.Name != accountName {
			continue
		}
		createdAt := ""
		if !session.CreatedAt.IsZero() {
			createdAt = session.CreatedAt.UTC().Format(time.RFC3339)
		}
		expiresAt := ""
		if !session.ExpiresAt.IsZero() {
			expiresAt = session.ExpiresAt.UTC().Format(time.RFC3339)
		}
		if account.SessionID != session.ID {
			account.SessionID = session.ID
			changed = true
		}
		if account.SessionUserID != session.UserID {
			account.SessionUserID = session.UserID
			changed = true
		}
		if account.SessionCreatedAt != createdAt {
			account.SessionCreatedAt = createdAt
			changed = true
		}
		if account.SessionExpiresAt != expiresAt {
			account.SessionExpiresAt = expiresAt
			changed = true
		}
		if account.SessionBaseURL != baseURL {
			account.SessionBaseURL = baseURL
			changed = true
		}
		if changed {
			p.config.Accounts[idx] = account
			if runtime := p.runtimes[accountKey(account)]; runtime != nil {
				runtime.cfg = account
			}
			snapshot = p.config
		}
		break
	}
	p.mu.Unlock()

	if !changed {
		return nil
	}
	payload, err := configMap(snapshot)
	if err != nil {
		return err
	}
	persistCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := coreapi.PersistPluginConfig(persistCtx, "petkit", payload); err != nil {
		return fmt.Errorf("persist petkit runtime config: %w", err)
	}
	return nil
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

func accountKeyString(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer(" ", "-", "/", "-", "\\", "-", "|", "-").Replace(value)
	return value
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
