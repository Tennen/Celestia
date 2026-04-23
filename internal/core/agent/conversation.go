package agent

import (
	"context"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

func (s *Service) ResolveDirectInput(ctx context.Context, input string) (string, bool, error) {
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return "", false, err
	}
	normalized := strings.TrimSpace(input)
	for _, rule := range snapshot.DirectInput.Rules {
		if !rule.Enabled || strings.TrimSpace(rule.Pattern) == "" {
			continue
		}
		pattern := strings.TrimSpace(rule.Pattern)
		if rule.MatchMode == "fuzzy" && strings.Contains(strings.ToLower(normalized), strings.ToLower(pattern)) {
			return strings.TrimSpace(rule.TargetText), true, nil
		}
		if rule.MatchMode != "fuzzy" && strings.EqualFold(normalized, pattern) {
			return strings.TrimSpace(rule.TargetText), true, nil
		}
	}
	return normalized, false, nil
}

func (s *Service) Converse(ctx context.Context, req models.AgentConversationRequest) (models.AgentConversation, error) {
	if err := requireText(req.Input, "input"); err != nil {
		return models.AgentConversation{}, err
	}
	sessionID := firstNonEmpty(req.SessionID, "default")
	resolved, mapped, err := s.ResolveDirectInput(ctx, req.Input)
	if err != nil {
		return models.AgentConversation{}, err
	}
	response, metadata, err := s.runReactConversation(ctx, sessionID, resolved, firstNonEmpty(req.Actor, "agent"))
	status := "succeeded"
	metadata["direct_input_mapped"] = mapped
	if err != nil {
		response = firstNonEmpty(response, err.Error())
		status = "failed"
	}
	item, saveErr := s.RecordConversationResult(ctx, req, resolved, response, status, metadata)
	if saveErr != nil {
		return models.AgentConversation{}, saveErr
	}
	return item, err
}

func (s *Service) RecordConversationResult(
	ctx context.Context,
	req models.AgentConversationRequest,
	resolved string,
	response string,
	status string,
	metadata map[string]any,
) (models.AgentConversation, error) {
	if err := requireText(req.Input, "input"); err != nil {
		return models.AgentConversation{}, err
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	status = firstNonEmpty(status, "succeeded")
	now := time.Now().UTC()
	item := models.AgentConversation{
		ID:        uuid.NewString(),
		SessionID: firstNonEmpty(req.SessionID, "default"),
		Input:     strings.TrimSpace(req.Input),
		Resolved:  strings.TrimSpace(resolved),
		Response:  strings.TrimSpace(response),
		Status:    status,
		Metadata:  metadata,
		CreatedAt: now,
	}
	_, saveErr := s.update(ctx, func(snapshot *models.AgentSnapshot) error {
		for key, value := range appendConversationMemory(snapshot, item, now) {
			item.Metadata[key] = value
		}
		snapshot.Conversations = append([]models.AgentConversation{item}, snapshot.Conversations...)
		snapshot.Conversations = truncateList(snapshot.Conversations, 80)
		snapshot.UpdatedAt = now
		return nil
	})
	if saveErr != nil {
		return models.AgentConversation{}, saveErr
	}
	return item, nil
}

func (s *Service) runReactConversation(ctx context.Context, sessionID string, input string, actor string) (string, map[string]any, error) {
	trace := newConversationTraceLogger()
	ctx = withConversationTrace(ctx, trace)
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return "", nil, err
	}
	model, err := newEinoChatModel(snapshot.Settings)
	if err != nil {
		return "", map[string]any{"runtime_mode": "react", "actor": actor}, err
	}
	runtimes, err := s.buildAgentToolRuntimes(ctx)
	if err != nil {
		return "", map[string]any{"runtime_mode": "react", "actor": actor}, err
	}
	runtimes = traceToolRuntimes(runtimes)
	memoryPrompt, memoryMetadata := buildMemoryPrompt(snapshot, sessionID, input)
	systemPrompt := buildReactSystemPrompt(memoryPrompt, runtimes)
	metadata := map[string]any{
		"runtime_mode": "react",
		"actor":        actor,
		"tool_count":   len(runtimes),
	}
	for key, value := range memoryMetadata {
		metadata[key] = value
	}
	agent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: model,
		ToolsConfig: compose.ToolsNodeConfig{
			Tools: agentBaseTools(runtimes),
		},
		MaxStep:            12,
		ToolReturnDirectly: agentReturnDirectly(runtimes),
	})
	if err != nil {
		return "", metadata, err
	}
	msg, err := agent.Generate(ctx, []*schema.Message{
		schema.SystemMessage(systemPrompt),
		schema.UserMessage(input),
	})
	if err != nil {
		metadata["process_log"] = trace.eventsSnapshot()
		metadata["process_event_count"] = len(trace.eventsSnapshot())
		return "", metadata, err
	}
	if msg == nil {
		metadata["process_log"] = trace.eventsSnapshot()
		metadata["process_event_count"] = len(trace.eventsSnapshot())
		return "", metadata, nil
	}
	metadata["process_log"] = trace.eventsSnapshot()
	metadata["process_event_count"] = len(trace.eventsSnapshot())
	return strings.TrimSpace(msg.Content), metadata, nil
}

func buildReactSystemPrompt(memoryPrompt string, runtimes []agentToolRuntime) string {
	toolNames := make([]string, 0, len(runtimes))
	for _, runtime := range runtimes {
		toolNames = append(toolNames, runtime.spec.Name)
	}
	return strings.Join([]string{
		"You are Celestia's local Agent. Use Eino ReAct tool calling for actions, retrieval, and local integrations.",
		"The available capabilities are Celestia Agent tools.",
		"Home Assistant and ChatGPT bridge tools are not available.",
		"Use search_web for current external information. Use terminal_run, codex_runner, and Apple tools only for explicit user intent.",
		"Project touchpoints such as WeCom and HTTP are handled before this Agent loop, not as Agent tools.",
		"After tool calls, return a concise final answer in the user's language.",
		"Available tool names: " + strings.Join(toolNames, ", "),
		"",
		"[session_context]",
		memoryPrompt,
	}, "\n")
}
