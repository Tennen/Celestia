package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/plugins/haier/internal/client"
)

// buildCapabilitiesFromDigitalModel infers the capability set from the digital model
// attribute keys. Returns commandNames (action → attribute key) and capabilitySet.
func buildCapabilitiesFromDigitalModel(attrs map[string]string) (map[string]string, map[string]bool) {
	commandNames := map[string]string{}
	capabilitySet := map[string]bool{}

	has := func(key string) bool {
		_, ok := attrs[key]
		return ok
	}

	// Writable control attributes imply command capabilities.
	if has("machMode") {
		commandNames["start"] = "machMode"
		commandNames["stop"] = "machMode"
		commandNames["pause"] = "machMode"
		commandNames["resume"] = "machMode"
		capabilitySet["start"] = true
		capabilitySet["stop"] = true
		capabilitySet["pause"] = true
		capabilitySet["resume"] = true
		capabilitySet["machine_status"] = true
	}
	if has("prCode") {
		commandNames["program"] = "prCode"
		capabilitySet["program"] = true
	}
	if has("prPhase") {
		capabilitySet["phase"] = true
	}
	if has("remainingTimeMM") {
		capabilitySet["remaining_time"] = true
	}
	if has("tempLevel") {
		commandNames["temp_level"] = "tempLevel"
		capabilitySet["temp_level"] = true
	}
	if has("spinSpeed") {
		commandNames["spin_speed"] = "spinSpeed"
		capabilitySet["spin_speed"] = true
	}
	if has("delayTime") {
		commandNames["delay_time"] = "delayTime"
		capabilitySet["delay_time"] = true
	}
	if has("prewash") {
		commandNames["prewash"] = "prewash"
		capabilitySet["prewash"] = true
	}
	if has("extraRinse") {
		commandNames["extra_rinse"] = "extraRinse"
		capabilitySet["extra_rinse"] = true
	}
	if has("goodNight") {
		commandNames["good_night"] = "goodNight"
		capabilitySet["good_night"] = true
	}
	if has("totalElectricityUsed") {
		capabilitySet["energy_usage"] = true
	}
	if has("totalWaterUsed") {
		capabilitySet["water_usage"] = true
	}
	return commandNames, capabilitySet
}

func buildDevice(
	account client.AccountConfig,
	appliance map[string]any,
	commandNames map[string]string,
	capabilitySet map[string]bool,
	stateDescriptors map[string]models.DeviceStateDescriptor,
) models.Device {
	capabilities := []string{}
	for _, name := range []string{
		"start", "stop", "pause", "resume", "remaining_time", "program", "phase", "machine_status",
		"delay_time", "temp_level", "spin_speed", "prewash", "extra_rinse", "good_night", "energy_usage", "water_usage",
	} {
		if capabilitySet[name] {
			capabilities = append(capabilities, name)
		}
	}
	deviceID := client.StringFromAny(appliance["deviceId"])
	metadata := map[string]any{
		"account":           account.NormalizedName(),
		"device_type":       appliance["deviceType"],
		"capability_matrix": capabilitySet,
		"command_names":     commandNames,
	}
	if len(stateDescriptors) > 0 {
		metadata["state_descriptors"] = stateDescriptors
	}
	if controls := buildControlSpecs(capabilitySet); len(controls) > 0 {
		metadata["controls"] = controls
	}
	return models.Device{
		ID:             fmt.Sprintf("haier:washer:%s:%s", strings.ToLower(account.NormalizedName()), sanitizeID(deviceID)),
		PluginID:       "haier",
		VendorDeviceID: deviceID,
		Kind:           models.DeviceKindWasher,
		Name: firstNonEmpty(
			client.StringFromAny(appliance["deviceName"]),
			client.StringFromAny(appliance["deviceType"]),
		),
		Online:       applianceOnline(appliance),
		Capabilities: capabilities,
		Metadata:     metadata,
	}
}

// buildStateSnapshot builds a unified state snapshot from UWS digital model attributes.
// attrs is a map[string]string of attribute name → value from LoadDigitalModels.
func buildStateSnapshot(device models.Device, appliance map[string]any, attrs map[string]string) models.DeviceStateSnapshot {
	normalized := normalizeHaierState(attrs)

	return models.DeviceStateSnapshot{
		DeviceID: device.ID,
		PluginID: device.PluginID,
		TS:       time.Now().UTC(),
		State:    normalized,
	}
}
