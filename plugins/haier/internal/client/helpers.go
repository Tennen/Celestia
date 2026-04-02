package client

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

func applianceOptions(appliance map[string]any) map[string]any {
	if options, ok := appliance["options"].(map[string]any); ok {
		return options
	}
	if model, ok := appliance["applianceModel"].(map[string]any); ok {
		if options, ok := model["options"].(map[string]any); ok {
			return options
		}
	}
	return map[string]any{}
}

// StringFromAny converts any value to its string representation.
func StringFromAny(v any) string {
	switch raw := v.(type) {
	case string:
		return raw
	case fmt.Stringer:
		return raw.String()
	case float64:
		if raw == float64(int64(raw)) {
			return fmt.Sprintf("%d", int64(raw))
		}
		return fmt.Sprintf("%v", raw)
	case int:
		return fmt.Sprintf("%d", raw)
	case json.Number:
		return raw.String()
	default:
		return ""
	}
}

func resultCodeFrom(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	if code := StringFromAny(payload["resultCode"]); code != "" {
		return code
	}
	if nested, ok := payload["payload"].(map[string]any); ok {
		return StringFromAny(nested["resultCode"])
	}
	return ""
}

func trimForError(text string) string {
	text = strings.TrimSpace(text)
	if len(text) > 240 {
		return text[:240] + "..."
	}
	return text
}

func mustJSONString(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

func mustJSONRawMessage(s string) json.RawMessage {
	return json.RawMessage([]byte(s))
}

func urlSafeUnescape(value string) string {
	if decoded, err := url.QueryUnescape(value); err == nil {
		return decoded
	}
	return value
}

func generateNonce() string {
	raw := fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().UnixNano()/97)
	return strings.ReplaceAll(raw, "--", "-")
}

func randomHex(n int) string {
	const chars = "abcdef0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[time.Now().UnixNano()%int64(len(chars))]
	}
	return string(b)
}

func timezoneOffset(timezone string) string {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		_, offset := time.Now().Zone()
		return formatOffset(offset)
	}
	_, offset := time.Now().In(loc).Zone()
	return formatOffset(offset)
}

func formatOffset(seconds int) string {
	sign := "+"
	if seconds < 0 {
		sign = "-"
		seconds = -seconds
	}
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	return fmt.Sprintf("%s%02d:%02d", sign, hours, minutes)
}
