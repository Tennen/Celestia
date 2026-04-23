package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

type einoChatModel struct {
	provider models.AgentLLMProvider
	tools    []*schema.ToolInfo
}

func newEinoChatModel(settings models.AgentSettings) (*einoChatModel, error) {
	provider, ok := selectProvider(settings)
	if !ok {
		return nil, errors.New("no LLM provider configured")
	}
	if !supportsToolCalling(provider.Type) {
		return nil, fmt.Errorf("LLM provider type %q cannot run the Eino ReAct agent; use an OpenAI-compatible or Ollama provider with tool calling", provider.Type)
	}
	return &einoChatModel{provider: provider}, nil
}

func supportsToolCalling(providerType string) bool {
	switch strings.ToLower(strings.TrimSpace(providerType)) {
	case "openai", "openai-like", "llama-server", "gpt-plugin", "ollama":
		return true
	default:
		return false
	}
}

func (m *einoChatModel) WithTools(tools []*schema.ToolInfo) (einomodel.ToolCallingChatModel, error) {
	next := *m
	next.tools = append([]*schema.ToolInfo{}, tools...)
	return &next, nil
}

func (m *einoChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...einomodel.Option) (*schema.Message, error) {
	started := time.Now().UTC()
	common := einomodel.GetCommonOptions(&einomodel.Options{Tools: m.tools}, opts...)
	var msg *schema.Message
	var err error
	switch strings.ToLower(strings.TrimSpace(m.provider.Type)) {
	case "openai", "openai-like", "llama-server", "gpt-plugin":
		msg, err = m.generateOpenAICompatible(ctx, input, common)
	case "ollama":
		msg, err = m.generateOllama(ctx, input, common)
	default:
		err = fmt.Errorf("unsupported Eino chat model provider type %q", m.provider.Type)
	}
	m.recordModelTrace(ctx, started, input, common, msg, err)
	return msg, err
}

func (m *einoChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...einomodel.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, err := m.Generate(ctx, input, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

func (m *einoChatModel) recordModelTrace(ctx context.Context, started time.Time, input []*schema.Message, opts *einomodel.Options, msg *schema.Message, err error) {
	logger := conversationTraceFromContext(ctx)
	if logger == nil {
		return
	}
	finished := time.Now().UTC()
	toolNames := make([]string, 0)
	for _, call := range outputToolCalls(msg) {
		toolNames = append(toolNames, call.Function.Name)
	}
	logger.record(conversationTraceEvent{
		Kind:       "model_call",
		Name:       firstNonEmpty(m.provider.Name, m.provider.ID, m.provider.Type),
		Status:     statusFromError(err),
		StartedAt:  started,
		FinishedAt: finished,
		DurationMS: finished.Sub(started).Milliseconds(),
		Error:      errorString(err),
		Metadata: map[string]any{
			"provider_id":     m.provider.ID,
			"provider_type":   m.provider.Type,
			"model":           firstNonEmpty(optionModel(opts), m.provider.Model),
			"message_count":   len(input),
			"available_tools": len(opts.Tools),
			"tool_calls":      toolNames,
			"content_chars":   messageContentChars(msg),
		},
	})
}

type llmToolCall struct {
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"`
	} `json:"function"`
}

type llmChatMessage struct {
	Role       string        `json:"role"`
	Content    string        `json:"content,omitempty"`
	ToolCalls  []llmToolCall `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
	ToolName   string        `json:"name,omitempty"`
}

func (m *einoChatModel) generateOpenAICompatible(ctx context.Context, input []*schema.Message, opts *einomodel.Options) (*schema.Message, error) {
	baseURL := strings.TrimRight(m.provider.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	chatPath := firstNonEmpty(m.provider.ChatPath, "/v1/chat/completions")
	payload := map[string]any{
		"model":    firstNonEmpty(optionModel(opts), m.provider.Model, "gpt-4.1-mini"),
		"messages": toLLMChatMessages(input),
	}
	applyChatOptions(payload, opts)
	if err := attachTools(payload, opts.Tools); err != nil {
		return nil, err
	}
	var out struct {
		Choices []struct {
			Message llmChatMessage `json:"message"`
		} `json:"choices"`
	}
	endpoint := baseURL + "/" + strings.TrimLeft(chatPath, "/")
	if err := postJSON(ctx, m.provider, endpoint, payload, &out); err != nil {
		return nil, err
	}
	if len(out.Choices) == 0 {
		return nil, errors.New("LLM response did not include a choice")
	}
	return fromLLMChatMessage(out.Choices[0].Message), nil
}

func (m *einoChatModel) generateOllama(ctx context.Context, input []*schema.Message, opts *einomodel.Options) (*schema.Message, error) {
	baseURL := strings.TrimRight(firstNonEmpty(m.provider.BaseURL, "http://127.0.0.1:11434"), "/")
	payload := map[string]any{
		"model":    firstNonEmpty(optionModel(opts), m.provider.Model),
		"stream":   false,
		"messages": toLLMChatMessages(input),
	}
	applyChatOptions(payload, opts)
	if err := attachTools(payload, opts.Tools); err != nil {
		return nil, err
	}
	var out struct {
		Message ollamaChatMessage `json:"message"`
	}
	if err := postJSON(ctx, m.provider, baseURL+"/api/chat", payload, &out); err != nil {
		return nil, err
	}
	return fromOllamaChatMessage(out.Message), nil
}

func toLLMChatMessages(input []*schema.Message) []llmChatMessage {
	out := make([]llmChatMessage, 0, len(input))
	for _, msg := range input {
		if msg == nil {
			continue
		}
		next := llmChatMessage{
			Role:       string(msg.Role),
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
			ToolName:   msg.ToolName,
		}
		for _, call := range msg.ToolCalls {
			next.ToolCalls = append(next.ToolCalls, llmToolCall{
				ID:   call.ID,
				Type: firstNonEmpty(call.Type, "function"),
				Function: struct {
					Name      string `json:"name,omitempty"`
					Arguments string `json:"arguments,omitempty"`
				}{
					Name:      call.Function.Name,
					Arguments: firstNonEmpty(call.Function.Arguments, "{}"),
				},
			})
		}
		out = append(out, next)
	}
	return out
}

func fromLLMChatMessage(msg llmChatMessage) *schema.Message {
	return schema.AssistantMessage(msg.Content, toSchemaToolCalls(msg.ToolCalls))
}

func outputToolCalls(msg *schema.Message) []schema.ToolCall {
	if msg == nil {
		return nil
	}
	return msg.ToolCalls
}

func messageContentChars(msg *schema.Message) int {
	if msg == nil {
		return 0
	}
	return len([]rune(msg.Content))
}

func toSchemaToolCalls(calls []llmToolCall) []schema.ToolCall {
	out := make([]schema.ToolCall, 0, len(calls))
	for _, call := range calls {
		if strings.TrimSpace(call.Function.Name) == "" {
			continue
		}
		out = append(out, schema.ToolCall{
			ID:   firstNonEmpty(call.ID, "call_"+uuid.NewString()),
			Type: firstNonEmpty(call.Type, "function"),
			Function: schema.FunctionCall{
				Name:      strings.TrimSpace(call.Function.Name),
				Arguments: firstNonEmpty(call.Function.Arguments, "{}"),
			},
		})
	}
	return out
}

func applyChatOptions(payload map[string]any, opts *einomodel.Options) {
	if opts == nil {
		return
	}
	if opts.Temperature != nil {
		payload["temperature"] = *opts.Temperature
	}
	if opts.MaxTokens != nil {
		payload["max_tokens"] = *opts.MaxTokens
	}
	if opts.TopP != nil {
		payload["top_p"] = *opts.TopP
	}
	if len(opts.Stop) > 0 {
		payload["stop"] = opts.Stop
	}
	if opts.ToolChoice != nil {
		payload["tool_choice"] = toolChoiceValue(*opts.ToolChoice)
	}
}

func attachTools(payload map[string]any, tools []*schema.ToolInfo) error {
	if len(tools) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(tools))
	for _, item := range tools {
		if item == nil {
			continue
		}
		params, err := toolJSONSchema(item)
		if err != nil {
			return err
		}
		out = append(out, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        item.Name,
				"description": item.Desc,
				"parameters":  params,
			},
		})
	}
	payload["tools"] = out
	if _, ok := payload["tool_choice"]; !ok {
		payload["tool_choice"] = "auto"
	}
	return nil
}

func toolJSONSchema(item *schema.ToolInfo) (map[string]any, error) {
	if item.ParamsOneOf == nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}, nil
	}
	params, err := item.ParamsOneOf.ToJSONSchema()
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out["type"] == nil {
		out["type"] = "object"
	}
	return out, nil
}

func optionModel(opts *einomodel.Options) string {
	if opts != nil && opts.Model != nil {
		return strings.TrimSpace(*opts.Model)
	}
	return ""
}

func toolChoiceValue(choice schema.ToolChoice) string {
	switch choice {
	case schema.ToolChoiceForbidden:
		return "none"
	case schema.ToolChoiceForced:
		return "required"
	default:
		return "auto"
	}
}

type ollamaChatMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
}

type ollamaToolCall struct {
	Function struct {
		Name      string          `json:"name,omitempty"`
		Arguments json.RawMessage `json:"arguments,omitempty"`
	} `json:"function"`
}

func fromOllamaChatMessage(msg ollamaChatMessage) *schema.Message {
	calls := make([]schema.ToolCall, 0, len(msg.ToolCalls))
	for _, call := range msg.ToolCalls {
		if strings.TrimSpace(call.Function.Name) == "" {
			continue
		}
		calls = append(calls, schema.ToolCall{
			ID:   "call_" + uuid.NewString(),
			Type: "function",
			Function: schema.FunctionCall{
				Name:      strings.TrimSpace(call.Function.Name),
				Arguments: normalizeOllamaArguments(call.Function.Arguments),
			},
		})
	}
	return schema.AssistantMessage(msg.Content, calls)
}

func normalizeOllamaArguments(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return "{}"
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return firstNonEmpty(text, "{}")
	}
	return trimmed
}
