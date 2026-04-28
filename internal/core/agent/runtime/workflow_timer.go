package runtime

import (
	"strings"

	"github.com/chentianyu/celestia/internal/core/timeschedule"
	"github.com/chentianyu/celestia/internal/models"
)

func (e *workflowExecutor) executeTimerNode(node models.AgentWorkflowNode) (workflowNodeValue, string, map[string]any, error) {
	config, err := decodeWorkflowNodeData[workflowTimerNodeConfig](node.Data)
	if err != nil {
		return workflowNodeValue{}, "", nil, err
	}
	spec, err := normalizeWorkflowTimerConfig(config)
	if err != nil {
		return workflowNodeValue{}, "", nil, err
	}
	triggered := e.runOptions.ManualRun
	summary := "Manual run bypassed timer"
	if !e.runOptions.ManualRun {
		_, triggered = e.runOptions.TriggeredTimerNode[node.ID]
		if triggered {
			summary = "Scheduled timer fired"
		} else {
			summary = "Timer inactive for this run"
		}
	}
	return workflowNodeValue{Triggered: triggered}, summary, map[string]any{
		"triggered": triggered,
		"schedule":  spec.Schedule,
		"at":        spec.At,
		"timezone":  spec.Timezone,
	}, nil
}

func normalizeWorkflowTimerConfig(config workflowTimerNodeConfig) (timeschedule.Spec, error) {
	return timeschedule.Normalize(&timeschedule.Spec{
		Schedule: strings.TrimSpace(config.Schedule),
		At:       strings.TrimSpace(config.At),
		Timezone: strings.TrimSpace(config.Timezone),
	})
}
