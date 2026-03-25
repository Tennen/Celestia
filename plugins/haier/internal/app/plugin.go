package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/pluginruntime"
	"github.com/chentianyu/celestia/internal/pluginutil"
	"github.com/google/uuid"
)

type Plugin struct {
	mu         sync.RWMutex
	config     Config
	accounts   map[string]*accountRuntime
	devices    map[string]*applianceRuntime
	events     chan models.Event
	cancel     context.CancelFunc
	started    bool
	polling    bool
	lastError  string
	lastSyncAt time.Time
}

func New() *Plugin {
	return &Plugin{
		accounts: map[string]*accountRuntime{},
		devices:  map[string]*applianceRuntime{},
		events:   make(chan models.Event, 128),
	}
}

func (p *Plugin) Manifest() models.PluginManifest {
	return models.PluginManifest{
		ID:      "haier",
		Name:    "Haier Washer Plugin",
		Version: "0.2.0",
		Vendor:  "haier",
		Capabilities: []string{
			"discover",
			"state",
			"command",
			"events",
			"real_cloud",
			"auth",
			"refresh_token",
			"washer_capability_matrix",
		},
		DeviceKinds: []models.DeviceKind{models.DeviceKindWasher},
	}
}

func (p *Plugin) ValidateConfig(_ context.Context, cfg map[string]any) error {
	accountsRaw, ok := cfg["accounts"].([]any)
	if !ok || len(accountsRaw) == 0 {
		return errors.New("accounts is required")
	}
	for i, raw := range accountsRaw {
		entry, _ := raw.(map[string]any)
		acct := parseAccountConfig(entry)
		if !acct.hasCredentials() {
			return fmt.Errorf("account %d requires email/password or refresh_token", i)
		}
	}
	return nil
}

func (p *Plugin) Setup(_ context.Context, cfg map[string]any) error {
	config := Config{PollIntervalSeconds: 20}
	if raw, ok := cfg["poll_interval_seconds"].(float64); ok && int(raw) > 0 {
		config.PollIntervalSeconds = int(raw)
	} else if raw, ok := cfg["pollIntervalSeconds"].(float64); ok && int(raw) > 0 {
		config.PollIntervalSeconds = int(raw)
	}
	accountsRaw, ok := cfg["accounts"].([]any)
	if !ok || len(accountsRaw) == 0 {
		return errors.New("accounts is required")
	}
	config.Accounts = make([]AccountConfig, 0, len(accountsRaw))
	accountRuntimes := make(map[string]*accountRuntime, len(accountsRaw))
	for i, raw := range accountsRaw {
		entry, _ := raw.(map[string]any)
		acct := parseAccountConfig(entry)
		if !acct.hasCredentials() {
			return fmt.Errorf("account %d requires email/password or refresh_token", i)
		}
		if acct.Name == "" {
			acct.Name = acct.normalizedName()
		}
		if acct.MobileID == "" {
			acct.MobileID = acct.normalizedMobileID()
		}
		if acct.Timezone == "" {
			acct.Timezone = acct.normalizedTimezone()
		}
		client, err := newHaierClient(acct)
		if err != nil {
			return err
		}
		config.Accounts = append(config.Accounts, acct)
		accountRuntimes[acct.normalizedName()] = &accountRuntime{
			Config:     acct,
			Client:     client,
			Appliances: map[string]*applianceRuntime{},
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.config = config
	p.accounts = accountRuntimes
	p.devices = map[string]*applianceRuntime{}
	p.lastError = ""
	p.lastSyncAt = time.Time{}
	return nil
}

func (p *Plugin) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.started {
		p.mu.Unlock()
		return nil
	}
	runCtx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.started = true
	interval := time.Duration(max(p.config.PollIntervalSeconds, 10)) * time.Second
	p.polling = true
	p.mu.Unlock()

	if err := p.refreshAll(runCtx); err != nil {
		p.mu.Lock()
		p.lastError = err.Error()
		p.mu.Unlock()
		// Keep the process alive so the operator can inspect logs and fix credentials.
	}

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				if err := p.refreshAll(runCtx); err != nil {
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
	p.polling = false
	return nil
}

func (p *Plugin) HealthCheck(_ context.Context) models.PluginHealth {
	p.mu.RLock()
	defer p.mu.RUnlock()
	status := models.HealthStateHealthy
	message := "hOn sessions active"
	if !p.started {
		status = models.HealthStateStopped
		message = "plugin idle"
	} else if p.lastError != "" {
		status = models.HealthStateDegraded
		message = p.lastError
	}
	return pluginruntime.NewHealth("haier", "0.2.0", status, message)
}

func (p *Plugin) DiscoverDevices(ctx context.Context) ([]models.Device, []models.DeviceStateSnapshot, error) {
	if err := p.refreshAll(ctx); err != nil {
		p.mu.Lock()
		p.lastError = err.Error()
		p.mu.Unlock()
	}
	return p.snapshot()
}

func (p *Plugin) ListDevices(ctx context.Context) ([]models.Device, error) {
	devices, _, err := p.DiscoverDevices(ctx)
	return devices, err
}

func (p *Plugin) GetDeviceState(ctx context.Context, deviceID string) (models.DeviceStateSnapshot, error) {
	if err := p.refreshSingle(ctx, deviceID); err != nil {
		p.mu.Lock()
		p.lastError = err.Error()
		p.mu.Unlock()
	} else {
		p.mu.Lock()
		p.lastError = ""
		p.mu.Unlock()
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	device, ok := p.devices[deviceID]
	if !ok {
		return models.DeviceStateSnapshot{}, errors.New("device not found")
	}
	return models.DeviceStateSnapshot{
		DeviceID: device.Device.ID,
		PluginID: device.Device.PluginID,
		TS:       device.LastSnapshotTS,
		State:    cloneMap(device.CurrentState),
	}, nil
}

func (p *Plugin) ExecuteCommand(ctx context.Context, req models.CommandRequest) (models.CommandResponse, error) {
	p.mu.RLock()
	device, ok := p.devices[req.DeviceID]
	p.mu.RUnlock()
	if !ok {
		return models.CommandResponse{}, errors.New("device not found")
	}
	commandName, params, ancillary, programName, err := commandForRequest(device, req)
	if err != nil {
		return models.CommandResponse{}, err
	}

	var account *accountRuntime
	for _, runtime := range p.accountRuntimes() {
		if runtime.Client != nil && runtime.Appliances != nil {
			if _, found := runtime.Appliances[req.DeviceID]; found {
				account = runtime
				break
			}
		}
	}
	if account == nil {
		return models.CommandResponse{}, errors.New("device account not found")
	}

	if _, err := account.Client.sendCommand(ctx, device.ApplianceInfo, commandName, params, ancillary, programName); err != nil {
		return models.CommandResponse{}, err
	}
	if err := p.refreshSingle(ctx, req.DeviceID); err != nil {
		p.mu.Lock()
		p.lastError = err.Error()
		p.mu.Unlock()
	} else {
		p.mu.Lock()
		p.lastError = ""
		p.mu.Unlock()
	}
	p.mu.RLock()
	updated, ok := p.devices[req.DeviceID]
	p.mu.RUnlock()
	if !ok {
		return models.CommandResponse{}, errors.New("device not found after command refresh")
	}
	p.emitLocked(models.Event{
		ID:       uuid.NewString(),
		Type:     models.EventDeviceCommandAccept,
		PluginID: "haier",
		DeviceID: req.DeviceID,
		TS:       time.Now().UTC(),
		Payload: map[string]any{
			"action":   req.Action,
			"command":  commandName,
			"params":   params,
			"program":  programName,
			"snapshot": cloneMap(updated.CurrentState),
		},
	})
	return models.CommandResponse{
		Accepted: true,
		JobID:    uuid.NewString(),
		Message:  "command accepted",
	}, nil
}

func (p *Plugin) Events() <-chan models.Event {
	return p.events
}

func (p *Plugin) refreshAll(ctx context.Context) error {
	runtimes := p.accountRuntimes()
	if len(runtimes) == 0 {
		return errors.New("no accounts configured")
	}
	nextDevices := map[string]*applianceRuntime{}
	var firstErr error
	successes := 0
	for _, account := range runtimes {
		if err := p.refreshAccount(ctx, account, nextDevices); err != nil {
			account.LastError = err.Error()
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		successes++
		account.LastError = ""
		account.LastSync = time.Now().UTC()
	}
	p.mu.Lock()
	p.devices = nextDevices
	p.lastSyncAt = time.Now().UTC()
	if firstErr != nil {
		p.lastError = firstErr.Error()
	} else {
		p.lastError = ""
	}
	p.mu.Unlock()
	if successes == 0 && firstErr != nil {
		return firstErr
	}
	return nil
}

func (p *Plugin) refreshSingle(ctx context.Context, deviceID string) error {
	p.mu.RLock()
	device, ok := p.devices[deviceID]
	p.mu.RUnlock()
	if !ok {
		return errors.New("device not found")
	}
	account := p.accountForDevice(deviceID)
	if account == nil {
		return errors.New("device account not found")
	}
	return p.syncDevice(ctx, account, device)
}

func (p *Plugin) refreshAccount(ctx context.Context, account *accountRuntime, nextDevices map[string]*applianceRuntime) error {
	if account == nil || account.Client == nil {
		return errors.New("account client missing")
	}
	if err := account.Client.authenticate(ctx); err != nil {
		return err
	}
	appliances, err := account.Client.loadAppliances(ctx)
	if err != nil {
		return err
	}
	current := map[string]*applianceRuntime{}
	for _, appliance := range appliances {
		commands, err := account.Client.loadCommands(ctx, appliance)
		if err != nil {
			continue
		}
		commandNames, capabilitySet := buildCapabilities(commands)
		if len(commandNames) == 0 {
			continue
		}
		if !capabilitySet["start"] && !capabilitySet["pause"] && !capabilitySet["resume"] && !capabilitySet["stop"] {
			continue
		}
		device := buildDevice(account.Config, appliance, commandNames, capabilitySet)
		state, err := account.Client.loadAttributes(ctx, appliance)
		if err != nil {
			continue
		}
		if stats, err := account.Client.loadStatistics(ctx, appliance); err == nil && len(stats) > 0 {
			mergeMap(state, stats)
			state["statistics"] = stats
		}
		if maintenance, err := account.Client.loadMaintenance(ctx, appliance); err == nil && len(maintenance) > 0 {
			mergeMap(state, maintenance)
			state["maintenance"] = maintenance
		}
		snapshot := buildStateSnapshot(device, appliance, state)
		runtime := &applianceRuntime{
			Device:         device,
			ApplianceInfo:  appliance,
			CommandData:    commands,
			CapabilitySet:  capabilitySet,
			CommandNames:   commandNames,
			CurrentState:   snapshot.State,
			LastSnapshotTS: snapshot.TS,
		}
		current[device.ID] = runtime
		nextDevices[device.ID] = runtime
	}
	account.Appliances = current
	return nil
}

func (p *Plugin) syncDevice(ctx context.Context, account *accountRuntime, device *applianceRuntime) error {
	if account == nil || account.Client == nil {
		return errors.New("account client missing")
	}
	state, err := account.Client.loadAttributes(ctx, device.ApplianceInfo)
	if err != nil {
		return err
	}
	if stats, err := account.Client.loadStatistics(ctx, device.ApplianceInfo); err == nil && len(stats) > 0 {
		mergeMap(state, stats)
		state["statistics"] = stats
	}
	if maintenance, err := account.Client.loadMaintenance(ctx, device.ApplianceInfo); err == nil && len(maintenance) > 0 {
		mergeMap(state, maintenance)
		state["maintenance"] = maintenance
	}
	snapshot := buildStateSnapshot(device.Device, device.ApplianceInfo, state)
	p.mu.Lock()
	defer p.mu.Unlock()
	if existing, ok := p.devices[device.Device.ID]; ok {
		if !reflect.DeepEqual(existing.CurrentState, snapshot.State) {
			existing.CurrentState = snapshot.State
			existing.LastSnapshotTS = snapshot.TS
			existing.ApplianceInfo = device.ApplianceInfo
			existing.CommandData = device.CommandData
			existing.CapabilitySet = device.CapabilitySet
			existing.CommandNames = device.CommandNames
			p.emitLocked(models.Event{
				ID:       uuid.NewString(),
				Type:     models.EventDeviceStateChanged,
				PluginID: "haier",
				DeviceID: device.Device.ID,
				TS:       snapshot.TS,
				Payload: map[string]any{
					"state": snapshot.State,
				},
			})
		}
	}
	return nil
}

func (p *Plugin) snapshot() ([]models.Device, []models.DeviceStateSnapshot, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	devices := make([]models.Device, 0, len(p.devices))
	states := make([]models.DeviceStateSnapshot, 0, len(p.devices))
	for _, device := range p.devices {
		devices = append(devices, device.Device)
		states = append(states, models.DeviceStateSnapshot{
			DeviceID: device.Device.ID,
			PluginID: device.Device.PluginID,
			TS:       device.LastSnapshotTS,
			State:    cloneMap(device.CurrentState),
		})
	}
	return devices, states, nil
}

func (p *Plugin) accountRuntimes() []*accountRuntime {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]*accountRuntime, 0, len(p.accounts))
	for _, account := range p.accounts {
		out = append(out, account)
	}
	return out
}

func (p *Plugin) accountForDevice(deviceID string) *accountRuntime {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, account := range p.accounts {
		if account.Appliances != nil {
			if _, ok := account.Appliances[deviceID]; ok {
				return account
			}
		}
	}
	return nil
}

func (p *Plugin) emitLocked(event models.Event) {
	select {
	case p.events <- event:
	default:
	}
}

func buildDevice(account AccountConfig, appliance map[string]any, commandNames map[string]string, capabilitySet map[string]bool) models.Device {
	capabilities := []string{}
	for _, name := range []string{
		"start", "stop", "pause", "resume", "remaining_time", "program", "phase", "machine_status",
		"delay_time", "temp_level", "spin_speed", "prewash", "extra_rinse", "good_night", "energy_usage", "water_usage",
	} {
		if capabilitySet[name] {
			capabilities = append(capabilities, name)
		}
	}
	mac := strings.ToLower(stringFromAny(appliance["macAddress"]))
	device := models.Device{
		ID:             fmt.Sprintf("haier:washer:%s:%s", strings.ToLower(account.normalizedName()), sanitizeID(mac)),
		PluginID:       "haier",
		VendorDeviceID: stringFromAny(appliance["macAddress"]),
		Kind:           models.DeviceKindWasher,
		Name: firstNonEmpty(
			stringFromAny(appliance["nickName"]),
			stringFromAny(appliance["modelName"]),
			stringFromAny(appliance["brand"]),
			stringFromAny(appliance["applianceTypeName"]),
		),
		Room:         stringFromAny(appliance["roomName"]),
		Online:       applianceOnline(appliance),
		Capabilities: capabilities,
		Metadata: map[string]any{
			"account":            account.normalizedName(),
			"mobile_id":          account.normalizedMobileID(),
			"timezone":           account.normalizedTimezone(),
			"appliance_type":     appliance["applianceTypeName"],
			"appliance_model_id": appliance["applianceModelId"],
			"brand":              appliance["brand"],
			"code":               appliance["code"],
			"mac_address":        appliance["macAddress"],
			"capability_matrix":  capabilitySet,
			"command_names":      commandNames,
		},
	}
	return device
}

func buildStateSnapshot(device models.Device, appliance map[string]any, raw map[string]any) models.DeviceStateSnapshot {
	normalized := map[string]any{}
	parameters := extractParameters(raw)
	for k, v := range parameters {
		normalized[k] = v
	}
	normalized["parameters"] = parameters
	if len(raw) > 0 {
		normalized["raw"] = raw
	}
	if stats, ok := raw["statistics"].(map[string]any); ok {
		normalized["statistics"] = stats
	}
	if maintenance, ok := raw["maintenance"].(map[string]any); ok {
		normalized["maintenance"] = maintenance
	}
	if status := stringFromAny(parameters["machMode"]); status != "" {
		switch status {
		case "3":
			normalized["machine_status"] = "paused"
		case "0":
			normalized["machine_status"] = "idle"
		default:
			normalized["machine_status"] = "running"
		}
	}
	if normalized["machine_status"] == nil {
		if active, ok := raw["active"].(bool); ok && active {
			normalized["machine_status"] = "running"
		} else {
			normalized["machine_status"] = "idle"
		}
	}
	if program := stringFromAny(raw["programName"]); program != "" {
		normalized["program"] = program
	} else if v := stringFromAny(parameters["prCode"]); v != "" {
		normalized["program"] = v
	}
	if phase := stringFromAny(parameters["prPhase"]); phase != "" {
		normalized["phase"] = phase
	}
	if remaining := intFromAny(parameters["remainingTimeMM"]); remaining >= 0 {
		normalized["remaining_minutes"] = remaining
	}
	if temp := intFromAny(parameters["tempLevel"]); temp > 0 {
		normalized["temperature"] = temp
	}
	if spin := intFromAny(parameters["spinSpeed"]); spin > 0 {
		normalized["spin_speed"] = spin
	}
	if delay := intFromAny(parameters["delayTime"]); delay >= 0 {
		normalized["delay_time"] = delay
	}
	if prewash, ok := parameters["prewash"].(bool); ok {
		normalized["prewash"] = prewash
	}
	if rinse := intFromAny(parameters["extraRinse"]); rinse >= 0 {
		normalized["extra_rinse"] = rinse
	}
	if gn := intFromAny(parameters["goodNight"]); gn >= 0 {
		normalized["good_night"] = gn
	}
	if electricity := floatFromAny(parameters["totalElectricityUsed"]); electricity >= 0 {
		normalized["total_electricity_used"] = electricity
	}
	if water := floatFromAny(parameters["totalWaterUsed"]); water >= 0 {
		normalized["total_water_used"] = water
	}
	if cycles := intFromAny(parameters["totalWashCycle"]); cycles >= 0 {
		normalized["total_wash_cycle"] = cycles
	}
	return models.DeviceStateSnapshot{
		DeviceID: device.ID,
		PluginID: device.PluginID,
		TS:       time.Now().UTC(),
		State:    normalized,
	}
}

func commandForRequest(device *applianceRuntime, req models.CommandRequest) (string, map[string]any, map[string]any, string, error) {
	if device == nil {
		return "", nil, nil, "", errors.New("device not found")
	}
	switch req.Action {
	case "start":
		if !device.CapabilitySet["start"] {
			return "", nil, nil, "", errors.New("start unsupported by model")
		}
		commandName, err := requireCommandName(device, "start")
		if err != nil {
			return "", nil, nil, "", err
		}
		params := map[string]any{}
		for _, key := range []string{"delayTime", "tempLevel", "spinSpeed", "prewash", "extraRinse", "goodNight"} {
			if value, ok := device.CurrentState[key]; ok {
				params[key] = value
			}
		}
		if program := stringFromAny(device.CurrentState["program"]); program != "" {
			params["program"] = program
		}
		if program := stringFromAny(req.Params["program"]); program != "" {
			params["program"] = program
		}
		if program := stringFromAny(req.Params["program_name"]); program != "" {
			params["program"] = program
		}
		return commandName, params, map[string]any{}, stringFromAny(params["program"]), nil
	case "stop":
		if !device.CapabilitySet["stop"] {
			return "", nil, nil, "", errors.New("stop unsupported by model")
		}
		commandName, err := requireCommandName(device, "stop")
		if err != nil {
			return "", nil, nil, "", err
		}
		return commandName, map[string]any{}, map[string]any{}, "", nil
	case "pause":
		if !device.CapabilitySet["pause"] {
			return "", nil, nil, "", errors.New("pause unsupported by model")
		}
		commandName, err := requireCommandName(device, "pause")
		if err != nil {
			return "", nil, nil, "", err
		}
		return commandName, map[string]any{}, map[string]any{}, "", nil
	case "resume":
		if !device.CapabilitySet["resume"] {
			return "", nil, nil, "", errors.New("resume unsupported by model")
		}
		commandName, err := requireCommandName(device, "resume")
		if err != nil {
			return "", nil, nil, "", err
		}
		return commandName, map[string]any{}, map[string]any{}, "", nil
	case "set_delay_time":
		if !device.CapabilitySet["delay_time"] {
			return "", nil, nil, "", errors.New("delay_time unsupported by model")
		}
		commandName, err := requireCommandName(device, "delay_time")
		if err != nil {
			return "", nil, nil, "", err
		}
		value := pluginutil.Int(req.Params["minutes"], intFromAny(device.CurrentState["delay_time"]))
		return commandName, map[string]any{"delayTime": value}, map[string]any{}, "", nil
	case "set_temp_level":
		if !device.CapabilitySet["temp_level"] {
			return "", nil, nil, "", errors.New("temp_level unsupported by model")
		}
		commandName, err := requireCommandName(device, "temp_level")
		if err != nil {
			return "", nil, nil, "", err
		}
		value := pluginutil.Int(req.Params["value"], intFromAny(device.CurrentState["temperature"]))
		return commandName, map[string]any{"tempLevel": value}, map[string]any{}, "", nil
	case "set_spin_speed":
		if !device.CapabilitySet["spin_speed"] {
			return "", nil, nil, "", errors.New("spin_speed unsupported by model")
		}
		commandName, err := requireCommandName(device, "spin_speed")
		if err != nil {
			return "", nil, nil, "", err
		}
		value := pluginutil.Int(req.Params["value"], intFromAny(device.CurrentState["spin_speed"]))
		return commandName, map[string]any{"spinSpeed": value}, map[string]any{}, "", nil
	case "set_prewash":
		if !device.CapabilitySet["prewash"] {
			return "", nil, nil, "", errors.New("prewash unsupported by model")
		}
		commandName, err := requireCommandName(device, "prewash")
		if err != nil {
			return "", nil, nil, "", err
		}
		value := pluginutil.Bool(req.Params["enabled"], false)
		return commandName, map[string]any{"prewash": value}, map[string]any{}, "", nil
	case "set_extra_rinse":
		if !device.CapabilitySet["extra_rinse"] {
			return "", nil, nil, "", errors.New("extra_rinse unsupported by model")
		}
		commandName, err := requireCommandName(device, "extra_rinse")
		if err != nil {
			return "", nil, nil, "", err
		}
		value := pluginutil.Int(req.Params["value"], intFromAny(device.CurrentState["extra_rinse"]))
		return commandName, map[string]any{"extraRinse": value}, map[string]any{}, "", nil
	case "set_good_night":
		if !device.CapabilitySet["good_night"] {
			return "", nil, nil, "", errors.New("good_night unsupported by model")
		}
		commandName, err := requireCommandName(device, "good_night")
		if err != nil {
			return "", nil, nil, "", err
		}
		value := pluginutil.Int(req.Params["value"], intFromAny(device.CurrentState["good_night"]))
		return commandName, map[string]any{"goodNight": value}, map[string]any{}, "", nil
	default:
		return "", nil, nil, "", fmt.Errorf("unsupported action %q", req.Action)
	}
}

func buildCapabilities(commands map[string]any) (map[string]string, map[string]bool) {
	names := collectCommandNames(commands)
	commandNames := map[string]string{}
	capabilitySet := map[string]bool{}
	assign := func(capability, action string, candidates ...string) {
		if cmd := matchCommandName(names, candidates...); cmd != "" {
			commandNames[action] = cmd
			capabilitySet[capability] = true
		}
	}
	assign("start", "start", "startProgram", "startprogram")
	assign("stop", "stop", "stopProgram", "stopprogram")
	assign("pause", "pause", "pauseProgram", "pauseprogram")
	assign("resume", "resume", "resumeProgram", "resumeprogram")
	assign("delay_time", "delay_time", "setDelayTime", "delayTime", "delaytime")
	assign("temp_level", "temp_level", "setTempLevel", "tempLevel", "templevel")
	assign("spin_speed", "spin_speed", "setSpinSpeed", "spinSpeed", "spinspeed")
	assign("prewash", "prewash", "setPrewash", "prewash")
	assign("extra_rinse", "extra_rinse", "setExtraRinse", "extraRinse", "extrarinse")
	assign("good_night", "good_night", "setGoodNight", "goodNight", "goodnight")
	if containsAny(names, "remainingtime", "remainingtimemm", "remaining_time") {
		capabilitySet["remaining_time"] = true
	}
	if containsAny(names, "program", "prcode") {
		capabilitySet["program"] = true
	}
	if containsAny(names, "phase", "prphase") {
		capabilitySet["phase"] = true
	}
	if containsAny(names, "machine_status", "machmode", "status") {
		capabilitySet["machine_status"] = true
	}
	if containsAny(names, "totalelectricityused", "electricity") {
		capabilitySet["energy_usage"] = true
	}
	if containsAny(names, "totalwaterused", "water") {
		capabilitySet["water_usage"] = true
	}
	return commandNames, capabilitySet
}

func collectCommandNames(data map[string]any) []string {
	out := []string{}
	for key, value := range data {
		if key == "applianceModel" {
			continue
		}
		if isCommandShape(value) {
			out = append(out, key)
			continue
		}
		if nested, ok := value.(map[string]any); ok {
			if containsCommandShape(nested) {
				out = append(out, key)
			}
		}
	}
	return out
}

func containsCommandShape(data map[string]any) bool {
	for _, value := range data {
		if isCommandShape(value) {
			return true
		}
		if nested, ok := value.(map[string]any); ok && containsCommandShape(nested) {
			return true
		}
	}
	return false
}

func isCommandShape(v any) bool {
	item, ok := v.(map[string]any)
	if !ok {
		return false
	}
	_, hasDescription := item["description"]
	_, hasProtocol := item["protocolType"]
	return hasDescription && hasProtocol
}

func matchCommandName(names []string, candidates ...string) string {
	for _, candidate := range candidates {
		needle := normalizeKey(candidate)
		for _, name := range names {
			if normalizeKey(name) == needle || strings.Contains(normalizeKey(name), needle) {
				return name
			}
		}
	}
	return ""
}

func containsAny(names []string, tokens ...string) bool {
	for _, name := range names {
		normalized := normalizeKey(name)
		for _, token := range tokens {
			if strings.Contains(normalized, normalizeKey(token)) {
				return true
			}
		}
	}
	return false
}

func normalizeKey(value string) string {
	value = strings.ToLower(value)
	value = strings.ReplaceAll(value, "_", "")
	value = strings.ReplaceAll(value, "-", "")
	value = strings.ReplaceAll(value, " ", "")
	return value
}

func requireCommandName(device *applianceRuntime, key string) (string, error) {
	if device == nil {
		return "", errors.New("device not found")
	}
	if command := strings.TrimSpace(device.CommandNames[key]); command != "" {
		return command, nil
	}
	return "", fmt.Errorf("missing command mapping for %s", key)
}

func parseAccountConfig(entry map[string]any) AccountConfig {
	return AccountConfig{
		Name:         pluginutil.String(entry["name"], ""),
		Email:        firstNonEmpty(pluginutil.String(entry["email"], ""), pluginutil.String(entry["username"], "")),
		Password:     pluginutil.String(entry["password"], ""),
		RefreshToken: firstNonEmpty(pluginutil.String(entry["refresh_token"], ""), pluginutil.String(entry["refreshToken"], "")),
		MobileID:     firstNonEmpty(pluginutil.String(entry["mobile_id"], ""), pluginutil.String(entry["mobileId"], "")),
		Timezone:     pluginutil.String(entry["timezone"], ""),
	}
}

func extractParameters(raw map[string]any) map[string]any {
	if raw == nil {
		return map[string]any{}
	}
	if shadow, ok := raw["shadow"].(map[string]any); ok {
		if params, ok := shadow["parameters"].(map[string]any); ok {
			return params
		}
	}
	if params, ok := raw["parameters"].(map[string]any); ok {
		return params
	}
	return map[string]any{}
}

func applianceOnline(appliance map[string]any) bool {
	if online, ok := appliance["online"].(bool); ok {
		return online
	}
	if connection, ok := appliance["connection"].(bool); ok {
		return connection
	}
	if lastConn, ok := appliance["lastConnEvent"].(map[string]any); ok {
		if category := stringFromAny(lastConn["category"]); strings.EqualFold(category, "DISCONNECTED") {
			return false
		}
	}
	return true
}

func cloneMap(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func mergeMap(dst map[string]any, src map[string]any) {
	for k, v := range src {
		if _, exists := dst[k]; !exists {
			dst[k] = v
		}
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func sanitizeID(value string) string {
	value = strings.ToLower(value)
	value = strings.ReplaceAll(value, ":", "-")
	value = strings.ReplaceAll(value, " ", "-")
	return value
}

func intFromAny(v any) int {
	switch raw := v.(type) {
	case int:
		return raw
	case int32:
		return int(raw)
	case int64:
		return int(raw)
	case float32:
		return int(raw)
	case float64:
		return int(raw)
	case json.Number:
		i, _ := raw.Int64()
		return int(i)
	case string:
		var i int
		_, _ = fmt.Sscanf(raw, "%d", &i)
		return i
	default:
		return 0
	}
}

func floatFromAny(v any) float64 {
	switch raw := v.(type) {
	case float64:
		return raw
	case float32:
		return float64(raw)
	case int:
		return float64(raw)
	case int64:
		return float64(raw)
	case json.Number:
		f, _ := raw.Float64()
		return f
	case string:
		var f float64
		_, _ = fmt.Sscanf(raw, "%f", &f)
		return f
	default:
		return -1
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
