package pluginutil

func Bool(value any, fallback bool) bool {
	if raw, ok := value.(bool); ok {
		return raw
	}
	return fallback
}

func Int(value any, fallback int) int {
	if raw, ok := value.(float64); ok {
		return int(raw)
	}
	if raw, ok := value.(int); ok {
		return raw
	}
	return fallback
}

func String(value any, fallback string) string {
	if raw, ok := value.(string); ok && raw != "" {
		return raw
	}
	return fallback
}

func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

