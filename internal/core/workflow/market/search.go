package market

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const eastmoneySearchToken = "44c9d251add88e27b65ed86506f6e5da"

func SearchSecurities(ctx context.Context, keyword string, limit int) ([]Security, error) {
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
		items, err := fetchSecurities(ctx, endpoint, query, limit)
		if err != nil {
			errorsOut = append(errorsOut, err.Error())
			continue
		}
		if len(items) > 0 {
			return items, nil
		}
	}
	if err := importProviderError(errorsOut); err != nil {
		return nil, err
	}
	return nil, nil
}

func fetchSecurities(ctx context.Context, endpoint string, query string, limit int) ([]Security, error) {
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
	items := normalizeSearchResults(payload, query)
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

func normalizeSearchResults(payload map[string]any, keyword string) []Security {
	rows := searchRows(payload)
	queryCode := NormalizeCode(keyword)
	scored := map[string]struct {
		item  Security
		score int
	}{}
	for _, row := range rows {
		item := normalizeSearchRow(row)
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
				item  Security
				score int
			}{item: item, score: score}
		}
	}
	out := make([]struct {
		item  Security
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
	items := make([]Security, 0, len(out))
	for _, entry := range out {
		items = append(items, entry.item)
	}
	return items
}

func searchRows(payload map[string]any) []map[string]any {
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

func normalizeSearchRow(row map[string]any) Security {
	return Security{
		Code: firstNonEmpty(NormalizeCode(stringFrom(row["Code"])), NormalizeCode(stringFrom(row["SecurityCode"])), NormalizeCode(stringFrom(row["QuoteID"]))),
		Name: firstNonEmpty(stringFrom(row["Name"]), stringFrom(row["SecurityName"]), stringFrom(row["ShortName"]), stringFrom(row["Zqmc"])),
	}
}
