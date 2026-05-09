package workflow

import (
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

const (
	NodeTypeGroup              = workflowNodeTypeGroup
	NodeTypeTimer              = workflowNodeTypeTimer
	NodeTypeDeviceStateChanged = workflowNodeTypeDeviceStateChanged
	NodeTypeDeviceStateIs      = workflowNodeTypeDeviceStateIs
	NodeTypeTimeWindow         = workflowNodeTypeTimeWindow
	NodeTypeRSSSources         = workflowNodeTypeRSSSources
	NodeTypeText               = workflowNodeTypeText
	NodeTypeLLM                = workflowNodeTypeLLM
	NodeTypeSearchProvider     = workflowNodeTypeSearchProvider
	NodeTypeWeComOutput        = workflowNodeTypeWeComOutput
	NodeTypeDeviceCommand      = workflowNodeTypeDeviceCommand
	NodeTypeAgentFunction      = workflowNodeTypeAgentFunction
)

func Select(snapshot models.AgentWorkflowSnapshot, workflowID string) (models.AgentWorkflow, bool) {
	return selectWorkflow(snapshot, workflowID)
}

func PruneSourceStates(states []models.AgentWorkflowSourceState, workflows []models.AgentWorkflow) []models.AgentWorkflowSourceState {
	return pruneWorkflowSourceStates(states, workflows)
}

func PruneTimerStates(states []models.AgentWorkflowTimerState, workflows []models.AgentWorkflow) []models.AgentWorkflowTimerState {
	return pruneWorkflowTimerStates(states, workflows)
}

func DueStateTriggerNodes(snapshot models.AgentWorkflowSnapshot, event models.Event, now time.Time) map[string][]string {
	return dueWorkflowStateTriggerNodes(snapshot, event, now)
}

func SchedulerTimeout(settings models.AgentSettings) time.Duration {
	return workflowSchedulerTimeout(settings)
}

func CloneParams(src map[string]any) map[string]any {
	return cloneWorkflowParams(src)
}
