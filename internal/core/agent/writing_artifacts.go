package agent

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

const writingRawMaxLines = 500

var writingIDPattern = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
var writingURLPattern = regexp.MustCompile(`https?://[^\s)]+`)

func prepareWritingTopic(topic models.AgentWritingTopic) (models.AgentWritingTopic, error) {
	root := writingTopicRoot(topic.ID)
	if err := ensureWritingDirs(root); err != nil {
		return topic, err
	}
	topic.ArtifactRoot = root
	topic.RawFiles = listWritingRawFiles(root)
	topic.Artifacts = collectWritingArtifacts(topic.ID)
	return topic, writeWritingMeta(root, topic)
}

func persistWritingMaterial(topic models.AgentWritingTopic, material models.AgentWritingMaterial) (models.AgentWritingMaterial, models.AgentWritingTopic, error) {
	root := writingTopicRoot(topic.ID)
	if err := ensureWritingDirs(root); err != nil {
		return material, topic, err
	}
	material = enrichWritingMaterial(material)
	rawFile, err := appendWritingRaw(root, material)
	if err != nil {
		return material, topic, err
	}
	material.RawFile = rawFile.Name
	artifactPath := writingKnowledgePath(root, "materials", material.CreatedAt, material.ID+".json")
	material.ArtifactPath = relativeSlash(root, artifactPath)
	if err := writeWritingJSON(artifactPath, material); err != nil {
		return material, topic, err
	}
	topic.ArtifactRoot = root
	topic.RawFiles = listWritingRawFiles(root)
	topic.Artifacts = collectWritingArtifacts(topic.ID)
	return material, topic, writeWritingMeta(root, topic)
}

func persistWritingState(topic models.AgentWritingTopic, previous models.AgentWritingState, next models.AgentWritingState, generatedAt time.Time) (models.AgentWritingTopic, error) {
	root := writingTopicRoot(topic.ID)
	if err := ensureWritingDirs(root); err != nil {
		return topic, err
	}
	writeStateFiles(root, "backup", previous, ".prev.md")
	writeStateFiles(root, "state", next, ".md")
	insight := buildWritingInsight(topic, generatedAt)
	insightPath := writingKnowledgePath(root, "insights", generatedAt, insight.ID+".json")
	insight.Path = relativeSlash(root, insightPath)
	if err := writeWritingJSON(insightPath, insight); err != nil {
		return topic, err
	}
	document := buildWritingDocument(topic, insight, generatedAt)
	markdownPath := filepath.Join(root, document.Path)
	if err := os.MkdirAll(filepath.Dir(markdownPath), 0o755); err != nil {
		return topic, err
	}
	if err := os.WriteFile(markdownPath, []byte(buildWritingDocumentMarkdown(document, insight, next)+"\n"), 0o644); err != nil {
		return topic, err
	}
	if err := writeWritingJSON(strings.TrimSuffix(markdownPath, ".md")+".meta.json", document); err != nil {
		return topic, err
	}
	topic.LastSummarizedAt = &generatedAt
	topic.RawFiles = listWritingRawFiles(root)
	topic.Artifacts = collectWritingArtifacts(topic.ID)
	topic.ArtifactRoot = root
	return topic, writeWritingMeta(root, topic)
}

func writingTopicRoot(topicID string) string {
	safeID := writingIDPattern.ReplaceAllString(strings.TrimSpace(topicID), "_")
	if safeID == "" {
		safeID = "topic"
	}
	return filepath.Join("data", "agent", "writing", "topics", safeID)
}

func ensureWritingDirs(root string) error {
	for _, dir := range []string{
		root,
		filepath.Join(root, "raw"),
		filepath.Join(root, "state"),
		filepath.Join(root, "backup"),
		filepath.Join(root, "knowledge", "materials"),
		filepath.Join(root, "knowledge", "insights"),
		filepath.Join(root, "knowledge", "documents"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func appendWritingRaw(root string, material models.AgentWritingMaterial) (models.AgentWritingRawFile, error) {
	rawDir := filepath.Join(root, "raw")
	files := listWritingRawFiles(root)
	nextIndex := 1
	target := filepath.Join(rawDir, "001.md")
	if len(files) > 0 {
		last := files[len(files)-1]
		target = filepath.Join(root, filepath.FromSlash(last.Path))
		nextIndex = rawFileIndex(last.Name)
		if last.LineCount >= writingRawMaxLines {
			nextIndex++
			target = filepath.Join(rawDir, rawFileName(nextIndex))
		}
	}
	entry := strings.TrimSpace(material.Content)
	if material.Title != "" {
		entry = "# " + material.Title + "\n\n" + entry
	}
	previous, _ := os.ReadFile(target)
	next := strings.TrimSpace(string(previous))
	if next != "" {
		next += "\n\n---\n\n"
	}
	next += entry
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		return models.AgentWritingRawFile{}, err
	}
	if err := os.WriteFile(target, []byte(strings.TrimSpace(next)+"\n"), 0o644); err != nil {
		return models.AgentWritingRawFile{}, err
	}
	return models.AgentWritingRawFile{
		Name:      filepath.Base(target),
		Path:      relativeSlash(root, target),
		LineCount: countNonEmptyLines(next),
	}, nil
}

func listWritingRawFiles(root string) []models.AgentWritingRawFile {
	rawDir := filepath.Join(root, "raw")
	entries, err := os.ReadDir(rawDir)
	if err != nil {
		return []models.AgentWritingRawFile{}
	}
	files := []models.AgentWritingRawFile{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			continue
		}
		path := filepath.Join(rawDir, entry.Name())
		content, _ := os.ReadFile(path)
		files = append(files, models.AgentWritingRawFile{
			Name:      entry.Name(),
			Path:      relativeSlash(root, path),
			LineCount: countNonEmptyLines(string(content)),
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	return files
}

func collectWritingArtifacts(topicID string) models.AgentWritingArtifacts {
	root := writingTopicRoot(topicID)
	insights := readWritingArtifactJSON[models.AgentWritingInsight](filepath.Join(root, "knowledge", "insights"))
	documents := readWritingArtifactJSON[models.AgentWritingDocument](filepath.Join(root, "knowledge", "documents"))
	materials := readWritingArtifactJSON[models.AgentWritingMaterial](filepath.Join(root, "knowledge", "materials"))
	sort.Slice(insights, func(i, j int) bool { return insights[i].CreatedAt.Before(insights[j].CreatedAt) })
	sort.Slice(documents, func(i, j int) bool { return documents[i].CreatedAt.Before(documents[j].CreatedAt) })
	out := models.AgentWritingArtifacts{
		MaterialCount: len(materials),
		InsightCount:  len(insights),
		DocumentCount: len(documents),
	}
	if len(insights) > 0 {
		out.LatestInsight = &insights[len(insights)-1]
	}
	if len(documents) > 0 {
		out.LatestDocument = &documents[len(documents)-1]
	}
	return out
}

func enrichWritingMaterial(material models.AgentWritingMaterial) models.AgentWritingMaterial {
	content := strings.TrimSpace(material.Content)
	urls := writingURLPattern.FindAllString(content, -1)
	material.URLs = uniqueStrings(urls)
	material.InputMode = inferWritingInputMode(content, material.URLs)
	material.Source = inferWritingSource(content, material.URLs)
	material.Type = inferWritingMaterialType(material.InputMode, material.Source)
	if material.Metadata == nil {
		material.Metadata = map[string]any{}
	}
	material.Metadata["ingest_channel"] = "agent.writing.append"
	return material
}

func buildWritingInsight(topic models.AgentWritingTopic, generatedAt time.Time) models.AgentWritingInsight {
	points := []string{}
	tags := map[string]struct{}{}
	for _, material := range topic.Materials {
		for _, line := range firstNonEmptyLines(material.Content, 3) {
			if len(points) < 8 {
				points = append(points, line)
			}
		}
		for _, tag := range []string{material.Source, material.Type, material.InputMode} {
			if tag != "" {
				tags[tag] = struct{}{}
			}
		}
	}
	tagList := make([]string, 0, len(tags))
	for tag := range tags {
		tagList = append(tagList, tag)
	}
	sort.Strings(tagList)
	ids := make([]string, 0, len(topic.Materials))
	for _, material := range topic.Materials {
		ids = append(ids, material.ID)
	}
	return models.AgentWritingInsight{
		ID:           "ins_" + generatedAt.Format("20060102_150405000"),
		TopicID:      topic.ID,
		MaterialIDs:  ids,
		Summary:      "Collected " + intString(len(topic.Materials)) + " materials for " + topic.Title + ".",
		KeyPoints:    points,
		Tags:         tagList,
		Entities:     extractWritingEntities(points),
		QualityScore: math.Min(1, float64(len(points))/8),
		CreatedAt:    generatedAt,
	}
}

func buildWritingDocument(topic models.AgentWritingTopic, insight models.AgentWritingInsight, generatedAt time.Time) models.AgentWritingDocument {
	version := nextWritingDocumentVersion(topic.ID)
	mode := inferWritingDocumentMode(topic.Title)
	id := "doc_" + generatedAt.Format("20060102_150405000")
	stem := id + "_v" + pad3(version) + "_" + mode
	path := filepath.ToSlash(filepath.Join("knowledge", "documents", generatedAt.Format("2006"), generatedAt.Format("01"), stem+".md"))
	return models.AgentWritingDocument{
		ID:          id,
		TopicID:     topic.ID,
		MaterialIDs: insight.MaterialIDs,
		InsightID:   insight.ID,
		Mode:        mode,
		Title:       topic.Title,
		Path:        path,
		Version:     version,
		CreatedAt:   generatedAt,
	}
}

func buildWritingDocumentMarkdown(document models.AgentWritingDocument, insight models.AgentWritingInsight, state models.AgentWritingState) string {
	lines := []string{"---", "id: " + document.ID, "topic_id: " + document.TopicID, "mode: " + document.Mode, "created_at: " + document.CreatedAt.Format(time.RFC3339), "---", "", "# " + document.Title, "", "## Summary", insight.Summary, "", "## Key Points"}
	if len(insight.KeyPoints) == 0 {
		lines = append(lines, "- (empty)")
	} else {
		for _, point := range insight.KeyPoints {
			lines = append(lines, "- "+point)
		}
	}
	lines = append(lines, "", "## Outline", strings.TrimSpace(state.Outline), "", "## Draft", strings.TrimSpace(state.Draft), "", "## Material IDs")
	for _, id := range document.MaterialIDs {
		lines = append(lines, "- "+id)
	}
	return strings.Join(lines, "\n")
}
