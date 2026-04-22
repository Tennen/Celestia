package agent

import (
	"context"

	agentmarket "github.com/chentianyu/celestia/internal/core/agent/market"
	"github.com/chentianyu/celestia/internal/models"
)

func (s *Service) ImportMarketPortfolioCodes(ctx context.Context, req models.AgentMarketImportCodesRequest) (models.AgentMarketImportCodesResponse, error) {
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return models.AgentMarketImportCodesResponse{}, err
	}
	portfolio, results, summary, err := agentmarket.ImportCodes(ctx, snapshot.Market.Portfolio, req.Codes, 80)
	if err != nil {
		return models.AgentMarketImportCodesResponse{}, err
	}
	if _, err := s.SaveMarketPortfolio(ctx, portfolio); err != nil {
		return models.AgentMarketImportCodesResponse{}, err
	}
	return models.AgentMarketImportCodesResponse{
		OK:        true,
		Portfolio: portfolio,
		Results:   results,
		Summary:   summary,
	}, nil
}
