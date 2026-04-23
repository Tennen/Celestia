package runtime

import (
	"context"
	"errors"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

func (s *Service) addMarketHolding(ctx context.Context, holding models.AgentMarketHolding) (models.AgentSnapshot, error) {
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return models.AgentSnapshot{}, err
	}
	code := strings.TrimSpace(holding.Code)
	if code == "" {
		return models.AgentSnapshot{}, errors.New("market code is required")
	}
	if holding.Quantity <= 0 {
		return models.AgentSnapshot{}, errors.New("market quantity must be positive")
	}
	if holding.AvgCost < 0 {
		return models.AgentSnapshot{}, errors.New("market avg_cost must be non-negative")
	}
	portfolio := snapshot.Market.Portfolio
	found := false
	for idx := range portfolio.Funds {
		if portfolio.Funds[idx].Code == code {
			portfolio.Funds[idx] = holding
			found = true
			break
		}
	}
	if !found {
		portfolio.Funds = append(portfolio.Funds, holding)
	}
	return s.SaveMarketPortfolio(ctx, portfolio)
}

func detectMarketPhase(input string) string {
	source := strings.ToLower(strings.TrimSpace(input))
	switch {
	case strings.Contains(source, "midday"), strings.Contains(source, "13:30"), strings.Contains(source, "盘中"), strings.Contains(source, "午盘"):
		return "midday"
	case strings.Contains(source, "close"), strings.Contains(source, "15:15"), strings.Contains(source, "收盘"), strings.Contains(source, "盘后"):
		return "close"
	default:
		return ""
	}
}

func (s *Service) setCodexEvolutionOption(ctx context.Context, key string, value string) (models.AgentEvolutionConfig, error) {
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return models.AgentEvolutionConfig{}, err
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return snapshot.Settings.Evolution, nil
	}
	settings := snapshot.Settings
	if key == "model" {
		settings.Evolution.CodexModel = trimmed
	} else {
		settings.Evolution.CodexReasoning = trimmed
	}
	next, err := s.SaveSettings(ctx, settings)
	return next.Settings.Evolution, err
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
