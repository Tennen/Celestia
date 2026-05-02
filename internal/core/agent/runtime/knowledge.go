package runtime

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/core/agent/workflows/renderer"
	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
)

func (s *Service) StartKnowledgeSession(ctx context.Context, req models.AgentKnowledgeRequest) (models.AgentKnowledgeSession, error) {
	userID := firstNonEmpty(req.UserID, "default")
	source := strings.TrimSpace(req.Source)
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return models.AgentKnowledgeSession{}, err
	}
	base, _, err := resolveKnowledgeBase(snapshot.Settings.Knowledge, req.KnowledgeBaseID)
	if err != nil {
		return models.AgentKnowledgeSession{}, err
	}
	var session models.AgentKnowledgeSession
	_, err = s.update(ctx, func(snapshot *models.AgentSnapshot) error {
		now := time.Now().UTC()
		session = newKnowledgeSession(userID, source, base.ID, now)
		snapshot.Knowledge.Sessions = activateKnowledgeSession(snapshot.Knowledge.Sessions, session)
		snapshot.Knowledge.UpdatedAt = now
		snapshot.UpdatedAt = now
		return nil
	})
	return session, err
}

func (s *Service) RunKnowledge(ctx context.Context, req models.AgentKnowledgeRequest) (models.AgentKnowledgeResult, error) {
	question := strings.TrimSpace(req.Question)
	if question == "" {
		return models.AgentKnowledgeResult{}, errors.New("knowledge question is required")
	}
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return models.AgentKnowledgeResult{}, err
	}
	config := snapshot.Settings.Knowledge
	base, baseDir, err := resolveKnowledgeBase(config, req.KnowledgeBaseID)
	if err != nil {
		return models.AgentKnowledgeResult{}, err
	}
	agentProvider, err := resolveCodexAgentProvider(snapshot.Settings, config.AgentProviderID)
	if err != nil {
		return models.AgentKnowledgeResult{}, err
	}
	userID := firstNonEmpty(req.UserID, "default")
	session, ok := activeKnowledgeSession(snapshot.Knowledge.Sessions, userID, base.ID)
	if req.NewSession || !ok {
		session = newKnowledgeSession(userID, req.Source, base.ID, time.Now().UTC())
	}
	resumeSessionID := ""
	if !req.NewSession && strings.TrimSpace(session.CodexSessionID) != "" {
		resumeSessionID = session.CodexSessionID
	}
	answerPath, err := prepareKnowledgeAnswerPath(baseDir, session.ID)
	if err != nil {
		return models.AgentKnowledgeResult{}, err
	}
	codexReq := codexRequestFromAgentProvider(agentProvider, config.TimeoutMS)
	outputDir, _ := filepath.Abs(filepath.Join("data", "agent", "knowledge", "codex"))
	codexReq.TaskID = "kb-" + session.ID + "-" + time.Now().UTC().Format("20060102150405")
	codexReq.Prompt = buildKnowledgePrompt(baseDir, answerPath, question, resumeSessionID != "")
	codexReq.CWD = baseDir
	codexReq.OutputDir = outputDir
	codexReq.Sandbox = "workspace-write"
	codexReq.ResumeSessionID = resumeSessionID
	codexReq.SkipGitRepoCheck = true
	result, runErr := s.RunCodex(ctx, codexReq)
	now := time.Now().UTC()
	session.Active = true
	session.Source = firstNonEmpty(req.Source, session.Source)
	session.LastQuestion = question
	session.LastMarkdown = answerPath
	session.UpdatedAt = now
	session.Status = "succeeded"
	session.LastError = ""
	if strings.TrimSpace(result.SessionID) != "" {
		session.CodexSessionID = strings.TrimSpace(result.SessionID)
	}
	if runErr != nil {
		session.Status = "failed"
		session.LastError = runErr.Error()
	}
	markdown := ""
	rendered := models.AgentMarkdownRenderResult{}
	if runErr == nil {
		markdown, runErr = readKnowledgeAnswer(answerPath)
	}
	if runErr == nil {
		rendered, runErr = s.renderKnowledgeAnswer(ctx, snapshot.Settings.MD2Img, markdown, answerPath)
	}
	if runErr != nil {
		session.Status = "failed"
		session.LastError = runErr.Error()
	}
	_, saveErr := s.update(ctx, func(snapshot *models.AgentSnapshot) error {
		snapshot.Knowledge.Sessions = activateKnowledgeSession(snapshot.Knowledge.Sessions, session)
		snapshot.Knowledge.UpdatedAt = now
		snapshot.UpdatedAt = now
		return nil
	})
	if saveErr != nil {
		return models.AgentKnowledgeResult{}, saveErr
	}
	return models.AgentKnowledgeResult{MarkdownPath: answerPath, Images: rendered.Images, Session: session, Codex: result}, runErr
}

func (s *Service) ListKnowledgeAnswers(ctx context.Context, req models.AgentKnowledgeAnswersRequest) ([]models.AgentKnowledgeAnswer, error) {
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	base, baseDir, err := resolveKnowledgeBase(snapshot.Settings.Knowledge, req.KnowledgeBaseID)
	if err != nil {
		return nil, err
	}
	answers, err := listKnowledgeAnswerFiles(base, baseDir)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(answers, func(i, j int) bool {
		return answers[i].CreatedAt.After(answers[j].CreatedAt)
	})
	limit := maxInt(req.Limit, 20)
	if len(answers) > limit {
		answers = answers[:limit]
	}
	return answers, nil
}

func (s *Service) RenderKnowledgeAnswer(ctx context.Context, req models.AgentKnowledgeAnswerRequest) (models.AgentKnowledgeAnswerRenderResult, error) {
	id := strings.TrimSpace(req.ID)
	if id == "" {
		return models.AgentKnowledgeAnswerRenderResult{}, errors.New("knowledge answer id is required")
	}
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return models.AgentKnowledgeAnswerRenderResult{}, err
	}
	base, baseDir, err := resolveKnowledgeBase(snapshot.Settings.Knowledge, req.KnowledgeBaseID)
	if err != nil {
		return models.AgentKnowledgeAnswerRenderResult{}, err
	}
	answer, err := knowledgeAnswerByID(base, baseDir, id)
	if err != nil {
		return models.AgentKnowledgeAnswerRenderResult{}, err
	}
	markdown, err := readKnowledgeAnswer(answer.Path)
	if err != nil {
		return models.AgentKnowledgeAnswerRenderResult{}, err
	}
	rendered, err := s.renderKnowledgeAnswer(ctx, snapshot.Settings.MD2Img, markdown, answer.Path)
	if err != nil {
		return models.AgentKnowledgeAnswerRenderResult{}, err
	}
	return models.AgentKnowledgeAnswerRenderResult{Answer: answer, Images: rendered.Images}, nil
}

func resolveKnowledgeBase(config models.AgentKnowledgeConfig, baseID string) (models.AgentKnowledgeBase, string, error) {
	if !config.Enabled {
		return models.AgentKnowledgeBase{}, "", errors.New("knowledge base is disabled")
	}
	target := strings.TrimSpace(baseID)
	if target == "" {
		target = strings.TrimSpace(config.DefaultBaseID)
	}
	if target == "" {
		return models.AgentKnowledgeBase{}, "", errors.New("knowledge_base_id is required")
	}
	for _, base := range config.Bases {
		if strings.TrimSpace(base.ID) != target {
			continue
		}
		if !base.Enabled {
			return models.AgentKnowledgeBase{}, "", fmt.Errorf("knowledge base %q is disabled", target)
		}
		abs, err := validateKnowledgeBaseDir(base)
		return base, abs, err
	}
	return models.AgentKnowledgeBase{}, "", fmt.Errorf("knowledge base %q is not configured", target)
}

func validateKnowledgeBaseDir(base models.AgentKnowledgeBase) (string, error) {
	baseDir := strings.TrimSpace(base.BaseDir)
	if baseDir == "" {
		return "", fmt.Errorf("knowledge base %q base_dir is required", strings.TrimSpace(base.ID))
	}
	abs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("knowledge base directory is not accessible: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("knowledge base path %q is not a directory", abs)
	}
	return abs, nil
}

func buildKnowledgePrompt(baseDir string, answerPath string, question string, resumed bool) string {
	mode := "Start a new Codex CLI knowledge-base QA session."
	if resumed {
		mode = "Continue the existing Codex CLI knowledge-base QA session."
	}
	return strings.TrimSpace(fmt.Sprintf(`
%s

Knowledge base root: %s
Required answer markdown path: %s

Instructions:
- Treat the knowledge base root as the only source of truth.
- Inspect files under the knowledge base root with local search/read commands such as rg, find, sed, and cat.
- Do not use web search or external sources.
- Do not modify, create, or delete knowledge-base files except the required Markdown answer file under .answers.
- Write the final answer as a complete Markdown document to the required answer markdown path.
- Use headings, lists, tables, and code blocks when they improve readability.
- Answer in the same language as the user's question.
- Cite supporting file paths and line numbers when possible.
- If the answer cannot be grounded in the knowledge-base files, say that the knowledge base does not contain enough information.
- After saving the Markdown file, keep your final chat response brief; Celestia will read and render the Markdown file.

User question:
%s
`, mode, baseDir, answerPath, question))
}

func newKnowledgeSession(userID string, source string, baseID string, now time.Time) models.AgentKnowledgeSession {
	return models.AgentKnowledgeSession{
		ID:              uuid.NewString(),
		KnowledgeBaseID: strings.TrimSpace(baseID),
		UserID:          strings.TrimSpace(userID),
		Source:          strings.TrimSpace(source),
		Active:          true,
		Status:          "ready",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

func activeKnowledgeSession(sessions []models.AgentKnowledgeSession, userID string, baseID string) (models.AgentKnowledgeSession, bool) {
	for _, session := range sessions {
		if session.Active && strings.TrimSpace(session.UserID) == strings.TrimSpace(userID) && strings.TrimSpace(session.KnowledgeBaseID) == strings.TrimSpace(baseID) {
			return session, true
		}
	}
	return models.AgentKnowledgeSession{}, false
}

func activateKnowledgeSession(sessions []models.AgentKnowledgeSession, next models.AgentKnowledgeSession) []models.AgentKnowledgeSession {
	out := []models.AgentKnowledgeSession{next}
	for _, session := range sessions {
		if session.ID == next.ID {
			continue
		}
		if strings.TrimSpace(session.UserID) == strings.TrimSpace(next.UserID) && strings.TrimSpace(session.KnowledgeBaseID) == strings.TrimSpace(next.KnowledgeBaseID) {
			session.Active = false
		}
		out = append(out, session)
	}
	return truncateList(out, 50)
}

func prepareKnowledgeAnswerPath(baseDir string, sessionID string) (string, error) {
	answerDir := filepath.Join(baseDir, ".answers")
	if err := os.MkdirAll(answerDir, 0o755); err != nil {
		return "", err
	}
	stem := time.Now().UTC().Format("20060102-150405") + "-" + strings.ReplaceAll(sessionID, "-", "")
	if len(stem) > 48 {
		stem = stem[:48]
	}
	return filepath.Join(answerDir, stem+".md"), nil
}

func listKnowledgeAnswerFiles(base models.AgentKnowledgeBase, baseDir string) ([]models.AgentKnowledgeAnswer, error) {
	answerDir := filepath.Join(baseDir, ".answers")
	entries, err := os.ReadDir(answerDir)
	if errors.Is(err, fs.ErrNotExist) {
		return []models.AgentKnowledgeAnswer{}, nil
	}
	if err != nil {
		return nil, err
	}
	answers := make([]models.AgentKnowledgeAnswer, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || strings.ToLower(filepath.Ext(entry.Name())) != ".md" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		path := filepath.Join(answerDir, entry.Name())
		answers = append(answers, knowledgeAnswerFromFile(base, path, info))
	}
	return answers, nil
}

func knowledgeAnswerByID(base models.AgentKnowledgeBase, baseDir string, id string) (models.AgentKnowledgeAnswer, error) {
	if strings.ContainsAny(id, `/\`) || id == "." || id == ".." {
		return models.AgentKnowledgeAnswer{}, errors.New("knowledge answer id is invalid")
	}
	path := filepath.Join(baseDir, ".answers", id+".md")
	info, err := os.Stat(path)
	if err != nil {
		return models.AgentKnowledgeAnswer{}, fmt.Errorf("knowledge answer %q is not accessible: %w", id, err)
	}
	if info.IsDir() {
		return models.AgentKnowledgeAnswer{}, fmt.Errorf("knowledge answer %q is not a markdown file", id)
	}
	return knowledgeAnswerFromFile(base, path, info), nil
}

func knowledgeAnswerFromFile(base models.AgentKnowledgeBase, path string, info os.FileInfo) models.AgentKnowledgeAnswer {
	filename := filepath.Base(path)
	id := strings.TrimSuffix(filename, filepath.Ext(filename))
	createdAt := knowledgeAnswerCreatedAt(id, info.ModTime())
	return models.AgentKnowledgeAnswer{
		ID:              id,
		KnowledgeBaseID: strings.TrimSpace(base.ID),
		Filename:        filename,
		Path:            path,
		Title:           readKnowledgeAnswerTitle(path),
		SizeBytes:       info.Size(),
		CreatedAt:       createdAt,
		UpdatedAt:       info.ModTime(),
	}
}

func knowledgeAnswerCreatedAt(id string, fallback time.Time) time.Time {
	if len(id) >= len("20060102-150405") {
		if parsed, err := time.ParseInLocation("20060102-150405", id[:15], time.UTC); err == nil {
			return parsed
		}
	}
	return fallback
}

func readKnowledgeAnswerTitle(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			return strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
		}
		if trimmed != "" {
			return truncateText(trimmed, 80)
		}
	}
	return ""
}

func readKnowledgeAnswer(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("knowledge answer markdown was not created: %w", err)
	}
	markdown := strings.TrimSpace(string(raw))
	if markdown == "" {
		return "", errors.New("knowledge answer markdown is empty")
	}
	return markdown, nil
}

func truncateText(value string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return string(runes[:maxRunes]) + "..."
}

func (s *Service) renderKnowledgeAnswer(ctx context.Context, settings models.AgentMD2ImgConfig, markdown string, answerPath string) (models.AgentMarkdownRenderResult, error) {
	if !settings.Enabled {
		return models.AgentMarkdownRenderResult{}, errors.New("md2img is disabled in agent settings")
	}
	outputDir := filepath.Join(filepath.Dir(answerPath), "images", strings.TrimSuffix(filepath.Base(answerPath), filepath.Ext(answerPath)))
	return renderer.RenderMarkdown(ctx, models.AgentMarkdownRenderRequest{
		Markdown:  markdown,
		Mode:      firstNonEmpty(settings.Mode, "long-image"),
		OutputDir: outputDir,
	}, settings)
}
