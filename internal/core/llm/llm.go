package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func GenerateTextWithProvider(ctx context.Context, settings models.AgentSettings, providerID string, prompt string) (string, error) {
	provider, ok := SelectProvider(settings, providerID)
	if !ok {
		return "", errors.New("no LLM provider configured")
	}
	switch strings.ToLower(strings.TrimSpace(provider.Type)) {
	case "openai", "openai-like", "llama-server", "gpt-plugin":
		return callOpenAICompatible(ctx, provider, prompt)
	case "ollama":
		return callOllama(ctx, provider, prompt)
	case "gemini", "gemini-like":
		return callGemini(ctx, provider, prompt)
	default:
		return "", fmt.Errorf("unsupported LLM provider type %q", provider.Type)
	}
}

func SelectProvider(settings models.AgentSettings, providerID string) (models.AgentLLMProvider, bool) {
	target := strings.TrimSpace(providerID)
	if target == "" {
		target = strings.TrimSpace(settings.DefaultLLMProviderID)
	}
	for _, provider := range settings.LLMProviders {
		if target != "" && provider.ID == target {
			return provider, true
		}
	}
	for _, provider := range settings.LLMProviders {
		if strings.TrimSpace(provider.ID) != "" {
			return provider, true
		}
	}
	return models.AgentLLMProvider{}, false
}

func callOpenAICompatible(ctx context.Context, provider models.AgentLLMProvider, prompt string) (string, error) {
	baseURL := strings.TrimRight(provider.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	chatPath := firstNonEmpty(provider.ChatPath, "/v1/chat/completions")
	endpoint := baseURL + "/" + strings.TrimLeft(chatPath, "/")
	payload := map[string]any{
		"model": firstNonEmpty(provider.Model, "gpt-4.1-mini"),
		"messages": []chatMessage{
			{Role: "system", Content: "You are Celestia's local automation runtime. Be concise and operational."},
			{Role: "user", Content: prompt},
		},
	}
	var out struct {
		Choices []struct {
			Message chatMessage `json:"message"`
		} `json:"choices"`
	}
	if err := PostJSON(ctx, provider, endpoint, payload, &out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return "", errors.New("LLM response did not include a message")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

func callOllama(ctx context.Context, provider models.AgentLLMProvider, prompt string) (string, error) {
	baseURL := strings.TrimRight(firstNonEmpty(provider.BaseURL, "http://127.0.0.1:11434"), "/")
	payload := map[string]any{
		"model":  provider.Model,
		"stream": false,
		"messages": []chatMessage{
			{Role: "system", Content: "You are Celestia's local automation runtime. Be concise and operational."},
			{Role: "user", Content: prompt},
		},
	}
	var out struct {
		Message chatMessage `json:"message"`
	}
	if err := PostJSON(ctx, provider, baseURL+"/api/chat", payload, &out); err != nil {
		return "", err
	}
	if strings.TrimSpace(out.Message.Content) == "" {
		return "", errors.New("Ollama response did not include a message")
	}
	return strings.TrimSpace(out.Message.Content), nil
}

func callGemini(ctx context.Context, provider models.AgentLLMProvider, prompt string) (string, error) {
	baseURL := strings.TrimRight(firstNonEmpty(provider.BaseURL, "https://generativelanguage.googleapis.com"), "/")
	model := firstNonEmpty(provider.Model, "gemini-1.5-flash")
	endpoint := fmt.Sprintf("%s/v1beta/models/%s:generateContent", baseURL, model)
	if strings.TrimSpace(provider.APIKey) != "" {
		endpoint += "?key=" + provider.APIKey
	}
	payload := map[string]any{
		"contents": []map[string]any{{
			"parts": []map[string]any{{"text": prompt}},
		}},
	}
	var out struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := PostJSON(ctx, models.AgentLLMProvider{}, endpoint, payload, &out); err != nil {
		return "", err
	}
	if len(out.Candidates) == 0 || len(out.Candidates[0].Content.Parts) == 0 {
		return "", errors.New("Gemini response did not include content")
	}
	return strings.TrimSpace(out.Candidates[0].Content.Parts[0].Text), nil
}

func PostJSON(ctx context.Context, provider models.AgentLLMProvider, endpoint string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	timeout := time.Duration(maxInt(provider.TimeoutMS, 60000)) * time.Millisecond
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(provider.APIKey) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(provider.APIKey))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(reqCtx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("LLM request timed out after %dms: %w", timeout/time.Millisecond, err)
		}
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("LLM request failed with status %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func maxInt(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}
