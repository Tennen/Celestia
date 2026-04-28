package runtime

import (
	"context"
	"log"
	"time"

	"github.com/chentianyu/celestia/internal/core/timeschedule"
	"github.com/chentianyu/celestia/internal/models"
)

func (s *Service) runWorkflowTimeScheduler() {
	timeschedule.RunLoop(s.stop, 30*time.Second, s.handleWorkflowTimeTick)
}

func (s *Service) handleWorkflowTimeTick(now time.Time) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		log.Printf("workflow: load snapshot for time scheduler failed: %v", err)
		return
	}
	dueByWorkflow := dueWorkflowTimerNodes(snapshot.Workflow, now)
	for workflowID, nodeIDs := range dueByWorkflow {
		options := workflowRunOptions{
			TriggeredTimerNode: workflowStringSet(nodeIDs),
		}
		if _, runErr := s.runWorkflow(ctx, workflowID, options); runErr != nil {
			log.Printf("workflow: scheduled run failed workflow=%s: %v", workflowID, runErr)
		}
		if updateErr := s.updateWorkflowTimerStates(ctx, workflowID, nodeIDs, now); updateErr != nil {
			log.Printf("workflow: persist timer state failed workflow=%s: %v", workflowID, updateErr)
		}
	}
}

func dueWorkflowTimerNodes(snapshot models.AgentWorkflowSnapshot, now time.Time) map[string][]string {
	states := workflowTimerStateSet(snapshot.TimerStates)
	out := map[string][]string{}
	for _, workflow := range snapshot.Workflows {
		for _, node := range workflow.Nodes {
			if node.Type != workflowNodeTypeTimer {
				continue
			}
			if len(workflowOutgoingEdges(workflow, node.ID)) == 0 {
				continue
			}
			config, err := decodeWorkflowNodeData[workflowTimerNodeConfig](node.Data)
			if err != nil {
				continue
			}
			spec, err := normalizeWorkflowTimerConfig(config)
			if err != nil {
				continue
			}
			state := states[workflowTimerStateKey(workflow.ID, node.ID)]
			var lastTriggeredAt *time.Time
			if !state.LastTriggeredAt.IsZero() {
				lastTriggeredAt = &state.LastTriggeredAt
			}
			if !timeschedule.Matches(now, &spec, lastTriggeredAt) {
				continue
			}
			out[workflow.ID] = append(out[workflow.ID], node.ID)
		}
	}
	return out
}

func workflowOutgoingEdges(workflow models.AgentWorkflow, nodeID string) []models.AgentWorkflowEdge {
	out := make([]models.AgentWorkflowEdge, 0)
	for _, edge := range workflow.Edges {
		if edge.Source == nodeID {
			out = append(out, edge)
		}
	}
	return out
}

func (s *Service) updateWorkflowTimerStates(ctx context.Context, workflowID string, nodeIDs []string, triggeredAt time.Time) error {
	if len(nodeIDs) == 0 {
		return nil
	}
	_, err := s.update(ctx, func(snapshot *models.AgentSnapshot) error {
		snapshot.Workflow.TimerStates = upsertWorkflowTimerState(snapshot.Workflow.TimerStates, workflowID, nodeIDs, triggeredAt)
		snapshot.Workflow.UpdatedAt = triggeredAt.UTC()
		snapshot.UpdatedAt = triggeredAt.UTC()
		return nil
	})
	return err
}
