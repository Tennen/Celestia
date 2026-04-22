package agent

import (
	"context"
	"errors"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

func (s *Service) runEvolutionCommand(ctx context.Context, rest string) (string, bool, error) {
	action, tail := splitWord(rest)
	switch strings.ToLower(action) {
	case "queue", "add", "create":
		goal, commit := splitEvolutionCommit(tail)
		item, err := s.CreateEvolutionGoal(ctx, EvolutionGoalRequest{Goal: goal, CommitMessage: commit})
		return marshalCommandResult(item), true, err
	case "run":
		item, err := s.RunEvolutionGoal(ctx, strings.TrimSpace(tail))
		return marshalCommandResult(item), true, err
	case "status", "":
		out, err := s.evolutionStatus(ctx, tail)
		return marshalCommandResult(out), true, err
	case "tick":
		item, err := s.runNextEvolutionGoal(ctx)
		return marshalCommandResult(item), true, err
	case "help":
		return evolutionHelpText(), true, nil
	default:
		return evolutionHelpText(), true, nil
	}
}

func (s *Service) commandEvolutionAlias(ctx context.Context, command string, rest string) (string, bool, error) {
	if strings.EqualFold(command, "/coding") {
		goal, commit := splitEvolutionCommit(rest)
		item, err := s.CreateEvolutionGoal(ctx, EvolutionGoalRequest{Goal: goal, CommitMessage: commit})
		return marshalCommandResult(item), true, err
	}
	return s.runEvolveCommand(ctx, rest)
}

func (s *Service) runEvolveCommand(ctx context.Context, rest string) (string, bool, error) {
	action, tail := splitWord(rest)
	switch strings.ToLower(action) {
	case "", "help":
		return evolutionHelpText(), true, nil
	case "status":
		out, err := s.evolutionStatus(ctx, tail)
		return marshalCommandResult(out), true, err
	case "tick":
		item, err := s.runNextEvolutionGoal(ctx)
		return marshalCommandResult(item), true, err
	case "run":
		item, err := s.RunEvolutionGoal(ctx, strings.TrimSpace(tail))
		return marshalCommandResult(item), true, err
	default:
		goal, commit := splitEvolutionCommit(rest)
		item, err := s.CreateEvolutionGoal(ctx, EvolutionGoalRequest{Goal: goal, CommitMessage: commit})
		return marshalCommandResult(item), true, err
	}
}

func (s *Service) runCodexCommand(ctx context.Context, rest string) (string, bool, error) {
	action, tail := splitWord(rest)
	switch strings.ToLower(action) {
	case "status":
		snapshot, err := s.Snapshot(ctx)
		if err != nil {
			return "", true, err
		}
		return marshalCommandResult(map[string]any{
			"codex_model":     snapshot.Settings.Evolution.CodexModel,
			"codex_reasoning": snapshot.Settings.Evolution.CodexReasoning,
			"goals":           snapshot.Evolution.Goals,
		}), true, nil
	case "model":
		return s.setCodexEvolutionOption(ctx, "model", tail)
	case "effort":
		return s.setCodexEvolutionOption(ctx, "effort", tail)
	default:
		result, err := s.RunCodex(ctx, models.AgentCodexRequest{Prompt: rest})
		return marshalCommandResult(result), true, err
	}
}

func (s *Service) setCodexEvolutionOption(ctx context.Context, key string, value string) (string, bool, error) {
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return "", true, err
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return marshalCommandResult(snapshot.Settings.Evolution), true, nil
	}
	settings := snapshot.Settings
	if key == "model" {
		settings.Evolution.CodexModel = trimmed
	} else {
		settings.Evolution.CodexReasoning = trimmed
	}
	next, err := s.SaveSettings(ctx, settings)
	return marshalCommandResult(next.Settings.Evolution), true, err
}

func (s *Service) evolutionStatus(ctx context.Context, id string) (any, error) {
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(id) == "" {
		return snapshot.Evolution.Goals, nil
	}
	goal, ok := findEvolutionGoal(snapshot.Evolution.Goals, strings.TrimSpace(id))
	if !ok {
		return nil, errors.New("evolution goal not found")
	}
	return goal, nil
}

func (s *Service) runNextEvolutionGoal(ctx context.Context) (models.AgentEvolutionGoal, error) {
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return models.AgentEvolutionGoal{}, err
	}
	for _, goal := range snapshot.Evolution.Goals {
		if goal.Status != "succeeded" && goal.Status != "running" {
			return s.RunEvolutionGoal(ctx, goal.ID)
		}
	}
	return models.AgentEvolutionGoal{}, errors.New("no queued evolution goal")
}

func splitEvolutionCommit(input string) (string, string) {
	for _, marker := range []string{" commit:", " 提交:"} {
		if goal, commit, ok := strings.Cut(input, marker); ok {
			return strings.TrimSpace(goal), strings.TrimSpace(commit)
		}
	}
	return strings.TrimSpace(input), ""
}

func evolutionHelpText() string {
	return "Evolution commands: /evolve <goal>, /coding <goal>, /evolve status [goal_id], /evolve tick, /evolution queue <goal>, /evolution run <goal_id>, /codex status, /codex model [model], /codex effort [effort]"
}
