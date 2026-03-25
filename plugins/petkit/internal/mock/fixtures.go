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
	return []Account{{Name: "pet-parent"}}
}

func SeedDevices(accounts []Account) ([]models.Device, map[string]models.DeviceStateSnapshot) {
	var devices []models.Device
	states := map[string]models.DeviceStateSnapshot{}
	for _, account := range accounts {
		accountPrefix := account.Name
		items := []models.Device{
			{
				ID:             fmt.Sprintf("petkit:feeder:%s", accountPrefix),
				PluginID:       "petkit",
				VendorDeviceID: accountPrefix + "-feeder",
				Kind:           models.DeviceKindPetFeeder,
				Name:           "Petkit Solo Feeder",
				Room:           "Kitchen",
				Online:         true,
				Capabilities:   []string{"feed_once", "get_food_level", "online", "error"},
				Metadata:       map[string]any{"account": account.Name, "transport": "cloud"},
			},
			{
				ID:             fmt.Sprintf("petkit:litter:%s", accountPrefix),
				PluginID:       "petkit",
				VendorDeviceID: accountPrefix + "-litter",
				Kind:           models.DeviceKindPetLitterBox,
				Name:           "Petkit Puramax",
				Room:           "Laundry",
				Online:         true,
				Capabilities:   []string{"clean_now", "pause", "resume", "waste_level", "error", "last_usage"},
				Metadata:       map[string]any{"account": account.Name, "transport": "cloud"},
			},
			{
				ID:             fmt.Sprintf("petkit:fountain:%s", accountPrefix),
				PluginID:       "petkit",
				VendorDeviceID: accountPrefix + "-fountain",
				Kind:           models.DeviceKindPetFountain,
				Name:           "Petkit Eversweet",
				Room:           "Kitchen",
				Online:         true,
				Capabilities:   []string{"online", "water_level", "filter_life", "relay_status"},
				Metadata:       map[string]any{"account": account.Name, "transport": "relay"},
			},
		}
		devices = append(devices, items...)
		for _, device := range items {
			states[device.ID] = initialState(device)
		}
	}
	return devices, states
}

func initialState(device models.Device) models.DeviceStateSnapshot {
	state := map[string]any{}
	switch device.Kind {
	case models.DeviceKindPetFeeder:
		state = map[string]any{"food_level": 78, "online": true, "error": ""}
	case models.DeviceKindPetLitterBox:
		state = map[string]any{"status": "idle", "waste_level": 20, "error": "", "last_usage": time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339)}
	case models.DeviceKindPetFountain:
		state = map[string]any{"online": true, "water_level": 64, "filter_life": 81, "relay_status": "connected"}
	}
	return models.DeviceStateSnapshot{
		DeviceID: device.ID,
		PluginID: device.PluginID,
		TS:       time.Now().UTC(),
		State:    state,
	}
}

