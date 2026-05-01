package runtime

import (
	"errors"
	"fmt"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

func resolveCodexAgentProvider(settings models.AgentSettings, providerID string) (models.AgentProvider, error) {
	target := strings.TrimSpace(providerID)
	if target == "" {
		return models.AgentProvider{}, errors.New("agent_provider_id is required")
	}
	for _, provider := range settings.AgentProviders {
		if strings.TrimSpace(provider.ID) != target {
			continue
		}
		if !equalProviderType(provider.Type, "codex") {
			return models.AgentProvider{}, fmt.Errorf("agent provider %q is type %q, want codex", target, provider.Type)
		}
		return provider, nil
	}
	return models.AgentProvider{}, fmt.Errorf("agent provider %q is not configured", target)
}

func codexRequestFromAgentProvider(provider models.AgentProvider, timeoutMS int) models.AgentCodexRequest {
	return models.AgentCodexRequest{
		Model:           provider.Model,
		ReasoningEffort: provider.ReasoningEffort,
		TimeoutMS:       maxInt(timeoutMS, provider.TimeoutMS),
	}
}

func equalProviderType(value string, want string) bool {
	return strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(want))
}
