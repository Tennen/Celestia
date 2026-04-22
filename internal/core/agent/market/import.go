package market

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

func ImportCodes(
	ctx context.Context,
	portfolio models.AgentMarketPortfolio,
	input string,
	limit int,
) (models.AgentMarketPortfolio, []models.AgentMarketImportCodeResult, map[string]int, error) {
	codes := ParseCodes(input, limit)
	if len(codes) == 0 {
		return models.AgentMarketPortfolio{}, nil, nil, errors.New("codes is required")
	}
	byCode := map[string]models.AgentMarketHolding{}
	order := []string{}
	for _, holding := range portfolio.Funds {
		code := NormalizeCode(holding.Code)
		if code == "" {
			continue
		}
		holding.Code = code
		if _, ok := byCode[code]; !ok {
			order = append(order, code)
		}
		byCode[code] = holding
	}

	results := make([]models.AgentMarketImportCodeResult, 0, len(codes))
	for _, code := range codes {
		matched, matchErr := ResolveSecurityByCode(ctx, code)
		if matchErr != nil {
			results = append(results, models.AgentMarketImportCodeResult{Code: code, Status: "error", Message: matchErr.Error()})
			continue
		}
		if matched.Code == "" {
			results = append(results, models.AgentMarketImportCodeResult{Code: code, Status: "not_found", Message: "security name not found"})
			continue
		}
		existing, existed := byCode[code]
		name := firstNonEmpty(matched.Name, existing.Name, code)
		next := models.AgentMarketHolding{Code: code, Name: name, Quantity: existing.Quantity, AvgCost: existing.AvgCost}
		if !existed {
			order = append(order, code)
			results = append(results, models.AgentMarketImportCodeResult{Code: code, Name: name, Status: "added"})
		} else if existing.Name != name {
			results = append(results, models.AgentMarketImportCodeResult{Code: code, Name: name, Status: "updated"})
		} else {
			results = append(results, models.AgentMarketImportCodeResult{Code: code, Name: name, Status: "exists"})
		}
		byCode[code] = next
	}

	next := portfolio
	next.Funds = make([]models.AgentMarketHolding, 0, len(order))
	for _, code := range order {
		if holding, ok := byCode[code]; ok {
			next.Funds = append(next.Funds, holding)
		}
	}
	return next, results, SummarizeImportResults(results), nil
}

func ParseCodes(input string, limit int) []string {
	matches := regexp.MustCompile(`\d{6}`).FindAllString(input, -1)
	out := []string{}
	seen := map[string]bool{}
	for _, raw := range matches {
		code := NormalizeCode(raw)
		if code == "" || seen[code] {
			continue
		}
		seen[code] = true
		out = append(out, code)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func NormalizeCode(input string) string {
	return regexp.MustCompile(`\d{6}`).FindString(strings.TrimSpace(input))
}

func ResolveSecurityByCode(ctx context.Context, code string) (Security, error) {
	items, err := SearchSecurities(ctx, code, 12)
	if err != nil {
		return Security{}, err
	}
	for _, item := range items {
		if item.Code == code {
			return item, nil
		}
	}
	return Security{}, nil
}

func SummarizeImportResults(results []models.AgentMarketImportCodeResult) map[string]int {
	summary := map[string]int{"added": 0, "updated": 0, "exists": 0, "not_found": 0, "error": 0}
	for _, result := range results {
		summary[result.Status]++
	}
	return summary
}

func importProviderError(errorsOut []string) error {
	if len(errorsOut) == 0 {
		return nil
	}
	return fmt.Errorf("market search provider unavailable: %s", strings.Join(errorsOut, " | "))
}
