package agent

import (
	"context"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
)

func (s *Service) ResolveDirectInput(ctx context.Context, input string) (string, bool, error) {
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return "", false, err
	}
	normalized := strings.TrimSpace(input)
	for _, rule := range snapshot.DirectInput.Rules {
		if !rule.Enabled || strings.TrimSpace(rule.Pattern) == "" {
			continue
		}
		pattern := strings.TrimSpace(rule.Pattern)
		if rule.MatchMode == "fuzzy" && strings.Contains(strings.ToLower(normalized), strings.ToLower(pattern)) {
			return strings.TrimSpace(rule.TargetText), true, nil
		}
		if rule.MatchMode != "fuzzy" && strings.EqualFold(normalized, pattern) {
			return strings.TrimSpace(rule.TargetText), true, nil
		}
	}
	return normalized, false, nil
}

func (s *Service) Converse(ctx context.Context, req models.AgentConversationRequest) (models.AgentConversation, error) {
	if err := requireText(req.Input, "input"); err != nil {
		return models.AgentConversation{}, err
	}
	resolved, mapped, err := s.ResolveDirectInput(ctx, req.Input)
	if err != nil {
		return models.AgentConversation{}, err
	}
	response, status := "", "succeeded"
	if strings.HasPrefix(strings.TrimSpace(resolved), "/celestia") {
		response = "Celestia command dispatch is available through /api/ai/v1/commands; use the AI command form for device execution."
	} else {
		response, err = s.GenerateText(ctx, resolved)
		if err != nil {
			response = err.Error()
			status = "failed"
		}
	}
	now := time.Now().UTC()
	item := models.AgentConversation{
		ID:        uuid.NewString(),
		SessionID: firstNonEmpty(req.SessionID, "default"),
		Input:     strings.TrimSpace(req.Input),
		Resolved:  resolved,
		Response:  response,
		Status:    status,
		Metadata:  map[string]any{"direct_input_mapped": mapped, "actor": firstNonEmpty(req.Actor, "agent")},
		CreatedAt: now,
	}
	_, saveErr := s.update(ctx, func(snapshot *models.AgentSnapshot) error {
		snapshot.Conversations = append([]models.AgentConversation{item}, snapshot.Conversations...)
		snapshot.Conversations = truncateList(snapshot.Conversations, 80)
		snapshot.UpdatedAt = now
		return nil
	})
	if saveErr != nil {
		return models.AgentConversation{}, saveErr
	}
	return item, err
}
