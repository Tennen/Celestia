package runtime

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
)

const (
	topicNodeTypeGroup          = "group"
	topicNodeTypeRSSSources     = "rss_sources"
	topicNodeTypePromptUnit     = "prompt_unit"
	topicNodeTypeLLM            = "llm"
	topicNodeTypeSearchProvider = "search_provider"
	topicNodeTypeWeComOutput    = "wecom_output"
)

type legacyTopicProfile struct {
	ID        string                    `json:"id"`
	Name      string                    `json:"name"`
	Sources   []models.AgentTopicSource `json:"sources"`
	UpdatedAt time.Time                 `json:"updated_at"`
}

func legacyTopicProfilesToWorkflows(profiles []legacyTopicProfile) []models.AgentTopicWorkflow {
	workflows := make([]models.AgentTopicWorkflow, 0, len(profiles))
	for _, profile := range profiles {
		workflowID := firstNonEmpty(strings.TrimSpace(profile.ID), uuid.NewString())
		workflows = append(workflows, models.AgentTopicWorkflow{
			ID:          workflowID,
			Name:        firstNonEmpty(strings.TrimSpace(profile.Name), workflowID),
			Description: "Migrated from legacy topic profile.",
			Nodes: []models.AgentTopicNode{{
				ID:    "rss-" + workflowID,
				Type:  topicNodeTypeRSSSources,
				Label: "RSS Sources",
				Position: models.AgentNodePoint{
					X: 120,
					Y: 120,
				},
				Data: map[string]any{
					"sources": profile.Sources,
				},
			}},
			Edges:     []models.AgentTopicEdge{},
			UpdatedAt: profile.UpdatedAt,
		})
	}
	return workflows
}

func defaultTopicNodeLabel(nodeType string) string {
	switch strings.TrimSpace(nodeType) {
	case topicNodeTypeGroup:
		return "Group"
	case topicNodeTypeRSSSources:
		return "RSS Sources"
	case topicNodeTypePromptUnit:
		return "Prompt Unit"
	case topicNodeTypeLLM:
		return "LLM"
	case topicNodeTypeSearchProvider:
		return "Search Provider"
	case topicNodeTypeWeComOutput:
		return "WeCom Output"
	default:
		return "Workflow Node"
	}
}

func normalizeTopicURL(raw string) string {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return ""
	}
	value = strings.TrimPrefix(value, "http://")
	value = strings.TrimPrefix(value, "https://")
	value = strings.TrimSuffix(value, "/")
	return value
}

func topicSentLogSet(items []models.AgentTopicSentLogItem) map[string]struct{} {
	out := make(map[string]struct{}, len(items))
	for _, item := range items {
		if normalized := normalizeTopicURL(item.URLNormalized); normalized != "" {
			out[normalized] = struct{}{}
		}
	}
	return out
}

func upsertTopicSentLog(log []models.AgentTopicSentLogItem, items []models.AgentTopicItem, sentAt time.Time) []models.AgentTopicSentLogItem {
	out := append([]models.AgentTopicSentLogItem{}, log...)
	for _, item := range items {
		normalized := normalizeTopicURL(item.URL)
		if normalized == "" {
			continue
		}
		out = append([]models.AgentTopicSentLogItem{{
			URLNormalized: normalized,
			SentAt:        sentAt,
			Title:         item.Title,
		}}, out...)
	}
	return truncateList(out, 1000)
}

func decodeTopicNodeData[T any](data map[string]any) (T, error) {
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

func uniqueTopicStrings(values []string) []string {
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

func topicFirstTime(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value.UTC()
		}
	}
	return time.Now().UTC()
}

func topicItemsText(items []models.AgentTopicItem) string {
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
