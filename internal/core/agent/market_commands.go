package agent

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

func (s *Service) runMarketCommand(ctx context.Context, rest string) (string, bool, error) {
	action, tail := splitWord(rest)
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "", "help", "h", "?":
		return marketHelpText(), true, nil
	case "status", "latest":
		snapshot, err := s.Snapshot(ctx)
		if err != nil {
			return "", true, err
		}
		return marshalCommandResult(snapshot.Market.Runs), true, nil
	case "portfolio", "holdings", "position":
		snapshot, err := s.Snapshot(ctx)
		if err != nil {
			return "", true, err
		}
		return marshalCommandResult(snapshot.Market.Portfolio), true, nil
	case "add", "new", "append", "create":
		holding, err := parseMarketHolding(tail)
		if err != nil {
			return "", true, err
		}
		snapshot, err := s.addMarketHolding(ctx, holding)
		return marshalCommandResult(snapshot.Market.Portfolio), true, err
	case "run":
		phase := firstNonEmpty(detectMarketPhase(tail), "close")
		run, err := s.RunMarketAnalysis(ctx, MarketRunRequest{Phase: phase})
		return marshalCommandResult(run), true, err
	default:
		phase := firstNonEmpty(detectMarketPhase(rest), action, "close")
		run, err := s.RunMarketAnalysis(ctx, MarketRunRequest{Phase: phase})
		return marshalCommandResult(run), true, err
	}
}

func parseMarketHolding(input string) (models.AgentMarketHolding, error) {
	tokens := shellFields(input)
	if len(tokens) < 3 {
		return models.AgentMarketHolding{}, errors.New("market add requires code, quantity, and avg_cost")
	}
	quantity, qErr := strconv.ParseFloat(tokens[1], 64)
	avgCost, cErr := strconv.ParseFloat(tokens[2], 64)
	if qErr != nil || quantity <= 0 {
		return models.AgentMarketHolding{}, errors.New("market quantity must be a positive number")
	}
	if cErr != nil || avgCost < 0 {
		return models.AgentMarketHolding{}, errors.New("market avg_cost must be a non-negative number")
	}
	return models.AgentMarketHolding{
		Code:     tokens[0],
		Quantity: quantity,
		AvgCost:  avgCost,
		Name:     strings.TrimSpace(strings.Join(tokens[3:], " ")),
	}, nil
}

func (s *Service) addMarketHolding(ctx context.Context, holding models.AgentMarketHolding) (models.AgentSnapshot, error) {
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return models.AgentSnapshot{}, err
	}
	code := strings.TrimSpace(holding.Code)
	if code == "" {
		return models.AgentSnapshot{}, errors.New("market code is required")
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

func marketHelpText() string {
	return "Market commands: /market midday, /market close, /market status, /market portfolio, /market add <code> <quantity> <avg_cost> [name]"
}
