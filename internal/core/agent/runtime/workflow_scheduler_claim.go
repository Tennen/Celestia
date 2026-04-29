package runtime

import (
	"context"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

func (s *Service) claimDueWorkflowTimerNodes(ctx context.Context, now time.Time) (map[string][]string, models.AgentSettings, error) {
	claimed := map[string][]string{}
	settings := models.AgentSettings{}
	_, err := s.update(ctx, func(snapshot *models.AgentSnapshot) error {
		settings = snapshot.Settings
		claimed = dueWorkflowTimerNodes(snapshot.Workflow, now)
		if len(claimed) == 0 {
			return nil
		}
		for workflowID, nodeIDs := range claimed {
			snapshot.Workflow.TimerStates = upsertWorkflowTimerState(snapshot.Workflow.TimerStates, workflowID, nodeIDs, now)
		}
		snapshot.Workflow.UpdatedAt = now.UTC()
		snapshot.UpdatedAt = now.UTC()
		return nil
	})
	return claimed, settings, err
}
