package app

import (
	"context"
	"errors"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/pluginruntime"
	"github.com/chentianyu/celestia/plugins/petkit/internal/client"
	"github.com/google/uuid"
)

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

	// Start MQTT listeners for each account (best-effort; polling is the fallback).
	p.startMQTTListeners(runCtx)

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
	// Stop all MQTT listeners.
	for _, runtime := range p.runtimes {
		if runtime.mqtt != nil {
			runtime.mqtt.Stop()
			runtime.mqtt = nil
		}
	}
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
		// Adjust next tick based on MQTT connectivity.
		next := interval
		if p.allMQTTConnected() {
			// MQTT is pushing updates; poll infrequently as a fallback.
			next = 5 * time.Minute
		}
		ticker.Reset(next)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// accountKey returns a stable key for an account config.
func accountKey(cfg client.AccountConfig) string {
	return client.AccountKey(cfg)
}
