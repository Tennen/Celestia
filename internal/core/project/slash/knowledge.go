package slash

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

func (s *Service) runKnowledge(ctx context.Context, req models.ProjectInputRequest, args []string) (string, map[string]any, error) {
	if s.agent == nil {
		return "", map[string]any{"domain": "knowledge"}, errors.New("knowledge runtime is not available")
	}
	if len(args) == 0 || equalRef(args[0], "help") {
		return knowledgeHelp(), map[string]any{"domain": "knowledge", "action": "help"}, nil
	}
	baseID, args := extractKnowledgeBaseSelector(args)
	if len(args) == 0 {
		return knowledgeHelp(), map[string]any{"domain": "knowledge", "action": "help"}, nil
	}
	action := strings.ToLower(strings.TrimSpace(args[0]))
	switch action {
	case "list", "bases":
		snapshot, err := s.agent.Snapshot(ctx)
		if err != nil {
			return "", map[string]any{"domain": "knowledge", "action": "list"}, err
		}
		return formatKnowledgeBases(snapshot), map[string]any{"domain": "knowledge", "action": "list"}, nil
	case "status":
		nextBaseID, _ := extractKnowledgeBaseSelector(args[1:])
		baseID = firstNonEmpty(nextBaseID, baseID)
		snapshot, err := s.agent.Snapshot(ctx)
		if err != nil {
			return "", map[string]any{"domain": "knowledge", "action": "status"}, err
		}
		return formatKnowledgeStatus(snapshot, knowledgeUserID(req), baseID), map[string]any{"domain": "knowledge", "action": "status"}, nil
	case "new":
		nextBaseID, questionArgs := extractKnowledgeBaseSelector(args[1:])
		baseID = firstNonEmpty(nextBaseID, baseID)
		question := strings.TrimSpace(strings.Join(questionArgs, " "))
		if question == "" {
			session, err := s.agent.StartKnowledgeSession(ctx, models.AgentKnowledgeRequest{
				KnowledgeBaseID: baseID,
				UserID:          knowledgeUserID(req),
				Source:          req.Source,
			})
			if err != nil {
				return "", map[string]any{"domain": "knowledge", "action": "new"}, err
			}
			return fmt.Sprintf("Knowledge Codex conversation started: %s", session.ID), map[string]any{
				"domain":            "knowledge",
				"action":            "new",
				"session_id":        session.ID,
				"knowledge_base_id": session.KnowledgeBaseID,
			}, nil
		}
		return s.runKnowledgeQuestion(ctx, req, baseID, question, true)
	case "ask":
		nextBaseID, questionArgs := extractKnowledgeBaseSelector(args[1:])
		baseID = firstNonEmpty(nextBaseID, baseID)
		question := strings.TrimSpace(strings.Join(questionArgs, " "))
		if question == "" {
			return "", map[string]any{"domain": "knowledge", "action": "ask"}, errors.New("usage: /kb ask <question>")
		}
		return s.runKnowledgeQuestion(ctx, req, baseID, question, false)
	default:
		return s.runKnowledgeQuestion(ctx, req, baseID, strings.Join(args, " "), false)
	}
}

func (s *Service) runKnowledgeQuestion(ctx context.Context, req models.ProjectInputRequest, baseID string, question string, newSession bool) (string, map[string]any, error) {
	result, err := s.agent.RunKnowledge(ctx, models.AgentKnowledgeRequest{
		Question:        question,
		KnowledgeBaseID: baseID,
		UserID:          knowledgeUserID(req),
		Source:          req.Source,
		NewSession:      newSession,
	})
	metadata := map[string]any{
		"domain":            "knowledge",
		"action":            "ask",
		"reply_kind":        "image",
		"new_session":       newSession,
		"session_id":        result.Session.ID,
		"knowledge_base_id": result.Session.KnowledgeBaseID,
		"codex_session_id":  result.Session.CodexSessionID,
		"codex_output_file": result.Codex.OutputFile,
		"codex_exit_code":   result.Codex.ExitCode,
		"markdown_path":     result.MarkdownPath,
	}
	metadata = withProjectImages(metadata, result.Images)
	if err != nil {
		return "", metadata, err
	}
	return "Knowledge answer rendered to image.", metadata, nil
}

func knowledgeUserID(req models.ProjectInputRequest) string {
	return firstNonEmpty(req.UserID, req.SessionID, req.Actor, "default")
}

func extractKnowledgeBaseSelector(args []string) (string, []string) {
	if len(args) == 0 {
		return "", args
	}
	value := strings.TrimSpace(args[0])
	if !strings.HasPrefix(value, "@") {
		return "", args
	}
	return strings.TrimPrefix(value, "@"), args[1:]
}

func formatKnowledgeBases(snapshot models.AgentSnapshot) string {
	config := snapshot.Settings.Knowledge
	status := "disabled"
	if config.Enabled {
		status = "enabled"
	}
	lines := []string{
		"Knowledge bases: " + status,
		"Default base: " + firstNonEmpty(config.DefaultBaseID, "(not configured)"),
		"Agent provider: " + firstNonEmpty(config.AgentProviderID, "(not configured)"),
	}
	for _, base := range config.Bases {
		baseStatus := "disabled"
		if base.Enabled {
			baseStatus = "enabled"
		}
		lines = append(lines, fmt.Sprintf("- @%s %s [%s] %s", base.ID, firstNonEmpty(base.Name, base.ID), baseStatus, base.BaseDir))
	}
	if len(config.Bases) == 0 {
		lines = append(lines, "- no knowledge bases configured")
	}
	return strings.Join(lines, "\n")
}

func formatKnowledgeStatus(snapshot models.AgentSnapshot, userID string, baseID string) string {
	lines := []string{formatKnowledgeBases(snapshot)}
	targetBaseID := firstNonEmpty(baseID, snapshot.Settings.Knowledge.DefaultBaseID)
	if session, ok := activeKnowledgeStatusSession(snapshot.Knowledge.Sessions, userID, targetBaseID); ok {
		lines = append(lines,
			"Active session: "+session.ID,
			"Session base: "+firstNonEmpty(session.KnowledgeBaseID, "(not configured)"),
			"Codex session: "+firstNonEmpty(session.CodexSessionID, "(not created yet)"),
			"Last status: "+firstNonEmpty(session.Status, "unknown"),
		)
	}
	return strings.Join(lines, "\n")
}

func activeKnowledgeStatusSession(sessions []models.AgentKnowledgeSession, userID string, baseID string) (models.AgentKnowledgeSession, bool) {
	for _, session := range sessions {
		if session.Active && strings.TrimSpace(session.UserID) == strings.TrimSpace(userID) && strings.TrimSpace(session.KnowledgeBaseID) == strings.TrimSpace(baseID) {
			return session, true
		}
	}
	return models.AgentKnowledgeSession{}, false
}

func knowledgeHelp() string {
	return strings.TrimSpace(`
Knowledge commands:
- /kb list
- /kb ask <question>
- /kb @<base-id> ask <question>
- /kb @<base-id> <question>
- /kb new [@base-id] [question]
- /kb status [@base-id]
`)
}

func withProjectImages(metadata map[string]any, images []models.AgentMarkdownImage) map[string]any {
	projectImages := make([]models.ProjectOutputImage, 0, len(images))
	imagePaths := make([]string, 0, len(images))
	for _, image := range images {
		if strings.TrimSpace(image.Path) == "" {
			continue
		}
		projectImages = append(projectImages, models.ProjectOutputImage{
			Path:        image.Path,
			ContentType: firstNonEmpty(image.ContentType, "image/png"),
			Filename:    pathBase(image.Path),
		})
		imagePaths = append(imagePaths, image.Path)
	}
	metadata["images"] = projectImages
	metadata["image_paths"] = imagePaths
	return metadata
}

func pathBase(path string) string {
	parts := strings.Split(strings.ReplaceAll(path, "\\", "/"), "/")
	if len(parts) == 0 {
		return "answer.png"
	}
	return firstNonEmpty(parts[len(parts)-1], "answer.png")
}
