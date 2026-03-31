package app

import (
	"context"
	"reflect"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
)

// startWSSListeners starts a WebSocket listener for each account after the
// initial device discovery. Called from Start after the first refreshAll.
func (p *Plugin) startWSSListeners(ctx context.Context) {
	runtimes := p.accountRuntimes()
	for _, account := range runtimes {
		go p.startAccountWSS(ctx, account)
	}
}

func (p *Plugin) startAccountWSS(ctx context.Context, account *accountRuntime) {
	p.mu.RLock()
	deviceIDs := make([]string, 0, len(account.Appliances))
	for id := range account.Appliances {
		deviceIDs = append(deviceIDs, id)
	}
	p.mu.RUnlock()

	if len(deviceIDs) == 0 {
		return
	}

	listener := newWSSListener(account.Client, deviceIDs, func(deviceID string, attributes map[string]string) {
		p.applyWSSUpdate(deviceID, attributes)
	})

	p.mu.Lock()
	if account.WSS != nil {
		account.WSS.stop()
	}
	account.WSS = listener
	p.mu.Unlock()

	listener.start(ctx)
}

// applyWSSUpdate merges WebSocket-pushed attributes into the device state and
// emits a state-changed event if anything changed.
func (p *Plugin) applyWSSUpdate(deviceID string, attributes map[string]string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	device, ok := p.devices[deviceID]
	if !ok {
		return
	}

	// Merge incoming attributes into current state.
	next := cloneMap(device.CurrentState)
	for k, v := range attributes {
		next[k] = v
	}

	if reflect.DeepEqual(device.CurrentState, next) {
		return
	}

	device.CurrentState = next
	device.LastSnapshotTS = time.Now().UTC()

	p.emitLocked(models.Event{
		ID:       uuid.NewString(),
		Type:     models.EventDeviceStateChanged,
		PluginID: "haier",
		DeviceID: deviceID,
		TS:       device.LastSnapshotTS,
		Payload: map[string]any{
			"state":  cloneMap(next),
			"source": "wss_push",
		},
	})
}

// allWSSConnected returns true when every account has an active WSS connection.
func (p *Plugin) allWSSConnected() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if len(p.accounts) == 0 {
		return false
	}
	for _, account := range p.accounts {
		if account.WSS == nil || !account.WSS.isConnected() {
			return false
		}
	}
	return true
}

// stopWSSListeners stops all active WebSocket listeners.
func (p *Plugin) stopWSSListeners() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, account := range p.accounts {
		if account.WSS != nil {
			account.WSS.stop()
			account.WSS = nil
		}
	}
}
