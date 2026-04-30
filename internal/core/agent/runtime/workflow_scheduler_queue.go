package runtime

import (
	"context"
	"log"

	"github.com/chentianyu/celestia/internal/models"
)

type workflowScheduledRun struct {
	workflowID  string
	timerNodeID string
	settings    models.AgentSettings
}

func (s *Service) startWorkflowSchedulerWorker() {
	s.workerOnce.Do(func() {
		go s.runWorkflowSchedulerWorker()
	})
}

func (s *Service) enqueueWorkflowScheduledRuns(dueByWorkflow map[string][]string, settings models.AgentSettings) {
	for workflowID, nodeIDs := range dueByWorkflow {
		for _, nodeID := range nodeIDs {
			job := workflowScheduledRun{
				workflowID:  workflowID,
				timerNodeID: nodeID,
				settings:    settings,
			}
			select {
			case <-s.stop:
				return
			case s.workflowJobs <- job:
			}
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
	ctx, cancel := context.WithTimeout(context.Background(), workflowSchedulerTimeout(job.settings))
	defer cancel()
	options := workflowRunOptions{
		TriggeredTimerNode: workflowStringSet([]string{job.timerNodeID}),
	}
	if _, err := s.runWorkflow(ctx, job.workflowID, options); err != nil {
		log.Printf("workflow: scheduled run failed workflow=%s timer=%s: %v", job.workflowID, job.timerNodeID, err)
	}
}
