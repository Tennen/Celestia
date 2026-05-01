package runtime

import (
	"errors"
	"fmt"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

func resolveCodexProvider(settings models.AgentSettings, providerID string) (models.AgentLLMProvider, error) {
	target := strings.TrimSpace(providerID)
	if target == "" {
		return models.AgentLLMProvider{}, errors.New("codex_provider_id is required")
	}
	for _, provider := range settings.LLMProviders {
		if strings.TrimSpace(provider.ID) != target {
			continue
		}
		if !equalProviderType(provider.Type, "codex") {
			return models.AgentLLMProvider{}, fmt.Errorf("LLM provider %q is type %q, want codex", target, provider.Type)
		}
		return provider, nil
	}
	return models.AgentLLMProvider{}, fmt.Errorf("codex provider %q is not configured", target)
}

func codexRequestFromProvider(provider models.AgentLLMProvider, timeoutMS int) models.AgentCodexRequest {
	return models.AgentCodexRequest{
		Model:           provider.Model,
		ReasoningEffort: provider.ChatPath,
		TimeoutMS:       maxInt(timeoutMS, provider.TimeoutMS),
	}
}

func equalProviderType(value string, want string) bool {
	return strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(want))
}
