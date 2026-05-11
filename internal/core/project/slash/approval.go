package slash

import (
	"context"
	"errors"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

func (s *Service) runApproval(ctx context.Context, req models.ProjectInputRequest, args []string, approved bool) (string, map[string]any, error) {
	if s.agent == nil {
		return "", nil, errors.New("agent runtime is not configured")
	}
	if len(args) == 0 || equalRef(args[0], "list") {
		snapshot, err := s.agent.Snapshot(ctx)
		if err != nil {
			return "", nil, err
		}
		lines := pendingApprovalLines(snapshot.Approvals.Requests)
		if len(lines) == 0 {
			lines = []string{"No pending approvals."}
		}
		return strings.Join(lines, "\n"), map[string]any{"domain": "approval"}, nil
	}
	id := strings.TrimSpace(args[0])
	decision := models.AgentApprovalDecisionRequest{Actor: actorOrInput(req)}
	var (
		item models.AgentApprovalRequest
		err  error
	)
	if approved {
		item, err = s.agent.ApproveApproval(ctx, id, decision)
	} else {
		item, err = s.agent.RejectApproval(ctx, id, decision)
	}
	return formatApproval(item), map[string]any{"domain": "approval", "approval_id": item.ID}, err
}

func formatApproval(item models.AgentApprovalRequest) string {
	lines := []string{
		"Approval: " + item.ID,
		"Status: " + item.Status,
		"Action: " + item.Action,
	}
	if item.Error != "" {
		lines = append(lines, "Error: "+item.Error)
	}
	if item.Result != nil && item.Result.Output != "" {
		lines = append(lines, item.Result.Output)
	}
	return strings.Join(lines, "\n")
}
