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
	jobs, _, err := s.claimDueWorkflowTimerNodes(context.Background(), now)
	if err != nil {
		log.Printf("workflow: claim due timers failed: %v", err)
		return
	}
	if len(jobs) == 0 {
		return
	}
	s.enqueueWorkflowScheduledRuns(jobs)
}

func (s *Service) startWorkflowEventTriggers() {
	if s.bus == nil {
		return
	}
	s.eventOnce.Do(func() {
		id, ch := s.bus.Subscribe(128)
		s.eventSubID = id
		s.eventSubscribed = true
		go func() {
			for {
				select {
				case <-s.stop:
					return
				case event, ok := <-ch:
					if !ok {
						return
					}
					if event.Type != models.EventDeviceStateChanged {
						continue
					}
					go s.handleWorkflowStateTriggerEvent(event)
				}
			}
		}()
	})
}

func (s *Service) handleWorkflowStateTriggerEvent(event models.Event) {
	s.startWorkflowSchedulerWorker()
	snapshot, err := s.Snapshot(context.Background())
	if err != nil {
		log.Printf("workflow: state trigger snapshot failed: %v", err)
		return
	}
	now := event.TS
	if now.IsZero() {
		now = time.Now().UTC()
	}
	dueByWorkflow := dueWorkflowStateTriggerNodes(snapshot.Workflow, event, now)
	if len(dueByWorkflow) == 0 {
		return
	}
	jobs := buildWorkflowScheduledRuns(snapshot.Workflow.Workflows, dueByWorkflow, snapshot.Settings, "state", event, now)
	s.enqueueWorkflowScheduledRuns(jobs)
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
			if !workflowTriggerWindowsMatch(workflow, node.ID, now) {
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

func dueWorkflowStateTriggerNodes(snapshot models.AgentWorkflowSnapshot, event models.Event, now time.Time) map[string][]string {
	out := map[string][]string{}
	for _, workflow := range snapshot.Workflows {
		for _, node := range workflow.Nodes {
			if node.Type != workflowNodeTypeDeviceStateChanged && node.Type != workflowNodeTypeDeviceStateIs {
				continue
			}
			if len(workflowOutgoingEdges(workflow, node.ID)) == 0 {
				continue
			}
			if !workflowTriggerWindowsMatch(workflow, node.ID, now) {
				continue
			}
			if workflowStateTriggerMatches(node, event) {
				out[workflow.ID] = append(out[workflow.ID], node.ID)
			}
		}
	}
	return out
}

func workflowStateTriggerMatches(node models.AgentWorkflowNode, event models.Event) bool {
	switch node.Type {
	case workflowNodeTypeDeviceStateChanged:
		config, err := decodeWorkflowNodeData[workflowDeviceStateChangedConfig](node.Data)
		return err == nil && matchesWorkflowStateChangedNode(config, event)
	case workflowNodeTypeDeviceStateIs:
		config, err := decodeWorkflowNodeData[workflowDeviceStateIsConfig](node.Data)
		return err == nil && matchesWorkflowStateIsNode(config, event)
	default:
		return false
	}
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
