package runtime

import (
	"context"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

func (s *Service) claimDueWorkflowTimerNodes(ctx context.Context, now time.Time) ([]workflowScheduledRun, models.AgentSettings, error) {
	claimed := []workflowScheduledRun{}
	settings := models.AgentSettings{}
	_, err := s.update(ctx, func(snapshot *models.AgentSnapshot) error {
		settings = snapshot.Settings
		dueByWorkflow := dueWorkflowTimerNodes(snapshot.Workflow, now)
		if len(dueByWorkflow) == 0 {
			return nil
		}
		claimed = buildWorkflowScheduledRuns(snapshot.Workflow.Workflows, dueByWorkflow, settings)
		for workflowID, nodeIDs := range dueByWorkflow {
			snapshot.Workflow.TimerStates = upsertWorkflowTimerState(snapshot.Workflow.TimerStates, workflowID, nodeIDs, now)
		}
		snapshot.Workflow.UpdatedAt = now.UTC()
		snapshot.UpdatedAt = now.UTC()
		return nil
	})
	return claimed, settings, err
}
