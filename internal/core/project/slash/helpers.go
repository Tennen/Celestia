package slash

import (
	"encoding/csv"
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

func equalRef(a string, b string) bool {
	return normalizeRef(a) == normalizeRef(b) && normalizeRef(a) != ""
}

func normalizeRef(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(" ", "", "_", "", "-", "", ".", "")
	return replacer.Replace(value)
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
- /home <device-or-room.command> [value|key=value ...]
- /home <command> [value|key=value ...]
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
- /home "Living Room Light.Power" off
- /home Power on
- /home "Living Room Light" Power off
- /home action <device> <raw_action> key=value
`)
}
