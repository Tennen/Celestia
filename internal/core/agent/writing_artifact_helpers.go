package agent

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

func writeWritingMeta(root string, topic models.AgentWritingTopic) error {
	meta := map[string]any{
		"topic_id":           topic.ID,
		"title":              topic.Title,
		"status":             topic.Status,
		"raw_file_count":     len(topic.RawFiles),
		"artifact_root":      topic.ArtifactRoot,
		"artifacts":          topic.Artifacts,
		"last_summarized_at": topic.LastSummarizedAt,
		"created_at":         topic.CreatedAt,
		"updated_at":         topic.UpdatedAt,
	}
	return writeWritingJSON(filepath.Join(root, "meta.json"), meta)
}

func writeWritingJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func readWritingArtifactJSON[T any](root string) []T {
	out := []T{}
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry == nil || entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		var value T
		if json.Unmarshal(data, &value) == nil {
			out = append(out, value)
		}
		return nil
	})
	return out
}

func writingKnowledgePath(root string, section string, at time.Time, name string) string {
	return filepath.Join(root, "knowledge", section, at.Format("2006"), at.Format("01"), name)
}

func writeStateFiles(root string, dir string, state models.AgentWritingState, suffix string) {
	writeStateFile(filepath.Join(root, dir, "summary"+suffix), state.Summary)
	writeStateFile(filepath.Join(root, dir, "outline"+suffix), state.Outline)
	writeStateFile(filepath.Join(root, dir, "draft"+suffix), state.Draft)
}

func writeStateFile(path string, content string) {
	if strings.TrimSpace(content) == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o644)
}

func relativeSlash(root string, target string) string {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return filepath.ToSlash(target)
	}
	return filepath.ToSlash(rel)
}

func rawFileName(index int) string {
	return pad3(index) + ".md"
}

func rawFileIndex(name string) int {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	value, err := strconv.Atoi(base)
	if err != nil || value < 1 {
		return 1
	}
	return value
}

func pad3(value int) string {
	if value < 0 {
		value = 0
	}
	if value < 10 {
		return "00" + strconv.Itoa(value)
	}
	if value < 100 {
		return "0" + strconv.Itoa(value)
	}
	return strconv.Itoa(value)
}

func countNonEmptyLines(text string) int {
	count := 0
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func inferWritingInputMode(text string, urls []string) string {
	hasImage := regexpImageMarker(text)
	hasText := strings.TrimSpace(writingURLPattern.ReplaceAllString(text, "")) != ""
	switch {
	case hasImage && hasText:
		return "mixed"
	case hasImage:
		return "image"
	case len(urls) > 0 && hasText:
		return "mixed"
	case len(urls) > 0:
		return "url"
	default:
		return "text"
	}
}

func regexpImageMarker(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	return strings.Contains(lower, "](") || strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") || strings.HasSuffix(lower, ".webp") || strings.HasSuffix(lower, ".gif")
}

func inferWritingSource(text string, urls []string) string {
	if len(urls) > 0 {
		parsed, err := url.Parse(urls[0])
		if err == nil && parsed.Hostname() != "" {
			host := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
			switch {
			case strings.Contains(host, "xiaohongshu"):
				return "xiaohongshu"
			case host == "x.com" || strings.Contains(host, "twitter"):
				return "x"
			case strings.Contains(host, "weibo"):
				return "weibo"
			default:
				return host
			}
		}
		return "url"
	}
	if strings.Contains(text, "assistant:") || strings.Contains(text, "user:") || strings.Contains(text, "系统:") {
		return "chat"
	}
	return "manual"
}

func inferWritingMaterialType(inputMode string, source string) string {
	if source == "xiaohongshu" || source == "x" || source == "weibo" {
		return "social_post"
	}
	if inputMode == "image" {
		return "image"
	}
	if inputMode == "mixed" {
		return "mixed"
	}
	if inputMode == "url" {
		return "web_page"
	}
	if source == "chat" {
		return "chat_record"
	}
	return "local_text"
}

func firstNonEmptyLines(text string, maxLines int) []string {
	lines := []string{}
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lines = append(lines, trimmed)
		if len(lines) >= maxLines {
			break
		}
	}
	return lines
}

func extractWritingEntities(points []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, point := range points {
		for _, token := range strings.Fields(point) {
			trimmed := strings.Trim(token, ".,;:!?()[]{}\"'")
			if len(trimmed) < 3 || strings.ToLower(trimmed) == trimmed {
				continue
			}
			if _, ok := seen[trimmed]; ok {
				continue
			}
			seen[trimmed] = struct{}{}
			out = append(out, trimmed)
			if len(out) >= 10 {
				return out
			}
		}
	}
	return out
}

func nextWritingDocumentVersion(topicID string) int {
	documents := readWritingArtifactJSON[models.AgentWritingDocument](filepath.Join(writingTopicRoot(topicID), "knowledge", "documents"))
	maxVersion := 0
	for _, document := range documents {
		if document.Version > maxVersion {
			maxVersion = document.Version
		}
	}
	return maxVersion + 1
}

func inferWritingDocumentMode(title string) string {
	normalized := strings.ToLower(strings.TrimSpace(title))
	if strings.Contains(normalized, "research") || strings.Contains(normalized, "研究") {
		return "research_note"
	}
	if strings.Contains(normalized, "memo") || strings.Contains(normalized, "备忘") {
		return "memo"
	}
	if strings.Contains(normalized, "article") || strings.Contains(normalized, "文章") {
		return "article"
	}
	return "knowledge_entry"
}
