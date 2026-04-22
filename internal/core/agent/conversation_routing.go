package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

type conversationRouteDecision struct {
	Decision       string         `json:"decision"`
	ResponseText   string         `json:"response_text"`
	CapabilityName string         `json:"capability_name"`
	Tool           string         `json:"tool"`
	Action         string         `json:"action"`
	Params         map[string]any `json:"params"`
}

func (s *Service) RouteConversation(ctx context.Context, input string, memoryPrompt string, snapshot models.AgentSnapshot) (string, map[string]any, bool, error) {
	mode := strings.TrimSpace(snapshot.Settings.RuntimeMode)
	if mode == "" {
		mode = "classic"
	}
	metadata := map[string]any{"runtime_mode": mode}
	raw, err := s.GenerateText(ctx, buildConversationRoutingPrompt(input, memoryPrompt, mode, snapshot))
	if err != nil {
		metadata["routing_error"] = err.Error()
		return "", metadata, false, nil
	}
	decision, err := parseConversationRouteDecision(raw)
	if err != nil {
		metadata["routing_error"] = err.Error()
		return "", metadata, false, nil
	}
	metadata["routing_decision"] = decision.Decision
	metadata["routing_tool"] = firstNonEmpty(decision.Tool, decision.CapabilityName)
	if decision.Decision == "respond" && strings.TrimSpace(decision.ResponseText) != "" {
		return strings.TrimSpace(decision.ResponseText), metadata, true, nil
	}
	command, err := routeDecisionCommand(decision)
	if err != nil {
		metadata["routing_error"] = err.Error()
		return "", metadata, false, nil
	}
	if command == "" {
		return "", metadata, false, nil
	}
	output, handled, commandErr := s.RunDirectCommand(ctx, command)
	if !handled {
		return "", metadata, false, nil
	}
	metadata["routing_command"] = command
	return output, metadata, true, commandErr
}

func buildConversationRoutingPrompt(input string, memoryPrompt string, mode string, snapshot models.AgentSnapshot) string {
	return strings.Join([]string{
		"Return JSON only. Route the current user turn for Celestia's local agent runtime.",
		"Allowed JSON shapes:",
		`{"decision":"respond","response_text":"short direct answer"}`,
		`{"decision":"tool_call","tool":"search","params":{"query":"..."}}`,
		`{"decision":"tool_call","tool":"topic-summary","params":{"profile_id":"optional"}}`,
		`{"decision":"tool_call","tool":"writing-organizer","action":"topics|show|create|append|summarize|restore|set","params":{}}`,
		`{"decision":"tool_call","tool":"market-analysis","params":{"phase":"midday|close"}}`,
		`{"decision":"tool_call","tool":"evolution-operator","action":"queue|run","params":{}}`,
		`{"decision":"tool_call","tool":"apple-notes","params":{"input":"memo notes args"}}`,
		`{"decision":"tool_call","tool":"apple-reminders","params":{"input":"remindctl args"}}`,
		`{"decision":"tool_call","tool":"agent-capability","action":"list|describe|run","params":{}}`,
		`{"decision":"tool_call","tool":"terminal","params":{"command":"..."}}`,
		`{"decision":"tool_call","tool":"codex","params":{"prompt":"..."}}`,
		`{"decision":"tool_call","tool":"md2img","params":{"markdown":"...","mode":"long-image|multi-page"}}`,
		"Use terminal only for explicit operator requests. Use respond for normal chat or uncertain routing.",
		"Home Assistant and ChatGPT bridge are not available.",
		"CONTEXT_JSON:",
		mustJSON(map[string]any{
			"mode":             mode,
			"terminal_enabled": snapshot.Settings.Terminal.Enabled,
			"market_holdings":  len(snapshot.Market.Portfolio.Funds),
			"writing_topics":   len(snapshot.Writing.Topics),
			"memory":           memoryPrompt,
		}),
		"USER_INPUT:",
		input,
	}, "\n")
}

func parseConversationRouteDecision(raw string) (conversationRouteDecision, error) {
	var decision conversationRouteDecision
	text := extractRouteJSONObject(raw)
	if text == "" {
		return decision, errors.New("routing response did not contain JSON")
	}
	if err := json.Unmarshal([]byte(text), &decision); err != nil {
		return decision, err
	}
	decision.Decision = strings.TrimSpace(decision.Decision)
	if decision.Decision == "" {
		return decision, errors.New("routing decision is required")
	}
	if decision.Decision != "respond" && decision.Decision != "tool_call" && decision.Decision != "use_planning" {
		return decision, errors.New("unsupported routing decision")
	}
	return decision, nil
}

func routeDecisionCommand(decision conversationRouteDecision) (string, error) {
	tool := normalizeRouteTool(firstNonEmpty(decision.Tool, decision.CapabilityName))
	action := strings.ToLower(strings.TrimSpace(decision.Action))
	params := decision.Params
	switch tool {
	case "search":
		return "/search " + stringParam(params, "query", "input"), nil
	case "topic-summary":
		return "/topic " + stringParam(params, "profile_id", "profileId"), nil
	case "writing-organizer":
		return writingRouteCommand(action, params)
	case "market-analysis":
		return "/market " + firstNonEmpty(stringParam(params, "phase"), "close"), nil
	case "evolution-operator":
		if action == "run" {
			return "/evolution run " + stringParam(params, "goal_id", "goalId", "id"), nil
		}
		return "/evolution queue " + stringParam(params, "goal", "input"), nil
	case "apple-notes":
		return "/apple-notes " + stringParam(params, "input", "args"), nil
	case "apple-reminders":
		return "/apple-reminders " + stringParam(params, "input", "args"), nil
	case "agent-capability":
		return capabilityRouteCommand(action, params), nil
	case "terminal":
		command := stringParam(params, "command", "input")
		if command == "" {
			return "", errors.New("terminal command is required")
		}
		return "/terminal " + command, nil
	case "codex":
		return "/codex " + stringParam(params, "prompt", "input"), nil
	case "md2img":
		markdown := stringParam(params, "markdown", "input")
		if markdown == "" {
			return "", errors.New("md2img markdown is required")
		}
		return "/md2img " + markdown, nil
	case "celestia":
		return "/celestia", nil
	default:
		return "", nil
	}
}

func writingRouteCommand(action string, params map[string]any) (string, error) {
	switch action {
	case "", "topics", "list":
		return "/writing topics", nil
	case "show":
		return "/writing show " + stringParam(params, "topic_id", "topicId", "id"), nil
	case "create":
		return "/writing create " + stringParam(params, "title", "input"), nil
	case "append":
		return strings.TrimSpace("/writing append " + stringParam(params, "topic_id", "topicId", "id") + " " + stringParam(params, "content", "input")), nil
	case "summarize":
		return "/writing summarize " + stringParam(params, "topic_id", "topicId", "id"), nil
	case "restore":
		return "/writing restore " + stringParam(params, "topic_id", "topicId", "id"), nil
	case "set":
		return strings.TrimSpace("/writing set " + stringParam(params, "topic_id", "topicId", "id") + " " + stringParam(params, "section") + " " + stringParam(params, "content", "input")), nil
	default:
		return "", nil
	}
}

func capabilityRouteCommand(action string, params map[string]any) string {
	name := stringParam(params, "name", "capability", "capability_name")
	input := stringParam(params, "input", "args")
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "describe", "get", "show":
		return "/agent-capability describe " + name
	case "run", "exec":
		return strings.TrimSpace("/agent-capability run " + name + " " + input)
	default:
		return "/agent-capability list"
	}
}

func normalizeRouteTool(tool string) string {
	value := strings.ToLower(strings.TrimSpace(tool))
	value = strings.TrimPrefix(value, "agent.")
	switch value {
	case "topic", "topic-summary":
		return "topic-summary"
	case "writing", "writing-organizer":
		return "writing-organizer"
	case "market", "market-analysis":
		return "market-analysis"
	case "evolution", "evolution-operator":
		return "evolution-operator"
	case "notes", "apple-notes":
		return "apple-notes"
	case "reminders", "apple-reminders":
		return "apple-reminders"
	case "capability", "agent-capability":
		return "agent-capability"
	}
	return value
}

func stringParam(params map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := stringFrom(params[key]); strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func extractRouteJSONObject(raw string) string {
	text := strings.TrimSpace(raw)
	if strings.HasPrefix(text, "```") {
		text = strings.TrimSpace(strings.TrimPrefix(text, "```json"))
		text = strings.TrimSpace(strings.TrimPrefix(text, "```"))
		text = strings.TrimSpace(strings.TrimSuffix(text, "```"))
	}
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return ""
	}
	return text[start : end+1]
}

func mustJSON(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(raw)
}
