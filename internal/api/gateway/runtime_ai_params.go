package gateway

import (
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

func cloneAIOptions(input []models.DeviceControlOption) []models.DeviceControlOption {
	if len(input) == 0 {
		return nil
	}
	out := make([]models.DeviceControlOption, len(input))
	copy(out, input)
	return out
}

func cloneAIParamsMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func cloneAINumber(value *float64) *float64 {
	if value == nil {
		return nil
	}
	number := *value
	return &number
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
