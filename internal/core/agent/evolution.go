package agent

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
)

type EvolutionGoalRequest struct {
	Goal          string `json:"goal"`
	CommitMessage string `json:"commit_message,omitempty"`
}

func (s *Service) CreateEvolutionGoal(ctx context.Context, req EvolutionGoalRequest) (models.AgentEvolutionGoal, error) {
	if err := requireText(req.Goal, "goal"); err != nil {
		return models.AgentEvolutionGoal{}, err
	}
	now := time.Now().UTC()
	goal := models.AgentEvolutionGoal{
		ID:            uuid.NewString(),
		Goal:          strings.TrimSpace(req.Goal),
		CommitMessage: strings.TrimSpace(req.CommitMessage),
		Status:        "pending",
		Stage:         "queued",
		Events: []models.AgentEvolutionEvent{{
			At:      now,
			Stage:   "queued",
			Message: "Goal queued.",
		}},
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := s.update(ctx, func(snapshot *models.AgentSnapshot) error {
		snapshot.Evolution.Goals = append([]models.AgentEvolutionGoal{goal}, snapshot.Evolution.Goals...)
		snapshot.Evolution.UpdatedAt = now
		snapshot.UpdatedAt = now
		return nil
	})
	return goal, err
}

func (s *Service) RunEvolutionGoal(ctx context.Context, goalID string) (models.AgentEvolutionGoal, error) {
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return models.AgentEvolutionGoal{}, err
	}
	settings := snapshot.Settings.Evolution
	if strings.TrimSpace(settings.Command) == "" {
		return s.failEvolutionGoal(ctx, goalID, "runner_missing", "evolution command is not configured")
	}
	goal, ok := findEvolutionGoal(snapshot.Evolution.Goals, goalID)
	if !ok {
		return models.AgentEvolutionGoal{}, errors.New("evolution goal not found")
	}
	started := time.Now().UTC()
	goal.Status = "running"
	goal.Stage = "running"
	goal.StartedAt = &started
	goal.UpdatedAt = started
	goal.Events = append(goal.Events, models.AgentEvolutionEvent{At: started, Stage: "running", Message: "Runner started."})
	if _, err := s.saveEvolutionGoal(ctx, goal); err != nil {
		return models.AgentEvolutionGoal{}, err
	}
	timeout := time.Duration(maxInt(settings.TimeoutMS, 600000)) * time.Millisecond
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, "/bin/sh", "-lc", settings.Command)
	if strings.TrimSpace(settings.CWD) != "" {
		cmd.Dir = settings.CWD
	}
	cmd.Stdin = strings.NewReader(goal.Goal)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err = cmd.Run()
	completed := time.Now().UTC()
	if err != nil {
		return s.failEvolutionGoal(ctx, goalID, "failed", output.String()+"\n"+err.Error())
	}
	goal.Status = "succeeded"
	goal.Stage = "completed"
	goal.CompletedAt = &completed
	goal.UpdatedAt = completed
	goal.Events = append(goal.Events, models.AgentEvolutionEvent{At: completed, Stage: "completed", Message: strings.TrimSpace(output.String())})
	return s.saveEvolutionGoal(ctx, goal)
}

func (s *Service) failEvolutionGoal(ctx context.Context, goalID string, stage string, message string) (models.AgentEvolutionGoal, error) {
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return models.AgentEvolutionGoal{}, err
	}
	goal, ok := findEvolutionGoal(snapshot.Evolution.Goals, goalID)
	if !ok {
		return models.AgentEvolutionGoal{}, errors.New("evolution goal not found")
	}
	now := time.Now().UTC()
	goal.Status = "failed"
	goal.Stage = stage
	goal.LastError = strings.TrimSpace(message)
	goal.CompletedAt = &now
	goal.UpdatedAt = now
	goal.Events = append(goal.Events, models.AgentEvolutionEvent{At: now, Stage: stage, Message: goal.LastError})
	saved, saveErr := s.saveEvolutionGoal(ctx, goal)
	if saveErr != nil {
		return models.AgentEvolutionGoal{}, saveErr
	}
	return saved, errors.New(goal.LastError)
}

func (s *Service) saveEvolutionGoal(ctx context.Context, goal models.AgentEvolutionGoal) (models.AgentEvolutionGoal, error) {
	_, err := s.update(ctx, func(snapshot *models.AgentSnapshot) error {
		for idx := range snapshot.Evolution.Goals {
			if snapshot.Evolution.Goals[idx].ID == goal.ID {
				snapshot.Evolution.Goals[idx] = goal
				snapshot.Evolution.UpdatedAt = goal.UpdatedAt
				snapshot.UpdatedAt = goal.UpdatedAt
				return nil
			}
		}
		return errors.New("evolution goal not found")
	})
	return goal, err
}

func findEvolutionGoal(goals []models.AgentEvolutionGoal, id string) (models.AgentEvolutionGoal, bool) {
	for _, goal := range goals {
		if goal.ID == id {
			return goal, true
		}
	}
	return models.AgentEvolutionGoal{}, false
}
