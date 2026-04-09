package app

import (
	"log"
	"reflect"
	"sort"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
)

func (p *Plugin) deviceMapSnapshot() map[string]models.Device {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make(map[string]models.Device, len(p.devices))
	for _, device := range p.devices {
		out[device.Device.ID] = device.Device
	}
	return out
}

func buildDeviceSyncEvents(previous map[string]models.Device, current map[string]*applianceRuntime) []models.Event {
	deviceIDs := make([]string, 0, len(current))
	for deviceID := range current {
		deviceIDs = append(deviceIDs, deviceID)
	}
	sort.Strings(deviceIDs)

	events := make([]models.Event, 0, len(deviceIDs))
	for _, deviceID := range deviceIDs {
		runtime := current[deviceID]
		device := runtime.Device
		prev, ok := previous[deviceID]
		if !ok {
			log.Printf("haier: poll discovered device=%s vendor_device_id=%s", deviceID, device.VendorDeviceID)
			events = append(events, models.Event{
				ID:       uuid.NewString(),
				Type:     models.EventDeviceDiscovered,
				PluginID: device.PluginID,
				DeviceID: device.ID,
				TS:       runtime.LastSnapshotTS,
				Payload: map[string]any{
					"device": device,
					"source": "poll_refresh",
				},
			})
			continue
		}
		if reflect.DeepEqual(prev, device) {
			continue
		}
		changedFields := changedDeviceFields(prev, device)
		log.Printf("haier: poll device synced device=%s changed_fields=%v", deviceID, changedFields)
		events = append(events, models.Event{
			ID:       uuid.NewString(),
			Type:     models.EventDeviceUpdated,
			PluginID: device.PluginID,
			DeviceID: device.ID,
			TS:       runtime.LastSnapshotTS,
			Payload: map[string]any{
				"device":          device,
				"previous_device": prev,
				"changed_fields":  changedFields,
				"source":          "poll_refresh",
			},
		})
	}
	return events
}

func changedDeviceFields(previous models.Device, current models.Device) []string {
	fields := make([]string, 0, 8)
	if previous.PluginID != current.PluginID {
		fields = append(fields, "plugin_id")
	}
	if previous.VendorDeviceID != current.VendorDeviceID {
		fields = append(fields, "vendor_device_id")
	}
	if previous.Kind != current.Kind {
		fields = append(fields, "kind")
	}
	if previous.Name != current.Name {
		fields = append(fields, "name")
	}
	if previous.DefaultName != current.DefaultName {
		fields = append(fields, "default_name")
	}
	if previous.Alias != current.Alias {
		fields = append(fields, "alias")
	}
	if previous.Room != current.Room {
		fields = append(fields, "room")
	}
	if previous.Online != current.Online {
		fields = append(fields, "online")
	}
	if !reflect.DeepEqual(previous.Capabilities, current.Capabilities) {
		fields = append(fields, "capabilities")
	}
	if !reflect.DeepEqual(previous.Metadata, current.Metadata) {
		fields = append(fields, "metadata")
	}
	return fields
}
