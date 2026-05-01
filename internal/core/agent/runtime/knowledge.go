package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
)

func (s *Service) StartKnowledgeSession(ctx context.Context, req models.AgentKnowledgeRequest) (models.AgentKnowledgeSession, error) {
	userID := firstNonEmpty(req.UserID, "default")
	source := strings.TrimSpace(req.Source)
	var session models.AgentKnowledgeSession
	_, err := s.update(ctx, func(snapshot *models.AgentSnapshot) error {
		now := time.Now().UTC()
		session = newKnowledgeSession(userID, source, now)
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
	baseDir, err := validateKnowledgeBaseDir(config)
	if err != nil {
		return models.AgentKnowledgeResult{}, err
	}
	userID := firstNonEmpty(req.UserID, "default")
	session, ok := activeKnowledgeSession(snapshot.Knowledge.Sessions, userID)
	if req.NewSession || !ok {
		session = newKnowledgeSession(userID, req.Source, time.Now().UTC())
	}
	resumeSessionID := ""
	if !req.NewSession && strings.TrimSpace(session.CodexSessionID) != "" {
		resumeSessionID = session.CodexSessionID
	}
	outputDir, _ := filepath.Abs(filepath.Join("data", "agent", "knowledge", "codex"))
	result, runErr := s.RunCodex(ctx, models.AgentCodexRequest{
		TaskID:           "kb-" + session.ID + "-" + time.Now().UTC().Format("20060102150405"),
		Prompt:           buildKnowledgePrompt(baseDir, question, resumeSessionID != ""),
		Model:            firstNonEmpty(config.CodexModel, snapshot.Settings.Evolution.CodexModel),
		ReasoningEffort:  firstNonEmpty(config.CodexReasoning, snapshot.Settings.Evolution.CodexReasoning),
		TimeoutMS:        config.TimeoutMS,
		CWD:              baseDir,
		OutputDir:        outputDir,
		Sandbox:          "read-only",
		ResumeSessionID:  resumeSessionID,
		SkipGitRepoCheck: true,
	})
	now := time.Now().UTC()
	session.Active = true
	session.Source = firstNonEmpty(req.Source, session.Source)
	session.LastQuestion = question
	session.LastOutputFile = result.OutputFile
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
	_, saveErr := s.update(ctx, func(snapshot *models.AgentSnapshot) error {
		snapshot.Knowledge.Sessions = activateKnowledgeSession(snapshot.Knowledge.Sessions, session)
		snapshot.Knowledge.UpdatedAt = now
		snapshot.UpdatedAt = now
		return nil
	})
	if saveErr != nil {
		return models.AgentKnowledgeResult{}, saveErr
	}
	answer := trimKnowledgeAnswer(result.Output, config.MaxOutputChars)
	return models.AgentKnowledgeResult{Answer: answer, Session: session, Codex: result}, runErr
}

func validateKnowledgeBaseDir(config models.AgentKnowledgeConfig) (string, error) {
	if !config.Enabled {
		return "", errors.New("knowledge base is disabled")
	}
	baseDir := strings.TrimSpace(config.BaseDir)
	if baseDir == "" {
		return "", errors.New("knowledge base base_dir is required")
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

func buildKnowledgePrompt(baseDir string, question string, resumed bool) string {
	mode := "Start a new Codex CLI knowledge-base QA session."
	if resumed {
		mode = "Continue the existing Codex CLI knowledge-base QA session."
	}
	return strings.TrimSpace(fmt.Sprintf(`
%s

Knowledge base root: %s

Instructions:
- Treat the knowledge base root as the only source of truth.
- Inspect files under the knowledge base root with local search/read commands such as rg, find, sed, and cat.
- Do not use web search or external sources.
- Do not modify, create, or delete knowledge-base files.
- Answer in the same language as the user's question.
- Cite supporting file paths and line numbers when possible.
- If the answer cannot be grounded in the knowledge-base files, say that the knowledge base does not contain enough information.

User question:
%s
`, mode, baseDir, question))
}

func newKnowledgeSession(userID string, source string, now time.Time) models.AgentKnowledgeSession {
	return models.AgentKnowledgeSession{
		ID:        uuid.NewString(),
		UserID:    strings.TrimSpace(userID),
		Source:    strings.TrimSpace(source),
		Active:    true,
		Status:    "ready",
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func activeKnowledgeSession(sessions []models.AgentKnowledgeSession, userID string) (models.AgentKnowledgeSession, bool) {
	for _, session := range sessions {
		if session.Active && strings.TrimSpace(session.UserID) == strings.TrimSpace(userID) {
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
		if strings.TrimSpace(session.UserID) == strings.TrimSpace(next.UserID) {
			session.Active = false
		}
		out = append(out, session)
	}
	return truncateList(out, 50)
}

func trimKnowledgeAnswer(answer string, maxChars int) string {
	trimmed := strings.TrimSpace(answer)
	limit := maxInt(maxChars, 1800)
	runes := []rune(trimmed)
	if len(runes) <= limit {
		return trimmed
	}
	return string(runes[:limit]) + "\n\n[truncated]"
}
