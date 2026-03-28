package control

import (
	"strconv"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

func stateValue(state models.DeviceStateSnapshot, key string) any {
	if key == "" || state.State == nil {
		return nil
	}
	value, ok := state.State[key]
	if !ok {
		return nil
	}
	return value
}

func toggleState(state models.DeviceStateSnapshot, keys ...string) *bool {
	for _, key := range keys {
		if key == "" || state.State == nil {
			continue
		}
		raw, ok := state.State[key]
		if !ok {
			continue
		}
		if value, ok := boolFromAny(raw); ok {
			return &value
		}
	}
	return nil
}

func controlKindOrder(kind models.DeviceControlKind) int {
	switch kind {
	case models.DeviceControlKindToggle:
		return 0
	case models.DeviceControlKindSelect, models.DeviceControlKindNumber:
		return 1
	case models.DeviceControlKindAction:
		return 2
	default:
		return 3
	}
}

func boolFromAny(value any) (bool, bool) {
	switch typed := value.(type) {
	case bool:
		return typed, true
	case int:
		return typed != 0, true
	case int64:
		return typed != 0, true
	case float64:
		return typed != 0, true
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "on", "opened", "running":
			return true, true
		case "0", "false", "off", "closed", "stopped":
			return false, true
		default:
			if number, err := strconv.Atoi(typed); err == nil {
				return number != 0, true
			}
		}
	}
	return false, false
}

func cloneCommand(input *models.DeviceControlCommand) *models.DeviceControlCommand {
	if input == nil {
		return nil
	}
	return &models.DeviceControlCommand{
		Action:     input.Action,
		Params:     cloneParams(input.Params),
		ValueParam: input.ValueParam,
	}
}

func cloneOptions(input []models.DeviceControlOption) []models.DeviceControlOption {
	if len(input) == 0 {
		return nil
	}
	out := make([]models.DeviceControlOption, len(input))
	copy(out, input)
	return out
}

func cloneNumberPtr(value *float64) *float64 {
	if value == nil {
		return nil
	}
	number := *value
	return &number
}

func cloneParams(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func numberPtr(value any) *float64 {
	switch typed := value.(type) {
	case float64:
		return &typed
	case int:
		number := float64(typed)
		return &number
	case int64:
		number := float64(typed)
		return &number
	default:
		return nil
	}
}

func parseControlOptions(value any) []models.DeviceControlOption {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	options := make([]models.DeviceControlOption, 0, len(items))
	for _, item := range items {
		option, ok := item.(map[string]any)
		if !ok {
			continue
		}
		optionValue := stringValue(option["value"])
		if optionValue == "" {
			continue
		}
		options = append(options, models.DeviceControlOption{
			Value: optionValue,
			Label: firstNonEmpty(stringValue(option["label"]), optionValue),
		})
	}
	return options
}

func mapValue(value any) map[string]any {
	item, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	return cloneParams(item)
}

func mapSlice(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		return typed
	case []any:
		out := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			mapped, ok := item.(map[string]any)
			if ok {
				out = append(out, mapped)
			}
		}
		return out
	default:
		return nil
	}
}

func stringValue(value any) string {
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
