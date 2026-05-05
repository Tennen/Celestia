package runtime

import (
	"testing"

	"github.com/chentianyu/celestia/internal/models"
)

func workflowRSSOnlyDefinition() models.AgentWorkflow {
	return models.AgentWorkflow{
		ID:   "workflow-rss-stateful",
		Name: "Stateful RSS Workflow",
		Nodes: []models.AgentWorkflowNode{{
			ID:    "rss-main",
			Type:  workflowNodeTypeRSSSources,
			Label: "RSS Sources",
			Position: models.AgentNodePoint{
				X: 80,
				Y: 80,
			},
			Data: map[string]any{
				"sources": []models.AgentWorkflowSource{{
					ID:       "feed-main",
					Name:     "Main Feed",
					Category: "news",
					FeedURL:  "https://rss.test/feed",
					Weight:   1,
					Enabled:  true,
				}},
			},
		}},
		Edges: []models.AgentWorkflowEdge{},
	}
}

func workflowTimerRSSDefinition(at string, timezone string) models.AgentWorkflow {
	workflow := workflowRSSOnlyDefinition()
	workflow.ID = "workflow-rss-timer"
	workflow.Name = "Timed RSS Workflow"
	workflow.Nodes = append([]models.AgentWorkflowNode{{
		ID:    "timer-main",
		Type:  workflowNodeTypeTimer,
		Label: "Timer",
		Position: models.AgentNodePoint{
			X: 80,
			Y: 20,
		},
		Data: map[string]any{
			"schedule": "daily",
			"at":       at,
			"timezone": timezone,
		},
	}}, workflow.Nodes...)
	workflow.Edges = []models.AgentWorkflowEdge{{
		ID:           "edge-timer-rss",
		Source:       "timer-main",
		SourceHandle: "trigger",
		Target:       "rss-main",
		TargetHandle: "trigger",
	}}
	return workflow
}

func workflowIntervalTimerRSSDefinition(windowStart string, windowEnd string, intervalSeconds int, timezone string) models.AgentWorkflow {
	workflow := workflowRSSOnlyDefinition()
	workflow.ID = "workflow-rss-timer-interval"
	workflow.Name = "Interval RSS Workflow"
	workflow.Nodes = append([]models.AgentWorkflowNode{{
		ID:    "timer-main",
		Type:  workflowNodeTypeTimer,
		Label: "Timer",
		Position: models.AgentNodePoint{
			X: 80,
			Y: 20,
		},
		Data: map[string]any{
			"schedule":         "interval",
			"interval_seconds": intervalSeconds,
			"timezone":         timezone,
		},
	}, {
		ID:    "window-main",
		Type:  workflowNodeTypeTimeWindow,
		Label: "Time Window",
		Position: models.AgentNodePoint{
			X: 260,
			Y: 20,
		},
		Data: map[string]any{
			"start":    windowStart,
			"end":      windowEnd,
			"timezone": timezone,
		},
	}}, workflow.Nodes...)
	workflow.Edges = []models.AgentWorkflowEdge{{
		ID:           "edge-timer-rss",
		Source:       "timer-main",
		SourceHandle: "trigger",
		Target:       "rss-main",
		TargetHandle: "trigger",
	}, {
		ID:           "edge-window-rss",
		Source:       "window-main",
		SourceHandle: "gate",
		Target:       "rss-main",
		TargetHandle: "trigger",
	}}
	return workflow
}

func workflowResultMetadataInt(t *testing.T, run models.AgentWorkflowRun, nodeID string, key string) int {
	t.Helper()
	result := workflowNodeResult(t, run, nodeID)
	value, ok := result.Metadata[key]
	if !ok {
		t.Fatalf("metadata %q not found on node %s", key, nodeID)
	}
	got, ok := value.(int)
	if !ok {
		t.Fatalf("metadata %q type = %T, want int", key, value)
	}
	return got
}

func workflowResultMetadataBool(t *testing.T, run models.AgentWorkflowRun, nodeID string, key string) bool {
	t.Helper()
	result := workflowNodeResult(t, run, nodeID)
	value, ok := result.Metadata[key]
	if !ok {
		t.Fatalf("metadata %q not found on node %s", key, nodeID)
	}
	got, ok := value.(bool)
	if !ok {
		t.Fatalf("metadata %q type = %T, want bool", key, value)
	}
	return got
}

func workflowNodeResult(t *testing.T, run models.AgentWorkflowRun, nodeID string) models.AgentWorkflowNodeResult {
	t.Helper()
	for _, result := range run.NodeResults {
		if result.NodeID != nodeID {
			continue
		}
		return result
	}
	t.Fatalf("node result for %s not found", nodeID)
	return models.AgentWorkflowNodeResult{}
}
