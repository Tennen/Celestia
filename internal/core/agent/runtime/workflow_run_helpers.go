package runtime

import (
	"fmt"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

func buildWorkflowLLMPrompt(promptText string, userPrompt string, contextText string, searchText string) string {
	sections := make([]string, 0, 4)
	if strings.TrimSpace(contextText) != "" {
		sections = append(sections, "Workflow Input:\n"+strings.TrimSpace(contextText))
	}
	if strings.TrimSpace(searchText) != "" {
		sections = append(sections, "Search Results:\n"+strings.TrimSpace(searchText))
	}
	if strings.TrimSpace(promptText) != "" {
		sections = append(sections, "Text Blocks:\n"+strings.TrimSpace(promptText))
	}
	if strings.TrimSpace(userPrompt) != "" {
		sections = append(sections, "User Prompt:\n"+strings.TrimSpace(userPrompt))
	}
	return strings.Join(sections, "\n\n")
}

func workflowSearchResultText(result models.AgentSearchResult) string {
	lines := make([]string, 0, len(result.Items))
	for idx, item := range result.Items {
		line := fmt.Sprintf("%d. %s", idx+1, firstNonEmpty(strings.TrimSpace(item.Title), "(untitled)"))
		if link := strings.TrimSpace(item.Link); link != "" {
			line += "\n" + link
		}
		if snippet := strings.TrimSpace(item.Snippet); snippet != "" {
			line += "\n" + snippet
		}
		lines = append(lines, line)
	}
	if len(result.Errors) > 0 {
		lines = append(lines, "Errors: "+strings.Join(result.Errors, "; "))
	}
	return strings.Join(lines, "\n\n")
}

func workflowRunStatus(run models.AgentWorkflowRun) string {
	failures := 0
	for _, result := range run.NodeResults {
		if result.Status == "failed" {
			failures++
		}
	}
	switch {
	case failures == len(run.NodeResults) && failures > 0:
		return "failed"
	case failures > 0 || len(run.FetchErrors) > 0 || len(run.DeliveryErrors) > 0:
		return "degraded"
	default:
		return "succeeded"
	}
}

func workflowRunSummary(workflowName string, run models.AgentWorkflowRun) string {
	switch run.Status {
	case "succeeded":
		return fmt.Sprintf("Workflow %s completed with %d items.", workflowName, len(run.Items))
	case "degraded":
		return fmt.Sprintf("Workflow %s completed with issues across %d nodes.", workflowName, len(run.NodeResults))
	default:
		return fmt.Sprintf("Workflow %s failed.", workflowName)
	}
}

func truncateWorkflowItems(items []models.AgentWorkflowItem, limit int) []models.AgentWorkflowItem {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}
