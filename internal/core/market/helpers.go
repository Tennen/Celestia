package market

import (
	"fmt"
	"strconv"
	"strings"
)

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func stringFrom(value any) string {
	return strings.TrimSpace(fmt.Sprint(value))
}

func parseFloat(value any) float64 {
	number, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(value)), 64)
	return number
}

func floatString(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func maxInt(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}
