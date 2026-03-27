package app

import (
	"errors"
	"fmt"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/pluginutil"
)

func commandForRequest(device *applianceRuntime, req models.CommandRequest) (string, map[string]any, map[string]any, string, error) {
	if device == nil {
		return "", nil, nil, "", errors.New("device not found")
	}
	switch req.Action {
	case "start":
		if !device.CapabilitySet["start"] {
			return "", nil, nil, "", errors.New("start unsupported by model")
		}
		commandName, err := requireCommandName(device, "start")
		if err != nil {
			return "", nil, nil, "", err
		}
		params := map[string]any{}
		for _, key := range []string{"delayTime", "tempLevel", "spinSpeed", "prewash", "extraRinse", "goodNight"} {
			if value, ok := device.CurrentState[key]; ok {
				params[key] = value
			}
		}
		if program := stringFromAny(device.CurrentState["program"]); program != "" {
			params["program"] = program
		}
		if program := stringFromAny(req.Params["program"]); program != "" {
			params["program"] = program
		}
		if program := stringFromAny(req.Params["program_name"]); program != "" {
			params["program"] = program
		}
		return commandName, params, map[string]any{}, stringFromAny(params["program"]), nil
	case "stop":
		if !device.CapabilitySet["stop"] {
			return "", nil, nil, "", errors.New("stop unsupported by model")
		}
		commandName, err := requireCommandName(device, "stop")
		if err != nil {
			return "", nil, nil, "", err
		}
		return commandName, map[string]any{}, map[string]any{}, "", nil
	case "pause":
		if !device.CapabilitySet["pause"] {
			return "", nil, nil, "", errors.New("pause unsupported by model")
		}
		commandName, err := requireCommandName(device, "pause")
		if err != nil {
			return "", nil, nil, "", err
		}
		return commandName, map[string]any{}, map[string]any{}, "", nil
	case "resume":
		if !device.CapabilitySet["resume"] {
			return "", nil, nil, "", errors.New("resume unsupported by model")
		}
		commandName, err := requireCommandName(device, "resume")
		if err != nil {
			return "", nil, nil, "", err
		}
		return commandName, map[string]any{}, map[string]any{}, "", nil
	case "set_delay_time":
		if !device.CapabilitySet["delay_time"] {
			return "", nil, nil, "", errors.New("delay_time unsupported by model")
		}
		commandName, err := requireCommandName(device, "delay_time")
		if err != nil {
			return "", nil, nil, "", err
		}
		value := pluginutil.Int(req.Params["minutes"], intFromAny(device.CurrentState["delay_time"]))
		return commandName, map[string]any{"delayTime": value}, map[string]any{}, "", nil
	case "set_temp_level":
		if !device.CapabilitySet["temp_level"] {
			return "", nil, nil, "", errors.New("temp_level unsupported by model")
		}
		commandName, err := requireCommandName(device, "temp_level")
		if err != nil {
			return "", nil, nil, "", err
		}
		value := pluginutil.Int(req.Params["value"], intFromAny(device.CurrentState["temperature"]))
		return commandName, map[string]any{"tempLevel": value}, map[string]any{}, "", nil
	case "set_spin_speed":
		if !device.CapabilitySet["spin_speed"] {
			return "", nil, nil, "", errors.New("spin_speed unsupported by model")
		}
		commandName, err := requireCommandName(device, "spin_speed")
		if err != nil {
			return "", nil, nil, "", err
		}
		value := pluginutil.Int(req.Params["value"], intFromAny(device.CurrentState["spin_speed"]))
		return commandName, map[string]any{"spinSpeed": value}, map[string]any{}, "", nil
	case "set_prewash":
		if !device.CapabilitySet["prewash"] {
			return "", nil, nil, "", errors.New("prewash unsupported by model")
		}
		commandName, err := requireCommandName(device, "prewash")
		if err != nil {
			return "", nil, nil, "", err
		}
		value := pluginutil.Bool(req.Params["enabled"], false)
		return commandName, map[string]any{"prewash": value}, map[string]any{}, "", nil
	case "set_extra_rinse":
		if !device.CapabilitySet["extra_rinse"] {
			return "", nil, nil, "", errors.New("extra_rinse unsupported by model")
		}
		commandName, err := requireCommandName(device, "extra_rinse")
		if err != nil {
			return "", nil, nil, "", err
		}
		value := pluginutil.Int(req.Params["value"], intFromAny(device.CurrentState["extra_rinse"]))
		return commandName, map[string]any{"extraRinse": value}, map[string]any{}, "", nil
	case "set_good_night":
		if !device.CapabilitySet["good_night"] {
			return "", nil, nil, "", errors.New("good_night unsupported by model")
		}
		commandName, err := requireCommandName(device, "good_night")
		if err != nil {
			return "", nil, nil, "", err
		}
		value := pluginutil.Int(req.Params["value"], intFromAny(device.CurrentState["good_night"]))
		return commandName, map[string]any{"goodNight": value}, map[string]any{}, "", nil
	default:
		return "", nil, nil, "", fmt.Errorf("unsupported action %q", req.Action)
	}
}

func buildCapabilities(commands map[string]any) (map[string]string, map[string]bool) {
	names := collectCommandNames(commands)
	commandNames := map[string]string{}
	capabilitySet := map[string]bool{}
	assign := func(capability, action string, candidates ...string) {
		if cmd := matchCommandName(names, candidates...); cmd != "" {
			commandNames[action] = cmd
			capabilitySet[capability] = true
		}
	}
	assign("start", "start", "startProgram", "startprogram")
	assign("stop", "stop", "stopProgram", "stopprogram")
	assign("pause", "pause", "pauseProgram", "pauseprogram")
	assign("resume", "resume", "resumeProgram", "resumeprogram")
	assign("delay_time", "delay_time", "setDelayTime", "delayTime", "delaytime")
	assign("temp_level", "temp_level", "setTempLevel", "tempLevel", "templevel")
	assign("spin_speed", "spin_speed", "setSpinSpeed", "spinSpeed", "spinspeed")
	assign("prewash", "prewash", "setPrewash", "prewash")
	assign("extra_rinse", "extra_rinse", "setExtraRinse", "extraRinse", "extrarinse")
	assign("good_night", "good_night", "setGoodNight", "goodNight", "goodnight")
	if containsAny(names, "remainingtime", "remainingtimemm", "remaining_time") {
		capabilitySet["remaining_time"] = true
	}
	if containsAny(names, "program", "prcode") {
		capabilitySet["program"] = true
	}
	if containsAny(names, "phase", "prphase") {
		capabilitySet["phase"] = true
	}
	if containsAny(names, "machine_status", "machmode", "status") {
		capabilitySet["machine_status"] = true
	}
	if containsAny(names, "totalelectricityused", "electricity") {
		capabilitySet["energy_usage"] = true
	}
	if containsAny(names, "totalwaterused", "water") {
		capabilitySet["water_usage"] = true
	}
	return commandNames, capabilitySet
}

func collectCommandNames(data map[string]any) []string {
	out := []string{}
	for key, value := range data {
		if key == "applianceModel" {
			continue
		}
		if isCommandShape(value) {
			out = append(out, key)
			continue
		}
		if nested, ok := value.(map[string]any); ok {
			if containsCommandShape(nested) {
				out = append(out, key)
			}
		}
	}
	return out
}

func containsCommandShape(data map[string]any) bool {
	for _, value := range data {
		if isCommandShape(value) {
			return true
		}
		if nested, ok := value.(map[string]any); ok && containsCommandShape(nested) {
			return true
		}
	}
	return false
}

func isCommandShape(v any) bool {
	item, ok := v.(map[string]any)
	if !ok {
		return false
	}
	_, hasDescription := item["description"]
	_, hasProtocol := item["protocolType"]
	return hasDescription && hasProtocol
}

func matchCommandName(names []string, candidates ...string) string {
	for _, candidate := range candidates {
		needle := normalizeKey(candidate)
		for _, name := range names {
			if normalizeKey(name) == needle || strings.Contains(normalizeKey(name), needle) {
				return name
			}
		}
	}
	return ""
}

func containsAny(names []string, tokens ...string) bool {
	for _, name := range names {
		normalized := normalizeKey(name)
		for _, token := range tokens {
			if strings.Contains(normalized, normalizeKey(token)) {
				return true
			}
		}
	}
	return false
}

func normalizeKey(value string) string {
	value = strings.ToLower(value)
	value = strings.ReplaceAll(value, "_", "")
	value = strings.ReplaceAll(value, "-", "")
	value = strings.ReplaceAll(value, " ", "")
	return value
}

func requireCommandName(device *applianceRuntime, key string) (string, error) {
	if device == nil {
		return "", errors.New("device not found")
	}
	if command := strings.TrimSpace(device.CommandNames[key]); command != "" {
		return command, nil
	}
	return "", fmt.Errorf("missing command mapping for %s", key)
}
