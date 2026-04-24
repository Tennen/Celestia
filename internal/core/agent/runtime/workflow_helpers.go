package runtime

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
)

const (
	workflowNodeTypeGroup          = "group"
	workflowNodeTypeRSSSources     = "rss_sources"
	workflowNodeTypePromptUnit     = "prompt_unit"
	workflowNodeTypeLLM            = "llm"
	workflowNodeTypeSearchProvider = "search_provider"
	workflowNodeTypeWeComOutput    = "wecom_output"
)

type legacyWorkflowProfile struct {
	ID        string                       `json:"id"`
	Name      string                       `json:"name"`
	Sources   []models.AgentWorkflowSource `json:"sources"`
	UpdatedAt time.Time                    `json:"updated_at"`
}

func legacyWorkflowProfilesToWorkflows(profiles []legacyWorkflowProfile) []models.AgentWorkflow {
	workflows := make([]models.AgentWorkflow, 0, len(profiles))
	for _, profile := range profiles {
		workflowID := firstNonEmpty(strings.TrimSpace(profile.ID), uuid.NewString())
		workflows = append(workflows, models.AgentWorkflow{
			ID:          workflowID,
			Name:        firstNonEmpty(strings.TrimSpace(profile.Name), workflowID),
			Description: "Migrated from legacy topic profile.",
			Nodes: []models.AgentWorkflowNode{{
				ID:    "rss-" + workflowID,
				Type:  workflowNodeTypeRSSSources,
				Label: "RSS Sources",
				Position: models.AgentNodePoint{
					X: 120,
					Y: 120,
				},
				Data: map[string]any{
					"sources": profile.Sources,
				},
			}},
			Edges:     []models.AgentWorkflowEdge{},
			UpdatedAt: profile.UpdatedAt,
		})
	}
	return workflows
}

func defaultWorkflowNodeLabel(nodeType string) string {
	switch strings.TrimSpace(nodeType) {
	case workflowNodeTypeGroup:
		return "Group"
	case workflowNodeTypeRSSSources:
		return "RSS Sources"
	case workflowNodeTypePromptUnit:
		return "Prompt Unit"
	case workflowNodeTypeLLM:
		return "LLM"
	case workflowNodeTypeSearchProvider:
		return "Search Provider"
	case workflowNodeTypeWeComOutput:
		return "WeCom Output"
	default:
		return "Workflow Node"
	}
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

func workflowSentLogSet(items []models.AgentWorkflowSentLogItem) map[string]struct{} {
	out := make(map[string]struct{}, len(items))
	for _, item := range items {
		if normalized := normalizeWorkflowURL(item.URLNormalized); normalized != "" {
			out[normalized] = struct{}{}
		}
	}
	return out
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
