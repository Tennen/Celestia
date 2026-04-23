package control

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

func (c homeResolvedCommand) buildRequest(input map[string]any, values []string) (string, map[string]any, error) {
	switch c.kind {
	case models.DeviceControlKindToggle:
		for key := range input {
			if normalizeHomeRef(key) != normalizeHomeRef("on") {
				return "", nil, fmt.Errorf("unsupported parameter %q", key)
			}
		}
		paramValue, ok := matchHomeInputParam(input, "on")
		if !ok && len(values) > 0 {
			paramValue = values[0]
			ok = true
		}
		if !ok {
			return "", nil, errors.New(`parameter "on" is required`)
		}
		on, ok := coerceHomeBool(paramValue)
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
		return command.Action, cloneParams(command.Params), nil
	case models.DeviceControlKindAction, models.DeviceControlKindSelect, models.DeviceControlKindNumber:
		if c.command == nil || strings.TrimSpace(c.command.Action) == "" {
			return "", nil, errors.New("resolved command is not executable")
		}
		params, err := buildHomeCommandParams(input, values, c.command, c.view.Params)
		if err != nil {
			return "", nil, err
		}
		return c.command.Action, params, nil
	default:
		return "", nil, errors.New("unsupported command type")
	}
}

func buildHomeCommandParams(input map[string]any, values []string, command *models.DeviceControlCommand, params []HomeCommandParam) (map[string]any, error) {
	out := cloneParams(command.Params)
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

	allowed := make(map[string]HomeCommandParam, len(params))
	for _, param := range params {
		allowed[normalizeHomeRef(param.Name)] = param
		if _, ok := out[param.Name]; !ok && param.Default != nil {
			out[param.Name] = param.Default
		}
	}

	valueParam := strings.TrimSpace(command.ValueParam)
	if valueParam != "" && len(values) > 0 {
		spec, ok := allowed[normalizeHomeRef(valueParam)]
		if ok {
			if out == nil {
				out = map[string]any{}
			}
			coerced, err := coerceHomeParam(spec, values[0])
			if err != nil {
				return nil, err
			}
			out[spec.Name] = coerced
		}
	}

	for rawKey, value := range input {
		spec, ok := allowed[normalizeHomeRef(rawKey)]
		if !ok {
			return nil, fmt.Errorf("unsupported parameter %q", rawKey)
		}
		if out == nil {
			out = map[string]any{}
		}
		coerced, err := coerceHomeParam(spec, value)
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

func coerceHomeParam(spec HomeCommandParam, value any) (any, error) {
	switch spec.Type {
	case models.DeviceCommandParamTypeBoolean:
		parsed, ok := coerceHomeBool(value)
		if !ok {
			return nil, fmt.Errorf("parameter %q must be a boolean", spec.Name)
		}
		return parsed, nil
	case models.DeviceCommandParamTypeNumber:
		number, ok := coerceHomeNumber(value)
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
			if normalizeHomeRef(option.Value) == normalizeHomeRef(text) || normalizeHomeRef(option.Label) == normalizeHomeRef(text) {
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

func homeMatches(primary string, aliases []string, ref string) bool {
	needle := normalizeHomeRef(ref)
	if needle == "" {
		return false
	}
	if normalizeHomeRef(primary) == needle {
		return true
	}
	for _, alias := range aliases {
		if normalizeHomeRef(alias) == needle {
			return true
		}
	}
	return false
}

func homePublicDefaults(base map[string]any, params []HomeCommandParam) map[string]any {
	if len(base) == 0 {
		return nil
	}
	paramDefaults := make(map[string]HomeCommandParam, len(params))
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

func normalizeHomeRef(value string) string {
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

func appendUniqueHomeName(values []string, value string) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return values
	}
	normalized := normalizeHomeRef(trimmed)
	for _, existing := range values {
		if normalizeHomeRef(existing) == normalized {
			return values
		}
	}
	return append(values, trimmed)
}

func matchHomeInputParam(input map[string]any, name string) (any, bool) {
	for key, value := range input {
		if normalizeHomeRef(key) == normalizeHomeRef(name) {
			return value, true
		}
	}
	return nil, false
}

func coerceHomeBool(value any) (bool, bool) {
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

func coerceHomeNumber(value any) (float64, bool) {
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

func cloneHomeControlCommand(input *models.DeviceControlCommand) *models.DeviceControlCommand {
	if input == nil {
		return nil
	}
	return &models.DeviceControlCommand{
		Action:     input.Action,
		Params:     cloneParams(input.Params),
		ValueParam: input.ValueParam,
	}
}

func cloneHomeCommandParams(input []models.DeviceCommandParamSpec) []HomeCommandParam {
	if len(input) == 0 {
		return nil
	}
	out := make([]HomeCommandParam, 0, len(input))
	for _, item := range input {
		out = append(out, HomeCommandParam{
			Name:     item.Name,
			Type:     item.Type,
			Required: item.Required,
			Default:  item.Default,
			Options:  cloneOptions(item.Options),
			Min:      cloneNumberPtr(item.Min),
			Max:      cloneNumberPtr(item.Max),
			Step:     cloneNumberPtr(item.Step),
			Unit:     item.Unit,
		})
	}
	return out
}
