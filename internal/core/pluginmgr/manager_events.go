package pluginmgr

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/pluginapi"
	"github.com/chentianyu/celestia/internal/storage"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (m *Manager) syncDevices(ctx context.Context, runtime *managedPlugin) error {
	list, err := runtime.client.DiscoverDevices(ctx, &emptypb.Empty{})
	if err != nil {
		return err
	}
	var devices []models.Device
	if err := pluginapi.DecodeList(list, &devices); err != nil {
		return err
	}
	existing, err := m.registry.List(ctx, storage.DeviceFilter{PluginID: runtime.record.PluginID})
	if err != nil {
		return err
	}
	if err := m.registry.Upsert(ctx, devices); err != nil {
		return err
	}
	if removedIDs := missingDeviceIDs(existing, devices); len(removedIDs) > 0 {
		if err := m.registry.DeleteIDs(ctx, removedIDs); err != nil {
			return err
		}
	}
	for _, device := range devices {
		event := models.Event{
			ID:       uuid.NewString(),
			Type:     models.EventDeviceDiscovered,
			PluginID: device.PluginID,
			DeviceID: device.ID,
			TS:       time.Now().UTC(),
			Payload: map[string]any{
				"device": device,
			},
		}
		_ = m.store.AppendEvent(context.Background(), event)
		m.bus.Publish(event)
		statePayload, err := pluginapi.EncodeStruct(map[string]any{"device_id": device.ID})
		if err != nil {
			return err
		}
		stateStruct, err := runtime.client.GetDeviceState(ctx, statePayload)
		if err != nil {
			return err
		}
		var snapshot models.DeviceStateSnapshot
		if err := pluginapi.DecodeStruct(stateStruct, &snapshot); err != nil {
			return err
		}
		if snapshot.TS.IsZero() {
			snapshot.TS = time.Now().UTC()
		}
		if snapshot.PluginID == "" {
			snapshot.PluginID = device.PluginID
		}
		if err := m.state.Upsert(ctx, []models.DeviceStateSnapshot{snapshot}); err != nil {
			return err
		}
	}
	return nil
}

func missingDeviceIDs(existing, discovered []models.Device) []string {
	seen := make(map[string]struct{}, len(discovered))
	for _, device := range discovered {
		seen[device.ID] = struct{}{}
	}
	out := make([]string, 0, len(existing))
	for _, device := range existing {
		if _, ok := seen[device.ID]; !ok {
			out = append(out, device.ID)
		}
	}
	return out
}

func (m *Manager) consumeEvents(ctx context.Context, runtime *managedPlugin) {
	stream, err := runtime.client.StreamEvents(ctx, &emptypb.Empty{})
	if err != nil {
		runtime.lastError = err.Error()
		return
	}
	for {
		msg, err := stream.Recv()
		if err != nil {
			if ctx.Err() == nil {
				runtime.lastError = err.Error()
			}
			return
		}
		var event models.Event
		if err := pluginapi.DecodeStruct(msg, &event); err != nil {
			runtime.logs.Append("stream decode error: " + err.Error())
			continue
		}
		if event.ID == "" {
			event.ID = uuid.NewString()
		}
		if event.TS.IsZero() {
			event.TS = time.Now().UTC()
		}
		if event.PluginID == "" {
			event.PluginID = runtime.record.PluginID
		}
		if err := m.store.AppendEvent(context.Background(), event); err != nil {
			runtime.logs.Append("persist event error: " + err.Error())
		}
		if statePayload, ok := event.Payload["state"].(map[string]any); ok && event.DeviceID != "" {
			_ = m.state.Upsert(context.Background(), []models.DeviceStateSnapshot{{
				DeviceID: event.DeviceID,
				PluginID: event.PluginID,
				TS:       event.TS,
				State:    statePayload,
			}})
		}
		if healthValue, ok := event.Payload["health_status"].(string); ok {
			runtime.health.Status = models.HealthState(healthValue)
			runtime.health.CheckedAt = time.Now().UTC()
			runtime.health.Message, _ = event.Payload["message"].(string)
		}
		m.bus.Publish(event)
	}
}

func (m *Manager) healthLoop(runtime *managedPlugin) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if !runtime.running || runtime.client == nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		resp, err := runtime.client.HealthCheck(ctx, &emptypb.Empty{})
		cancel()
		if err != nil {
			runtime.lastError = err.Error()
			runtime.health.Status = models.HealthStateUnhealthy
			runtime.health.Message = err.Error()
			continue
		}
		var health models.PluginHealth
		if err := pluginapi.DecodeStruct(resp, &health); err != nil {
			runtime.lastError = err.Error()
			continue
		}
		health.ProcessPID = runtime.pid
		runtime.health = health
		now := time.Now().UTC()
		runtime.record.LastHealthStatus = health.Status
		runtime.record.LastHeartbeatAt = &now
		runtime.record.UpdatedAt = now
		_ = m.store.UpsertPluginRecord(context.Background(), runtime.record)
	}
}

func (m *Manager) watchExit(pluginID string, runtime *managedPlugin) {
	err := runtime.cmd.Wait()
	if runtime.conn != nil {
		_ = runtime.conn.Close()
	}
	runtime.running = false
	if err != nil && !errors.Is(err, context.Canceled) {
		runtime.lastError = err.Error()
		runtime.health = models.PluginHealth{
			PluginID:   pluginID,
			Status:     models.HealthStateUnhealthy,
			Message:    err.Error(),
			CheckedAt:  time.Now().UTC(),
			ProcessPID: runtime.pid,
		}
		now := time.Now().UTC()
		runtime.record.LastHealthStatus = models.HealthStateUnhealthy
		runtime.record.UpdatedAt = now
		runtime.record.LastHeartbeatAt = &now
		_ = m.store.UpsertPluginRecord(context.Background(), runtime.record)
		m.publishLifecycle(pluginID, "crashed")
		if !runtime.stoppedByManager && runtime.record.Status == models.PluginStatusEnabled {
			go func() {
				time.Sleep(3 * time.Second)
				if restartErr := m.Enable(context.Background(), pluginID); restartErr != nil {
					runtime.logs.Append("restart failed: " + restartErr.Error())
				}
			}()
		}
	}
}

func (m *Manager) setRuntimeError(pluginID string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	runtime := m.runtimes[pluginID]
	if runtime == nil {
		runtime = &managedPlugin{logs: newLogBuffer(200)}
		m.runtimes[pluginID] = runtime
	}
	runtime.lastError = err.Error()
}

func (m *Manager) publishLifecycle(pluginID, state string) {
	event := models.Event{
		ID:       uuid.NewString(),
		Type:     models.EventPluginLifecycleState,
		PluginID: pluginID,
		TS:       time.Now().UTC(),
		Payload: map[string]any{
			"state": state,
		},
	}
	_ = m.store.AppendEvent(context.Background(), event)
	m.bus.Publish(event)
}

func consumeLogs(buffer *logBuffer, pluginID, stream string, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		buffer.Append(fmt.Sprintf("[%s][%s] %s", pluginID, stream, scanner.Text()))
	}
}
