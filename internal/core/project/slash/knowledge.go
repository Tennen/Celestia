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
	action := strings.ToLower(strings.TrimSpace(args[0]))
	switch action {
	case "status":
		snapshot, err := s.agent.Snapshot(ctx)
		if err != nil {
			return "", map[string]any{"domain": "knowledge", "action": "status"}, err
		}
		return formatKnowledgeStatus(snapshot, knowledgeUserID(req)), map[string]any{"domain": "knowledge", "action": "status"}, nil
	case "new":
		question := strings.TrimSpace(strings.Join(args[1:], " "))
		if question == "" {
			session, err := s.agent.StartKnowledgeSession(ctx, models.AgentKnowledgeRequest{
				UserID: knowledgeUserID(req),
				Source: req.Source,
			})
			if err != nil {
				return "", map[string]any{"domain": "knowledge", "action": "new"}, err
			}
			return fmt.Sprintf("Knowledge Codex conversation started: %s", session.ID), map[string]any{
				"domain":     "knowledge",
				"action":     "new",
				"session_id": session.ID,
			}, nil
		}
		return s.runKnowledgeQuestion(ctx, req, question, true)
	case "ask":
		question := strings.TrimSpace(strings.Join(args[1:], " "))
		if question == "" {
			return "", map[string]any{"domain": "knowledge", "action": "ask"}, errors.New("usage: /kb ask <question>")
		}
		return s.runKnowledgeQuestion(ctx, req, question, false)
	default:
		return s.runKnowledgeQuestion(ctx, req, strings.Join(args, " "), false)
	}
}

func (s *Service) runKnowledgeQuestion(ctx context.Context, req models.ProjectInputRequest, question string, newSession bool) (string, map[string]any, error) {
	result, err := s.agent.RunKnowledge(ctx, models.AgentKnowledgeRequest{
		Question:   question,
		UserID:     knowledgeUserID(req),
		Source:     req.Source,
		NewSession: newSession,
	})
	metadata := map[string]any{
		"domain":            "knowledge",
		"action":            "ask",
		"reply_kind":        "image",
		"new_session":       newSession,
		"session_id":        result.Session.ID,
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

func formatKnowledgeStatus(snapshot models.AgentSnapshot, userID string) string {
	config := snapshot.Settings.Knowledge
	status := "disabled"
	if config.Enabled {
		status = "enabled"
	}
	lines := []string{
		"Knowledge base: " + status,
		"Base dir: " + firstNonEmpty(config.BaseDir, "(not configured)"),
		"Agent provider: " + firstNonEmpty(config.AgentProviderID, "(not configured)"),
	}
	if session, ok := activeKnowledgeStatusSession(snapshot.Knowledge.Sessions, userID); ok {
		lines = append(lines,
			"Active session: "+session.ID,
			"Codex session: "+firstNonEmpty(session.CodexSessionID, "(not created yet)"),
			"Last status: "+firstNonEmpty(session.Status, "unknown"),
		)
	}
	return strings.Join(lines, "\n")
}

func activeKnowledgeStatusSession(sessions []models.AgentKnowledgeSession, userID string) (models.AgentKnowledgeSession, bool) {
	for _, session := range sessions {
		if session.Active && strings.TrimSpace(session.UserID) == strings.TrimSpace(userID) {
			return session, true
		}
	}
	return models.AgentKnowledgeSession{}, false
}

func knowledgeHelp() string {
	return strings.TrimSpace(`
Knowledge commands:
- /kb ask <question>
- /kb <question>
- /kb new [question]
- /kb status
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
