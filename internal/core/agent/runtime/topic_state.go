package runtime

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
)

func (s *Service) SaveTopic(ctx context.Context, topic models.AgentTopicSnapshot) (models.AgentSnapshot, error) {
	return s.update(ctx, func(snapshot *models.AgentSnapshot) error {
		now := time.Now().UTC()
		snapshot.TopicSummary = normalizeTopicSnapshot(topic, now)
		snapshot.UpdatedAt = now
		return nil
	})
}

func normalizeTopicSnapshot(topic models.AgentTopicSnapshot, now time.Time) models.AgentTopicSnapshot {
	topic.ActiveWorkflowID = firstNonEmpty(strings.TrimSpace(topic.ActiveWorkflowID), strings.TrimSpace(topic.ActiveProfileID))
	topic.ActiveProfileID = topic.ActiveWorkflowID
	topic.Workflows = append([]models.AgentTopicWorkflow{}, topic.Workflows...)
	for idx := range topic.Workflows {
		topic.Workflows[idx] = normalizeTopicWorkflow(topic.Workflows[idx], now)
	}
	if topic.ActiveWorkflowID == "" && len(topic.Workflows) > 0 {
		topic.ActiveWorkflowID = topic.Workflows[0].ID
		topic.ActiveProfileID = topic.ActiveWorkflowID
	}
	topic.Runs = truncateList(topic.Runs, 50)
	if topic.Workflows == nil {
		topic.Workflows = []models.AgentTopicWorkflow{}
	}
	if topic.Runs == nil {
		topic.Runs = []models.AgentTopicRun{}
	}
	if topic.SentLog == nil {
		topic.SentLog = []models.AgentTopicSentLogItem{}
	}
	topic.UpdatedAt = now
	return topic
}

func normalizeTopicWorkflow(workflow models.AgentTopicWorkflow, now time.Time) models.AgentTopicWorkflow {
	workflow.ID = firstNonEmpty(strings.TrimSpace(workflow.ID), uuid.NewString())
	workflow.Name = firstNonEmpty(strings.TrimSpace(workflow.Name), workflow.ID)
	workflow.Nodes = append([]models.AgentTopicNode{}, workflow.Nodes...)
	workflow.Edges = append([]models.AgentTopicEdge{}, workflow.Edges...)
	for idx := range workflow.Nodes {
		workflow.Nodes[idx] = normalizeTopicNode(workflow.Nodes[idx], now)
	}
	for idx := range workflow.Edges {
		workflow.Edges[idx] = normalizeTopicEdge(workflow.Edges[idx])
	}
	if workflow.Nodes == nil {
		workflow.Nodes = []models.AgentTopicNode{}
	}
	if workflow.Edges == nil {
		workflow.Edges = []models.AgentTopicEdge{}
	}
	workflow.UpdatedAt = now
	return workflow
}

func normalizeTopicNode(node models.AgentTopicNode, now time.Time) models.AgentTopicNode {
	node.ID = firstNonEmpty(strings.TrimSpace(node.ID), uuid.NewString())
	node.Type = strings.TrimSpace(node.Type)
	node.Label = firstNonEmpty(strings.TrimSpace(node.Label), defaultTopicNodeLabel(node.Type))
	if node.Type == topicNodeTypeGroup {
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

func normalizeTopicEdge(edge models.AgentTopicEdge) models.AgentTopicEdge {
	edge.ID = firstNonEmpty(strings.TrimSpace(edge.ID), uuid.NewString())
	edge.Source = strings.TrimSpace(edge.Source)
	edge.SourceHandle = strings.TrimSpace(edge.SourceHandle)
	edge.Target = strings.TrimSpace(edge.Target)
	edge.TargetHandle = strings.TrimSpace(edge.TargetHandle)
	edge.Label = strings.TrimSpace(edge.Label)
	return edge
}

func selectTopicWorkflow(snapshot models.AgentTopicSnapshot, workflowID string) (models.AgentTopicWorkflow, bool) {
	target := firstNonEmpty(strings.TrimSpace(workflowID), strings.TrimSpace(snapshot.ActiveWorkflowID), strings.TrimSpace(snapshot.ActiveProfileID))
	for _, workflow := range snapshot.Workflows {
		if workflow.ID == target {
			return workflow, true
		}
	}
	if target == "" && len(snapshot.Workflows) > 0 {
		return snapshot.Workflows[0], true
	}
	return models.AgentTopicWorkflow{}, false
}

func (s *Service) appendTopicRun(ctx context.Context, run models.AgentTopicRun, sentItems []models.AgentTopicItem) error {
	if run.ID == "" {
		return errors.New("topic run id is required")
	}
	_, err := s.update(ctx, func(snapshot *models.AgentSnapshot) error {
		now := topicFirstTime(run.FinishedAt, run.StartedAt, run.CreatedAt, time.Now().UTC())
		run.WorkflowID = firstNonEmpty(strings.TrimSpace(run.WorkflowID), strings.TrimSpace(run.ProfileID))
		run.ProfileID = run.WorkflowID
		if run.CreatedAt.IsZero() {
			run.CreatedAt = now
		}
		if run.StartedAt.IsZero() {
			run.StartedAt = run.CreatedAt
		}
		snapshot.TopicSummary.Runs = append([]models.AgentTopicRun{run}, snapshot.TopicSummary.Runs...)
		snapshot.TopicSummary.Runs = truncateList(snapshot.TopicSummary.Runs, 50)
		if len(sentItems) > 0 {
			snapshot.TopicSummary.SentLog = upsertTopicSentLog(snapshot.TopicSummary.SentLog, sentItems, now)
		}
		snapshot.TopicSummary.UpdatedAt = now
		snapshot.UpdatedAt = now
		return nil
	})
	return err
}
