package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

func buildDevice(account AccountConfig, appliance map[string]any, commandNames map[string]string, capabilitySet map[string]bool) models.Device {
	capabilities := []string{}
	for _, name := range []string{
		"start", "stop", "pause", "resume", "remaining_time", "program", "phase", "machine_status",
		"delay_time", "temp_level", "spin_speed", "prewash", "extra_rinse", "good_night", "energy_usage", "water_usage",
	} {
		if capabilitySet[name] {
			capabilities = append(capabilities, name)
		}
	}
	mac := strings.ToLower(stringFromAny(appliance["macAddress"]))
	device := models.Device{
		ID:             fmt.Sprintf("haier:washer:%s:%s", strings.ToLower(account.normalizedName()), sanitizeID(mac)),
		PluginID:       "haier",
		VendorDeviceID: stringFromAny(appliance["macAddress"]),
		Kind:           models.DeviceKindWasher,
		Name: firstNonEmpty(
			stringFromAny(appliance["nickName"]),
			stringFromAny(appliance["modelName"]),
			stringFromAny(appliance["brand"]),
			stringFromAny(appliance["applianceTypeName"]),
		),
		Room:         stringFromAny(appliance["roomName"]),
		Online:       applianceOnline(appliance),
		Capabilities: capabilities,
		Metadata: map[string]any{
			"account":            account.normalizedName(),
			"mobile_id":          account.normalizedMobileID(),
			"timezone":           account.normalizedTimezone(),
			"appliance_type":     appliance["applianceTypeName"],
			"appliance_model_id": appliance["applianceModelId"],
			"brand":              appliance["brand"],
			"code":               appliance["code"],
			"mac_address":        appliance["macAddress"],
			"capability_matrix":  capabilitySet,
			"command_names":      commandNames,
		},
	}
	return device
}

func buildStateSnapshot(device models.Device, appliance map[string]any, raw map[string]any) models.DeviceStateSnapshot {
	normalized := map[string]any{}
	parameters := extractParameters(raw)
	for k, v := range parameters {
		normalized[k] = v
	}
	normalized["parameters"] = parameters
	if len(raw) > 0 {
		normalized["raw"] = raw
	}
	if stats, ok := raw["statistics"].(map[string]any); ok {
		normalized["statistics"] = stats
	}
	if maintenance, ok := raw["maintenance"].(map[string]any); ok {
		normalized["maintenance"] = maintenance
	}
	if status := stringFromAny(parameters["machMode"]); status != "" {
		switch status {
		case "3":
			normalized["machine_status"] = "paused"
		case "0":
			normalized["machine_status"] = "idle"
		default:
			normalized["machine_status"] = "running"
		}
	}
	if normalized["machine_status"] == nil {
		if active, ok := raw["active"].(bool); ok && active {
			normalized["machine_status"] = "running"
		} else {
			normalized["machine_status"] = "idle"
		}
	}
	if program := stringFromAny(raw["programName"]); program != "" {
		normalized["program"] = program
	} else if v := stringFromAny(parameters["prCode"]); v != "" {
		normalized["program"] = v
	}
	if phase := stringFromAny(parameters["prPhase"]); phase != "" {
		normalized["phase"] = phase
	}
	if remaining := intFromAny(parameters["remainingTimeMM"]); remaining >= 0 {
		normalized["remaining_minutes"] = remaining
	}
	if temp := intFromAny(parameters["tempLevel"]); temp > 0 {
		normalized["temperature"] = temp
	}
	if spin := intFromAny(parameters["spinSpeed"]); spin > 0 {
		normalized["spin_speed"] = spin
	}
	if delay := intFromAny(parameters["delayTime"]); delay >= 0 {
		normalized["delay_time"] = delay
	}
	if prewash, ok := parameters["prewash"].(bool); ok {
		normalized["prewash"] = prewash
	}
	if rinse := intFromAny(parameters["extraRinse"]); rinse >= 0 {
		normalized["extra_rinse"] = rinse
	}
	if gn := intFromAny(parameters["goodNight"]); gn >= 0 {
		normalized["good_night"] = gn
	}
	if electricity := floatFromAny(parameters["totalElectricityUsed"]); electricity >= 0 {
		normalized["total_electricity_used"] = electricity
	}
	if water := floatFromAny(parameters["totalWaterUsed"]); water >= 0 {
		normalized["total_water_used"] = water
	}
	if cycles := intFromAny(parameters["totalWashCycle"]); cycles >= 0 {
		normalized["total_wash_cycle"] = cycles
	}
	return models.DeviceStateSnapshot{
		DeviceID: device.ID,
		PluginID: device.PluginID,
		TS:       time.Now().UTC(),
		State:    normalized,
	}
}
