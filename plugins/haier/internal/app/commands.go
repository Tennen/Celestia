package app

import (
	"errors"
	"fmt"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/pluginutil"
)

// commandForRequest translates a CommandRequest into a flat params map
// suitable for WSS BatchCmdReq cmdArgs payloads.
func commandForRequest(device *applianceRuntime, req models.CommandRequest) (map[string]any, error) {
	if device == nil {
		return nil, errors.New("device not found")
	}
	switch req.Action {
	case "start":
		if !device.CapabilitySet["start"] {
			return nil, errors.New("start unsupported by model")
		}
		params := map[string]any{"machMode": "1"}
		if program := pluginutil.String(req.Params["program"], ""); program != "" {
			params["prCode"] = program
		} else if program := pluginutil.String(req.Params["program_name"], ""); program != "" {
			params["prCode"] = program
		}
		return params, nil
	case "stop":
		if !device.CapabilitySet["stop"] {
			return nil, errors.New("stop unsupported by model")
		}
		return map[string]any{"machMode": "0"}, nil
	case "pause":
		if !device.CapabilitySet["pause"] {
			return nil, errors.New("pause unsupported by model")
		}
		return map[string]any{"machMode": "3"}, nil
	case "resume":
		if !device.CapabilitySet["resume"] {
			return nil, errors.New("resume unsupported by model")
		}
		return map[string]any{"machMode": "1"}, nil
	case "set_delay_time":
		if !device.CapabilitySet["delay_time"] {
			return nil, errors.New("delay_time unsupported by model")
		}
		value := pluginutil.Int(req.Params["minutes"], intFromAny(device.CurrentState["delay_time"]))
		return map[string]any{"delayTime": value}, nil
	case "set_temp_level":
		if !device.CapabilitySet["temp_level"] {
			return nil, errors.New("temp_level unsupported by model")
		}
		value := pluginutil.Int(req.Params["value"], intFromAny(device.CurrentState["temperature"]))
		return map[string]any{"tempLevel": value}, nil
	case "set_spin_speed":
		if !device.CapabilitySet["spin_speed"] {
			return nil, errors.New("spin_speed unsupported by model")
		}
		value := pluginutil.Int(req.Params["value"], intFromAny(device.CurrentState["spin_speed"]))
		return map[string]any{"spinSpeed": value}, nil
	case "set_prewash":
		if !device.CapabilitySet["prewash"] {
			return nil, errors.New("prewash unsupported by model")
		}
		value := pluginutil.Bool(req.Params["enabled"], false)
		return map[string]any{"prewash": value}, nil
	case "set_extra_rinse":
		if !device.CapabilitySet["extra_rinse"] {
			return nil, errors.New("extra_rinse unsupported by model")
		}
		value := pluginutil.Int(req.Params["value"], intFromAny(device.CurrentState["extra_rinse"]))
		return map[string]any{"extraRinse": value}, nil
	case "set_good_night":
		if !device.CapabilitySet["good_night"] {
			return nil, errors.New("good_night unsupported by model")
		}
		value := pluginutil.Int(req.Params["value"], intFromAny(device.CurrentState["good_night"]))
		return map[string]any{"goodNight": value}, nil
	default:
		return nil, fmt.Errorf("unsupported action %q", req.Action)
	}
}
