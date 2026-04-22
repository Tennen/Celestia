package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

func (s *Service) RunDirectCommand(ctx context.Context, input string) (string, bool, error) {
	text := strings.TrimSpace(input)
	if text == "" || !strings.HasPrefix(text, "/") {
		return "", false, nil
	}
	name, rest := splitCommand(text)
	switch name {
	case "/search":
		return s.commandSearch(ctx, rest)
	case "/topic":
		return s.commandTopic(ctx, rest)
	case "/writing":
		return s.commandWriting(ctx, rest)
	case "/market":
		return s.commandMarket(ctx, rest)
	case "/evolution":
		return s.commandEvolution(ctx, rest)
	case "/evolve", "/coding":
		return s.commandEvolutionAlias(ctx, name, rest)
	case "/agent-capability":
		return s.commandAgentCapability(ctx, rest)
	case "/apple-notes", "/notes":
		return s.commandRunAgentCapability(ctx, "apple-notes", rest)
	case "/apple-reminders", "/reminders":
		return s.commandRunAgentCapability(ctx, "apple-reminders", rest)
	case "/terminal":
		return s.commandTerminal(ctx, rest)
	case "/codex":
		return s.commandCodex(ctx, rest)
	case "/md2img":
		return s.commandMD2Img(ctx, rest)
	case "/sync":
		return s.commandTerminal(ctx, "git pull")
	case "/build":
		return s.commandTerminal(ctx, "npm install && npm run build")
	case "/restart":
		return s.commandTerminal(ctx, "pm2 restart 0")
	case "/deploy":
		return s.commandTerminal(ctx, "./deploy.sh")
	case "/celestia":
		return "Celestia device execution is handled by /api/ai/v1/commands and the admin AI command form inside this gateway.", true, nil
	default:
		return "", false, nil
	}
}

func (s *Service) commandSearch(ctx context.Context, query string) (string, bool, error) {
	if strings.TrimSpace(query) == "" {
		return "", true, errors.New("/search requires a query")
	}
	result, err := s.RunSearch(ctx, models.AgentSearchRequest{
		MaxItems: 8,
		Plans:    []models.AgentSearchPlan{{Label: "direct", Query: query, Recency: "month"}},
	})
	return marshalCommandResult(result), true, err
}

func (s *Service) commandTopic(ctx context.Context, profileID string) (string, bool, error) {
	return s.runTopicCommand(ctx, profileID)
}

func (s *Service) commandWriting(ctx context.Context, rest string) (string, bool, error) {
	action, tail := splitWord(rest)
	switch action {
	case "", "topics", "list":
		snapshot, err := s.Snapshot(ctx)
		return marshalCommandResult(snapshot.Writing.Topics), true, err
	case "show":
		snapshot, err := s.Snapshot(ctx)
		if err != nil {
			return "", true, err
		}
		topic, ok := findWritingTopic(snapshot.Writing.Topics, strings.TrimSpace(tail))
		if !ok {
			return "", true, errors.New("writing topic not found")
		}
		return marshalCommandResult(topic), true, nil
	case "topic", "create":
		topic, err := s.SaveWritingTopic(ctx, WritingTopicRequest{Title: tail})
		return marshalCommandResult(topic), true, err
	case "append":
		topicID, content := splitWord(tail)
		topic, err := s.AddWritingMaterial(ctx, topicID, WritingMaterialRequest{Content: content})
		return marshalCommandResult(topic), true, err
	case "summarize":
		topic, err := s.SummarizeWritingTopic(ctx, strings.TrimSpace(tail))
		return marshalCommandResult(topic), true, err
	case "restore":
		topic, err := s.RestoreWritingTopic(ctx, strings.TrimSpace(tail))
		return marshalCommandResult(topic), true, err
	case "set":
		topicID, rest := splitWord(tail)
		section, content := splitWord(rest)
		topic, err := s.SetWritingTopicState(ctx, topicID, WritingStateUpdateRequest{Section: section, Content: content})
		return marshalCommandResult(topic), true, err
	default:
		return "Writing commands: /writing topics, /writing show <topic_id>, /writing create <title>, /writing append <topic_id> <content>, /writing summarize <topic_id>, /writing restore <topic_id>, /writing set <topic_id> <summary|outline|draft> <content>", true, nil
	}
}

func (s *Service) commandMarket(ctx context.Context, rest string) (string, bool, error) {
	return s.runMarketCommand(ctx, rest)
}

func (s *Service) commandEvolution(ctx context.Context, rest string) (string, bool, error) {
	return s.runEvolutionCommand(ctx, rest)
}

func (s *Service) commandTerminal(ctx context.Context, command string) (string, bool, error) {
	result, err := s.RunTerminal(ctx, models.AgentTerminalRequest{Command: command})
	return marshalCommandResult(result), true, err
}

func (s *Service) commandCodex(ctx context.Context, prompt string) (string, bool, error) {
	return s.runCodexCommand(ctx, prompt)
}

func (s *Service) commandMD2Img(ctx context.Context, markdown string) (string, bool, error) {
	result, err := s.RunMarkdownRender(ctx, models.AgentMarkdownRenderRequest{Markdown: markdown})
	return marshalCommandResult(result), true, err
}

func splitCommand(input string) (string, string) {
	name, rest := splitWord(input)
	return strings.ToLower(name), rest
}

func splitWord(input string) (string, string) {
	parts := strings.Fields(strings.TrimSpace(input))
	if len(parts) == 0 {
		return "", ""
	}
	first := parts[0]
	rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(input), first))
	return first, rest
}

func marshalCommandResult(value any) string {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return strings.TrimSpace(err.Error())
	}
	return strings.TrimSpace(string(data))
}
