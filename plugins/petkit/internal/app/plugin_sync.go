package app

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
)

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
		p.emitSnapshotEventLocked(snapshot)
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
	p.emitSnapshotEventLocked(snapshot)
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
