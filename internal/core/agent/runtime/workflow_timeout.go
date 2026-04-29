package runtime

import (
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

const (
	workflowSchedulerMinimumTimeout  = 15 * time.Minute
	workflowSchedulerTimeoutOverhead = time.Minute
)

func workflowSchedulerTimeout(settings models.AgentSettings) time.Duration {
	maxProviderTimeoutMS := 0
	for _, provider := range settings.LLMProviders {
		maxProviderTimeoutMS = maxInt(maxProviderTimeoutMS, provider.TimeoutMS)
	}
	base := time.Duration(maxInt(maxProviderTimeoutMS, int(workflowSchedulerMinimumTimeout/time.Millisecond))) * time.Millisecond
	return base + workflowSchedulerTimeoutOverhead
}
