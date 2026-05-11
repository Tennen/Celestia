package slash

import (
	"context"
	"errors"
	"fmt"
	"strings"

	coreagent "github.com/chentianyu/celestia/internal/core/agent"
	"github.com/chentianyu/celestia/internal/models"
)

func (s *Service) runEvolution(ctx context.Context, req models.ProjectInputRequest, args []string) (string, map[string]any, error) {
	if s.agent == nil {
		return "", nil, errors.New("agent runtime is not configured")
	}
	if len(args) == 0 || equalRef(args[0], "status") {
		id := ""
		if len(args) > 1 {
			id = args[1]
		}
		return s.evolutionStatus(ctx, id)
	}
	switch normalizeRef(args[0]) {
	case "help":
		return evolutionHelp(), map[string]any{"domain": "evolution"}, nil
	case "queue", "add", "create":
		params, values, err := parseSlashParams(args[1:])
		if err != nil {
			return "", nil, err
		}
		goalText := strings.TrimSpace(strings.Join(values, " "))
		goal, err := s.agent.CreateEvolutionGoal(ctx, coreagent.EvolutionGoalRequest{
			Goal:          goalText,
			CommitMessage: stringParam(params, "commit_message", "commit"),
		})
		if err != nil {
			return "", nil, err
		}
		return "Evolution goal queued: " + goal.ID, map[string]any{"domain": "evolution", "goal_id": goal.ID}, nil
	case "run", "next":
		id := ""
		if len(args) > 1 {
			id = strings.TrimSpace(args[1])
		}
		if id == "" {
			return s.runNextEvolution(ctx)
		}
		goal, err := s.agent.RunEvolutionGoal(ctx, id)
		return formatEvolutionGoal(goal), map[string]any{"domain": "evolution", "goal_id": goal.ID}, err
	case "request":
		return s.requestEvolutionApproval(ctx, req, args[1:])
	case "commit", "push", "rebuild", "restart", "build":
		return s.runEvolutionOperation(ctx, args)
	default:
		return "", nil, fmt.Errorf("unknown evolution action %q", args[0])
	}
}

func (s *Service) evolutionStatus(ctx context.Context, id string) (string, map[string]any, error) {
	snapshot, err := s.agent.Snapshot(ctx)
	if err != nil {
		return "", nil, err
	}
	if strings.TrimSpace(id) != "" {
		for _, goal := range snapshot.Evolution.Goals {
			if goal.ID == strings.TrimSpace(id) {
				return formatEvolutionGoal(goal), map[string]any{"domain": "evolution", "goal_id": goal.ID}, nil
			}
		}
		return "", nil, errors.New("evolution goal not found")
	}
	lines := []string{"Evolution goals:"}
	for _, goal := range snapshot.Evolution.Goals {
		lines = append(lines, "- "+goal.ID+" "+goal.Status+" "+goal.Stage+": "+goal.Goal)
	}
	if len(snapshot.Evolution.Goals) == 0 {
		lines = append(lines, "- none")
	}
	if pending := pendingApprovalLines(snapshot.Approvals.Requests); len(pending) > 0 {
		lines = append(lines, "", "Pending approvals:")
		lines = append(lines, pending...)
	}
	return strings.Join(lines, "\n"), map[string]any{"domain": "evolution"}, nil
}

func (s *Service) runNextEvolution(ctx context.Context) (string, map[string]any, error) {
	snapshot, err := s.agent.Snapshot(ctx)
	if err != nil {
		return "", nil, err
	}
	for _, goal := range snapshot.Evolution.Goals {
		if goal.Status != "succeeded" && goal.Status != "running" {
			next, err := s.agent.RunEvolutionGoal(ctx, goal.ID)
			return formatEvolutionGoal(next), map[string]any{"domain": "evolution", "goal_id": next.ID}, err
		}
	}
	return "", nil, errors.New("no queued evolution goal")
}

func (s *Service) requestEvolutionApproval(ctx context.Context, req models.ProjectInputRequest, args []string) (string, map[string]any, error) {
	if len(args) == 0 {
		return "", nil, errors.New("approval action is required")
	}
	action := strings.TrimSpace(args[0])
	goalID := ""
	if len(args) > 1 {
		goalID = strings.TrimSpace(args[1])
	}
	item, err := s.agent.CreateApproval(ctx, models.AgentApprovalCreateRequest{
		Kind:        "evolution_operation",
		Action:      action,
		GoalID:      goalID,
		RequestedBy: actorOrInput(req),
		Title:       "Approve evolution " + action,
	})
	if err != nil {
		return "", nil, err
	}
	return "Approval requested: " + item.ID, map[string]any{"domain": "approval", "approval_id": item.ID}, nil
}

func (s *Service) runEvolutionOperation(ctx context.Context, args []string) (string, map[string]any, error) {
	params, _, err := parseSlashParams(args[1:])
	if err != nil {
		return "", nil, err
	}
	result, err := s.agent.RunEvolutionOperation(ctx, coreagent.EvolutionOperationRequest{
		Action:        args[0],
		GoalID:        stringParam(params, "goal_id", "goal"),
		CommitMessage: stringParam(params, "commit_message", "commit"),
	})
	return formatEvolutionOperation(result), map[string]any{"domain": "evolution", "action": args[0]}, err
}

func formatEvolutionGoal(goal models.AgentEvolutionGoal) string {
	lines := []string{
		"Evolution goal: " + goal.ID,
		"Status: " + goal.Status,
		"Stage: " + goal.Stage,
		"Goal: " + goal.Goal,
	}
	if goal.CompletedCommit != "" {
		lines = append(lines, "Commit: "+goal.CompletedCommit)
	}
	if goal.LastError != "" {
		lines = append(lines, "Error: "+goal.LastError)
	}
	return strings.Join(lines, "\n")
}

func formatEvolutionOperation(result models.AgentEvolutionTestResult) string {
	status := "failed"
	if result.OK {
		status = "succeeded"
	}
	return strings.TrimSpace("Evolution operation " + status + ": " + firstNonEmpty(result.Name, result.Command) + "\n" + result.Output)
}

func pendingApprovalLines(items []models.AgentApprovalRequest) []string {
	lines := []string{}
	for _, item := range items {
		if item.Status == "pending" {
			lines = append(lines, "- "+item.ID+" "+item.Action+" "+item.Title)
		}
	}
	return lines
}

func stringParam(params map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := params[key]; ok {
			return strings.TrimSpace(fmt.Sprint(value))
		}
	}
	return ""
}

func evolutionHelp() string {
	return strings.TrimSpace(`
Evolution commands:
- /evolution status [goal-id]
- /evolution queue <goal> [commit_message=...]
- /evolution run [goal-id]
- /evolution request <commit|push|rebuild|restart> [goal-id]
- /evolution <commit|push|rebuild|restart> [goal_id=...] [commit_message=...]
`)
}
