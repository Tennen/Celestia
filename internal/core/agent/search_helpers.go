package agent

import (
	"fmt"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

func normalizeSerpItems(payload map[string]any) []models.AgentSearchResultItem {
	rows := anySlice(payload["news_results"])
	if len(rows) == 0 {
		rows = anySlice(payload["organic_results"])
	}
	items := make([]models.AgentSearchResultItem, 0, len(rows))
	for _, row := range rows {
		obj, ok := row.(map[string]any)
		if !ok {
			continue
		}
		title := stringFrom(obj["title"])
		if title == "" {
			continue
		}
		source := "unknown"
		if src, ok := obj["source"].(map[string]any); ok {
			source = firstNonEmpty(stringFrom(src["name"]), source)
		} else {
			source = firstNonEmpty(stringFrom(obj["source"]), source)
		}
		items = append(items, models.AgentSearchResultItem{
			Title:       title,
			Source:      source,
			Link:        firstNonEmpty(stringFrom(obj["link"]), stringFrom(obj["url"])),
			PublishedAt: firstNonEmpty(stringFrom(obj["date"]), stringFrom(obj["published_date"])),
			Snippet:     firstNonEmpty(stringFrom(obj["snippet"]), stringFrom(obj["summary"])),
		})
	}
	return dedupSearchItems(items)
}

func normalizeQianfanItems(payload map[string]any) []models.AgentSearchResultItem {
	rows := anySlice(payload["references"])
	items := make([]models.AgentSearchResultItem, 0, len(rows))
	for _, row := range rows {
		obj, ok := row.(map[string]any)
		if !ok {
			continue
		}
		title := stringFrom(obj["title"])
		if title == "" {
			continue
		}
		items = append(items, models.AgentSearchResultItem{
			Title:       title,
			Source:      firstNonEmpty(stringFrom(obj["website"]), stringFrom(obj["web_anchor"]), "baidu"),
			Link:        stringFrom(obj["url"]),
			PublishedAt: stringFrom(obj["date"]),
			Snippet:     clipText(stringFrom(obj["content"]), 400),
		})
	}
	return dedupSearchItems(items)
}

func dedupSearchItems(items []models.AgentSearchResultItem) []models.AgentSearchResultItem {
	out := make([]models.AgentSearchResultItem, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		key := strings.ToLower(strings.TrimSpace(item.Title)) + "|" + strings.TrimSpace(item.Link)
		if _, ok := seen[key]; ok || strings.TrimSpace(item.Title) == "" {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func anySlice(value any) []any {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	return items
}

func stringFrom(value any) string {
	return strings.TrimSpace(fmt.Sprint(value))
}

func configString(config map[string]any, key string) string {
	for candidate, value := range config {
		if normalizeSearchRef(candidate) == normalizeSearchRef(key) {
			return stringFrom(value)
		}
	}
	return ""
}

func configInt(config map[string]any, key string) int {
	value := configString(config, key)
	if value == "" {
		return 0
	}
	var out int
	_, _ = fmt.Sscanf(value, "%d", &out)
	return out
}

func normalizeSearchRef(value string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "_", "-"))
}

func withSiteQuery(query string, sites []string) string {
	if len(sites) == 0 {
		return query
	}
	site := strings.TrimSpace(sites[0])
	if site == "" || strings.Contains(query, "site:"+site) {
		return query
	}
	return "site:" + site + " " + query
}

func googleRecency(recency string) string {
	switch strings.ToLower(strings.TrimSpace(recency)) {
	case "week":
		return "qdr:w"
	case "month":
		return "qdr:m"
	case "year":
		return "qdr:y"
	default:
		return ""
	}
}

func truncateQuery(input string, maxUnits int) string {
	runes := []rune(strings.TrimSpace(input))
	if len(runes) <= maxUnits {
		return string(runes)
	}
	return string(runes[:maxUnits])
}

func clipText(input string, limit int) string {
	source := strings.TrimSpace(input)
	if len([]rune(source)) <= limit {
		return source
	}
	return string([]rune(source)[:limit]) + "..."
}

func searchStatus(items []models.AgentSearchResultItem, errors []string) string {
	if len(items) > 0 {
		return "search_status:hit"
	}
	if len(errors) > 0 {
		return "search_status:error"
	}
	return "search_status:no_hit"
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

func appendUniqueStrings(base []string, values ...string) []string {
	seen := map[string]struct{}{}
	for _, item := range base {
		seen[item] = struct{}{}
	}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		base = append(base, trimmed)
	}
	return base
}
