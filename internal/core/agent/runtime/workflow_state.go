package runtime

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
)

func (s *Service) SaveWorkflow(ctx context.Context, workflow models.AgentWorkflowSnapshot) (models.AgentSnapshot, error) {
	return s.update(ctx, func(snapshot *models.AgentSnapshot) error {
		now := time.Now().UTC()
		snapshot.Workflow = normalizeWorkflowSnapshot(workflow, now)
		snapshot.UpdatedAt = now
		return nil
	})
}

func normalizeWorkflowSnapshot(workflow models.AgentWorkflowSnapshot, now time.Time) models.AgentWorkflowSnapshot {
	workflow.ActiveWorkflowID = strings.TrimSpace(workflow.ActiveWorkflowID)
	workflow.Workflows = append([]models.AgentWorkflow{}, workflow.Workflows...)
	for idx := range workflow.Workflows {
		workflow.Workflows[idx] = normalizeWorkflowDefinition(workflow.Workflows[idx], now)
	}
	if workflow.ActiveWorkflowID == "" && len(workflow.Workflows) > 0 {
		workflow.ActiveWorkflowID = workflow.Workflows[0].ID
	}
	workflow.Runs = truncateList(workflow.Runs, 50)
	if workflow.Workflows == nil {
		workflow.Workflows = []models.AgentWorkflow{}
	}
	if workflow.Runs == nil {
		workflow.Runs = []models.AgentWorkflowRun{}
	}
	if workflow.SentLog == nil {
		workflow.SentLog = []models.AgentWorkflowSentLogItem{}
	}
	workflow.UpdatedAt = now
	return workflow
}

func normalizeWorkflowDefinition(workflow models.AgentWorkflow, now time.Time) models.AgentWorkflow {
	workflow.ID = firstNonEmpty(strings.TrimSpace(workflow.ID), uuid.NewString())
	workflow.Name = firstNonEmpty(strings.TrimSpace(workflow.Name), workflow.ID)
	workflow.Nodes = append([]models.AgentWorkflowNode{}, workflow.Nodes...)
	workflow.Edges = append([]models.AgentWorkflowEdge{}, workflow.Edges...)
	for idx := range workflow.Nodes {
		workflow.Nodes[idx] = normalizeWorkflowNode(workflow.Nodes[idx], now)
	}
	for idx := range workflow.Edges {
		workflow.Edges[idx] = normalizeWorkflowEdge(workflow.Edges[idx])
	}
	if workflow.Nodes == nil {
		workflow.Nodes = []models.AgentWorkflowNode{}
	}
	if workflow.Edges == nil {
		workflow.Edges = []models.AgentWorkflowEdge{}
	}
	workflow.UpdatedAt = now
	return workflow
}

func normalizeWorkflowNode(node models.AgentWorkflowNode, now time.Time) models.AgentWorkflowNode {
	node.ID = firstNonEmpty(strings.TrimSpace(node.ID), uuid.NewString())
	node.Type = strings.TrimSpace(node.Type)
	node.Label = firstNonEmpty(strings.TrimSpace(node.Label), defaultWorkflowNodeLabel(node.Type))
	if node.Type == workflowNodeTypeGroup {
		if node.Width <= 0 {
			node.Width = 360
		}
		if node.Height <= 0 {
			node.Height = 240
		}
	}
	if node.Data == nil {
		node.Data = map[string]any{}
	}
	_ = now
	return node
}

func normalizeWorkflowEdge(edge models.AgentWorkflowEdge) models.AgentWorkflowEdge {
	edge.ID = firstNonEmpty(strings.TrimSpace(edge.ID), uuid.NewString())
	edge.Source = strings.TrimSpace(edge.Source)
	edge.SourceHandle = strings.TrimSpace(edge.SourceHandle)
	edge.Target = strings.TrimSpace(edge.Target)
	edge.TargetHandle = strings.TrimSpace(edge.TargetHandle)
	edge.Label = strings.TrimSpace(edge.Label)
	return edge
}

func selectWorkflow(snapshot models.AgentWorkflowSnapshot, workflowID string) (models.AgentWorkflow, bool) {
	target := firstNonEmpty(strings.TrimSpace(workflowID), strings.TrimSpace(snapshot.ActiveWorkflowID))
	for _, workflow := range snapshot.Workflows {
		if workflow.ID == target {
			return workflow, true
		}
	}
	if target == "" && len(snapshot.Workflows) > 0 {
		return snapshot.Workflows[0], true
	}
	return models.AgentWorkflow{}, false
}

func (s *Service) appendWorkflowRun(ctx context.Context, run models.AgentWorkflowRun, sentItems []models.AgentWorkflowItem) error {
	if run.ID == "" {
		return errors.New("workflow run id is required")
	}
	_, err := s.update(ctx, func(snapshot *models.AgentSnapshot) error {
		now := workflowFirstTime(run.FinishedAt, run.StartedAt, run.CreatedAt, time.Now().UTC())
		if run.CreatedAt.IsZero() {
			run.CreatedAt = now
		}
		if run.StartedAt.IsZero() {
			run.StartedAt = run.CreatedAt
		}
		snapshot.Workflow.Runs = append([]models.AgentWorkflowRun{run}, snapshot.Workflow.Runs...)
		snapshot.Workflow.Runs = truncateList(snapshot.Workflow.Runs, 50)
		if len(sentItems) > 0 {
			snapshot.Workflow.SentLog = upsertWorkflowSentLog(snapshot.Workflow.SentLog, sentItems, now)
		}
		snapshot.Workflow.UpdatedAt = now
		snapshot.UpdatedAt = now
		return nil
	})
	return err
}
