package app

import (
	"fmt"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/plugins/xiaomi/internal/spec"
)

func encodePropertyValue(prop spec.Property, raw any) (any, error) {
	switch prop.Format {
	case "bool":
		return boolParam(raw, "", false), nil
	case "string":
		return stringParam(raw), nil
	default:
		if len(prop.ValueList) > 0 {
			switch typed := raw.(type) {
			case string:
				if value, ok := prop.EnumValue(typed); ok {
					return value, nil
				}
				return nil, fmt.Errorf("unsupported enum value %q", typed)
			default:
				return intParam(raw), nil
			}
		}
		number := floatParam(raw)
		if min, max, _, ok := prop.RangeBounds(); ok {
			if number < min {
				number = min
			}
			if number > max {
				number = max
			}
		}
		if strings.HasPrefix(prop.Format, "uint") || strings.HasPrefix(prop.Format, "int") {
			return int(number), nil
		}
		return number, nil
	}
}

func decodePropertyValue(prop spec.Property, key string, raw any) any {
	if len(prop.ValueList) > 0 {
		if desc, ok := prop.EnumDescription(intParam(raw)); ok {
			return desc
		}
	}
	switch key {
	case "brightness", "color_temp", "target_temperature", "light_brightness", "filter_life", "volume":
		return intParam(raw)
	case "temperature", "water_temperature":
		return floatParam(raw)
	default:
		switch prop.Format {
		case "bool":
			return boolParam(raw, "", false)
		case "string":
			return stringParam(raw)
		default:
			if strings.HasPrefix(prop.Format, "uint") || strings.HasPrefix(prop.Format, "int") {
				return intParam(raw)
			}
			return raw
		}
	}
}

func cloneViews(devices map[string]models.Device, _ map[string]models.DeviceStateSnapshot) []models.Device {
	out := make([]models.Device, 0, len(devices))
	for _, device := range devices {
		out = append(out, device)
	}
	return out
}

func cloneStates(states map[string]models.DeviceStateSnapshot) []models.DeviceStateSnapshot {
	out := make([]models.DeviceStateSnapshot, 0, len(states))
	for _, state := range states {
		out = append(out, state)
	}
	return out
}

func cloneStateMap(in map[string]models.DeviceStateSnapshot) map[string]models.DeviceStateSnapshot {
	out := make(map[string]models.DeviceStateSnapshot, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func stringParam(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}

func boolParam(value any, key string, fallback bool) bool {
	if key != "" {
		if typed, ok := value.(map[string]any); ok {
			value = typed[key]
		}
	}
	switch typed := value.(type) {
	case bool:
		return typed
	default:
		return fallback
	}
}

func intParam(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func floatParam(value any) float64 {
	switch typed := value.(type) {
	case int:
		return float64(typed)
	case int32:
		return float64(typed)
	case int64:
		return float64(typed)
	case float64:
		return typed
	default:
		return 0
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
