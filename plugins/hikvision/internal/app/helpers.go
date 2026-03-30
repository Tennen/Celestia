package app

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func cloneMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return map[string]any{}
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func stateChanged(a, b map[string]any) bool {
	return !reflect.DeepEqual(a, b)
}

func stringParam(params map[string]any, key string) string {
	if params == nil {
		return ""
	}
	if value, ok := params[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func intParam(params map[string]any, key string, fallback int) int {
	if params == nil {
		return fallback
	}
	switch typed := params[key].(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case int64:
		return int(typed)
	case string:
		if typed == "" {
			return fallback
		}
		if n, err := strconv.Atoi(strings.TrimSpace(typed)); err == nil {
			return n
		}
	}
	return fallback
}

func floatParam(params map[string]any, key string) (float64, bool) {
	if params == nil {
		return 0, false
	}
	switch typed := params[key].(type) {
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case string:
		n, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err == nil {
			return n, true
		}
	}
	return 0, false
}

func parseISOTime(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, fmt.Errorf("time is required")
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err == nil {
		return parsed, nil
	}
	return time.Parse("2006-01-02T15:04:05", value)
}
