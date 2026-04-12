package vision

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
)

func (s *Service) EnrichDevice(device models.Device) models.Device {
	config, ok, err := s.store.GetVisionConfig(context.Background())
	if err != nil || !ok || len(config.Rules) == 0 {
		return device
	}
	descriptors := mergeStateDescriptors(device.Metadata, buildVisionStateDescriptors(device.ID, config.Rules))
	if len(descriptors) == 0 {
		return device
	}
	device.Metadata = cloneMap(device.Metadata)
	device.Metadata["state_descriptors"] = descriptors
	return device
}

func defaultConfig() models.VisionCapabilityConfig {
	return models.VisionCapabilityConfig{
		ServiceWSURL:               "",
		ModelName:                  "",
		RecognitionEnabled:         false,
		EventCaptureRetentionHours: models.DefaultVisionEventCaptureRetentionHours,
		Rules:                      []models.VisionRule{},
		UpdatedAt:                  time.Time{},
	}
}

func defaultStatus(config models.VisionCapabilityConfig) models.VisionCapabilityStatus {
	status := models.VisionCapabilityStatus{
		Status:    models.HealthStateStopped,
		Message:   "vision capability not configured",
		Runtime:   map[string]any{},
		UpdatedAt: time.Time{},
	}
	if config.RecognitionEnabled {
		status.Status = models.HealthStateUnknown
		status.Message = "vision capability awaiting websocket session"
	}
	return status
}

func buildVisionStateDescriptors(deviceID string, rules []models.VisionRule) map[string]models.DeviceStateDescriptor {
	descriptors := map[string]models.DeviceStateDescriptor{}
	for _, rule := range rules {
		if rule.CameraDeviceID != deviceID {
			continue
		}
		keys := ruleStateKeys(rule)
		title := strings.TrimSpace(rule.Name)
		if title == "" {
			title = rule.ID
		}
		descriptors[keys.MatchCount] = models.DeviceStateDescriptor{Label: title + " Match Count"}
		descriptors[keys.Active] = models.DeviceStateDescriptor{Label: title + " Active"}
		descriptors[keys.LastEventAt] = models.DeviceStateDescriptor{Label: title + " Last Event At"}
		descriptors[keys.LastEntityValue] = models.DeviceStateDescriptor{Label: title + " Last Entity"}
		descriptors[keys.LastDwellSeconds] = models.DeviceStateDescriptor{Label: title + " Last Dwell Seconds"}
		descriptors[keys.LastStatus] = models.DeviceStateDescriptor{Label: title + " Last Status"}
	}
	return descriptors
}

func mergeStateDescriptors(metadata map[string]any, descriptors map[string]models.DeviceStateDescriptor) map[string]models.DeviceStateDescriptor {
	if len(descriptors) == 0 {
		return nil
	}
	out := map[string]models.DeviceStateDescriptor{}
	if metadata != nil {
		switch current := metadata["state_descriptors"].(type) {
		case map[string]models.DeviceStateDescriptor:
			for key, value := range current {
				out[key] = value
			}
		case map[string]any:
			raw, err := json.Marshal(current)
			if err == nil {
				_ = json.Unmarshal(raw, &out)
			}
		}
	}
	for key, value := range descriptors {
		out[key] = value
	}
	return out
}

type ruleStateKeySet struct {
	MatchCount       string
	Active           string
	LastEventAt      string
	LastEntityValue  string
	LastDwellSeconds string
	LastStatus       string
}

func ruleStateKeys(rule models.VisionRule) ruleStateKeySet {
	base := visionStatePrefix + sanitizeRuleID(rule.ID)
	return ruleStateKeySet{
		MatchCount:       base + "_match_count",
		Active:           base + "_active",
		LastEventAt:      base + "_last_event_at",
		LastEntityValue:  base + "_last_entity_value",
		LastDwellSeconds: base + "_last_dwell_seconds",
		LastStatus:       base + "_last_status",
	}
}

func initialRuleState(rule models.VisionRule) map[string]any {
	keys := ruleStateKeys(rule)
	return map[string]any{
		keys.MatchCount:       0,
		keys.Active:           false,
		keys.LastEventAt:      "",
		keys.LastEntityValue:  "",
		keys.LastDwellSeconds: 0,
		keys.LastStatus:       "",
	}
}

func applyReportedEvent(previous map[string]any, rule models.VisionRule, item models.VisionServiceEvent, observedAt time.Time) map[string]any {
	next := cloneMap(previous)
	for key, value := range initialRuleState(rule) {
		if _, ok := next[key]; !ok {
			next[key] = value
		}
	}
	keys := ruleStateKeys(rule)
	matchCount := intValue(next[keys.MatchCount])
	if item.Status == models.VisionServiceEventStatusThresholdMet {
		matchCount++
		next[keys.Active] = true
		next[keys.MatchCount] = matchCount
	}
	if item.Status == models.VisionServiceEventStatusCleared {
		next[keys.Active] = false
	}
	next[keys.LastEventAt] = observedAt.Format(time.RFC3339Nano)
	next[keys.LastEntityValue] = summarizeReportedEntities(normalizeReportedEntities(item), item.EntityValue)
	next[keys.LastDwellSeconds] = max(item.DwellSeconds, 0)
	next[keys.LastStatus] = string(item.Status)
	return next
}

func changedStateKeys(previousState map[string]any, currentState map[string]any) []string {
	keys := make([]string, 0, len(currentState))
	for key, value := range currentState {
		if !reflect.DeepEqual(previousState[key], value) {
			keys = append(keys, key)
		}
	}
	for key := range previousState {
		if _, ok := currentState[key]; !ok {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	return keys
}

func sanitizeRuleID(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if lastDash {
			continue
		}
		builder.WriteRune('-')
		lastDash = true
	}
	return strings.Trim(builder.String(), "-")
}

func cloneMap(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	raw, err := json.Marshal(src)
	if err != nil {
		out := make(map[string]any, len(src))
		for key, value := range src {
			out[key] = value
		}
		return out
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func intValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func formatNullableTime(ts *time.Time) string {
	if ts == nil || ts.IsZero() {
		return ""
	}
	return ts.UTC().Format(time.RFC3339Nano)
}

func uuidOrNew(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed != "" {
		return trimmed
	}
	return uuid.NewString()
}
