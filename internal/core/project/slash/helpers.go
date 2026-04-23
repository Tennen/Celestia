package slash

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

func splitSlashFields(input string) ([]string, error) {
	reader := csv.NewReader(strings.NewReader(input))
	reader.Comma = ' '
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1
	fields, err := reader.Read()
	if err != nil && err != io.EOF {
		return nil, err
	}
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if strings.TrimSpace(field) != "" {
			out = append(out, strings.TrimSpace(field))
		}
	}
	return out, nil
}

func parseSlashParams(args []string) (map[string]any, []string, error) {
	params := map[string]any{}
	values := []string{}
	for _, arg := range args {
		trimmed := strings.TrimSpace(arg)
		if trimmed == "" {
			continue
		}
		key, value, ok := strings.Cut(trimmed, "=")
		if !ok {
			values = append(values, trimmed)
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, nil, fmt.Errorf("invalid param %q", arg)
		}
		params[key] = coerceSlashValue(value)
	}
	return params, values, nil
}

func coerceSlashValue(value string) any {
	trimmed := strings.TrimSpace(value)
	if parsed, ok := parseBoolWord(trimmed); ok {
		return parsed
	}
	if number, err := strconv.ParseFloat(trimmed, 64); err == nil {
		return number
	}
	return trimmed
}

func parseBoolWord(value string) (bool, bool) {
	switch normalizeRef(value) {
	case "on", "true", "1", "open", "enable", "enabled", "开", "打开", "开启":
		return true, true
	case "off", "false", "0", "close", "disable", "disabled", "关", "关闭":
		return false, true
	default:
		return false, false
	}
}

func boolCommandRef(value string) (bool, bool) {
	return parseBoolWord(value)
}

func boolParam(params map[string]any, key string) (bool, bool) {
	value, ok := params[key]
	if !ok {
		return false, false
	}
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		return parseBoolWord(typed)
	default:
		return false, false
	}
}

func singleToggle(controls []models.DeviceControl) (models.DeviceControl, error) {
	var matches []models.DeviceControl
	for _, control := range controls {
		if control.Kind == models.DeviceControlKindToggle && !control.Disabled && control.Visible {
			matches = append(matches, control)
		}
	}
	switch len(matches) {
	case 0:
		return models.DeviceControl{}, errors.New("device has no executable toggle")
	case 1:
		return matches[0], nil
	default:
		names := make([]string, 0, len(matches))
		for _, control := range matches {
			names = append(names, firstNonEmpty(control.Label, control.ID))
		}
		return models.DeviceControl{}, fmt.Errorf("toggle is ambiguous: %s", strings.Join(names, ", "))
	}
}

func findControl(controls []models.DeviceControl, ref string) (models.DeviceControl, bool) {
	for _, control := range controls {
		aliases := []string{control.ID, control.Label, control.DefaultLabel, control.Alias}
		if control.Command != nil {
			aliases = append(aliases, control.Command.Action)
		}
		for _, alias := range aliases {
			if equalRef(alias, ref) {
				return control, true
			}
		}
	}
	return models.DeviceControl{}, false
}

func deviceMatches(device models.Device, ref string) bool {
	aliases := []string{device.ID, device.Name, device.DefaultName, device.Alias, device.Room, device.VendorDeviceID}
	for _, alias := range aliases {
		if equalRef(alias, ref) {
			return true
		}
	}
	return false
}

func equalRef(a string, b string) bool {
	return normalizeRef(a) == normalizeRef(b) && normalizeRef(a) != ""
}

func normalizeRef(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(" ", "", "_", "", "-", "", ".", "")
	return replacer.Replace(value)
}

func cloneMap(input map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range input {
		out[key] = value
	}
	return out
}

func actorOrInput(req models.ProjectInputRequest) string {
	return firstNonEmpty(req.Actor, req.Source, "slash")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func slashHelp() string {
	return strings.TrimSpace(`
Slash commands:
- /home list [query]
- /home <device> <command> [value|key=value ...]
- /home action <device> <action> [key=value ...]
- /market portfolio
- /market run [open|midday|close] [notes]
- /market import <fund codes>
`)
}

func homeHelp() string {
	return strings.TrimSpace(`
Home commands:
- /home list
- /home "Living Room Light" on
- /home "Living Room Light" Power off
- /home action <device> <raw_action> key=value
`)
}
