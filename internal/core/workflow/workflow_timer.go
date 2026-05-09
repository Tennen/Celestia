package workflow

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
	_, triggered := e.runOptions.TriggeredNode[node.ID]
	summary := "Timer inactive for this run"
	if triggered {
		summary = "Scheduled timer fired"
	}
	metadata := map[string]any{
		"triggered": triggered,
		"schedule":  spec.Schedule,
		"timezone":  spec.Timezone,
	}
	switch spec.Schedule {
	case timeschedule.ScheduleDaily:
		metadata["at"] = spec.At
	case timeschedule.ScheduleInterval:
		metadata["interval_seconds"] = spec.IntervalSeconds
	}
	return workflowNodeValue{Triggered: triggered}, summary, metadata, nil
}

func normalizeWorkflowTimerConfig(config workflowTimerNodeConfig) (timeschedule.Spec, error) {
	return timeschedule.Normalize(&timeschedule.Spec{
		Schedule:        strings.TrimSpace(config.Schedule),
		At:              strings.TrimSpace(config.At),
		IntervalStart:   "00:00",
		IntervalEnd:     "00:00",
		IntervalSeconds: config.IntervalSeconds,
		Timezone:        strings.TrimSpace(config.Timezone),
	})
}
