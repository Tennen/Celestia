package runtime

import (
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

func workflowTriggerWindowsMatch(workflow models.AgentWorkflow, triggerNodeID string, now time.Time) bool {
	windows := workflowWindowsForTrigger(workflow, triggerNodeID)
	for _, window := range windows {
		matched, err := workflowTimeWindowMatches(now, window)
		if err != nil || !matched {
			return false
		}
	}
	return true
}

func workflowWindowsForTrigger(workflow models.AgentWorkflow, triggerNodeID string) []workflowTimeWindowConfig {
	nodes := make(map[string]models.AgentWorkflowNode, len(workflow.Nodes))
	for _, node := range workflow.Nodes {
		nodes[node.ID] = node
	}
	seen := map[string]struct{}{}
	windows := make([]workflowTimeWindowConfig, 0)
	addWindow := func(nodeID string) {
		if _, ok := seen[nodeID]; ok {
			return
		}
		node, ok := nodes[nodeID]
		if !ok || node.Type != workflowNodeTypeTimeWindow {
			return
		}
		config, err := decodeWorkflowNodeData[workflowTimeWindowConfig](node.Data)
		if err != nil {
			return
		}
		seen[nodeID] = struct{}{}
		windows = append(windows, config)
	}
	for _, edge := range workflow.Edges {
		if edge.Target == triggerNodeID {
			addWindow(edge.Source)
		}
		if edge.Source == triggerNodeID {
			addWindow(edge.Target)
			for _, peer := range workflow.Edges {
				if peer.Target == edge.Target {
					addWindow(peer.Source)
				}
			}
		}
	}
	return windows
}
