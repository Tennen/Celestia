package app

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"
)

func stringValue(value any, fallback string) string {
	if s, ok := value.(string); ok && s != "" {
		return s
	}
	return fallback
}

func intValue(value any, fallback int) int {
	switch raw := value.(type) {
	case int:
		return raw
	case int32:
		return int(raw)
	case int64:
		return int(raw)
	case float64:
		return int(raw)
	case json.Number:
		if v, err := raw.Int64(); err == nil {
			return int(v)
		}
	}
	return fallback
}

func boolValue(value any, fallback bool) bool {
	if b, ok := value.(bool); ok {
		return b
	}
	return fallback
}

func stateEqual(left, right map[string]any) bool {
	return reflect.DeepEqual(left, right)
}

func timezoneOffset(timezone string) string {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return "+00:00"
	}
	_, offset := time.Now().In(loc).Zone()
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	return formatOffset(sign, offset)
}

func formatOffset(sign string, offsetSeconds int) string {
	hours := offsetSeconds / 3600
	minutes := (offsetSeconds % 3600) / 60
	return fmt.Sprintf("%s%02d:%02d", sign, hours, minutes)
}
