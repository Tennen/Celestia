package runtime

import (
	"context"
	"log"

	"github.com/chentianyu/celestia/internal/models"
)

type workflowScheduledRun struct {
	workflowID        string
	workflowUpdatedAt int64
	timerNodeID       string
	settings          models.AgentSettings
}

func (s *Service) startWorkflowSchedulerWorker() {
	s.workerOnce.Do(func() {
		go s.runWorkflowSchedulerWorker()
	})
}

func buildWorkflowScheduledRuns(workflows []models.AgentWorkflow, dueByWorkflow map[string][]string, settings models.AgentSettings) []workflowScheduledRun {
	out := make([]workflowScheduledRun, 0)
	for _, workflow := range workflows {
		nodeIDs := dueByWorkflow[workflow.ID]
		for _, nodeID := range nodeIDs {
			out = append(out, workflowScheduledRun{
				workflowID:        workflow.ID,
				workflowUpdatedAt: workflow.UpdatedAt.UTC().UnixNano(),
				timerNodeID:       nodeID,
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
		log.Printf("workflow: scheduled run skipped workflow=%s timer=%s: load snapshot failed: %v", job.workflowID, job.timerNodeID, err)
		return
	}
	workflow, ok := selectWorkflow(snapshot.Workflow, job.workflowID)
	if !ok {
		return
	}
	if job.workflowUpdatedAt != 0 && workflow.UpdatedAt.UTC().UnixNano() != job.workflowUpdatedAt {
		return
	}
	if !workflowHasTimerNode(workflow, job.timerNodeID) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), workflowSchedulerTimeout(job.settings))
	defer cancel()
	options := workflowRunOptions{
		TriggeredTimerNode: workflowStringSet([]string{job.timerNodeID}),
	}
	if _, err := s.runWorkflow(ctx, workflow.ID, options); err != nil {
		log.Printf("workflow: scheduled run failed workflow=%s timer=%s: %v", workflow.ID, job.timerNodeID, err)
	}
}

func workflowHasTimerNode(workflow models.AgentWorkflow, timerNodeID string) bool {
	for _, node := range workflow.Nodes {
		if node.ID != timerNodeID || node.Type != workflowNodeTypeTimer {
			continue
		}
		return len(workflowOutgoingEdges(workflow, node.ID)) > 0
	}
	return false
}
