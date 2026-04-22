package agent

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	agentcaps "github.com/chentianyu/celestia/internal/core/agent/capabilities"
	"github.com/chentianyu/celestia/internal/models"
)

func (s *Service) ListAgentCapabilities(_ context.Context) ([]models.AgentCapabilityInfo, error) {
	return agentcaps.List(), nil
}

func (s *Service) DescribeAgentCapability(_ context.Context, name string) (models.AgentCapabilityInfo, error) {
	capability, ok := agentcaps.Get(name)
	if !ok {
		return models.AgentCapabilityInfo{}, errors.New("agent capability not found")
	}
	return capability, nil
}

func (s *Service) RunAgentCapability(
	ctx context.Context,
	name string,
	req models.AgentCapabilityRunRequest,
) (models.AgentCapabilityRunResult, error) {
	capability, ok := agentcaps.Get(name)
	if !ok {
		return models.AgentCapabilityRunResult{}, errors.New("agent capability not found")
	}
	input := capabilityRunInput(req)
	if capability.Terminal {
		return s.runTerminalCapability(ctx, capability, req, input)
	}
	command := builtInCapabilityCommand(capability.Name, input)
	if command == "" {
		return models.AgentCapabilityRunResult{}, errors.New("agent capability is not executable")
	}
	output, handled, err := s.RunDirectCommand(ctx, command)
	if !handled {
		return models.AgentCapabilityRunResult{}, errors.New("agent capability command was not handled")
	}
	return models.AgentCapabilityRunResult{
		Capability:  capability.Name,
		Tool:        capability.Tool,
		Action:      capability.Action,
		Input:       input,
		UsedCommand: command,
		Output:      output,
	}, err
}

func capabilityRunInput(req models.AgentCapabilityRunRequest) string {
	if text := strings.TrimSpace(req.Input); text != "" {
		return text
	}
	if len(req.Args) > 0 {
		return strings.Join(req.Args, " ")
	}
	return ""
}

func (s *Service) runTerminalCapability(
	ctx context.Context,
	capability models.AgentCapabilityInfo,
	req models.AgentCapabilityRunRequest,
	input string,
) (models.AgentCapabilityRunResult, error) {
	command, err := terminalCapabilityCommand(capability, req, input)
	if err != nil {
		return models.AgentCapabilityRunResult{}, err
	}
	result, runErr := s.RunTerminal(ctx, models.AgentTerminalRequest{Command: command})
	output := result.Output
	return models.AgentCapabilityRunResult{
		Capability:  capability.Name,
		Tool:        capability.Tool,
		Action:      capability.Action,
		Input:       input,
		UsedCommand: command,
		Output:      output,
		Terminal:    &result,
	}, runErr
}

func terminalCapabilityCommand(
	capability models.AgentCapabilityInfo,
	req models.AgentCapabilityRunRequest,
	input string,
) (string, error) {
	configured := strings.TrimSpace(capability.Command)
	if configured == "" {
		return "", errors.New("agent capability terminal command is not configured")
	}
	if req.Command != "" && commandName(req.Command) != commandName(configured) {
		return "", errors.New("agent capability command does not match configured executable")
	}
	if len(req.Args) > 0 {
		parts := []string{configured}
		for _, arg := range req.Args {
			parts = append(parts, shellQuote(arg))
		}
		return strings.Join(parts, " "), nil
	}
	text := strings.TrimSpace(firstNonEmpty(req.Command, input))
	if text == "" {
		text = defaultTerminalCapabilityArgs(capability.Name)
	}
	if commandName(text) == commandName(configured) {
		return text, nil
	}
	return strings.TrimSpace(configured + " " + defaultTerminalCapabilityPrefix(capability.Name, text)), nil
}

func builtInCapabilityCommand(name string, input string) string {
	switch agentcaps.NormalizeName(name) {
	case "topic-summary":
		return strings.TrimSpace("/topic " + strings.TrimPrefix(input, "/topic"))
	case "writing-organizer":
		return strings.TrimSpace("/writing " + strings.TrimPrefix(input, "/writing"))
	case "market-analysis":
		return strings.TrimSpace("/market " + strings.TrimPrefix(input, "/market"))
	case "evolution-operator":
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(input)), "/") {
			return input
		}
		return strings.TrimSpace("/evolve " + input)
	default:
		return ""
	}
}

func defaultTerminalCapabilityArgs(name string) string {
	switch agentcaps.NormalizeName(name) {
	case "apple-notes":
		return "notes"
	default:
		return ""
	}
}

func defaultTerminalCapabilityPrefix(name string, input string) string {
	if agentcaps.NormalizeName(name) == "apple-notes" && !strings.HasPrefix(strings.TrimSpace(input), "notes") {
		return strings.TrimSpace("notes " + input)
	}
	return input
}

func commandName(command string) string {
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) == 0 {
		return ""
	}
	return filepath.Base(fields[0])
}
