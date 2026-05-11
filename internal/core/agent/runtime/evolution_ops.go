package runtime

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

type EvolutionOperationRequest struct {
	Action        string `json:"action"`
	GoalID        string `json:"goal_id,omitempty"`
	CommitMessage string `json:"commit_message,omitempty"`
}

func (s *Service) RunEvolutionOperation(ctx context.Context, req EvolutionOperationRequest) (models.AgentEvolutionTestResult, error) {
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return models.AgentEvolutionTestResult{}, err
	}
	settings := snapshot.Settings.Evolution
	action := normalizeEvolutionOperation(req.Action)
	if action == "" {
		return models.AgentEvolutionTestResult{}, errors.New("unsupported evolution operation")
	}
	switch action {
	case "commit":
		message := strings.TrimSpace(req.CommitMessage)
		if message == "" && strings.TrimSpace(req.GoalID) != "" {
			if goal, ok := findEvolutionGoal(snapshot.Evolution.Goals, strings.TrimSpace(req.GoalID)); ok {
				message = firstNonEmpty(goal.CommitMessage, deterministicEvolutionCommitMessage(goal.Goal))
			}
		}
		return runEvolutionCommitCommand(ctx, settings, firstNonEmpty(message, "chore: apply evolution goal"))
	case "push":
		return runEvolutionPushCommand(ctx, settings)
	case "rebuild":
		result := runNamedEvolutionCommand(ctx, settings, "rebuild", firstNonEmpty(settings.RebuildCommand, "./deploy.sh"), settings.TimeoutMS)
		if !result.OK {
			return result, errors.New(result.Output)
		}
		return result, nil
	case "restart":
		result := runNamedEvolutionCommand(ctx, settings, "restart", settings.RestartCommand, settings.TimeoutMS)
		if !result.OK {
			return result, errors.New(result.Output)
		}
		return result, nil
	default:
		return models.AgentEvolutionTestResult{}, errors.New("unsupported evolution operation")
	}
}

func (s *Service) evolutionRebuild(ctx context.Context, goal models.AgentEvolutionGoal, settings models.AgentEvolutionConfig) (models.AgentEvolutionGoal, error) {
	result := runNamedEvolutionCommand(ctx, settings, "rebuild", firstNonEmpty(settings.RebuildCommand, "./deploy.sh"), settings.TimeoutMS)
	goal.TestResults = append(goal.TestResults, result)
	if !result.OK {
		return goal, errors.New(result.Output)
	}
	goal.Stage = "rebuild_done"
	goal.UpdatedAt = time.Now().UTC()
	goal.Events = append(goal.Events, models.AgentEvolutionEvent{At: goal.UpdatedAt, Stage: "rebuild", Message: "Local rebuild completed."})
	goal.RawTail = appendEvolutionRaw(goal.RawTail, result.Output)
	return s.saveEvolutionGoal(ctx, goal)
}

func (s *Service) evolutionRestart(ctx context.Context, goal models.AgentEvolutionGoal, settings models.AgentEvolutionConfig) (models.AgentEvolutionGoal, error) {
	result := runNamedEvolutionCommand(ctx, settings, "restart", settings.RestartCommand, settings.TimeoutMS)
	goal.TestResults = append(goal.TestResults, result)
	if !result.OK {
		return goal, errors.New(result.Output)
	}
	goal.Stage = "restart_done"
	goal.UpdatedAt = time.Now().UTC()
	goal.Events = append(goal.Events, models.AgentEvolutionEvent{At: goal.UpdatedAt, Stage: "restart", Message: "Restart command completed."})
	goal.RawTail = appendEvolutionRaw(goal.RawTail, result.Output)
	return s.saveEvolutionGoal(ctx, goal)
}

func runEvolutionCommitCommand(ctx context.Context, settings models.AgentEvolutionConfig, message string) (models.AgentEvolutionTestResult, error) {
	if result := runNamedEvolutionCommand(ctx, settings, "git add", "git add -A", 120000); !result.OK {
		return result, errors.New(result.Output)
	}
	result := runNamedEvolutionCommand(ctx, settings, "git commit", "git commit --allow-empty -m "+evolutionShellQuote(message), 120000)
	if !result.OK {
		return result, errors.New(result.Output)
	}
	return result, nil
}

func runEvolutionPushCommand(ctx context.Context, settings models.AgentEvolutionConfig) (models.AgentEvolutionTestResult, error) {
	remote := firstNonEmpty(settings.PushRemote, "origin")
	branch := strings.TrimSpace(settings.PushBranch)
	if branch == "" {
		out, err := evolutionGitOutput(ctx, settings, "git rev-parse --abbrev-ref HEAD")
		if err != nil {
			return models.AgentEvolutionTestResult{}, err
		}
		branch = strings.TrimSpace(out)
	}
	command := "git push " + evolutionShellQuote(remote) + " HEAD:" + evolutionShellQuote(branch)
	result := runNamedEvolutionCommand(ctx, settings, "git push", command, 120000)
	if !result.OK {
		return result, errors.New(result.Output)
	}
	return result, nil
}

func runNamedEvolutionCommand(ctx context.Context, settings models.AgentEvolutionConfig, name string, command string, timeoutMS int) models.AgentEvolutionTestResult {
	if strings.TrimSpace(command) == "" {
		return models.AgentEvolutionTestResult{
			Name:       name,
			Command:    command,
			OK:         false,
			ExitCode:   1,
			Output:     name + " command is required",
			StartedAt:  time.Now().UTC(),
			FinishedAt: time.Now().UTC(),
		}
	}
	result := runEvolutionShell(ctx, settings, command, maxInt(timeoutMS, settings.TimeoutMS))
	result.Name = name
	return result
}

func normalizeEvolutionOperation(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "commit", "push", "rebuild", "build", "restart":
		if strings.EqualFold(strings.TrimSpace(action), "build") {
			return "rebuild"
		}
		return strings.ToLower(strings.TrimSpace(action))
	default:
		return ""
	}
}
