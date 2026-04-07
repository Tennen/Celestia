package app

import (
	"reflect"
	"sort"
	"strings"
)

func normalizeHaierState(attrs map[string]string) map[string]any {
	state := make(map[string]any, len(attrs)+8)
	for key, value := range attrs {
		state[key] = value
	}
	applyDerivedHaierState(state, attrs)
	return state
}

func mergeHaierStateUpdate(current map[string]any, attrs map[string]string) map[string]any {
	next := cloneMap(current)
	for key, value := range attrs {
		next[key] = value
	}
	applyDerivedHaierState(next, attrs)
	return next
}

func applyDerivedHaierState(state map[string]any, attrs map[string]string) {
	if value, ok := attrs["machMode"]; ok {
		state["machine_status"] = haierMachineStatus(value)
	}
	if value, ok := attrs["prCode"]; ok {
		setStringOrDelete(state, "program", value)
	}
	if value, ok := attrs["prPhase"]; ok {
		setStringOrDelete(state, "phase", value)
	}
	if value, ok := attrs["remainingTimeMM"]; ok {
		state["remaining_minutes"] = intFromAny(value)
	}
	if value, ok := attrs["tempLevel"]; ok {
		setPositiveIntOrDelete(state, "temperature", value)
	}
	if value, ok := attrs["spinSpeed"]; ok {
		setPositiveIntOrDelete(state, "spin_speed", value)
	}
	if value, ok := attrs["delayTime"]; ok {
		state["delay_time"] = intFromAny(value)
	}
	if value, ok := attrs["prewash"]; ok {
		state["prewash"] = value == "1" || strings.EqualFold(value, "true")
	}
	if value, ok := attrs["extraRinse"]; ok {
		state["extra_rinse"] = intFromAny(value)
	}
	if value, ok := attrs["goodNight"]; ok {
		state["good_night"] = intFromAny(value)
	}
	if value, ok := attrs["totalElectricityUsed"]; ok {
		state["total_electricity_used"] = floatFromAny(value)
	}
	if value, ok := attrs["totalWaterUsed"]; ok {
		state["total_water_used"] = floatFromAny(value)
	}
	if value, ok := attrs["totalWashCycle"]; ok {
		state["total_wash_cycle"] = intFromAny(value)
	}
}

func haierMachineStatus(value string) string {
	switch value {
	case "3":
		return "paused"
	case "", "0":
		return "idle"
	default:
		return "running"
	}
}

func setStringOrDelete(state map[string]any, key, value string) {
	if strings.TrimSpace(value) == "" {
		delete(state, key)
		return
	}
	state[key] = value
}

func setPositiveIntOrDelete(state map[string]any, key string, value any) {
	if parsed := intFromAny(value); parsed > 0 {
		state[key] = parsed
		return
	}
	delete(state, key)
}

func changedStateKeys(previous map[string]any, current map[string]any) []string {
	keys := make([]string, 0, len(current))
	for key, value := range current {
		if !reflect.DeepEqual(previous[key], value) {
			keys = append(keys, key)
		}
	}
	for key := range previous {
		if _, ok := current[key]; !ok {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}
