package app

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/chentianyu/celestia/internal/pluginutil"
)

func parseAccountConfig(entry map[string]any) AccountConfig {
	return AccountConfig{
		Name:         pluginutil.String(entry["name"], ""),
		Email:        firstNonEmpty(pluginutil.String(entry["email"], ""), pluginutil.String(entry["username"], "")),
		Password:     pluginutil.String(entry["password"], ""),
		RefreshToken: firstNonEmpty(pluginutil.String(entry["refresh_token"], ""), pluginutil.String(entry["refreshToken"], "")),
		MobileID:     firstNonEmpty(pluginutil.String(entry["mobile_id"], ""), pluginutil.String(entry["mobileId"], "")),
		Timezone:     pluginutil.String(entry["timezone"], ""),
	}
}

func extractParameters(raw map[string]any) map[string]any {
	if raw == nil {
		return map[string]any{}
	}
	if shadow, ok := raw["shadow"].(map[string]any); ok {
		if params, ok := shadow["parameters"].(map[string]any); ok {
			return params
		}
	}
	if params, ok := raw["parameters"].(map[string]any); ok {
		return params
	}
	return map[string]any{}
}

func applianceOnline(appliance map[string]any) bool {
	if online, ok := appliance["online"].(bool); ok {
		return online
	}
	if connection, ok := appliance["connection"].(bool); ok {
		return connection
	}
	if lastConn, ok := appliance["lastConnEvent"].(map[string]any); ok {
		if category := stringFromAny(lastConn["category"]); strings.EqualFold(category, "DISCONNECTED") {
			return false
		}
	}
	return true
}

func cloneMap(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func mergeMap(dst map[string]any, src map[string]any) {
	for k, v := range src {
		if _, exists := dst[k]; !exists {
			dst[k] = v
		}
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func sanitizeID(value string) string {
	value = strings.ToLower(value)
	value = strings.ReplaceAll(value, ":", "-")
	value = strings.ReplaceAll(value, " ", "-")
	return value
}

func intFromAny(v any) int {
	switch raw := v.(type) {
	case int:
		return raw
	case int32:
		return int(raw)
	case int64:
		return int(raw)
	case float32:
		return int(raw)
	case float64:
		return int(raw)
	case json.Number:
		i, _ := raw.Int64()
		return int(i)
	case string:
		var i int
		_, _ = fmt.Sscanf(raw, "%d", &i)
		return i
	default:
		return 0
	}
}

func floatFromAny(v any) float64 {
	switch raw := v.(type) {
	case float64:
		return raw
	case float32:
		return float64(raw)
	case int:
		return float64(raw)
	case int64:
		return float64(raw)
	case json.Number:
		f, _ := raw.Float64()
		return f
	case string:
		var f float64
		_, _ = fmt.Sscanf(raw, "%f", &f)
		return f
	default:
		return -1
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
