package agent

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
)

type MarketRunRequest struct {
	Phase string `json:"phase"`
	Notes string `json:"notes,omitempty"`
}

func (s *Service) SaveMarketPortfolio(ctx context.Context, portfolio models.AgentMarketPortfolio) (models.AgentSnapshot, error) {
	return s.update(ctx, func(snapshot *models.AgentSnapshot) error {
		now := time.Now().UTC()
		snapshot.Market.Portfolio = portfolio
		snapshot.Market.UpdatedAt = now
		snapshot.UpdatedAt = now
		return nil
	})
}

func (s *Service) RunMarketAnalysis(ctx context.Context, req MarketRunRequest) (models.AgentMarketRun, error) {
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return models.AgentMarketRun{}, err
	}
	if len(snapshot.Market.Portfolio.Funds) == 0 && snapshot.Market.Portfolio.Cash == 0 {
		return models.AgentMarketRun{}, errors.New("market portfolio is empty")
	}
	phase := firstNonEmpty(req.Phase, "close")
	summary := fallbackMarketSummary(snapshot.Market.Portfolio, req.Notes)
	if generated, genErr := s.GenerateText(ctx, buildMarketPrompt(snapshot.Market.Portfolio, phase, req.Notes)); genErr == nil {
		summary = generated
	}
	run := models.AgentMarketRun{
		ID:          uuid.NewString(),
		CreatedAt:   time.Now().UTC(),
		Phase:       phase,
		MarketState: "portfolio_only_no_market_feed",
		Summary:     summary,
	}
	_, err = s.update(ctx, func(snapshot *models.AgentSnapshot) error {
		snapshot.Market.Runs = append([]models.AgentMarketRun{run}, snapshot.Market.Runs...)
		snapshot.Market.Runs = truncateList(snapshot.Market.Runs, 50)
		snapshot.Market.UpdatedAt = run.CreatedAt
		snapshot.UpdatedAt = run.CreatedAt
		return nil
	})
	return run, err
}

func fallbackMarketSummary(portfolio models.AgentMarketPortfolio, notes string) string {
	var b strings.Builder
	b.WriteString("Portfolio-only analysis. Cash: ")
	b.WriteString(floatString(portfolio.Cash))
	b.WriteString(". Holdings: ")
	if len(portfolio.Funds) == 0 {
		b.WriteString("none.")
	} else {
		for idx, fund := range portfolio.Funds {
			if idx > 0 {
				b.WriteString("; ")
			}
			b.WriteString(firstNonEmpty(fund.Name, fund.Code))
			if fund.Code != "" {
				b.WriteString(" (")
				b.WriteString(fund.Code)
				b.WriteString(")")
			}
		}
		b.WriteString(".")
	}
	if strings.TrimSpace(notes) != "" {
		b.WriteString(" Notes: ")
		b.WriteString(strings.TrimSpace(notes))
	}
	return b.String()
}

func buildMarketPrompt(portfolio models.AgentMarketPortfolio, phase string, notes string) string {
	var b strings.Builder
	b.WriteString("Analyze this fund portfolio for the ")
	b.WriteString(phase)
	b.WriteString(" phase. Be explicit that no live market feed is attached unless data appears in notes.\n")
	b.WriteString("Cash: ")
	b.WriteString(floatString(portfolio.Cash))
	b.WriteString("\nHoldings:\n")
	for _, fund := range portfolio.Funds {
		b.WriteString("- ")
		b.WriteString(fund.Code)
		b.WriteString(" ")
		b.WriteString(fund.Name)
		b.WriteString(" qty=")
		b.WriteString(floatString(fund.Quantity))
		b.WriteString(" avg_cost=")
		b.WriteString(floatString(fund.AvgCost))
		b.WriteString("\n")
	}
	if strings.TrimSpace(notes) != "" {
		b.WriteString("\nNotes:\n")
		b.WriteString(notes)
	}
	return b.String()
}
