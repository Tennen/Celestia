package app

import (
	"testing"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

func TestBuildDeviceSyncEventsEmitsUpdateWhenOnlineChanges(t *testing.T) {
	ts := time.Now().UTC()
	deviceID := "haier:washer:primary:abc123"
	previous := map[string]models.Device{
		deviceID: {
			ID:             deviceID,
			PluginID:       "haier",
			VendorDeviceID: "abc123",
			Kind:           models.DeviceKindWasher,
			Name:           "Laundry Room",
			Online:         false,
		},
	}
	current := map[string]*applianceRuntime{
		deviceID: {
			Device: models.Device{
				ID:             deviceID,
				PluginID:       "haier",
				VendorDeviceID: "abc123",
				Kind:           models.DeviceKindWasher,
				Name:           "Laundry Room",
				Online:         true,
			},
			LastSnapshotTS: ts,
		},
	}

	events := buildDeviceSyncEvents(previous, current)
	if len(events) != 1 {
		t.Fatalf("events len = %d, want 1", len(events))
	}
	if events[0].Type != models.EventDeviceUpdated {
		t.Fatalf("event type = %s, want %s", events[0].Type, models.EventDeviceUpdated)
	}
	device, ok := events[0].Payload["device"].(models.Device)
	if !ok {
		t.Fatalf("payload device type = %T, want models.Device", events[0].Payload["device"])
	}
	if !device.Online {
		t.Fatal("payload device online = false, want true")
	}
	changedFields, ok := events[0].Payload["changed_fields"].([]string)
	if !ok {
		t.Fatalf("payload changed_fields type = %T, want []string", events[0].Payload["changed_fields"])
	}
	if len(changedFields) != 1 || changedFields[0] != "online" {
		t.Fatalf("changed_fields = %#v, want [\"online\"]", changedFields)
	}
}
