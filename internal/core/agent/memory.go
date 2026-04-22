package agent

import (
	"fmt"
	"hash/fnv"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
)

const memoryVectorDimension = 128

type memoryHit struct {
	record models.AgentSummaryMemoryRecord
	score  float64
}

func buildMemoryPrompt(snapshot models.AgentSnapshot, sessionID string, input string) (string, map[string]any) {
	cfg := snapshot.Settings.Memory
	metadata := map[string]any{"memory_enabled": cfg.Enabled}
	if !cfg.Enabled {
		return input, metadata
	}

	lines := []string{}
	if window, ok := activeMemoryWindow(snapshot.Memory.Windows, sessionID, cfg, time.Now().UTC()); ok {
		metadata["window_id"] = window.WindowID
		lines = append(lines, formatWindowMemory(window))
	}

	hits := searchSummaryMemory(snapshot.Memory.Summaries, sessionID, input, cfg.SummaryTopK)
	rawRecords := rawRecordsForHits(snapshot.Memory.RawRecords, sessionID, hits, cfg.RawRefLimit, cfg.RawRecordLimit)
	if len(hits) > 0 {
		metadata["summary_hits"] = len(hits)
		metadata["raw_replay"] = len(rawRecords)
		lines = append(lines, formatHybridMemory(input, hits, rawRecords))
	} else if fallback := recentRawMemory(snapshot.Memory.RawRecords, sessionID, cfg.RawRecordLimit); len(fallback) > 0 {
		metadata["summary_hits"] = 0
		metadata["raw_replay"] = len(fallback)
		lines = append(lines, formatRawReplay(fallback))
	}

	if len(lines) == 0 {
		return input, metadata
	}
	metadata["memory_context_used"] = true
	return strings.Join([]string{
		"Use the following session memory only when it is relevant. Do not mention it unless needed.",
		strings.Join(lines, "\n\n"),
		"[current_user_input]",
		input,
	}, "\n"), metadata
}

func appendConversationMemory(snapshot *models.AgentSnapshot, item models.AgentConversation, now time.Time) map[string]any {
	cfg := snapshot.Settings.Memory
	if !cfg.Enabled {
		return map[string]any{"memory_enabled": false}
	}
	metadata := map[string]any{"memory_enabled": true}
	source := stringFrom(item.Metadata["actor"])
	raw := models.AgentRawMemoryRecord{
		ID:        "raw_" + uuid.NewString(),
		SessionID: item.SessionID,
		RequestID: item.ID,
		Source:    firstNonEmpty(source, "agent"),
		User:      item.Input,
		Assistant: item.Response,
		Metadata: map[string]any{
			"status":              item.Status,
			"direct_input_mapped": item.Metadata["direct_input_mapped"],
		},
		CreatedAt: now,
	}
	snapshot.Memory.RawRecords = append(snapshot.Memory.RawRecords, raw)
	if len(snapshot.Memory.RawRecords) > 500 {
		snapshot.Memory.RawRecords = snapshot.Memory.RawRecords[len(snapshot.Memory.RawRecords)-500:]
	}

	windowID := upsertConversationWindow(snapshot, item, now)
	if windowID != "" {
		metadata["window_id"] = windowID
	}
	compaction := compactMemory(snapshot, cfg, item.SessionID, now)
	for key, value := range compaction {
		metadata[key] = value
	}
	snapshot.Memory.UpdatedAt = now
	return metadata
}

func upsertConversationWindow(snapshot *models.AgentSnapshot, item models.AgentConversation, now time.Time) string {
	cfg := snapshot.Settings.Memory
	existingIndex := -1
	var existing models.AgentConversationWindow
	if window, ok := activeMemoryWindow(snapshot.Memory.Windows, item.SessionID, cfg, now); ok {
		existing = window
		for idx := range snapshot.Memory.Windows {
			if snapshot.Memory.Windows[idx].WindowID == window.WindowID {
				existingIndex = idx
				break
			}
		}
	}
	if existing.WindowID == "" {
		existing = models.AgentConversationWindow{
			SessionID: item.SessionID,
			WindowID:  "win_" + uuid.NewString(),
			StartedAt: now,
			Turns:     []models.AgentMemoryTurn{},
		}
	}
	userAt := item.CreatedAt
	assistantAt := now
	existing.LastUserAt = &userAt
	existing.LastAssistantAt = &assistantAt
	existing.Turns = append(existing.Turns,
		models.AgentMemoryTurn{Role: "user", Content: item.Input, CreatedAt: userAt},
		models.AgentMemoryTurn{Role: "assistant", Content: item.Response, CreatedAt: assistantAt},
	)
	maxMessages := maxInt(cfg.WindowMaxTurns, 6) * 2
	if len(existing.Turns) > maxMessages {
		existing.Turns = existing.Turns[len(existing.Turns)-maxMessages:]
	}
	if existingIndex >= 0 {
		snapshot.Memory.Windows[existingIndex] = existing
	} else {
		snapshot.Memory.Windows = append(snapshot.Memory.Windows, existing)
	}
	if len(snapshot.Memory.Windows) > 100 {
		snapshot.Memory.Windows = snapshot.Memory.Windows[len(snapshot.Memory.Windows)-100:]
	}
	return existing.WindowID
}

func compactMemory(snapshot *models.AgentSnapshot, cfg models.AgentMemoryConfig, sessionID string, now time.Time) map[string]any {
	pending := []int{}
	for idx, item := range snapshot.Memory.RawRecords {
		if item.SessionID == sessionID && item.SummarizedAt == nil {
			pending = append(pending, idx)
		}
	}
	metadata := map[string]any{"memory_pending": len(pending)}
	threshold := maxInt(cfg.CompactEveryRounds, 4)
	if len(pending) < threshold {
		metadata["memory_compacted"] = false
		return metadata
	}
	limit := maxInt(cfg.CompactMaxBatchSize, 8)
	if limit < threshold {
		limit = threshold
	}
	if len(pending) > limit {
		pending = pending[:limit]
	}

	batch := make([]models.AgentRawMemoryRecord, 0, len(pending))
	rawRefs := make([]string, 0, len(pending))
	for _, idx := range pending {
		batch = append(batch, snapshot.Memory.RawRecords[idx])
		rawRefs = append(rawRefs, snapshot.Memory.RawRecords[idx].ID)
	}
	summary := buildFallbackSummary(sessionID, batch, rawRefs, now)
	upsertSummary(snapshot, summary)
	for _, idx := range pending {
		snapshot.Memory.RawRecords[idx].SummarizedAt = &now
	}
	if len(snapshot.Memory.Summaries) > 200 {
		snapshot.Memory.Summaries = snapshot.Memory.Summaries[len(snapshot.Memory.Summaries)-200:]
	}
	metadata["memory_compacted"] = true
	metadata["memory_summary_id"] = summary.ID
	metadata["memory_batch_count"] = len(batch)
	return metadata
}

func buildFallbackSummary(sessionID string, batch []models.AgentRawMemoryRecord, rawRefs []string, now time.Time) models.AgentSummaryMemoryRecord {
	environment := []string{}
	taskResults := []string{}
	for _, item := range batch {
		if item.Source != "" {
			environment = appendUniqueStrings(environment, "source="+item.Source)
		}
		taskResults = append(taskResults, fmt.Sprintf("[%s] user=%q assistant=%q", item.ID, clip(oneLine(item.User), 120), clip(oneLine(item.Assistant), 160)))
	}
	return models.AgentSummaryMemoryRecord{
		ID:          "summary_batch_" + hashString(sessionID+"|"+strings.Join(rawRefs, "|")),
		SessionID:   sessionID,
		Environment: environment,
		TaskResults: taskResults,
		RawRefs:     append([]string{}, rawRefs...),
		CreatedAt:   batch[0].CreatedAt,
		UpdatedAt:   now,
	}
}

func upsertSummary(snapshot *models.AgentSnapshot, summary models.AgentSummaryMemoryRecord) {
	for idx := range snapshot.Memory.Summaries {
		if snapshot.Memory.Summaries[idx].ID == summary.ID {
			snapshot.Memory.Summaries[idx] = summary
			return
		}
	}
	snapshot.Memory.Summaries = append(snapshot.Memory.Summaries, summary)
}

func activeMemoryWindow(windows []models.AgentConversationWindow, sessionID string, cfg models.AgentMemoryConfig, at time.Time) (models.AgentConversationWindow, bool) {
	timeout := time.Duration(maxInt(cfg.WindowTimeoutSeconds, 180)) * time.Second
	for idx := len(windows) - 1; idx >= 0; idx-- {
		window := windows[idx]
		if window.SessionID != sessionID || window.LastAssistantAt == nil {
			continue
		}
		if at.Sub(*window.LastAssistantAt) <= timeout {
			return window, true
		}
	}
	return models.AgentConversationWindow{}, false
}

func searchSummaryMemory(records []models.AgentSummaryMemoryRecord, sessionID string, query string, topK int) []memoryHit {
	candidates := []memoryHit{}
	queryTokens := tokenizeMemory(query)
	queryVector := buildMemoryVector(queryTokens)
	for _, record := range records {
		if record.SessionID != sessionID {
			continue
		}
		text := summaryMemoryText(record)
		score := 0.0
		if len(queryTokens) == 0 {
			score = 0
		} else {
			docTokens := tokenizeMemory(text)
			vectorScore := cosine(queryVector, buildMemoryVector(docTokens))
			lexical := lexicalScore(queryTokens, docTokens, strings.ToLower(query), strings.ToLower(text))
			score = 0.6*vectorScore + 0.4*lexical
		}
		candidates = append(candidates, memoryHit{record: record, score: score})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].record.UpdatedAt.After(candidates[j].record.UpdatedAt)
		}
		return candidates[i].score > candidates[j].score
	})
	limit := maxInt(topK, 4)
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return candidates
}

func rawRecordsForHits(records []models.AgentRawMemoryRecord, sessionID string, hits []memoryHit, refLimit int, recordLimit int) []models.AgentRawMemoryRecord {
	refs := []string{}
	for _, hit := range hits {
		refs = appendUniqueStrings(refs, hit.record.RawRefs...)
	}
	if len(refs) > maxInt(refLimit, 8) {
		refs = refs[:maxInt(refLimit, 8)]
	}
	byID := map[string]models.AgentRawMemoryRecord{}
	for _, item := range records {
		if item.SessionID == sessionID {
			byID[item.ID] = item
		}
	}
	out := []models.AgentRawMemoryRecord{}
	for _, id := range refs {
		if item, ok := byID[id]; ok {
			out = append(out, item)
		}
	}
	if len(out) > maxInt(recordLimit, 3) {
		out = out[:maxInt(recordLimit, 3)]
	}
	return out
}

func recentRawMemory(records []models.AgentRawMemoryRecord, sessionID string, limit int) []models.AgentRawMemoryRecord {
	out := []models.AgentRawMemoryRecord{}
	for idx := len(records) - 1; idx >= 0 && len(out) < maxInt(limit, 3); idx-- {
		if records[idx].SessionID == sessionID {
			out = append(out, records[idx])
		}
	}
	return out
}

func formatWindowMemory(window models.AgentConversationWindow) string {
	lines := []string{"[conversation_window]", "window_id: " + window.WindowID}
	for _, turn := range window.Turns {
		lines = append(lines, fmt.Sprintf("- %s: %s", turn.Role, clip(oneLine(turn.Content), 260)))
	}
	return strings.Join(lines, "\n")
}

func formatHybridMemory(query string, hits []memoryHit, rawRecords []models.AgentRawMemoryRecord) string {
	lines := []string{"[hybrid_memory]", "query: " + clip(oneLine(query), 380), "summary_hits:"}
	for idx, hit := range hits {
		lines = append(lines, fmt.Sprintf("- #%d id=%s score=%.3f updated_at=%s", idx+1, hit.record.ID, hit.score, hit.record.UpdatedAt.Format(time.RFC3339)))
		lines = append(lines, "  summary: "+clip(oneLine(summaryMemoryText(hit.record)), 380))
	}
	lines = append(lines, "raw_replay:")
	if len(rawRecords) == 0 {
		lines = append(lines, "- none")
		return strings.Join(lines, "\n")
	}
	for idx, item := range rawRecords {
		lines = append(lines, fmt.Sprintf("- #%d id=%s request=%s source=%s created_at=%s", idx+1, item.ID, item.RequestID, item.Source, item.CreatedAt.Format(time.RFC3339)))
		lines = append(lines, "  user: "+clip(oneLine(item.User), 260))
		lines = append(lines, "  assistant: "+clip(oneLine(item.Assistant), 260))
	}
	return strings.Join(lines, "\n")
}

func formatRawReplay(rawRecords []models.AgentRawMemoryRecord) string {
	lines := []string{"[raw_memory_recent]"}
	for idx, item := range rawRecords {
		lines = append(lines, fmt.Sprintf("- #%d user=%s assistant=%s", idx+1, clip(oneLine(item.User), 220), clip(oneLine(item.Assistant), 220)))
	}
	return strings.Join(lines, "\n")
}

func summaryMemoryText(record models.AgentSummaryMemoryRecord) string {
	return strings.Join(append(append(append(append([]string{}, record.UserFacts...), record.Environment...), record.LongTermPreferences...), record.TaskResults...), "\n")
}

func tokenizeMemory(input string) []string {
	tokens := []string{}
	var current strings.Builder
	flush := func() {
		if current.Len() > 0 {
			tokens = append(tokens, strings.ToLower(current.String()))
			current.Reset()
		}
	}
	for _, r := range input {
		if r >= '\u4e00' && r <= '\u9fff' {
			flush()
			tokens = append(tokens, string(r))
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(unicode.ToLower(r))
			continue
		}
		flush()
	}
	flush()
	return tokens
}

func buildMemoryVector(tokens []string) []float64 {
	vector := make([]float64, memoryVectorDimension)
	for _, token := range tokens {
		vector[int(hashUint(token)%memoryVectorDimension)]++
	}
	norm := 0.0
	for _, value := range vector {
		norm += value * value
	}
	if norm == 0 {
		return vector
	}
	length := math.Sqrt(norm)
	for idx := range vector {
		vector[idx] /= length
	}
	return vector
}

func cosine(left []float64, right []float64) float64 {
	if len(left) != len(right) {
		return 0
	}
	score := 0.0
	for idx := range left {
		score += left[idx] * right[idx]
	}
	return score
}

func lexicalScore(queryTokens []string, docTokens []string, query string, doc string) float64 {
	if len(queryTokens) == 0 || len(docTokens) == 0 {
		return 0
	}
	docFreq := map[string]int{}
	for _, token := range docTokens {
		docFreq[token]++
	}
	matched := 0
	seen := map[string]bool{}
	for _, token := range queryTokens {
		if seen[token] {
			continue
		}
		seen[token] = true
		if docFreq[token] > 0 {
			matched++
		}
	}
	coverage := float64(matched) / float64(len(seen))
	exact := 0.0
	if query != "" && doc != "" {
		if doc == query {
			exact = 1
		} else if strings.HasPrefix(doc, query) {
			exact = 0.9
		} else if strings.Contains(doc, query) {
			exact = 0.75
		}
	}
	return 0.75*coverage + 0.25*exact
}

func hashString(value string) string {
	return fmt.Sprintf("%08x", hashUint(value))
}

func hashUint(value string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(value))
	return h.Sum32()
}

func oneLine(input string) string {
	return strings.Join(strings.Fields(input), " ")
}

func clip(input string, max int) string {
	if len(input) <= max {
		return input
	}
	if max <= 3 {
		return input[:max]
	}
	return input[:max-3] + "..."
}
