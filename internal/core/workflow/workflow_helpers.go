package workflow

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

const (
	workflowNodeTypeGroup              = "group"
	workflowNodeTypeTimer              = "timer"
	workflowNodeTypeDeviceStateChanged = "device_state_changed"
	workflowNodeTypeDeviceStateIs      = "device_state_is"
	workflowNodeTypeTimeWindow         = "time_window"
	workflowNodeTypeRSSSources         = "rss_sources"
	workflowNodeTypeWeather            = "weather"
	workflowNodeTypeText               = "text"
	workflowNodeTypeLLM                = "llm"
	workflowNodeTypeSearchProvider     = "search_provider"
	workflowNodeTypeWeComOutput        = "wecom_output"
	workflowNodeTypeDeviceCommand      = "device_command"
	workflowNodeTypeAgentFunction      = "agent_function"
)

func defaultWorkflowNodeLabel(nodeType string) string {
	switch canonicalWorkflowNodeType(nodeType) {
	case workflowNodeTypeGroup:
		return "Group"
	case workflowNodeTypeRSSSources:
		return "RSS Sources"
	case workflowNodeTypeWeather:
		return "Weather"
	case workflowNodeTypeTimer:
		return "Timer"
	case workflowNodeTypeDeviceStateChanged:
		return "Device State Changed"
	case workflowNodeTypeDeviceStateIs:
		return "Device State Is"
	case workflowNodeTypeTimeWindow:
		return "Time Window"
	case workflowNodeTypeText:
		return "Text"
	case workflowNodeTypeLLM:
		return "LLM"
	case workflowNodeTypeSearchProvider:
		return "Search Provider"
	case workflowNodeTypeWeComOutput:
		return "WeCom Output"
	case workflowNodeTypeDeviceCommand:
		return "Device Command"
	case workflowNodeTypeAgentFunction:
		return "Agent Function"
	default:
		return "Workflow Node"
	}
}

func workflowNodeIsAutonomousTrigger(nodeType string) bool {
	switch canonicalWorkflowNodeType(nodeType) {
	case workflowNodeTypeTimer, workflowNodeTypeDeviceStateChanged, workflowNodeTypeDeviceStateIs:
		return true
	default:
		return false
	}
}

func workflowNodeCannotBeRunTarget(nodeType string) bool {
	switch canonicalWorkflowNodeType(nodeType) {
	case workflowNodeTypeGroup, workflowNodeTypeTimer, workflowNodeTypeDeviceStateChanged, workflowNodeTypeDeviceStateIs, workflowNodeTypeTimeWindow:
		return true
	default:
		return false
	}
}

func canonicalWorkflowNodeType(nodeType string) string {
	return strings.TrimSpace(nodeType)
}

func normalizeWorkflowURL(raw string) string {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return ""
	}
	value = strings.TrimPrefix(value, "http://")
	value = strings.TrimPrefix(value, "https://")
	value = strings.TrimSuffix(value, "/")
	return value
}

func upsertWorkflowSentLog(log []models.AgentWorkflowSentLogItem, items []models.AgentWorkflowItem, sentAt time.Time) []models.AgentWorkflowSentLogItem {
	out := append([]models.AgentWorkflowSentLogItem{}, log...)
	for _, item := range items {
		normalized := normalizeWorkflowURL(item.URL)
		if normalized == "" {
			continue
		}
		out = append([]models.AgentWorkflowSentLogItem{{
			URLNormalized: normalized,
			SentAt:        sentAt,
			Title:         item.Title,
		}}, out...)
	}
	return truncateList(out, 1000)
}

func decodeWorkflowNodeData[T any](data map[string]any) (T, error) {
	var out T
	raw, err := json.Marshal(data)
	if err != nil {
		return out, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return out, nil
	}
	err = json.Unmarshal(raw, &out)
	return out, err
}

func uniqueWorkflowStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, item := range values {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func orderedWorkflowStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, item := range values {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func workflowFirstTime(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value.UTC()
		}
	}
	return time.Now().UTC()
}

func workflowItemsContextJSON(items []models.AgentWorkflowItem) string {
	type workflowContextItem struct {
		Title       string `json:"title,omitempty"`
		GUID        string `json:"guid,omitempty"`
		Source      string `json:"source,omitempty"`
		PublishedAt string `json:"published_at,omitempty"`
		Summary     string `json:"summary,omitempty"`
		URL         string `json:"url,omitempty"`
	}
	if len(items) == 0 {
		return ""
	}
	payload := make([]workflowContextItem, 0, len(items))
	for _, item := range items {
		payload = append(payload, workflowContextItem{
			Title:       strings.TrimSpace(item.Title),
			GUID:        strings.TrimSpace(item.GUID),
			Source:      strings.TrimSpace(item.SourceName),
			PublishedAt: strings.TrimSpace(item.PublishedAt),
			Summary:     strings.TrimSpace(item.Summary),
			URL:         strings.TrimSpace(item.URL),
		})
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(raw)
}

func workflowStringSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		out[trimmed] = struct{}{}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func maxInt(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func truncateList[T any](items []T, limit int) []T {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}
