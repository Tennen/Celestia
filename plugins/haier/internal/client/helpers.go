package client

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strconv"
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
	case int64:
		return strconv.FormatInt(raw, 10)
	case int32:
		return strconv.FormatInt(int64(raw), 10)
	case float32:
		if raw == float32(int64(raw)) {
			return fmt.Sprintf("%d", int64(raw))
		}
		return fmt.Sprintf("%v", raw)
	case bool:
		return strconv.FormatBool(raw)
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func randomHex(n int) string {
	const chars = "abcdef0123456789"
	b := make([]byte, n)
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err == nil {
		for i := range b {
			b[i] = chars[int(raw[i])%len(chars)]
		}
		return string(b)
	}
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
