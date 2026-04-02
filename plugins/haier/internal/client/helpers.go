package client

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

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

func trimForError(text string) string {
	text = strings.TrimSpace(text)
	if len(text) > 240 {
		return text[:240] + "..."
	}
	return text
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
