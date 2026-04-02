package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/chentianyu/celestia/internal/coreapi"
	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/plugins/haier/internal/client"
	"github.com/google/uuid"
)

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
	if err := p.syncDevice(ctx, account, device); err != nil {
		return err
	}
	return p.syncAccountConfig(account)
}

func (p *Plugin) refreshAccount(ctx context.Context, account *accountRuntime, nextDevices map[string]*applianceRuntime) error {
	if account == nil || account.Client == nil {
		return errors.New("account client missing")
	}
	if err := account.Client.Authenticate(ctx); err != nil {
		return err
	}
	if err := p.syncAccountConfig(account); err != nil {
		return err
	}
	appliances, err := account.Client.LoadAppliances(ctx)
	if err != nil {
		return err
	}

	// Collect device IDs for batch digital model fetch.
	deviceIDs := make([]string, 0, len(appliances))
	for _, appliance := range appliances {
		if id := client.StringFromAny(appliance["deviceId"]); id != "" {
			deviceIDs = append(deviceIDs, id)
		}
	}

	digitalModels := map[string]map[string]string{}
	if len(deviceIDs) > 0 {
		if models, err := account.Client.LoadDigitalModels(ctx, deviceIDs); err == nil {
			digitalModels = models
		}
	}

	current := map[string]*applianceRuntime{}
	for _, appliance := range appliances {
		deviceID := client.StringFromAny(appliance["deviceId"])
		if deviceID == "" {
			continue
		}
		attrs := digitalModels[deviceID]
		if attrs == nil {
			attrs = map[string]string{}
		}
		commandNames, capabilitySet := buildCapabilitiesFromDigitalModel(attrs)
		device := buildDevice(account.Config, appliance, commandNames, capabilitySet)
		snapshot := buildStateSnapshot(device, appliance, attrs)
		runtime := &applianceRuntime{
			Device:         device,
			ApplianceInfo:  appliance,
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
	deviceID := client.StringFromAny(device.ApplianceInfo["deviceId"])
	if deviceID == "" {
		return errors.New("device has no deviceId")
	}
	digitalModels, err := account.Client.LoadDigitalModels(ctx, []string{deviceID})
	if err != nil {
		return err
	}
	attrs := digitalModels[deviceID]
	if attrs == nil {
		attrs = map[string]string{}
	}
	snapshot := buildStateSnapshot(device.Device, device.ApplianceInfo, attrs)
	p.mu.Lock()
	defer p.mu.Unlock()
	if existing, ok := p.devices[device.Device.ID]; ok {
		if !reflect.DeepEqual(existing.CurrentState, snapshot.State) {
			existing.CurrentState = snapshot.State
			existing.LastSnapshotTS = snapshot.TS
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

func (p *Plugin) syncAccountConfig(account *accountRuntime) error {
	if account == nil || account.Client == nil {
		return nil
	}
	refreshToken := account.Client.CurrentRefreshToken()
	if refreshToken == "" {
		return nil
	}

	var snapshot Config
	changed := false
	accountName := account.Config.NormalizedName()

	p.mu.Lock()
	for idx := range p.config.Accounts {
		cfg := p.config.Accounts[idx]
		if cfg.NormalizedName() != accountName {
			continue
		}
		if cfg.RefreshToken != refreshToken {
			cfg.RefreshToken = refreshToken
			changed = true
		}
		if changed {
			p.config.Accounts[idx] = cfg
			account.Config = cfg
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
	if err := coreapi.PersistPluginConfig(persistCtx, "haier", payload); err != nil {
		return fmt.Errorf("persist haier runtime config: %w", err)
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
