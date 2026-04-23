package input

import (
	"context"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

type AgentRuntime interface {
	Converse(context.Context, models.AgentConversationRequest) (models.AgentConversation, error)
	RecordConversationResult(context.Context, models.AgentConversationRequest, string, string, string, map[string]any) (models.AgentConversation, error)
}

type SlashRuntime interface {
	Run(context.Context, models.ProjectInputRequest) (models.SlashCommandResult, bool, error)
}

type Service struct {
	agent AgentRuntime
	slash SlashRuntime
}

func New(agent AgentRuntime, slash SlashRuntime) *Service {
	return &Service{agent: agent, slash: slash}
}

func (s *Service) HandleInput(ctx context.Context, req models.ProjectInputRequest) (models.ProjectInputResult, error) {
	req.Input = strings.TrimSpace(req.Input)
	if req.Input == "" {
		return models.ProjectInputResult{}, errInputRequired()
	}
	if s.slash != nil {
		result, handled, err := s.slash.Run(ctx, req)
		if handled {
			status := "succeeded"
			response := strings.TrimSpace(result.Output)
			if err != nil {
				status = "failed"
				if response == "" {
					response = err.Error()
				}
			}
			conversation, saveErr := s.recordSlashConversation(ctx, req, result, response, status)
			if saveErr != nil {
				return models.ProjectInputResult{}, saveErr
			}
			return models.ProjectInputResult{
				Handled:      true,
				Conversation: conversation,
				ResponseText: conversation.Response,
				Slash:        &result,
			}, err
		}
	}
	conversation, err := s.agent.Converse(ctx, models.AgentConversationRequest{
		SessionID: req.SessionID,
		Input:     req.Input,
		Actor:     req.Actor,
	})
	return models.ProjectInputResult{
		Handled:      false,
		Conversation: conversation,
		ResponseText: conversation.Response,
	}, err
}

func (s *Service) recordSlashConversation(
	ctx context.Context,
	req models.ProjectInputRequest,
	result models.SlashCommandResult,
	response string,
	status string,
) (models.AgentConversation, error) {
	metadata := map[string]any{
		"runtime_mode":      "slash",
		"actor":             firstNonEmpty(req.Actor, req.Source, "slash"),
		"source":            req.Source,
		"slash_command":     result.Command,
		"slash_args":        result.Args,
		"slash_executed_at": result.ExecutedAt,
	}
	for key, value := range result.Metadata {
		metadata[key] = value
	}
	return s.agent.RecordConversationResult(ctx, models.AgentConversationRequest{
		SessionID: req.SessionID,
		Input:     req.Input,
		Actor:     req.Actor,
	}, strings.TrimSpace(req.Input), response, status, metadata)
}

func errInputRequired() error {
	return inputError("input is required")
}

type inputError string

func (e inputError) Error() string {
	return string(e)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
