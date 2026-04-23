package search

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

type RunResult struct {
	Profile models.AgentSearchProvider
	Result  models.AgentSearchResult
}

func Run(ctx context.Context, profiles []models.AgentSearchProvider, req models.AgentSearchRequest) (RunResult, error) {
	if len(req.Plans) == 0 {
		return RunResult{}, errors.New("at least one search plan is required")
	}
	profile, ok := selectSearchProvider(profiles, req.EngineSelector)
	if !ok {
		result := models.AgentSearchResult{
			SourceChain: []string{"search_engine:missing"},
			Errors:      []string{"search engine profile not found"},
		}
		return RunResult{Profile: profile, Result: result}, nil
	}
	if !profile.Enabled {
		result := models.AgentSearchResult{
			SourceChain: []string{"search_engine:" + profile.ID + ":disabled", "search_provider:" + profile.Type},
			Errors:      []string{"search engine profile is disabled"},
		}
		return RunResult{Profile: profile, Result: result}, nil
	}
	var result models.AgentSearchResult
	var err error
	switch strings.ToLower(strings.TrimSpace(profile.Type)) {
	case "serpapi":
		result, err = runSerpAPI(ctx, profile, req)
	case "qianfan":
		result, err = runQianfan(ctx, profile, req)
	default:
		result = models.AgentSearchResult{
			SourceChain: []string{"search_provider:" + profile.Type},
			Errors:      []string{"unsupported search engine type: " + profile.Type},
		}
	}
	return RunResult{Profile: profile, Result: result}, err
}

func joinedSearchQueries(plans []models.AgentSearchPlan) string {
	queries := make([]string, 0, len(plans))
	for _, plan := range plans {
		if query := strings.TrimSpace(plan.Query); query != "" {
			queries = append(queries, query)
		}
	}
	return strings.Join(queries, " | ")
}

func cloneSearchPlans(plans []models.AgentSearchPlan) []models.AgentSearchPlan {
	out := make([]models.AgentSearchPlan, 0, len(plans))
	for _, plan := range plans {
		plan.Sites = append([]string{}, plan.Sites...)
		out = append(out, plan)
	}
	return out
}

func selectSearchProvider(profiles []models.AgentSearchProvider, selector string) (models.AgentSearchProvider, bool) {
	if len(profiles) == 0 {
		profiles = defaultSearchProvidersFromEnv()
	}
	target := normalizeSearchRef(selector)
	if target == "" || target == "default" || target == "auto" {
		for _, profile := range profiles {
			if profile.Enabled {
				return profile, true
			}
		}
		if len(profiles) > 0 {
			return profiles[0], true
		}
		return models.AgentSearchProvider{}, false
	}
	for _, profile := range profiles {
		if normalizeSearchRef(profile.ID) == target || normalizeSearchRef(profile.Type) == target {
			return profile, true
		}
	}
	return models.AgentSearchProvider{}, false
}

func defaultSearchProvidersFromEnv() []models.AgentSearchProvider {
	providers := []models.AgentSearchProvider{{
		ID:      "serpapi-default",
		Name:    "SerpAPI Default",
		Type:    "serpapi",
		Enabled: true,
		Config: map[string]any{
			"endpoint": "https://serpapi.com/search.json",
			"apiKey":   os.Getenv("SERPAPI_KEY"),
			"engine":   "google_news",
			"hl":       "zh-cn",
			"gl":       "cn",
			"num":      10,
		},
	}}
	if strings.TrimSpace(os.Getenv("QIANFAN_SEARCH_API_KEY")) != "" {
		providers = append([]models.AgentSearchProvider{{
			ID:      "qianfan-default",
			Name:    "Qianfan Default",
			Type:    "qianfan",
			Enabled: true,
			Config: map[string]any{
				"endpoint":      firstNonEmpty(os.Getenv("QIANFAN_SEARCH_ENDPOINT"), "https://qianfan.baidubce.com/v2/ai_search/web_search"),
				"apiKey":        os.Getenv("QIANFAN_SEARCH_API_KEY"),
				"searchSource":  firstNonEmpty(os.Getenv("QIANFAN_SEARCH_SOURCE"), "baidu_search_v2"),
				"edition":       firstNonEmpty(os.Getenv("QIANFAN_SEARCH_EDITION"), "standard"),
				"topK":          10,
				"recencyFilter": firstNonEmpty(os.Getenv("QIANFAN_SEARCH_RECENCY_FILTER"), "month"),
			},
		}}, providers...)
	}
	return providers
}

func runSerpAPI(ctx context.Context, profile models.AgentSearchProvider, req models.AgentSearchRequest) (models.AgentSearchResult, error) {
	apiKey := configString(profile.Config, "apiKey")
	if apiKey == "" {
		return models.AgentSearchResult{SourceChain: []string{"search_provider:serpapi", "search_status:disabled_no_key"}}, nil
	}
	endpoint := firstNonEmpty(configString(profile.Config, "endpoint"), "https://serpapi.com/search.json")
	engine := firstNonEmpty(configString(profile.Config, "engine"), "google_news")
	hl := firstNonEmpty(configString(profile.Config, "hl"), "zh-cn")
	gl := firstNonEmpty(configString(profile.Config, "gl"), "cn")
	maxItems := maxInt(req.MaxItems, 10)
	num := maxInt(configInt(profile.Config, "num"), maxItems*2)
	var out models.AgentSearchResult
	out.SourceChain = append(out.SourceChain, "search_engine:"+profile.ID, "search_provider:serpapi")
	for _, plan := range req.Plans {
		u, err := url.Parse(endpoint)
		if err != nil {
			return models.AgentSearchResult{}, err
		}
		q := u.Query()
		q.Set("engine", engine)
		q.Set("q", withSiteQuery(plan.Query, plan.Sites))
		q.Set("hl", hl)
		q.Set("gl", gl)
		q.Set("num", fmt.Sprintf("%d", minInt(num, 20)))
		q.Set("api_key", apiKey)
		if plan.Recency != "" && engine == "google" {
			q.Set("tbs", googleRecency(plan.Recency))
		}
		u.RawQuery = q.Encode()
		var payload map[string]any
		if err := getJSON(ctx, u.String(), maxInt(req.TimeoutMS, 12000), &payload); err != nil {
			out.Errors = append(out.Errors, "serpapi "+plan.Label+": "+err.Error())
			continue
		}
		out.Items = append(out.Items, normalizeSerpItems(payload)...)
		out.SourceChain = append(out.SourceChain, "search_plan:"+plan.Label)
		if len(dedupSearchItems(out.Items)) >= maxItems {
			break
		}
	}
	out.Items = truncateList(dedupSearchItems(out.Items), maxItems)
	out.SourceChain = append(out.SourceChain, searchStatus(out.Items, out.Errors))
	return out, nil
}

func runQianfan(ctx context.Context, profile models.AgentSearchProvider, req models.AgentSearchRequest) (models.AgentSearchResult, error) {
	apiKey := configString(profile.Config, "apiKey")
	if apiKey == "" {
		return models.AgentSearchResult{SourceChain: []string{"search_provider:qianfan", "search_status:disabled_no_key"}}, nil
	}
	endpoint := firstNonEmpty(configString(profile.Config, "endpoint"), "https://qianfan.baidubce.com/v2/ai_search/web_search")
	maxItems := maxInt(req.MaxItems, 10)
	topK := maxInt(configInt(profile.Config, "topK"), maxItems*2)
	var out models.AgentSearchResult
	out.SourceChain = append(out.SourceChain, "search_engine:"+profile.ID, "search_provider:qianfan")
	for _, plan := range req.Plans {
		body := map[string]any{
			"messages":             []map[string]string{{"role": "user", "content": truncateQuery(plan.Query, 72)}},
			"search_source":        firstNonEmpty(configString(profile.Config, "searchSource"), "baidu_search_v2"),
			"resource_type_filter": []map[string]any{{"type": "web", "top_k": minInt(maxInt(topK, 1), 50)}},
		}
		if edition := configString(profile.Config, "edition"); edition != "" {
			body["edition"] = edition
		}
		if recency := firstNonEmpty(plan.Recency, configString(profile.Config, "recencyFilter")); recency != "" {
			body["search_recency_filter"] = recency
		}
		if len(plan.Sites) > 0 {
			body["search_filter"] = map[string]any{"match": map[string]any{"site": plan.Sites}}
		}
		var payload map[string]any
		headers := map[string]string{
			"Authorization":              "Bearer " + apiKey,
			"X-Appbuilder-Authorization": "Bearer " + apiKey,
			"Content-Type":               "application/json",
		}
		if err := postSearchJSON(ctx, endpoint, maxInt(req.TimeoutMS, 12000), headers, body, &payload); err != nil {
			out.Errors = append(out.Errors, "qianfan "+plan.Label+": "+err.Error())
			continue
		}
		out.Items = append(out.Items, normalizeQianfanItems(payload)...)
		out.SourceChain = append(out.SourceChain, "search_plan:"+plan.Label)
		if len(dedupSearchItems(out.Items)) >= maxItems {
			break
		}
	}
	out.Items = truncateList(dedupSearchItems(out.Items), maxItems)
	out.SourceChain = append(out.SourceChain, searchStatus(out.Items, out.Errors))
	return out, nil
}

func getJSON(ctx context.Context, endpoint string, timeoutMS int, out any) error {
	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func postSearchJSON(ctx context.Context, endpoint string, timeoutMS int, headers map[string]string, body any, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
