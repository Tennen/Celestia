package runtime

import (
	"testing"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

func TestWorkflowSchedulerTimeoutUsesConfiguredProviderTimeout(t *testing.T) {
	settings := models.AgentSettings{
		LLMProviders: []models.AgentLLMProvider{{
			ID:        "slow-llm",
			TimeoutMS: 20 * 60 * 1000,
		}},
	}
	got := workflowSchedulerTimeout(settings)
	want := 21 * time.Minute
	if got != want {
		t.Fatalf("workflowSchedulerTimeout() = %s, want %s", got, want)
	}
}

func TestWorkflowSchedulerTimeoutUsesMinimumFloor(t *testing.T) {
	got := workflowSchedulerTimeout(models.AgentSettings{})
	want := 16 * time.Minute
	if got != want {
		t.Fatalf("workflowSchedulerTimeout() = %s, want %s", got, want)
	}
}
