package slash

import (
	"context"
	"errors"
	"strings"

	coreagent "github.com/chentianyu/celestia/internal/core/agent"
)

func (s *Service) runServiceOperation(ctx context.Context, args []string) (string, map[string]any, error) {
	if s.agent == nil {
		return "", nil, errors.New("agent runtime is not configured")
	}
	action := "status"
	if len(args) > 0 {
		action = args[0]
	}
	params, _, err := parseSlashParams(args[1:])
	if err != nil {
		return "", nil, err
	}
	result, err := s.agent.RunServiceOperation(ctx, coreagent.ServiceOperationRequest{
		Action: action,
		Lines:  intParam(params, "lines"),
	})
	output := strings.TrimSpace(result.Output)
	if output == "" {
		output = "service operation completed"
	}
	return output, map[string]any{"domain": "service", "action": action}, err
}
