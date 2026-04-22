package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
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
	assets := make([]models.AgentMarketAssetContext, 0, len(snapshot.Market.Portfolio.Funds))
	sourceChain := []string{}
	errorsOut := []models.AgentRunError{}
	for _, holding := range snapshot.Market.Portfolio.Funds {
		asset := models.AgentMarketAssetContext{Code: holding.Code, Name: holding.Name}
		estimate, estimateErr := fetchEastmoneyEstimate(ctx, holding.Code, 12000)
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
	summary := fallbackMarketSummary(snapshot.Market.Portfolio, assets, req.Notes)
	if generated, genErr := s.GenerateText(ctx, buildMarketPrompt(snapshot.Market.Portfolio, assets, phase, req.Notes)); genErr == nil {
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

type eastmoneyEstimate struct {
	EstimateNAV float64
	ChangePct   float64
	AsOf        string
}

func fetchEastmoneyEstimate(ctx context.Context, code string, timeoutMS int) (eastmoneyEstimate, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return eastmoneyEstimate{}, errors.New("fund code is required")
	}
	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()
	endpoint := "https://fundgz.1234567.com.cn/js/" + code + ".js?rt=" + fmt.Sprintf("%d", time.Now().UnixMilli())
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return eastmoneyEstimate{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return eastmoneyEstimate{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return eastmoneyEstimate{}, fmt.Errorf("eastmoney HTTP %s", resp.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return eastmoneyEstimate{}, err
	}
	matches := regexp.MustCompile(`jsonpgz\((.*)\);?`).FindSubmatch(raw)
	if len(matches) < 2 {
		return eastmoneyEstimate{}, errors.New("eastmoney response did not include jsonpgz payload")
	}
	var payload map[string]any
	if err := json.Unmarshal(matches[1], &payload); err != nil {
		return eastmoneyEstimate{}, err
	}
	return eastmoneyEstimate{
		EstimateNAV: parseFloat(payload["gsz"]),
		ChangePct:   parseFloat(payload["gszzl"]),
		AsOf:        firstNonEmpty(stringFrom(payload["gztime"]), stringFrom(payload["jzrq"])),
	}, nil
}

func fallbackMarketSummary(portfolio models.AgentMarketPortfolio, assets []models.AgentMarketAssetContext, notes string) string {
	var b strings.Builder
	b.WriteString("Fund analysis with Eastmoney estimate and configured search engine. Cash: ")
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
			for _, asset := range assets {
				if asset.Code == fund.Code && asset.AsOf != "" {
					b.WriteString(" nav=")
					b.WriteString(floatString(asset.EstimateNAV))
					b.WriteString(" change=")
					b.WriteString(floatString(asset.ChangePct))
					b.WriteString("%")
				}
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

func buildMarketPrompt(portfolio models.AgentMarketPortfolio, assets []models.AgentMarketAssetContext, phase string, notes string) string {
	var b strings.Builder
	b.WriteString("Analyze this fund portfolio for the ")
	b.WriteString(phase)
	b.WriteString(" phase using Eastmoney estimate data and search news below.\n")
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
	b.WriteString("\nMarket context:\n")
	for _, asset := range assets {
		b.WriteString("- ")
		b.WriteString(asset.Code)
		b.WriteString(" ")
		b.WriteString(asset.Name)
		b.WriteString(" nav=")
		b.WriteString(floatString(asset.EstimateNAV))
		b.WriteString(" change_pct=")
		b.WriteString(floatString(asset.ChangePct))
		b.WriteString(" as_of=")
		b.WriteString(asset.AsOf)
		b.WriteString("\n")
		for _, news := range asset.News {
			b.WriteString("  * ")
			b.WriteString(news.Title)
			if news.Link != "" {
				b.WriteString(" ")
				b.WriteString(news.Link)
			}
			b.WriteString("\n")
		}
	}
	if strings.TrimSpace(notes) != "" {
		b.WriteString("\nNotes:\n")
		b.WriteString(notes)
	}
	return b.String()
}

func parseFloat(value any) float64 {
	number, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(value)), 64)
	return number
}
