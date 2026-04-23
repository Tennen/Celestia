package agent

import (
	"context"
	"errors"
	"sort"

	"github.com/chentianyu/celestia/internal/models"
)

func (s *Service) ListAgentCapabilities(ctx context.Context) ([]models.AgentCapabilityInfo, error) {
	runtimes, err := s.buildAgentToolRuntimes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]models.AgentCapabilityInfo, 0, len(runtimes))
	for _, runtime := range runtimes {
		out = append(out, capabilityFromRuntime(runtime))
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *Service) DescribeAgentCapability(ctx context.Context, name string) (models.AgentCapabilityInfo, error) {
	runtime, ok, err := s.agentToolRuntime(ctx, name)
	if err != nil {
		return models.AgentCapabilityInfo{}, err
	}
	if !ok {
		return models.AgentCapabilityInfo{}, errors.New("agent capability not found")
	}
	return capabilityFromRuntime(runtime), nil
}

func (s *Service) RunAgentCapability(
	ctx context.Context,
	name string,
	req models.AgentCapabilityRunRequest,
) (models.AgentCapabilityRunResult, error) {
	runtime, ok, err := s.agentToolRuntime(ctx, name)
	if err != nil {
		return models.AgentCapabilityRunResult{}, err
	}
	if !ok {
		return models.AgentCapabilityRunResult{}, errors.New("agent capability not found")
	}
	args, err := runtime.spec.RequestToJSON(req)
	if err != nil {
		return models.AgentCapabilityRunResult{}, err
	}
	output, runErr := runtime.tool.InvokableRun(ctx, args)
	return models.AgentCapabilityRunResult{
		Capability: runtime.spec.Name,
		Tool:       runtime.spec.Name,
		Action:     "invoke",
		Input:      capabilityRunInput(req),
		Output:     output,
		Metadata:   map[string]any{"tool_args": args},
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

func capabilityRunInput(req models.AgentCapabilityRunRequest) string {
	if req.Input != "" {
		return req.Input
	}
	if len(req.Args) > 0 {
		return marshalToolOutput(req.Args)
	}
	return req.Command
}
