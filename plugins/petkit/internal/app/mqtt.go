package app

import (
	"context"
	"time"

	"github.com/chentianyu/celestia/plugins/petkit/internal/client"
)

// startMQTTListeners fetches IoT MQTT credentials for each account and starts
// a listener. Failures are non-fatal; polling remains the fallback.
func (p *Plugin) startMQTTListeners(ctx context.Context) {
	p.mu.RLock()
	runtimes := make([]*accountRuntime, 0, len(p.runtimes))
	for _, r := range p.runtimes {
		runtimes = append(runtimes, r)
	}
	p.mu.RUnlock()

	for _, runtime := range runtimes {
		go p.startAccountMQTT(ctx, runtime)
	}
}

func (p *Plugin) startAccountMQTT(ctx context.Context, runtime *accountRuntime) {
	// Ensure we have a valid session before fetching MQTT config.
	if err := runtime.client.EnsureSession(ctx); err != nil {
		return
	}

	cfg, err := runtime.client.FetchIoTMQTTConfig(ctx)
	if err != nil {
		// MQTT config unavailable for this account; polling will cover it.
		return
	}

	accountName := client.AccountKey(runtime.cfg)
	listener := client.NewMQTTListener(cfg, accountName, func() {
		// Triggered on every MQTT message: refresh this account's devices.
		refreshCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		snapshots, err := runtime.client.Sync(refreshCtx)
		if err != nil {
			return
		}
		if err := p.syncAccountSession(runtime.cfg.Name, runtime.client); err != nil {
			return
		}
		p.applyAccountSnapshots(runtime.cfg, snapshots)
	})

	if err := listener.Start(ctx); err != nil {
		// Connection failed; polling remains active.
		return
	}

	p.mu.Lock()
	if r, ok := p.runtimes[accountName]; ok {
		if r.mqtt != nil {
			r.mqtt.Stop()
		}
		r.mqtt = listener
	}
	p.mu.Unlock()
}

// allMQTTConnected returns true when every account has an active MQTT connection.
func (p *Plugin) allMQTTConnected() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if len(p.runtimes) == 0 {
		return false
	}
	for _, r := range p.runtimes {
		if r.mqtt == nil || !r.mqtt.IsConnected() {
			return false
		}
	}
	return true
}
