package app

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

func parseFamily(value any) (petkitFamily, bool) {
	entry, ok := value.(map[string]any)
	if !ok {
		return petkitFamily{}, false
	}
	family := petkitFamily{}
	if rawDevices, ok := entry["deviceList"].([]any); ok {
		for _, raw := range rawDevices {
			device, ok := parseDeviceRecord(raw)
			if !ok {
				continue
			}
			family.DeviceList = append(family.DeviceList, device)
		}
	}
	if rawPets, ok := entry["petList"].([]any); ok {
		for _, raw := range rawPets {
			pet, ok := parsePetRecord(raw)
			if !ok {
				continue
			}
			family.PetList = append(family.PetList, pet)
		}
	}
	return family, true
}

func parseDeviceRecord(value any) (petkitDeviceInfo, bool) {
	entry, ok := value.(map[string]any)
	if !ok {
		return petkitDeviceInfo{}, false
	}
	deviceID := intFromAny(entry["deviceId"], 0)
	deviceType := strings.ToLower(stringFromAny(entry["deviceType"], ""))
	groupID := intFromAny(entry["groupId"], 0)
	typeCode := intFromAny(entry["typeCode"], 0)
	uniqueID := stringFromAny(entry["uniqueId"], "")
	if uniqueID == "" {
		uniqueID = strings.ToLower(fmt.Sprintf("%s-%d", deviceType, deviceID))
	}
	return petkitDeviceInfo{
		DeviceID:   deviceID,
		DeviceType: deviceType,
		GroupID:    groupID,
		TypeCode:   typeCode,
		UniqueID:   uniqueID,
		DeviceName: stringFromAny(entry["deviceName"], ""),
		CreatedAt:  intFromAny(entry["createdAt"], 0),
	}, true
}

func parsePetRecord(value any) (petkitPetInfo, bool) {
	entry, ok := value.(map[string]any)
	if !ok {
		return petkitPetInfo{}, false
	}
	return petkitPetInfo{
		ID:   intFromAny(entry["petId"], 0),
		Name: stringFromAny(entry["petName"], ""),
		SN:   stringFromAny(entry["sn"], ""),
	}, true
}

func buildDeviceInfo(info petkitDeviceInfo) (petkitDeviceInfo, bool) {
	if info.DeviceID == 0 || info.DeviceType == "" {
		return petkitDeviceInfo{}, false
	}
	return info, true
}

func buildDevice(info petkitDeviceInfo, kind models.DeviceKind, detail map[string]any, records map[string]any, accountLabel string) models.Device {
	name := info.DeviceName
	if name == "" {
		name = strings.Title(strings.ReplaceAll(info.DeviceType, "_", " "))
	}
	state := buildState(info, kind, detail, records)
	caps := capabilitiesForDevice(info, kind, state)
	metadata := map[string]any{
		"account":     accountLabel,
		"group_id":    info.GroupID,
		"device_type": info.DeviceType,
		"type_code":   info.TypeCode,
		"unique_id":   info.UniqueID,
		"created_at":  info.CreatedAt,
		"source":      "petkit-cloud",
	}
	if controls := controlSpecsForDevice(info, kind); len(controls) > 0 {
		metadata["controls"] = controls
	}
	return models.Device{
		ID:             fmt.Sprintf("petkit:%s:%d", info.DeviceType, info.DeviceID),
		PluginID:       "petkit",
		VendorDeviceID: strconv.Itoa(info.DeviceID),
		Kind:           kind,
		Name:           name,
		Room:           "",
		Online:         boolFromAny(state["online"], true),
		Capabilities:   caps,
		Metadata:       metadata,
	}
}

func buildState(info petkitDeviceInfo, kind models.DeviceKind, detail map[string]any, records map[string]any) map[string]any {
	state := map[string]any{
		"online": true,
		"raw":    detail,
	}
	switch kind {
	case models.DeviceKindPetFeeder:
		stateMap := mapFromAny(detail["state"])
		if stateMap == nil {
			stateMap = detail
		}
		settingsMap := mapFromAny(detail["settings"])
		state["food_level"] = intFromAny(firstAny(stateMap, "food", "foodLevel"), 0)
		state["battery_power"] = intFromAny(firstAny(stateMap, "batteryPower"), 0)
		state["feeding"] = intFromAny(firstAny(stateMap, "feeding"), 0)
		state["error_code"] = stringFromAny(firstAny(stateMap, "errorCode"), "")
		state["error_msg"] = stringFromAny(firstAny(stateMap, "errorMsg"), "")
		state["status"] = feederStatusFromDetail(detail)
		if value := firstAny(stateMap, "food1"); value != nil {
			state["food_level_hopper_1"] = intFromAny(value, 0)
		}
		if value := firstAny(stateMap, "food2"); value != nil {
			state["food_level_hopper_2"] = intFromAny(value, 0)
		}
		if value := firstAny(settingsMap, "surplusControl"); value != nil {
			state["surplus_control"] = intFromAny(value, 0)
		}
		if value := firstAny(settingsMap, "surplusStandard"); value != nil {
			state["surplus_standard"] = intFromAny(value, 0)
		}
		if value := firstAny(settingsMap, "selectedSound"); value != nil {
			state["selected_sound"] = intFromAny(value, 0)
		}
		if value := firstAny(stateMap, "desiccantLeftDays"); value != nil {
			state["desiccant_left_days"] = intFromAny(value, 0)
		}
		if latest := latestFeederOccurredEvent(records); latest != nil {
			state["last_feed_event"] = stringFromAny(latest.Payload["event"], "")
			state["last_feed_at"] = latest.TS.Format(time.RFC3339)
		}
	case models.DeviceKindPetLitterBox:
		stateMap := mapFromAny(detail["state"])
		if stateMap == nil {
			stateMap = detail
		}
		state["waste_level"] = intFromAny(firstAny(stateMap, "sandPercent"), 0)
		state["box_full"] = boolFromAny(firstAny(stateMap, "boxFull"), false)
		state["low_power"] = boolFromAny(firstAny(stateMap, "lowPower"), false)
		state["error_code"] = stringFromAny(firstAny(stateMap, "errorCode"), "")
		state["error_msg"] = stringFromAny(firstAny(stateMap, "errorMsg"), "")
		state["status"] = litterStatusFromDetail(detail)
		state["last_usage"] = stringFromAny(firstAny(stateMap, "lastOutTime"), "")
	case models.DeviceKindPetFountain:
		statusMap := mapFromAny(detail["status"])
		if statusMap == nil {
			statusMap = detail
		}
		state["power_status"] = intFromAny(firstAny(statusMap, "powerStatus"), 0)
		state["run_status"] = intFromAny(firstAny(statusMap, "runStatus"), 0)
		state["suspend_status"] = intFromAny(firstAny(statusMap, "suspendStatus"), 0)
		state["detect_status"] = intFromAny(firstAny(statusMap, "detectStatus"), 0)
		state["filter_percent"] = intFromAny(firstAny(detail, "filterPercent"), 0)
		state["water_pump_run_time"] = intFromAny(firstAny(detail, "waterPumpRunTime"), 0)
		state["relay_status"] = "cloud_ble"
	default:
		state["supported"] = false
	}
	return state
}

func capabilitiesForDevice(info petkitDeviceInfo, kind models.DeviceKind, state map[string]any) []string {
	switch kind {
	case models.DeviceKindPetFeeder:
		caps := []string{"feed_once", "manual_feed", "cancel_manual_feed", "reset_desiccant", "food_level", "online", "error"}
		if isDualHopperFeeder(info.DeviceType) {
			caps = append(caps, "manual_feed_dual")
			if _, ok := state["food_level_hopper_1"]; ok {
				caps = append(caps, "food_level_hopper_1")
			}
			if _, ok := state["food_level_hopper_2"]; ok {
				caps = append(caps, "food_level_hopper_2")
			}
		}
		if supportsFeederFoodReplenished(info.DeviceType) {
			caps = append(caps, "food_replenished")
		}
		if supportsFeederPlaySound(info.DeviceType) {
			caps = append(caps, "play_sound")
		}
		if supportsFeederCallPet(info.DeviceType) {
			caps = append(caps, "call_pet")
		}
		return caps
	case models.DeviceKindPetLitterBox:
		return []string{"clean_now", "pause", "resume", "waste_level", "online", "error", "last_usage"}
	case models.DeviceKindPetFountain:
		return []string{"set_power", "turn_on", "turn_off", "pause", "resume", "reset_filter", "relay_status"}
	default:
		return nil
	}
}

func kindFromPetkitType(deviceType string) (models.DeviceKind, bool) {
	switch strings.ToLower(deviceType) {
	case "feeder", "feedermini", "d3", "d4", "d4s", "d4h", "d4sh":
		return models.DeviceKindPetFeeder, true
	case "t3", "t4", "t5", "t6", "t7":
		return models.DeviceKindPetLitterBox, true
	case "w4", "w5", "ctw2", "ctw3":
		return models.DeviceKindPetFountain, true
	default:
		return "", false
	}
}

func feederStatusFromDetail(detail map[string]any) string {
	stateMap := mapFromAny(detail["state"])
	if stateMap == nil {
		stateMap = detail
	}
	if intFromAny(firstAny(stateMap, "feeding"), 0) > 0 {
		return "feeding"
	}
	if stringFromAny(firstAny(stateMap, "errorCode"), "") != "" {
		return "error"
	}
	return "idle"
}

func litterStatusFromDetail(detail map[string]any) string {
	stateMap := mapFromAny(detail["state"])
	if stateMap == nil {
		stateMap = detail
	}
	if stringFromAny(firstAny(stateMap, "errorCode"), "") != "" {
		return "error"
	}
	workState := mapFromAny(firstAny(stateMap, "workState"))
	if workState != nil && intFromAny(firstAny(workState, "workProcess"), 0) > 0 {
		return "cleaning"
	}
	return "idle"
}

func isFeederMini(deviceType string) bool {
	return strings.EqualFold(deviceType, "feedermini")
}

type petkitFamily struct {
	DeviceList []petkitDeviceInfo
	PetList    []petkitPetInfo
}

type petkitPetInfo struct {
	ID   int
	Name string
	SN   string
}

func stringFromAny(value any, fallback string) string {
	switch raw := value.(type) {
	case string:
		if raw != "" {
			return raw
		}
	case fmt.Stringer:
		if raw.String() != "" {
			return raw.String()
		}
	}
	return fallback
}

func intFromAny(value any, fallback int) int {
	switch raw := value.(type) {
	case int:
		return raw
	case int32:
		return int(raw)
	case int64:
		return int(raw)
	case float64:
		return int(raw)
	case json.Number:
		if v, err := raw.Int64(); err == nil {
			return int(v)
		}
	}
	return fallback
}

func boolFromAny(value any, fallback bool) bool {
	switch raw := value.(type) {
	case bool:
		return raw
	case int:
		return raw != 0
	case float64:
		return raw != 0
	case string:
		switch strings.ToLower(raw) {
		case "1", "true", "yes":
			return true
		case "0", "false", "no":
			return false
		}
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstAny(detail map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := detail[key]; ok && value != nil {
			return value
		}
	}
	return nil
}

func mapFromAny(value any) map[string]any {
	if value == nil {
		return nil
	}
	if raw, ok := value.(map[string]any); ok {
		return raw
	}
	return nil
}
