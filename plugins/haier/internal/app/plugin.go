package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/pluginruntime"
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
