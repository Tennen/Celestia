package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	coremarket "github.com/chentianyu/celestia/internal/core/agent/workflows/market"
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
	assets := make([]models.AgentMarketAssetContext, 0, len(snapshot.Market.Portfolio.Funds))
	sourceChain := []string{}
	errorsOut := []models.AgentRunError{}
	for _, holding := range snapshot.Market.Portfolio.Funds {
		asset := models.AgentMarketAssetContext{Code: holding.Code, Name: holding.Name}
		estimate, estimateErr := coremarket.FetchEastmoneyEstimate(ctx, holding.Code, 12000)
		if estimateErr != nil {
			asset.Errors = append(asset.Errors, estimateErr.Error())
			errorsOut = append(errorsOut, models.AgentRunError{Target: holding.Code, Error: estimateErr.Error()})
		} else {
			asset.EstimateNAV = estimate.EstimateNAV
			asset.ChangePct = estimate.ChangePct
			asset.AsOf = estimate.AsOf
			asset.SourceChain = append(asset.SourceChain, "eastmoney:fundgz")
		}
		searchResult, searchErr := s.RunSearch(ctx, models.AgentSearchRequest{
			EngineSelector: snapshot.Market.Config.SearchEngine,
			TimeoutMS:      12000,
			MaxItems:       6,
			Plans: []models.AgentSearchPlan{{
				Label:   "fund-news",
				Query:   strings.TrimSpace(holding.Name + " " + holding.Code + " 基金 公告 净值 风险"),
				Recency: "month",
			}},
			LogContext: holding.Code,
		})
		if searchErr != nil {
			asset.Errors = append(asset.Errors, searchErr.Error())
		} else {
			asset.News = searchResult.Items
			asset.SourceChain = append(asset.SourceChain, searchResult.SourceChain...)
			for _, item := range searchResult.Errors {
				asset.Errors = append(asset.Errors, item)
			}
		}
		sourceChain = appendUniqueStrings(sourceChain, asset.SourceChain...)
		assets = append(assets, asset)
	}
	summary := coremarket.FallbackSummary(snapshot.Market.Portfolio, assets, req.Notes)
	if generated, genErr := s.GenerateText(ctx, coremarket.BuildPrompt(snapshot.Market.Portfolio, assets, phase, req.Notes)); genErr == nil {
		summary = generated
	}
	images := []models.AgentMarkdownImage{}
	if snapshot.Settings.MD2Img.Enabled {
		rendered, renderErr := s.RunMarkdownRender(ctx, models.AgentMarkdownRenderRequest{
			Markdown: summary,
			Mode:     snapshot.Settings.MD2Img.Mode,
		})
		if renderErr != nil {
			return models.AgentMarketRun{}, fmt.Errorf("MARKET_IMAGE_PIPELINE_FAILED: %w", renderErr)
		}
		images = rendered.Images
		sourceChain = appendUniqueStrings(sourceChain, "md2img:"+rendered.Mode)
	}
	run := models.AgentMarketRun{
		ID:          uuid.NewString(),
		CreatedAt:   time.Now().UTC(),
		Phase:       phase,
		MarketState: "eastmoney_search",
		Summary:     summary,
		Images:      images,
		Assets:      assets,
		SourceChain: sourceChain,
		Errors:      errorsOut,
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
