package workflow

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

func openMeteoWeatherCodeText(code int) string {
	switch code {
	case 0:
		return "晴"
	case 1, 2, 3:
		return "多云"
	case 45, 48:
		return "雾"
	case 51, 53, 55, 56, 57:
		return "毛毛雨"
	case 61, 63, 65, 66, 67, 80, 81, 82:
		return "雨"
	case 71, 73, 75, 77, 85, 86:
		return "雪"
	case 95, 96, 99:
		return "雷雨"
	default:
		return "未知"
	}
}

func caiyunSkyconText(value string) string {
	switch strings.TrimSpace(value) {
	case "CLEAR_DAY", "CLEAR_NIGHT":
		return "晴"
	case "PARTLY_CLOUDY_DAY", "PARTLY_CLOUDY_NIGHT":
		return "多云"
	case "CLOUDY":
		return "阴"
	case "LIGHT_HAZE", "MODERATE_HAZE", "HEAVY_HAZE":
		return "雾霾"
	case "LIGHT_RAIN", "MODERATE_RAIN", "HEAVY_RAIN", "STORM_RAIN":
		return "雨"
	case "LIGHT_SNOW", "MODERATE_SNOW", "HEAVY_SNOW", "STORM_SNOW":
		return "雪"
	case "FOG":
		return "雾"
	case "WIND":
		return "大风"
	default:
		return firstNonEmpty(value, "未知")
	}
}

func firstWeatherFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	return values[0]
}

func firstWeatherDescription(values []struct {
	Value string `json:"value"`
}) string {
	for _, value := range values {
		if text := strings.TrimSpace(value.Value); text != "" {
			return text
		}
	}
	return ""
}

func maxWttrRainChance(values []struct {
	ChanceOfRain string `json:"chanceofrain"`
}) string {
	maxChance := 0
	for _, value := range values {
		next, err := strconv.Atoi(strings.TrimSpace(value.ChanceOfRain))
		if err == nil && next > maxChance {
			maxChance = next
		}
	}
	return strconv.Itoa(maxChance)
}

func formatWeatherTemperature(value float64) string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "未知"
	}
	return fmt.Sprintf("%.0f℃", value)
}

func formatWeatherPercent(value float64) string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "未知"
	}
	return fmt.Sprintf("%.0f%%", value)
}

func formatWeatherSpeed(value float64) string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "未知"
	}
	return fmt.Sprintf("%.0f km/h", value)
}

func formatWeatherMillimeter(value float64) string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "未知"
	}
	return fmt.Sprintf("%.1f mm", value)
}

func formatWeatherRawTemperature(value string) string {
	if strings.TrimSpace(value) == "" {
		return "未知"
	}
	return strings.TrimSpace(value) + "℃"
}

func formatWeatherRawPercent(value string) string {
	if strings.TrimSpace(value) == "" {
		return "未知"
	}
	return strings.TrimSpace(value) + "%"
}

func formatWeatherRawSpeed(value string) string {
	if strings.TrimSpace(value) == "" {
		return "未知"
	}
	return strings.TrimSpace(value) + " km/h"
}

func formatWeatherRawMillimeter(value string) string {
	if strings.TrimSpace(value) == "" {
		return "未知"
	}
	return strings.TrimSpace(value) + " mm"
}
