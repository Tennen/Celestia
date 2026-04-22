package agent

import (
	"context"
	"errors"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

func (s *Service) commandAgentCapability(ctx context.Context, rest string) (string, bool, error) {
	action, tail := splitWord(rest)
	switch strings.ToLower(action) {
	case "", "list", "ls":
		items, err := s.ListAgentCapabilities(ctx)
		return marshalCommandResult(items), true, err
	case "describe", "get", "show":
		item, err := s.DescribeAgentCapability(ctx, tail)
		return marshalCommandResult(item), true, err
	case "run", "exec":
		name, input := splitWord(tail)
		if name == "" {
			return "", true, errors.New("/agent-capability run requires a capability name")
		}
		result, err := s.RunAgentCapability(ctx, name, models.AgentCapabilityRunRequest{Input: input})
		return marshalCommandResult(result), true, err
	default:
		if strings.TrimSpace(tail) == "" {
			item, err := s.DescribeAgentCapability(ctx, action)
			return marshalCommandResult(item), true, err
		}
		result, err := s.RunAgentCapability(ctx, action, models.AgentCapabilityRunRequest{Input: tail})
		return marshalCommandResult(result), true, err
	}
}

func (s *Service) commandRunAgentCapability(ctx context.Context, name string, input string) (string, bool, error) {
	result, err := s.RunAgentCapability(ctx, name, models.AgentCapabilityRunRequest{Input: input})
	return marshalCommandResult(result), true, err
}
