package runtime

import (
	"context"
	"errors"
	"sort"

	"github.com/chentianyu/celestia/internal/models"
)

func (s *Service) ListAgentTools(ctx context.Context) ([]models.AgentToolInfo, error) {
	runtimes, err := s.buildAgentToolRuntimes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]models.AgentToolInfo, 0, len(runtimes))
	for _, runtime := range runtimes {
		out = append(out, toolInfoFromRuntime(runtime))
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *Service) DescribeAgentTool(ctx context.Context, name string) (models.AgentToolInfo, error) {
	runtime, ok, err := s.agentToolRuntime(ctx, name)
	if err != nil {
		return models.AgentToolInfo{}, err
	}
	if !ok {
		return models.AgentToolInfo{}, errors.New("agent tool not found")
	}
	return toolInfoFromRuntime(runtime), nil
}

func (s *Service) RunAgentTool(
	ctx context.Context,
	name string,
	req models.AgentToolRunRequest,
) (models.AgentToolRunResult, error) {
	runtime, ok, err := s.agentToolRuntime(ctx, name)
	if err != nil {
		return models.AgentToolRunResult{}, err
	}
	if !ok {
		return models.AgentToolRunResult{}, errors.New("agent tool not found")
	}
	args, err := runtime.spec.RequestToJSON(req)
	if err != nil {
		return models.AgentToolRunResult{}, err
	}
	output, runErr := runtime.tool.InvokableRun(ctx, args)
	return models.AgentToolRunResult{
		Tool:     runtime.spec.Name,
		Action:   "invoke",
		Input:    toolRunInput(req),
		Output:   output,
		Metadata: map[string]any{"tool_args": args},
	}, runErr
}

func (s *Service) agentToolRuntime(ctx context.Context, name string) (agentToolRuntime, bool, error) {
	target := normalizeToolName(name)
	runtimes, err := s.buildAgentToolRuntimes(ctx)
	if err != nil {
		return agentToolRuntime{}, false, err
	}
	for _, runtime := range runtimes {
		if normalizeToolName(runtime.spec.Name) == target || normalizeToolName(runtime.info.Name) == target {
			return runtime, true, nil
		}
	}
	return agentToolRuntime{}, false, nil
}

func toolRunInput(req models.AgentToolRunRequest) string {
	if req.Input != "" {
		return req.Input
	}
	if len(req.Args) > 0 {
		return marshalToolOutput(req.Args)
	}
	return req.Command
}
