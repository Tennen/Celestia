package workflow

import (
	"context"
	"testing"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

func TestWorkflowPersistenceIgnoresLegacyTopicDocuments(t *testing.T) {
	ctx := context.Background()
	svc, store := newWorkflowPersistenceTestService(t)
	currentAt := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	legacyAt := currentAt.Add(time.Minute)
	currentDefinitions := workflowDefinitionsDocument{
		ActiveWorkflowID: "workflow-current",
		Workflows: []models.AgentWorkflow{{
			ID:   "workflow-current",
			Name: "Current Workflow",
			Nodes: []models.AgentWorkflowNode{{
				ID:   "rss-main",
				Type: "rss_sources",
				Data: map[string]any{"sources": []models.AgentWorkflowSource{{ID: "feed-main", FeedURL: "https://rss.test/feed"}}},
			}},
		}},
		UpdatedAt: currentAt,
	}
	currentRuns := workflowRunsDocument{
		SourceStates: []models.AgentWorkflowSourceState{{
			WorkflowID:       "workflow-current",
			NodeID:           "rss-main",
			SourceID:         "feed-main",
			FeedURL:          "https://rss.test/feed",
			LastRequestedAt:  currentAt,
			LastResponseBody: "<rss/>",
			UpdatedAt:        currentAt,
		}},
		UpdatedAt: currentAt,
	}
	if err := writeWorkflowTestDoc(store, workflowDefinitionsDocumentKey, "workflow", currentDefinitions, currentAt); err != nil {
		t.Fatalf("write definitions: %v", err)
	}
	if err := writeWorkflowTestDoc(store, "agent/topic/profiles", "workflow", map[string]any{"active_profile_id": "legacy-profile"}, legacyAt); err != nil {
		t.Fatalf("write legacy definitions: %v", err)
	}
	if err := writeWorkflowTestDoc(store, workflowRunsDocumentKey, "workflow", currentRuns, currentAt); err != nil {
		t.Fatalf("write runs: %v", err)
	}
	if err := writeWorkflowTestDoc(store, "agent/topic/runs", "workflow", map[string]any{"timer_states": []map[string]string{{"workflow_id": "legacy", "node_id": "timer-legacy"}}}, legacyAt); err != nil {
		t.Fatalf("write legacy runs: %v", err)
	}

	snapshot, err := svc.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if snapshot.Workflow.ActiveWorkflowID != "workflow-current" {
		t.Fatalf("active workflow = %q, want workflow-current", snapshot.Workflow.ActiveWorkflowID)
	}
	if len(snapshot.Workflow.Workflows) != 1 || snapshot.Workflow.Workflows[0].ID != "workflow-current" {
		t.Fatalf("workflows = %+v, want only current workflow", snapshot.Workflow.Workflows)
	}
	if len(snapshot.Workflow.SourceStates) != 1 || snapshot.Workflow.SourceStates[0].WorkflowID != "workflow-current" {
		t.Fatalf("source states = %+v, want current workflow source state", snapshot.Workflow.SourceStates)
	}
	if len(snapshot.Workflow.TimerStates) != 0 {
		t.Fatalf("timer states = %+v, want legacy timer states ignored", snapshot.Workflow.TimerStates)
	}
}
