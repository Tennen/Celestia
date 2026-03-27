package app

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/plugins/xiaomi/internal/mapper"
	"github.com/google/uuid"
)

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
		if syncErr := p.syncAccountSessionConfig(account.cfg.Name, account.client); syncErr != nil {
			errs = append(errs, syncErr.Error())
		}
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
	if err := p.syncAccountSessionConfig(runtime.accountName, runtime.account.client); err != nil {
		return err
	}
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

func (p *Plugin) emit(event models.Event) {
	select {
	case p.events <- event:
	default:
	}
}
