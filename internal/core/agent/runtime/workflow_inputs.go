package runtime

import (
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

type collectedWorkflowInputs struct {
	prompts         []string
	texts           []string
	searches        []string
	items           []models.AgentWorkflowItem
	triggers        int
	blocked         int
	blockedByTimer  int
	blockedByWindow int
}

func (c collectedWorkflowInputs) count() int {
	return len(c.prompts) + len(c.texts) + len(c.searches) + len(c.items) + c.triggers + c.blocked
}

func (c collectedWorkflowInputs) hasActiveContent() bool {
	return len(c.prompts) > 0 || len(c.texts) > 0 || len(c.searches) > 0 || len(c.items) > 0 || c.triggers > 0
}

func (c collectedWorkflowInputs) onlyBlockedByTimer() bool {
	return c.onlyBlocked() && c.blocked == c.blockedByTimer
}

func (c collectedWorkflowInputs) hasBlockingWindow() bool {
	return c.blockedByWindow > 0
}

func (c collectedWorkflowInputs) onlyBlocked() bool {
	return c.blocked > 0 && !c.hasActiveContent()
}

func (e *workflowExecutor) collect(nodeID string, targetHandle string) (collectedWorkflowInputs, error) {
	out := collectedWorkflowInputs{}
	for _, edge := range e.incomingByHandle(nodeID, targetHandle) {
		if !e.shouldCollectEdgeInTriggeredRun(nodeID, edge) {
			continue
		}
		value, err := e.evaluate(edge.Source)
		if err != nil {
			return out, err
		}
		if value.Prompt != "" {
			out.prompts = append(out.prompts, value.Prompt)
		}
		if value.Text != "" {
			out.texts = append(out.texts, value.Text)
		}
		if value.Search != nil {
			out.searches = append(out.searches, workflowSearchResultText(*value.Search))
		}
		if value.Triggered {
			out.triggers++
		}
		if value.Blocked {
			out.blocked++
			if value.BlockedByTimer {
				out.blockedByTimer++
			}
			if value.BlockedByWindow {
				out.blockedByWindow++
			}
		}
		out.items = append(out.items, value.Items...)
	}
	return out, nil
}

func (e *workflowExecutor) incomingByHandle(nodeID string, targetHandle string) []models.AgentWorkflowEdge {
	edges := e.incoming[nodeID]
	if strings.TrimSpace(targetHandle) == "" {
		return append([]models.AgentWorkflowEdge{}, edges...)
	}
	filtered := make([]models.AgentWorkflowEdge, 0, len(edges))
	for _, edge := range edges {
		if strings.TrimSpace(edge.TargetHandle) == strings.TrimSpace(targetHandle) {
			filtered = append(filtered, edge)
		}
	}
	return filtered
}
