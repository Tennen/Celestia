package app

import (
	"context"
	"log"
	"reflect"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/plugins/haier/internal/client"
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
	deviceIDs := make([]string, 0, len(account.VendorToDeviceID))
	for vendorDeviceID := range account.VendorToDeviceID {
		deviceIDs = append(deviceIDs, vendorDeviceID)
	}
	p.mu.RUnlock()

	if len(deviceIDs) == 0 {
		log.Printf("haier: skip WSS account=%s no vendor devices", account.Config.NormalizedName())
		return
	}

	listener := client.NewWssListener(account.Client, deviceIDs, func(deviceID string, attributes map[string]string) {
		p.applyWSSUpdate(deviceID, attributes)
	})

	p.mu.Lock()
	if account.WSS != nil {
		account.WSS.Stop()
	}
	account.WSS = listener
	p.mu.Unlock()

	log.Printf("haier: starting WSS account=%s vendor_devices=%d", account.Config.NormalizedName(), len(deviceIDs))
	listener.Start(ctx)
}

// applyWSSUpdate merges WebSocket-pushed attributes into the device state and
// emits a state-changed event if anything changed.
func (p *Plugin) applyWSSUpdate(vendorDeviceID string, attributes map[string]string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	deviceID := ""
	for _, account := range p.accounts {
		if resolved, ok := account.VendorToDeviceID[vendorDeviceID]; ok {
			deviceID = resolved
			break
		}
	}
	if deviceID == "" {
		log.Printf("haier: WSS update ignored unknown vendor_device_id=%s", vendorDeviceID)
		return
	}

	device, ok := p.devices[deviceID]
	if !ok {
		log.Printf("haier: WSS update ignored missing device_id=%s vendor_device_id=%s", deviceID, vendorDeviceID)
		return
	}

	// Merge incoming attributes into current state.
	previous := cloneMap(device.CurrentState)
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
			"state":          cloneMap(next),
			"previous_state": previous,
			"source":         "wss_push",
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
		if account.WSS == nil || !account.WSS.IsConnected() {
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
			account.WSS.Stop()
			account.WSS = nil
		}
	}
}
