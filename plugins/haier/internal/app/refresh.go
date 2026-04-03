package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
	previousDevices := p.deviceSnapshotMap()
	nextDevices := map[string]*applianceRuntime{}
	var firstErr error
	successes := 0
	for _, account := range runtimes {
		if err := p.refreshAccount(ctx, account, nextDevices); err != nil {
			account.LastError = err.Error()
			log.Printf("haier: refresh account=%s failed: %v", account.Config.NormalizedName(), err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		successes++
		account.LastError = ""
		account.LastSync = time.Now().UTC()
	}
	events := buildStateChangeEvents(previousDevices, nextDevices)
	p.mu.Lock()
	p.devices = nextDevices
	p.lastSyncAt = time.Now().UTC()
	if firstErr != nil {
		p.lastError = firstErr.Error()
	} else {
		p.lastError = ""
	}
	for _, event := range events {
		p.emitLocked(event)
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
	accountName := account.Config.NormalizedName()
	if err := account.Client.Authenticate(ctx); err != nil {
		return fmt.Errorf("authenticate account %s: %w", accountName, err)
	}
	if err := p.syncAccountConfig(account); err != nil {
		return fmt.Errorf("persist account %s runtime config: %w", accountName, err)
	}
	appliances, err := account.Client.LoadAppliances(ctx)
	if err != nil {
		return fmt.Errorf("load appliances for %s: %w", accountName, err)
	}

	// Collect device IDs for batch digital model fetch.
	deviceIDs := make([]string, 0, len(appliances))
	for _, appliance := range appliances {
		if id := client.StringFromAny(appliance["deviceId"]); id != "" {
			deviceIDs = append(deviceIDs, id)
		}
	}

	digitalModels := map[string]client.DigitalModel{}
	if len(deviceIDs) > 0 {
		if details, err := account.Client.LoadDigitalModelDetails(ctx, deviceIDs); err == nil {
			digitalModels = details
		}
	}

	current := map[string]*applianceRuntime{}
	vendorIndex := map[string]string{}
	for _, appliance := range appliances {
		deviceID := client.StringFromAny(appliance["deviceId"])
		if deviceID == "" {
			continue
		}
		digitalModel := digitalModels[deviceID]
		attrs := digitalModel.Values()
		commandNames, capabilitySet := buildCapabilitiesFromDigitalModel(attrs)
		stateDescriptors := buildStateDescriptors(digitalModel)
		device := buildDevice(account.Config, appliance, commandNames, capabilitySet, stateDescriptors)
		snapshot := buildStateSnapshot(device, appliance, attrs)
		runtime := &applianceRuntime{
			Device:           device,
			ApplianceInfo:    appliance,
			CapabilitySet:    capabilitySet,
			CommandNames:     commandNames,
			StateDescriptors: stateDescriptors,
			CurrentState:     snapshot.State,
			LastSnapshotTS:   snapshot.TS,
		}
		current[device.ID] = runtime
		vendorIndex[deviceID] = device.ID
		nextDevices[device.ID] = runtime
	}
	account.Appliances = current
	account.VendorToDeviceID = vendorIndex
	if account.WSS != nil {
		account.WSS.UpdateDevices(deviceIDs)
	}
	log.Printf("haier: account=%s appliances=%d digital_models=%d", accountName, len(appliances), len(digitalModels))
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
	digitalModels, err := account.Client.LoadDigitalModelDetails(ctx, []string{deviceID})
	if err != nil {
		return err
	}
	digitalModel := digitalModels[deviceID]
	attrs := digitalModel.Values()
	stateDescriptors := buildStateDescriptors(digitalModel)
	snapshot := buildStateSnapshot(device.Device, device.ApplianceInfo, attrs)
	p.mu.Lock()
	defer p.mu.Unlock()
	if existing, ok := p.devices[device.Device.ID]; ok {
		if len(stateDescriptors) > 0 {
			if existing.Device.Metadata == nil {
				existing.Device.Metadata = map[string]any{}
			}
			existing.Device.Metadata["state_descriptors"] = stateDescriptors
			existing.StateDescriptors = stateDescriptors
		}
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

func (p *Plugin) deviceSnapshotMap() map[string]models.DeviceStateSnapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make(map[string]models.DeviceStateSnapshot, len(p.devices))
	for _, device := range p.devices {
		out[device.Device.ID] = models.DeviceStateSnapshot{
			DeviceID: device.Device.ID,
			PluginID: device.Device.PluginID,
			TS:       device.LastSnapshotTS,
			State:    cloneMap(device.CurrentState),
		}
	}
	return out
}

func buildStateChangeEvents(previous map[string]models.DeviceStateSnapshot, current map[string]*applianceRuntime) []models.Event {
	events := make([]models.Event, 0, len(current))
	for deviceID, runtime := range current {
		snapshot := models.DeviceStateSnapshot{
			DeviceID: runtime.Device.ID,
			PluginID: runtime.Device.PluginID,
			TS:       runtime.LastSnapshotTS,
			State:    cloneMap(runtime.CurrentState),
		}
		prev, ok := previous[deviceID]
		if !ok || reflect.DeepEqual(prev.State, snapshot.State) {
			continue
		}
		events = append(events, models.Event{
			ID:       uuid.NewString(),
			Type:     models.EventDeviceStateChanged,
			PluginID: "haier",
			DeviceID: deviceID,
			TS:       snapshot.TS,
			Payload: map[string]any{
				"state":          snapshot.State,
				"previous_state": cloneMap(prev.State),
				"source":         "poll_refresh",
			},
		})
	}
	return events
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
