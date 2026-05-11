package runtime

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
)

func (s *Service) CreateApproval(ctx context.Context, req models.AgentApprovalCreateRequest) (models.AgentApprovalRequest, error) {
	kind := strings.ToLower(strings.TrimSpace(req.Kind))
	action := normalizeEvolutionOperation(req.Action)
	if kind == "" {
		kind = "evolution_operation"
	}
	if kind != "evolution_operation" {
		return models.AgentApprovalRequest{}, errors.New("unsupported approval kind")
	}
	if action == "" {
		return models.AgentApprovalRequest{}, errors.New("unsupported approval action")
	}
	now := time.Now().UTC()
	item := models.AgentApprovalRequest{
		ID:          uuid.NewString(),
		Kind:        kind,
		Action:      action,
		GoalID:      strings.TrimSpace(req.GoalID),
		Title:       firstNonEmpty(req.Title, "Approve "+action),
		Detail:      strings.TrimSpace(req.Detail),
		Status:      "pending",
		RequestedBy: strings.TrimSpace(req.RequestedBy),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	_, err := s.update(ctx, func(snapshot *models.AgentSnapshot) error {
		snapshot.Approvals.Requests = append([]models.AgentApprovalRequest{item}, snapshot.Approvals.Requests...)
		snapshot.Approvals.UpdatedAt = now
		snapshot.UpdatedAt = now
		return nil
	})
	return item, err
}

func (s *Service) ApproveApproval(ctx context.Context, id string, req models.AgentApprovalDecisionRequest) (models.AgentApprovalRequest, error) {
	return s.decideApproval(ctx, id, req, true)
}

func (s *Service) RejectApproval(ctx context.Context, id string, req models.AgentApprovalDecisionRequest) (models.AgentApprovalRequest, error) {
	return s.decideApproval(ctx, id, req, false)
}

func (s *Service) decideApproval(ctx context.Context, id string, req models.AgentApprovalDecisionRequest, approved bool) (models.AgentApprovalRequest, error) {
	item, err := s.updateApprovalDecision(ctx, id, req, approved)
	if err != nil || !approved {
		return item, err
	}
	result, runErr := s.RunEvolutionOperation(ctx, EvolutionOperationRequest{Action: item.Action, GoalID: item.GoalID})
	item.Result = &result
	now := time.Now().UTC()
	item.ExecutedAt = &now
	item.UpdatedAt = now
	if runErr != nil {
		item.Status = "failed"
		item.Error = runErr.Error()
	} else {
		item.Status = "executed"
	}
	saved, saveErr := s.replaceApproval(ctx, item)
	if saveErr != nil {
		return models.AgentApprovalRequest{}, saveErr
	}
	if runErr != nil {
		return saved, runErr
	}
	return saved, nil
}

func (s *Service) updateApprovalDecision(ctx context.Context, id string, req models.AgentApprovalDecisionRequest, approved bool) (models.AgentApprovalRequest, error) {
	var out models.AgentApprovalRequest
	_, err := s.update(ctx, func(snapshot *models.AgentSnapshot) error {
		for idx := range snapshot.Approvals.Requests {
			if snapshot.Approvals.Requests[idx].ID != strings.TrimSpace(id) {
				continue
			}
			item := snapshot.Approvals.Requests[idx]
			if item.Status != "pending" {
				return errors.New("approval request is not pending")
			}
			now := time.Now().UTC()
			item.DecidedAt = &now
			item.UpdatedAt = now
			item.DecisionBy = strings.TrimSpace(req.Actor)
			item.DecisionNote = strings.TrimSpace(req.Note)
			if approved {
				item.Status = "approved"
			} else {
				item.Status = "rejected"
			}
			snapshot.Approvals.Requests[idx] = item
			snapshot.Approvals.UpdatedAt = now
			snapshot.UpdatedAt = now
			out = item
			return nil
		}
		return errors.New("approval request not found")
	})
	return out, err
}

func (s *Service) replaceApproval(ctx context.Context, item models.AgentApprovalRequest) (models.AgentApprovalRequest, error) {
	_, err := s.update(ctx, func(snapshot *models.AgentSnapshot) error {
		for idx := range snapshot.Approvals.Requests {
			if snapshot.Approvals.Requests[idx].ID == item.ID {
				snapshot.Approvals.Requests[idx] = item
				snapshot.Approvals.UpdatedAt = item.UpdatedAt
				snapshot.UpdatedAt = item.UpdatedAt
				return nil
			}
		}
		return errors.New("approval request not found")
	})
	return item, err
}
