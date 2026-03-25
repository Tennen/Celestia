package mock

import (
	"fmt"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

type Account struct {
	Name string `json:"name"`
}

func DefaultAccounts() []Account {
	return []Account{{Name: "hon-home"}}
}

func SeedDevices(accounts []Account) ([]models.Device, map[string]models.DeviceStateSnapshot) {
	var devices []models.Device
	states := map[string]models.DeviceStateSnapshot{}
	for _, account := range accounts {
		base := []models.Device{
			{
				ID:             fmt.Sprintf("haier:washer:%s-alpha", account.Name),
				PluginID:       "haier",
				VendorDeviceID: account.Name + "-alpha",
				Kind:           models.DeviceKindWasher,
				Name:           "Haier I-Pro Alpha",
				Room:           "Laundry",
				Online:         true,
				Capabilities:   []string{"start", "stop", "pause", "resume", "machine_status", "program", "phase", "remaining_time", "delay_time", "temp_level", "spin_speed", "prewash"},
				Metadata:       map[string]any{"account": account.Name, "model": "alpha-100", "capability_tier": "L2"},
			},
			{
				ID:             fmt.Sprintf("haier:washer:%s-beta", account.Name),
				PluginID:       "haier",
				VendorDeviceID: account.Name + "-beta",
				Kind:           models.DeviceKindWasher,
				Name:           "Hoover X Beta",
				Room:           "Laundry",
				Online:         true,
				Capabilities:   []string{"start", "stop", "pause", "resume", "machine_status", "program", "phase", "remaining_time"},
				Metadata:       map[string]any{"account": account.Name, "model": "beta-40", "capability_tier": "L1"},
			},
		}
		devices = append(devices, base...)
		for _, device := range base {
			states[device.ID] = models.DeviceStateSnapshot{
				DeviceID: device.ID,
				PluginID: device.PluginID,
				TS:       time.Now().UTC(),
				State: map[string]any{
					"machine_status":    "idle",
					"program":           "cotton",
					"phase":             "ready",
					"remaining_minutes": 0,
					"temperature":       40,
					"spin_speed":        1000,
					"delay_time":        0,
					"prewash":           false,
				},
			}
		}
	}
	return devices, states
}

