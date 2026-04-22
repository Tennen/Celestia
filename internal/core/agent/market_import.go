package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

const eastmoneySearchToken = "44c9d251add88e27b65ed86506f6e5da"

type marketSecurity struct {
	Code string
	Name string
}

func (s *Service) ImportMarketPortfolioCodes(ctx context.Context, req models.AgentMarketImportCodesRequest) (models.AgentMarketImportCodesResponse, error) {
	codes := parseMarketCodes(req.Codes, 80)
	if len(codes) == 0 {
		return models.AgentMarketImportCodesResponse{}, errors.New("codes is required")
	}
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return models.AgentMarketImportCodesResponse{}, err
	}
	byCode := map[string]models.AgentMarketHolding{}
	order := []string{}
	for _, holding := range snapshot.Market.Portfolio.Funds {
		code := normalizeMarketCode(holding.Code)
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
		matched, matchErr := resolveMarketSecurityByCode(ctx, code)
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

	funds := make([]models.AgentMarketHolding, 0, len(order))
	for _, code := range order {
		if holding, ok := byCode[code]; ok {
			funds = append(funds, holding)
		}
	}
	portfolio := snapshot.Market.Portfolio
	portfolio.Funds = funds
	if _, err := s.SaveMarketPortfolio(ctx, portfolio); err != nil {
		return models.AgentMarketImportCodesResponse{}, err
	}
	return models.AgentMarketImportCodesResponse{
		OK:        true,
		Portfolio: portfolio,
		Results:   results,
		Summary:   summarizeImportResults(results),
	}, nil
}

func resolveMarketSecurityByCode(ctx context.Context, code string) (marketSecurity, error) {
	items, err := searchMarketSecurities(ctx, code, 12)
	if err != nil {
		return marketSecurity{}, err
	}
	for _, item := range items {
		if item.Code == code {
			return item, nil
		}
	}
	return marketSecurity{}, nil
}

func searchMarketSecurities(ctx context.Context, keyword string, limit int) ([]marketSecurity, error) {
	query := strings.TrimSpace(keyword)
	if query == "" {
		return nil, nil
	}
	endpoints := []string{
		"https://searchapi.eastmoney.com/api/suggest/get",
		"https://searchadapter.eastmoney.com/api/suggest/get",
	}
	errorsOut := []string{}
	for _, endpoint := range endpoints {
		items, err := fetchMarketSecurities(ctx, endpoint, query, limit)
		if err != nil {
			errorsOut = append(errorsOut, err.Error())
			continue
		}
		if len(items) > 0 {
			return items, nil
		}
	}
	if len(errorsOut) > 0 {
		return nil, fmt.Errorf("market search provider unavailable: %s", strings.Join(errorsOut, " | "))
	}
	return nil, nil
}

func fetchMarketSecurities(ctx context.Context, endpoint string, query string, limit int) ([]marketSecurity, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 12000*time.Millisecond)
	defer cancel()
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	values := u.Query()
	values.Set("input", query)
	values.Set("type", "14")
	values.Set("count", fmt.Sprintf("%d", maxInt(limit*2, 20)))
	values.Set("token", eastmoneySearchToken)
	values.Set("_", fmt.Sprintf("%d", time.Now().UnixMilli()))
	u.RawQuery = values.Encode()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("eastmoney search HTTP %s", resp.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	payload, err := parseJSONOrJSONP(raw)
	if err != nil {
		return nil, err
	}
	items := normalizeMarketSearchResults(payload, query)
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func parseJSONOrJSONP(raw []byte) (map[string]any, error) {
	text := strings.TrimSpace(string(raw))
	var payload map[string]any
	if err := json.Unmarshal([]byte(text), &payload); err == nil {
		return payload, nil
	}
	start := strings.Index(text, "(")
	end := strings.LastIndex(text, ")")
	if start <= 0 || end <= start {
		return nil, errors.New("unexpected search payload")
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(text[start+1:end])), &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func normalizeMarketSearchResults(payload map[string]any, keyword string) []marketSecurity {
	rows := marketSearchRows(payload)
	queryCode := normalizeMarketCode(keyword)
	scored := map[string]struct {
		item  marketSecurity
		score int
	}{}
	for _, row := range rows {
		item := normalizeMarketSearchRow(row)
		if item.Code == "" || item.Name == "" {
			continue
		}
		score := 10
		if queryCode != "" && item.Code == queryCode {
			score = 0
		} else if strings.Contains(item.Name, strings.TrimSpace(keyword)) {
			score = 2
		}
		if existing, ok := scored[item.Code]; !ok || score < existing.score {
			scored[item.Code] = struct {
				item  marketSecurity
				score int
			}{item: item, score: score}
		}
	}
	out := make([]struct {
		item  marketSecurity
		score int
	}, 0, len(scored))
	for _, item := range scored {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].score == out[j].score {
			return out[i].item.Code < out[j].item.Code
		}
		return out[i].score < out[j].score
	})
	items := make([]marketSecurity, 0, len(out))
	for _, entry := range out {
		items = append(items, entry.item)
	}
	return items
}

func marketSearchRows(payload map[string]any) []map[string]any {
	if table, ok := payload["QuotationCodeTable"].(map[string]any); ok {
		if rows := rowsFromAny(table["Data"]); len(rows) > 0 {
			return rows
		}
		return rowsFromAny(table["data"])
	}
	if rows := rowsFromAny(payload["Data"]); len(rows) > 0 {
		return rows
	}
	return rowsFromAny(payload["data"])
}

func rowsFromAny(value any) []map[string]any {
	source, ok := value.([]any)
	if !ok {
		return nil
	}
	out := []map[string]any{}
	for _, item := range source {
		if row, ok := item.(map[string]any); ok {
			out = append(out, row)
		}
	}
	return out
}

func normalizeMarketSearchRow(row map[string]any) marketSecurity {
	return marketSecurity{
		Code: firstNonEmpty(normalizeMarketCode(stringFrom(row["Code"])), normalizeMarketCode(stringFrom(row["SecurityCode"])), normalizeMarketCode(stringFrom(row["QuoteID"]))),
		Name: firstNonEmpty(stringFrom(row["Name"]), stringFrom(row["SecurityName"]), stringFrom(row["ShortName"]), stringFrom(row["Zqmc"])),
	}
}

func parseMarketCodes(input string, limit int) []string {
	matches := regexp.MustCompile(`\d{6}`).FindAllString(input, -1)
	out := []string{}
	seen := map[string]bool{}
	for _, raw := range matches {
		code := normalizeMarketCode(raw)
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

func normalizeMarketCode(input string) string {
	match := regexp.MustCompile(`\d{6}`).FindString(strings.TrimSpace(input))
	return match
}

func summarizeImportResults(results []models.AgentMarketImportCodeResult) map[string]int {
	summary := map[string]int{"added": 0, "updated": 0, "exists": 0, "not_found": 0, "error": 0}
	for _, result := range results {
		summary[result.Status]++
	}
	return summary
}
