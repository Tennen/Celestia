package runtime

import (
	"context"
	"log"
	"time"

	"github.com/chentianyu/celestia/internal/core/timeschedule"
	"github.com/chentianyu/celestia/internal/models"
)

func (s *Service) runWorkflowTimeScheduler() {
	timeschedule.RunLoop(s.stop, timeschedule.DefaultTickInterval, s.handleWorkflowTimeTick)
}

func (s *Service) handleWorkflowTimeTick(now time.Time) {
	s.startWorkflowSchedulerWorker()
	dueByWorkflow, settings, err := s.claimDueWorkflowTimerNodes(context.Background(), now)
	if err != nil {
		log.Printf("workflow: claim due timers failed: %v", err)
		return
	}
	if len(dueByWorkflow) == 0 {
		return
	}
	s.enqueueWorkflowScheduledRuns(dueByWorkflow, settings)
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
