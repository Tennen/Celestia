package gateway

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/chentianyu/celestia/internal/models"
)

func (c aiResolvedCommand) buildRequest(input map[string]any) (string, map[string]any, error) {
	switch c.kind {
	case models.DeviceControlKindToggle:
		for key := range input {
			if normalizeAIRef(key) != normalizeAIRef("on") {
				return "", nil, fmt.Errorf("unsupported parameter %q", key)
			}
		}
		paramValue, ok := matchAIInputParam(input, "on")
		if !ok {
			return "", nil, errors.New(`parameter "on" is required`)
		}
		on, ok := coerceAIBool(paramValue)
		if !ok {
			return "", nil, errors.New(`parameter "on" must be a boolean`)
		}
		command := c.offCommand
		if on {
			command = c.onCommand
		}
		if command == nil || strings.TrimSpace(command.Action) == "" {
			return "", nil, errors.New("resolved command is not executable")
		}
		return command.Action, cloneAIParamsMap(command.Params), nil
	case models.DeviceControlKindAction, models.DeviceControlKindSelect, models.DeviceControlKindNumber:
		if c.command == nil || strings.TrimSpace(c.command.Action) == "" {
			return "", nil, errors.New("resolved command is not executable")
		}
		params, err := buildAICommandParams(input, c.command, c.view.Params)
		if err != nil {
			return "", nil, err
		}
		return c.command.Action, params, nil
	default:
		return "", nil, errors.New("unsupported command type")
	}
}

func buildAICommandParams(input map[string]any, command *models.DeviceControlCommand, params []AICommandParam) (map[string]any, error) {
	out := cloneAIParamsMap(command.Params)
	if len(params) == 0 {
		if len(input) > 0 {
			keys := make([]string, 0, len(input))
			for key := range input {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			return nil, fmt.Errorf("command does not accept parameters: %s", strings.Join(keys, ", "))
		}
		return out, nil
	}

	allowed := make(map[string]AICommandParam, len(params))
	for _, param := range params {
		allowed[normalizeAIRef(param.Name)] = param
		if _, ok := out[param.Name]; !ok && param.Default != nil {
			out[param.Name] = param.Default
		}
	}

	for rawKey, value := range input {
		spec, ok := allowed[normalizeAIRef(rawKey)]
		if !ok {
			return nil, fmt.Errorf("unsupported parameter %q", rawKey)
		}
		coerced, err := coerceAIParam(spec, value)
		if err != nil {
			return nil, err
		}
		out[spec.Name] = coerced
	}

	for _, param := range params {
		if param.Required {
			if _, ok := out[param.Name]; !ok {
				return nil, fmt.Errorf("parameter %q is required", param.Name)
			}
		}
	}
	return out, nil
}

func coerceAIParam(spec AICommandParam, value any) (any, error) {
	switch spec.Type {
	case models.DeviceCommandParamTypeBoolean:
		parsed, ok := coerceAIBool(value)
		if !ok {
			return nil, fmt.Errorf("parameter %q must be a boolean", spec.Name)
		}
		return parsed, nil
	case models.DeviceCommandParamTypeNumber:
		number, ok := coerceAINumber(value)
		if !ok {
			return nil, fmt.Errorf("parameter %q must be a number", spec.Name)
		}
		if spec.Min != nil && number < *spec.Min {
			return nil, fmt.Errorf("parameter %q must be >= %v", spec.Name, *spec.Min)
		}
		if spec.Max != nil && number > *spec.Max {
			return nil, fmt.Errorf("parameter %q must be <= %v", spec.Name, *spec.Max)
		}
		return number, nil
	case models.DeviceCommandParamTypeString:
		text := strings.TrimSpace(fmt.Sprint(value))
		if text == "" && spec.Required {
			return nil, fmt.Errorf("parameter %q must be a non-empty string", spec.Name)
		}
		if len(spec.Options) == 0 {
			return text, nil
		}
		for _, option := range spec.Options {
			if normalizeAIRef(option.Value) == normalizeAIRef(text) || normalizeAIRef(option.Label) == normalizeAIRef(text) {
				return option.Value, nil
			}
		}
		choices := make([]string, 0, len(spec.Options))
		for _, option := range spec.Options {
			choices = append(choices, option.Value)
		}
		sort.Strings(choices)
		return nil, fmt.Errorf("parameter %q must be one of %s", spec.Name, strings.Join(choices, ", "))
	default:
		return value, nil
	}
}

func aiMatches(primary string, aliases []string, ref string) bool {
	needle := normalizeAIRef(ref)
	if needle == "" {
		return false
	}
	if normalizeAIRef(primary) == needle {
		return true
	}
	for _, alias := range aliases {
		if normalizeAIRef(alias) == needle {
			return true
		}
	}
	return false
}

func aiPublicDefaults(base map[string]any, params []AICommandParam) map[string]any {
	if len(base) == 0 {
		return nil
	}
	paramDefaults := make(map[string]AICommandParam, len(params))
	for _, param := range params {
		paramDefaults[param.Name] = param
	}
	out := make(map[string]any, len(base))
	for key, value := range base {
		if param, ok := paramDefaults[key]; ok && reflect.DeepEqual(param.Default, value) {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeAIRef(value string) string {
	var b strings.Builder
	lastSpace := true
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastSpace = false
			continue
		}
		if !lastSpace {
			b.WriteByte(' ')
			lastSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

func appendUniqueAIName(values []string, value string) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return values
	}
	normalized := normalizeAIRef(trimmed)
	for _, existing := range values {
		if normalizeAIRef(existing) == normalized {
			return values
		}
	}
	return append(values, trimmed)
}

func matchAIInputParam(input map[string]any, name string) (any, bool) {
	for key, value := range input {
		if normalizeAIRef(key) == normalizeAIRef(name) {
			return value, true
		}
	}
	return nil, false
}

func coerceAIBool(value any) (bool, bool) {
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "on", "yes":
			return true, true
		case "0", "false", "off", "no":
			return false, true
		}
	case int:
		return typed != 0, true
	case int64:
		return typed != 0, true
	case float64:
		return typed != 0, true
	}
	return false, false
}

func coerceAINumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case string:
		number, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil {
			return 0, false
		}
		return number, true
	default:
		return 0, false
	}
}

func cloneAIControlCommand(input *models.DeviceControlCommand) *models.DeviceControlCommand {
	if input == nil {
		return nil
	}
	return &models.DeviceControlCommand{
		Action:     input.Action,
		Params:     cloneAIParamsMap(input.Params),
		ValueParam: input.ValueParam,
		ParamsSpec: cloneAIParamSpecs(input.ParamsSpec),
	}
}

func cloneAICommandParams(input []models.DeviceCommandParamSpec) []AICommandParam {
	if len(input) == 0 {
		return nil
	}
	out := make([]AICommandParam, 0, len(input))
	for _, item := range input {
		out = append(out, AICommandParam{
			Name:     item.Name,
			Type:     item.Type,
			Required: item.Required,
			Default:  item.Default,
			Options:  cloneAIOptions(item.Options),
			Min:      cloneAINumber(item.Min),
			Max:      cloneAINumber(item.Max),
			Step:     cloneAINumber(item.Step),
			Unit:     item.Unit,
		})
	}
	return out
}

func cloneAIParamSpecs(input []models.DeviceCommandParamSpec) []models.DeviceCommandParamSpec {
	if len(input) == 0 {
		return nil
	}
	out := make([]models.DeviceCommandParamSpec, len(input))
	for idx, item := range input {
		out[idx] = models.DeviceCommandParamSpec{
			Name:     item.Name,
			Type:     item.Type,
			Required: item.Required,
			Default:  item.Default,
			Options:  cloneAIOptions(item.Options),
			Min:      cloneAINumber(item.Min),
			Max:      cloneAINumber(item.Max),
			Step:     cloneAINumber(item.Step),
			Unit:     item.Unit,
		}
	}
	return out
}

func cloneAIOptions(input []models.DeviceControlOption) []models.DeviceControlOption {
	if len(input) == 0 {
		return nil
	}
	out := make([]models.DeviceControlOption, len(input))
	copy(out, input)
	return out
}

func cloneAIParamsMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func cloneAINumber(value *float64) *float64 {
	if value == nil {
		return nil
	}
	number := *value
	return &number
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
