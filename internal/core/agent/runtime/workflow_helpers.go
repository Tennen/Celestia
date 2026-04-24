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

func workflowItemsText(items []models.AgentWorkflowItem) string {
	lines := make([]string, 0, len(items))
	for idx, item := range items {
		var b strings.Builder
		b.WriteString(intString(idx + 1))
		b.WriteString(". ")
		b.WriteString(firstNonEmpty(strings.TrimSpace(item.Title), "(untitled)"))
		if source := strings.TrimSpace(item.SourceName); source != "" {
			b.WriteString(" [")
			b.WriteString(source)
			b.WriteString("]")
		}
		if summary := strings.TrimSpace(item.Summary); summary != "" {
			b.WriteString("\n")
			b.WriteString(summary)
		}
		if link := strings.TrimSpace(item.URL); link != "" {
			b.WriteString("\n")
			b.WriteString(link)
		}
		lines = append(lines, b.String())
	}
	return strings.Join(lines, "\n\n")
}
