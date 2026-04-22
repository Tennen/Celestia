package market

import (
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

func FallbackSummary(portfolio models.AgentMarketPortfolio, assets []models.AgentMarketAssetContext, notes string) string {
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

func BuildPrompt(portfolio models.AgentMarketPortfolio, assets []models.AgentMarketAssetContext, phase string, notes string) string {
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
