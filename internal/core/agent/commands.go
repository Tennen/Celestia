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
	case "/terminal":
		return s.commandTerminal(ctx, rest)
	case "/codex":
		return s.commandCodex(ctx, rest)
	case "/sync":
		return s.commandTerminal(ctx, "git pull")
	case "/build":
		return s.commandTerminal(ctx, "npm install && npm run build")
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
	run, err := s.RunTopicSummary(ctx, strings.TrimSpace(profileID))
	return marshalCommandResult(run), true, err
}

func (s *Service) commandWriting(ctx context.Context, rest string) (string, bool, error) {
	action, tail := splitWord(rest)
	switch action {
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
	default:
		return "Writing commands: /writing create <title>, /writing append <topic_id> <content>, /writing summarize <topic_id>", true, nil
	}
}

func (s *Service) commandMarket(ctx context.Context, rest string) (string, bool, error) {
	phase := firstNonEmpty(rest, "close")
	run, err := s.RunMarketAnalysis(ctx, MarketRunRequest{Phase: phase})
	return marshalCommandResult(run), true, err
}

func (s *Service) commandEvolution(ctx context.Context, rest string) (string, bool, error) {
	action, tail := splitWord(rest)
	switch action {
	case "queue":
		goal, err := s.CreateEvolutionGoal(ctx, EvolutionGoalRequest{Goal: tail})
		return marshalCommandResult(goal), true, err
	case "run":
		goal, err := s.RunEvolutionGoal(ctx, strings.TrimSpace(tail))
		return marshalCommandResult(goal), true, err
	default:
		return "Evolution commands: /evolution queue <goal>, /evolution run <goal_id>", true, nil
	}
}

func (s *Service) commandTerminal(ctx context.Context, command string) (string, bool, error) {
	result, err := s.RunTerminal(ctx, models.AgentTerminalRequest{Command: command})
	return marshalCommandResult(result), true, err
}

func (s *Service) commandCodex(ctx context.Context, prompt string) (string, bool, error) {
	result, err := s.RunCodex(ctx, models.AgentCodexRequest{Prompt: prompt})
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
