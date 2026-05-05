package runtime

import (
	"context"
	"log"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

type workflowScheduledRun struct {
	workflowID        string
	workflowUpdatedAt int64
	triggerNodeID     string
	triggerKind       string
	sourceEvent       models.Event
	triggeredAt       time.Time
	settings          models.AgentSettings
}

func (s *Service) startWorkflowSchedulerWorker() {
	s.workerOnce.Do(func() {
		go s.runWorkflowSchedulerWorker()
	})
}

func buildWorkflowScheduledRuns(workflows []models.AgentWorkflow, dueByWorkflow map[string][]string, settings models.AgentSettings, triggerKind string, sourceEvent models.Event, triggeredAt time.Time) []workflowScheduledRun {
	out := make([]workflowScheduledRun, 0)
	for _, workflow := range workflows {
		nodeIDs := dueByWorkflow[workflow.ID]
		for _, nodeID := range nodeIDs {
			out = append(out, workflowScheduledRun{
				workflowID:        workflow.ID,
				workflowUpdatedAt: workflow.UpdatedAt.UTC().UnixNano(),
				triggerNodeID:     nodeID,
				triggerKind:       triggerKind,
				sourceEvent:       sourceEvent,
				triggeredAt:       triggeredAt.UTC(),
				settings:          settings,
			})
		}
	}
	return out
}

func (s *Service) enqueueWorkflowScheduledRuns(jobs []workflowScheduledRun) {
	for _, job := range jobs {
		select {
		case <-s.stop:
			return
		case s.workflowJobs <- job:
		}
	}
}

func (s *Service) runWorkflowSchedulerWorker() {
	for {
		select {
		case <-s.stop:
			return
		case job := <-s.workflowJobs:
			s.runWorkflowScheduledJob(job)
		}
	}
}

func (s *Service) runWorkflowScheduledJob(job workflowScheduledRun) {
	snapshot, err := s.Snapshot(context.Background())
	if err != nil {
		log.Printf("workflow: scheduled run skipped workflow=%s trigger=%s: load snapshot failed: %v", job.workflowID, job.triggerNodeID, err)
		return
	}
	workflow, ok := selectWorkflow(snapshot.Workflow, job.workflowID)
	if !ok {
		return
	}
	if job.workflowUpdatedAt != 0 && workflow.UpdatedAt.UTC().UnixNano() != job.workflowUpdatedAt {
		return
	}
	if !workflowHasTriggerNode(workflow, job.triggerNodeID, job.triggerKind) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), workflowSchedulerTimeout(job.settings))
	defer cancel()
	options := workflowRunOptions{
		TriggeredNode: workflowStringSet([]string{job.triggerNodeID}),
		SourceEvent:   job.sourceEvent,
		TriggeredAt:   job.triggeredAt,
	}
	if _, err := s.runWorkflow(ctx, workflow.ID, options); err != nil {
		log.Printf("workflow: scheduled run failed workflow=%s trigger=%s: %v", workflow.ID, job.triggerNodeID, err)
	}
}

func workflowHasTriggerNode(workflow models.AgentWorkflow, triggerNodeID string, triggerKind string) bool {
	for _, node := range workflow.Nodes {
		if node.ID != triggerNodeID || !workflowNodeKindMatches(node.Type, triggerKind) {
			continue
		}
		return len(workflowOutgoingEdges(workflow, node.ID)) > 0
	}
	return false
}

func workflowNodeKindMatches(nodeType string, triggerKind string) bool {
	switch triggerKind {
	case "timer":
		return nodeType == workflowNodeTypeTimer
	case "state":
		return nodeType == workflowNodeTypeDeviceStateChanged || nodeType == workflowNodeTypeDeviceStateIs
	default:
		return workflowNodeIsAutonomousTrigger(nodeType)
	}
}
