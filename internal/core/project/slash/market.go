package slash

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	coreagent "github.com/chentianyu/celestia/internal/core/agent"
	"github.com/chentianyu/celestia/internal/models"
)

func (s *Service) runMarket(ctx context.Context, args []string) (string, map[string]any, error) {
	if s.agent == nil {
		return "", map[string]any{"domain": "market"}, errors.New("market runtime is not available")
	}
	if len(args) == 0 || equalRef(args[0], "portfolio") {
		snapshot, err := s.agent.Snapshot(ctx)
		if err != nil {
			return "", map[string]any{"domain": "market", "action": "portfolio"}, err
		}
		return formatMarketPortfolio(snapshot.Market.Portfolio), map[string]any{"domain": "market", "action": "portfolio", "funds": len(snapshot.Market.Portfolio.Funds)}, nil
	}
	action := strings.ToLower(strings.TrimSpace(args[0]))
	switch action {
	case "run", "open", "midday", "close":
		phase := "close"
		notes := ""
		if action == "run" {
			if len(args) > 1 {
				phase = args[1]
			}
			if len(args) > 2 {
				notes = strings.Join(args[2:], " ")
			}
		} else {
			phase = action
			if len(args) > 1 {
				notes = strings.Join(args[1:], " ")
			}
		}
		run, err := s.agent.RunMarketAnalysis(ctx, coreagent.MarketRunRequest{Phase: phase, Notes: notes})
		metadata := map[string]any{"domain": "market", "action": "run", "phase": phase, "run_id": run.ID}
		if err != nil {
			return "", metadata, err
		}
		output := strings.TrimSpace(run.Summary)
		if len(run.Images) > 0 {
			paths := make([]string, 0, len(run.Images))
			for _, image := range run.Images {
				paths = append(paths, image.Path)
			}
			output += "\n\nImages: " + strings.Join(paths, ", ")
		}
		return output, metadata, nil
	case "import":
		if len(args) < 2 {
			return "", map[string]any{"domain": "market", "action": "import"}, errors.New("usage: /market import <fund codes>")
		}
		result, err := s.agent.ImportMarketPortfolioCodes(ctx, models.AgentMarketImportCodesRequest{Codes: strings.Join(args[1:], " ")})
		metadata := map[string]any{"domain": "market", "action": "import", "ok": result.OK}
		if err != nil {
			return "", metadata, err
		}
		return firstNonEmpty(formatMarketImportSummary(result.Summary), formatMarketPortfolio(result.Portfolio)), metadata, nil
	default:
		return "", map[string]any{"domain": "market"}, fmt.Errorf("unknown market command %q", action)
	}
}

func formatMarketPortfolio(portfolio models.AgentMarketPortfolio) string {
	lines := []string{"Market portfolio:"}
	for _, holding := range portfolio.Funds {
		detail := strings.TrimSpace(holding.Code)
		if strings.TrimSpace(holding.Name) != "" {
			detail += " " + strings.TrimSpace(holding.Name)
		}
		if holding.Quantity != 0 {
			detail += fmt.Sprintf(" qty=%g", holding.Quantity)
		}
		if holding.AvgCost != 0 {
			detail += fmt.Sprintf(" avg=%g", holding.AvgCost)
		}
		lines = append(lines, "- "+strings.TrimSpace(detail))
	}
	if len(portfolio.Funds) == 0 {
		lines = append(lines, "- no funds configured")
	}
	if portfolio.Cash != 0 {
		lines = append(lines, fmt.Sprintf("Cash: %g", portfolio.Cash))
	}
	return strings.Join(lines, "\n")
}

func formatMarketImportSummary(summary map[string]int) string {
	if len(summary) == 0 {
		return ""
	}
	keys := make([]string, 0, len(summary))
	for key := range summary {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, summary[key]))
	}
	return "Market import: " + strings.Join(parts, ", ")
}
