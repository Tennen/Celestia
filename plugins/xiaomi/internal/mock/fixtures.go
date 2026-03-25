package mock

import (
	"fmt"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

type Account struct {
	Name   string `json:"name"`
	Region string `json:"region"`
}

func DefaultAccounts() []Account {
	return []Account{
		{Name: "demo-cn", Region: "cn"},
		{Name: "demo-us", Region: "us"},
	}
}

func SeedDevices(accounts []Account) ([]models.Device, map[string]models.DeviceStateSnapshot) {
	var devices []models.Device
	states := map[string]models.DeviceStateSnapshot{}
	for _, account := range accounts {
		region := strings.ToLower(account.Region)
		devicesForAccount := []models.Device{
			{
				ID:             fmt.Sprintf("xiaomi:%s:%s-light", region, account.Name),
				PluginID:       "xiaomi",
				VendorDeviceID: fmt.Sprintf("%s-light", account.Name),
				Kind:           models.DeviceKindLight,
				Name:           fmt.Sprintf("%s Main Light", strings.ToUpper(region)),
				Room:           "Living Room",
				Online:         true,
				Capabilities:   []string{"power", "brightness", "color_temp"},
				Metadata:       map[string]any{"account": account.Name, "region": region, "miot_type": "light"},
			},
			{
				ID:             fmt.Sprintf("xiaomi:%s:%s-switch", region, account.Name),
				PluginID:       "xiaomi",
				VendorDeviceID: fmt.Sprintf("%s-switch", account.Name),
				Kind:           models.DeviceKindSwitch,
				Name:           fmt.Sprintf("%s Smart Plug", strings.ToUpper(region)),
				Room:           "Study",
				Online:         true,
				Capabilities:   []string{"power"},
				Metadata:       map[string]any{"account": account.Name, "region": region, "miot_type": "switch"},
			},
			{
				ID:             fmt.Sprintf("xiaomi:%s:%s-sensor", region, account.Name),
				PluginID:       "xiaomi",
				VendorDeviceID: fmt.Sprintf("%s-sensor", account.Name),
				Kind:           models.DeviceKindSensor,
				Name:           fmt.Sprintf("%s Temperature Sensor", strings.ToUpper(region)),
				Room:           "Bedroom",
				Online:         true,
				Capabilities:   []string{"temperature", "humidity"},
				Metadata:       map[string]any{"account": account.Name, "region": region, "miot_type": "sensor"},
			},
			{
				ID:             fmt.Sprintf("xiaomi:%s:%s-climate", region, account.Name),
				PluginID:       "xiaomi",
				VendorDeviceID: fmt.Sprintf("%s-climate", account.Name),
				Kind:           models.DeviceKindClimate,
				Name:           fmt.Sprintf("%s Bedroom AC", strings.ToUpper(region)),
				Room:           "Bedroom",
				Online:         true,
				Capabilities:   []string{"power", "target_temperature", "mode", "fan_speed"},
				Metadata:       map[string]any{"account": account.Name, "region": region, "miot_type": "climate"},
			},
		}
		devices = append(devices, devicesForAccount...)
		for _, device := range devicesForAccount {
			states[device.ID] = initialState(device)
		}
	}
	return devices, states
}

func initialState(device models.Device) models.DeviceStateSnapshot {
	state := map[string]any{}
	switch device.Kind {
	case models.DeviceKindLight:
		state = map[string]any{"power": true, "brightness": 72, "color_temp": 4200}
	case models.DeviceKindSwitch:
		state = map[string]any{"power": false}
	case models.DeviceKindSensor:
		state = map[string]any{"temperature": 24.1, "humidity": 49}
	case models.DeviceKindClimate:
		state = map[string]any{"power": true, "target_temperature": 25, "mode": "cool", "fan_speed": "auto"}
	}
	return models.DeviceStateSnapshot{
		DeviceID: device.ID,
		PluginID: device.PluginID,
		TS:       time.Now().UTC(),
		State:    state,
	}
}

