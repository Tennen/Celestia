package runtime

import (
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

const workflowSourceStateLimit = 1000

type workflowSourceStateUpdate struct {
	Key          string
	RequestState models.AgentWorkflowSourceState
	CommitState  models.AgentWorkflowSourceState
}

func workflowSourceStateSet(states []models.AgentWorkflowSourceState) map[string]models.AgentWorkflowSourceState {
	out := make(map[string]models.AgentWorkflowSourceState, len(states))
	for _, state := range states {
		normalized, key, ok := normalizeWorkflowSourceState(state)
		if !ok {
			continue
		}
		out[key] = normalized
	}
	return out
}

func pruneWorkflowSourceStates(states []models.AgentWorkflowSourceState, workflows []models.AgentWorkflow) []models.AgentWorkflowSourceState {
	allowed := workflowSourceStateAllowSet(workflows)
	out := make([]models.AgentWorkflowSourceState, 0, len(states))
	seen := map[string]struct{}{}
	for _, state := range states {
		normalized, key, ok := normalizeWorkflowSourceState(state)
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
	return truncateList(out, workflowSourceStateLimit)
}

func applyWorkflowSourceStateUpdates(existing []models.AgentWorkflowSourceState, updates []workflowSourceStateUpdate, commitResponses bool) []models.AgentWorkflowSourceState {
	order := make([]string, 0, len(existing)+len(updates))
	byKey := make(map[string]models.AgentWorkflowSourceState, len(existing)+len(updates))
	for _, state := range existing {
		normalized, key, ok := normalizeWorkflowSourceState(state)
		if !ok {
			continue
		}
		if _, seen := byKey[key]; !seen {
			order = append(order, key)
		}
		byKey[key] = normalized
	}
	for _, update := range updates {
		key := strings.TrimSpace(update.Key)
		if key == "" {
			continue
		}
		next := update.RequestState
		if commitResponses {
			next = update.CommitState
		}
		normalized, normalizedKey, ok := normalizeWorkflowSourceState(next)
		if !ok {
			continue
		}
		if _, seen := byKey[normalizedKey]; !seen {
			order = append(order, normalizedKey)
		}
		byKey[normalizedKey] = normalized
	}
	out := make([]models.AgentWorkflowSourceState, 0, len(order))
	for _, key := range order {
		state, ok := byKey[key]
		if !ok {
			continue
		}
		out = append(out, state)
	}
	return truncateList(out, workflowSourceStateLimit)
}

func workflowSourceStateAllowSet(workflows []models.AgentWorkflow) map[string]struct{} {
	out := map[string]struct{}{}
	for _, workflow := range workflows {
		for _, node := range workflow.Nodes {
			if node.Type != workflowNodeTypeRSSSources {
				continue
			}
			config, err := decodeWorkflowNodeData[rssNodeConfig](node.Data)
			if err != nil {
				continue
			}
			for _, source := range config.Sources {
				key := workflowSourceStateKeyForSource(workflow.ID, node.ID, source)
				if key == "" {
					continue
				}
				out[key] = struct{}{}
			}
		}
	}
	return out
}

func normalizeWorkflowSourceState(state models.AgentWorkflowSourceState) (models.AgentWorkflowSourceState, string, bool) {
	state.WorkflowID = strings.TrimSpace(state.WorkflowID)
	state.NodeID = strings.TrimSpace(state.NodeID)
	state.SourceID = strings.TrimSpace(state.SourceID)
	state.FeedURL = strings.TrimSpace(state.FeedURL)
	state.LastModified = strings.TrimSpace(state.LastModified)
	if !state.LastRequestedAt.IsZero() {
		state.LastRequestedAt = state.LastRequestedAt.UTC()
	}
	if !state.UpdatedAt.IsZero() {
		state.UpdatedAt = state.UpdatedAt.UTC()
	} else if !state.LastRequestedAt.IsZero() {
		state.UpdatedAt = state.LastRequestedAt
	}
	key := workflowSourceStateKey(state.WorkflowID, state.NodeID, state.SourceID, state.FeedURL)
	return state, key, key != ""
}

func workflowSourceStateKeyForSource(workflowID string, nodeID string, source models.AgentWorkflowSource) string {
	return workflowSourceStateKey(workflowID, nodeID, strings.TrimSpace(source.ID), strings.TrimSpace(source.FeedURL))
}

func workflowSourceStateKey(workflowID string, nodeID string, sourceID string, feedURL string) string {
	workflowID = strings.TrimSpace(workflowID)
	nodeID = strings.TrimSpace(nodeID)
	sourceID = strings.TrimSpace(sourceID)
	feedURL = normalizeWorkflowURL(feedURL)
	if workflowID == "" || nodeID == "" {
		return ""
	}
	identity := sourceID
	if identity == "" {
		identity = feedURL
	}
	if identity == "" {
		return ""
	}
	return workflowID + "\x1f" + nodeID + "\x1f" + identity
}

func (e *workflowExecutor) sourceStateFor(nodeID string, source models.AgentWorkflowSource) (models.AgentWorkflowSourceState, string) {
	key := workflowSourceStateKeyForSource(e.workflow.ID, nodeID, source)
	if key == "" {
		return models.AgentWorkflowSourceState{}, ""
	}
	if state, ok := e.sourceStates[key]; ok {
		return state, key
	}
	state := models.AgentWorkflowSourceState{
		WorkflowID: e.workflow.ID,
		NodeID:     strings.TrimSpace(nodeID),
		SourceID:   strings.TrimSpace(source.ID),
		FeedURL:    strings.TrimSpace(source.FeedURL),
	}
	return state, key
}

func (e *workflowExecutor) stageSourceStateUpdate(update workflowSourceStateUpdate) {
	key := strings.TrimSpace(update.Key)
	if key == "" {
		return
	}
	e.sourceUpdates[key] = update
	e.sourceStates[key] = update.RequestState
}

func (e *workflowExecutor) sourceStateUpdateList() []workflowSourceStateUpdate {
	updates := make([]workflowSourceStateUpdate, 0, len(e.sourceUpdates))
	for _, update := range e.sourceUpdates {
		updates = append(updates, update)
	}
	return updates
}

func workflowSourcePollInterval(source models.AgentWorkflowSource) time.Duration {
	seconds := source.PollIntervalSeconds
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}
