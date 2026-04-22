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
	sessionID := firstNonEmpty(req.SessionID, "default")
	resolved, mapped, err := s.ResolveDirectInput(ctx, req.Input)
	if err != nil {
		return models.AgentConversation{}, err
	}
	response, status := "", "succeeded"
	metadata := map[string]any{"direct_input_mapped": mapped, "actor": firstNonEmpty(req.Actor, "agent")}
	if commandResponse, handled, commandErr := s.RunDirectCommand(ctx, resolved); handled {
		response = commandResponse
		if commandErr != nil {
			response = strings.TrimSpace(commandResponse + "\n" + commandErr.Error())
			status = "failed"
			err = commandErr
		}
	} else {
		prompt := resolved
		var routeSnapshot models.AgentSnapshot
		var routeSnapshotOK bool
		if snapshot, snapshotErr := s.Snapshot(ctx); snapshotErr == nil {
			routeSnapshot = snapshot
			routeSnapshotOK = true
			var memoryMetadata map[string]any
			prompt, memoryMetadata = buildMemoryPrompt(snapshot, sessionID, resolved)
			for key, value := range memoryMetadata {
				metadata[key] = value
			}
		}
		if routeSnapshotOK {
			var routeMetadata map[string]any
			var routed bool
			response, routeMetadata, routed, err = s.RouteConversation(ctx, resolved, prompt, routeSnapshot)
			for key, value := range routeMetadata {
				metadata[key] = value
			}
			if routed && err != nil {
				status = "failed"
				response = strings.TrimSpace(response + "\n" + err.Error())
			}
			if !routed {
				response, err = s.GenerateText(ctx, prompt)
			}
		} else {
			response, err = s.GenerateText(ctx, prompt)
		}
		if err != nil && status != "failed" {
			response = err.Error()
			status = "failed"
		}
	}
	now := time.Now().UTC()
	item := models.AgentConversation{
		ID:        uuid.NewString(),
		SessionID: sessionID,
		Input:     strings.TrimSpace(req.Input),
		Resolved:  resolved,
		Response:  response,
		Status:    status,
		Metadata:  metadata,
		CreatedAt: now,
	}
	_, saveErr := s.update(ctx, func(snapshot *models.AgentSnapshot) error {
		for key, value := range appendConversationMemory(snapshot, item, now) {
			item.Metadata[key] = value
		}
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
