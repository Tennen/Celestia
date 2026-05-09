package workflow

import (
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

const workflowTimerStateLimit = 1000

func workflowTimerStateSet(states []models.AgentWorkflowTimerState) map[string]models.AgentWorkflowTimerState {
	out := make(map[string]models.AgentWorkflowTimerState, len(states))
	for _, state := range states {
		normalized, key, ok := normalizeWorkflowTimerState(state)
		if !ok {
			continue
		}
		out[key] = normalized
	}
	return out
}

func pruneWorkflowTimerStates(states []models.AgentWorkflowTimerState, workflows []models.AgentWorkflow) []models.AgentWorkflowTimerState {
	allowed := workflowTimerStateAllowSet(workflows)
	out := make([]models.AgentWorkflowTimerState, 0, len(states))
	seen := map[string]struct{}{}
	for _, state := range states {
		normalized, key, ok := normalizeWorkflowTimerState(state)
		if !ok {
			continue
		}
		if _, ok := allowed[key]; !ok {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, normalized)
	}
	return truncateList(out, workflowTimerStateLimit)
}

func upsertWorkflowTimerState(states []models.AgentWorkflowTimerState, workflowID string, nodeIDs []string, triggeredAt time.Time) []models.AgentWorkflowTimerState {
	order := make([]string, 0, len(states)+len(nodeIDs))
	byKey := make(map[string]models.AgentWorkflowTimerState, len(states)+len(nodeIDs))
	for _, state := range states {
		normalized, key, ok := normalizeWorkflowTimerState(state)
		if !ok {
			continue
		}
		if _, seen := byKey[key]; !seen {
			order = append(order, key)
		}
		byKey[key] = normalized
	}
	for _, nodeID := range nodeIDs {
		key := workflowTimerStateKey(workflowID, nodeID)
		if key == "" {
			continue
		}
		if _, seen := byKey[key]; !seen {
			order = append(order, key)
		}
		byKey[key] = models.AgentWorkflowTimerState{
			WorkflowID:      strings.TrimSpace(workflowID),
			NodeID:          strings.TrimSpace(nodeID),
			LastTriggeredAt: triggeredAt.UTC(),
			UpdatedAt:       triggeredAt.UTC(),
		}
	}
	out := make([]models.AgentWorkflowTimerState, 0, len(order))
	for _, key := range order {
		state, ok := byKey[key]
		if !ok {
			continue
		}
		out = append(out, state)
	}
	return truncateList(out, workflowTimerStateLimit)
}

func workflowTimerStateAllowSet(workflows []models.AgentWorkflow) map[string]struct{} {
	out := map[string]struct{}{}
	for _, workflow := range workflows {
		for _, node := range workflow.Nodes {
			if node.Type != workflowNodeTypeTimer {
				continue
			}
			key := workflowTimerStateKey(workflow.ID, node.ID)
			if key == "" {
				continue
			}
			out[key] = struct{}{}
		}
	}
	return out
}

func normalizeWorkflowTimerState(state models.AgentWorkflowTimerState) (models.AgentWorkflowTimerState, string, bool) {
	state.WorkflowID = strings.TrimSpace(state.WorkflowID)
	state.NodeID = strings.TrimSpace(state.NodeID)
	if !state.LastTriggeredAt.IsZero() {
		state.LastTriggeredAt = state.LastTriggeredAt.UTC()
	}
	if !state.UpdatedAt.IsZero() {
		state.UpdatedAt = state.UpdatedAt.UTC()
	} else if !state.LastTriggeredAt.IsZero() {
		state.UpdatedAt = state.LastTriggeredAt
	}
	key := workflowTimerStateKey(state.WorkflowID, state.NodeID)
	return state, key, key != ""
}

func workflowTimerStateKey(workflowID string, nodeID string) string {
	workflowID = strings.TrimSpace(workflowID)
	nodeID = strings.TrimSpace(nodeID)
	if workflowID == "" || nodeID == "" {
		return ""
	}
	return workflowID + "\x1f" + nodeID
}
