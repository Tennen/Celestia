package agent

import (
	"context"
	"errors"
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
		Plan:          models.AgentEvolutionPlan{Steps: []string{}},
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
	goal, ok := findEvolutionGoal(snapshot.Evolution.Goals, goalID)
	if !ok {
		return models.AgentEvolutionGoal{}, errors.New("evolution goal not found")
	}
	started := time.Now().UTC()
	goal.Status = "running"
	goal.Stage = "prepare"
	if goal.StartedAt == nil {
		goal.StartedAt = &started
	}
	goal.UpdatedAt = started
	goal.Events = append(goal.Events, models.AgentEvolutionEvent{At: started, Stage: "prepare", Message: "Evolution runner started."})
	goal.RawTail = appendEvolutionRaw(goal.RawTail, "runner started")
	if goal.Plan.Steps == nil {
		goal.Plan.Steps = []string{}
	}
	if goal, err = s.saveEvolutionGoal(ctx, goal); err != nil {
		return models.AgentEvolutionGoal{}, err
	}

	if goal.StartedFromRef == "" {
		if ref, refErr := evolutionGitOutput(ctx, settings, "git rev-parse --short HEAD"); refErr == nil {
			goal.StartedFromRef = strings.TrimSpace(ref)
			goal.Events = append(goal.Events, models.AgentEvolutionEvent{At: time.Now().UTC(), Stage: "git", Message: "Baseline commit: " + goal.StartedFromRef})
			if goal, err = s.saveEvolutionGoal(ctx, goal); err != nil {
				return models.AgentEvolutionGoal{}, err
			}
		}
	}

	if len(goal.Plan.Steps) == 0 {
		goal, err = s.evolutionPlan(ctx, goal, settings)
		if err != nil {
			if strings.TrimSpace(settings.Command) != "" {
				return s.runLegacyEvolutionCommand(ctx, goal, settings)
			}
			return s.failEvolutionGoal(ctx, goalID, "plan_failed", err.Error())
		}
	}

	for goal.Plan.CurrentStep < len(goal.Plan.Steps) {
		goal, err = s.evolutionStep(ctx, goal, settings, goal.Plan.CurrentStep)
		if err != nil {
			return s.failEvolutionGoal(ctx, goalID, goal.Stage, err.Error())
		}
	}

	goal, checksOK, err := s.evolutionChecks(ctx, goal, settings)
	if err != nil {
		return s.failEvolutionGoal(ctx, goalID, "checks_failed", err.Error())
	}
	for attempt := goal.FixAttempts; !checksOK && attempt < maxInt(settings.MaxFixAttempts, 2); attempt++ {
		goal, err = s.evolutionFix(ctx, goal, settings, attempt+1)
		if err != nil {
			return s.failEvolutionGoal(ctx, goalID, "fix_failed", err.Error())
		}
		goal, checksOK, err = s.evolutionChecks(ctx, goal, settings)
		if err != nil {
			return s.failEvolutionGoal(ctx, goalID, "checks_failed", err.Error())
		}
	}
	if !checksOK {
		return s.failEvolutionGoal(ctx, goalID, "checks_failed", "checks failed after auto-fix attempts")
	}

	if settings.StructureReview {
		goal, err = s.evolutionStructureReview(ctx, goal, settings)
		if err != nil {
			return s.failEvolutionGoal(ctx, goalID, "structure_failed", err.Error())
		}
	}

	if settings.AutoCommit {
		goal, err = s.evolutionCommit(ctx, goal, settings)
		if err != nil {
			return s.failEvolutionGoal(ctx, goalID, "commit_failed", err.Error())
		}
	}
	if settings.AutoPush {
		goal, err = s.evolutionPush(ctx, goal, settings)
		if err != nil {
			return s.failEvolutionGoal(ctx, goalID, "push_failed", err.Error())
		}
	}

	completed := time.Now().UTC()
	goal.Status = "succeeded"
	goal.Stage = "completed"
	goal.CompletedAt = &completed
	goal.UpdatedAt = completed
	goal.Events = append(goal.Events, models.AgentEvolutionEvent{At: completed, Stage: "completed", Message: "Evolution goal completed."})
	goal.RawTail = appendEvolutionRaw(goal.RawTail, "goal completed")
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
