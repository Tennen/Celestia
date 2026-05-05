package runtime

import "github.com/chentianyu/celestia/internal/models"

func buildWorkflowTriggerScopes(nodes map[string]models.AgentWorkflowNode, outgoing map[string][]models.AgentWorkflowEdge) map[string]map[string]struct{} {
	scopes := map[string]map[string]struct{}{}
	for nodeID, node := range nodes {
		if !workflowNodeIsAutonomousTrigger(node.Type) {
			continue
		}
		queue := []string{nodeID}
		seen := map[string]struct{}{nodeID: {}}
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			if scopes[current] == nil {
				scopes[current] = map[string]struct{}{}
			}
			scopes[current][nodeID] = struct{}{}
			for _, edge := range outgoing[current] {
				if _, ok := seen[edge.Target]; ok {
					continue
				}
				seen[edge.Target] = struct{}{}
				queue = append(queue, edge.Target)
			}
		}
	}
	return scopes
}

func (e *workflowExecutor) hasTriggeredNodes() bool {
	return len(e.runOptions.TriggeredNode) > 0
}

func (e *workflowExecutor) nodeHasTriggerScope(nodeID string) bool {
	return len(e.triggerScopes[nodeID]) > 0
}

func (e *workflowExecutor) nodeMatchesTriggeredNode(nodeID string) bool {
	scope := e.triggerScopes[nodeID]
	if len(scope) == 0 {
		return false
	}
	for triggerID := range e.runOptions.TriggeredNode {
		if _, ok := scope[triggerID]; ok {
			return true
		}
	}
	return false
}

func (e *workflowExecutor) shouldEvaluateNodeInTriggeredRun(nodeID string) bool {
	if !e.hasTriggeredNodes() {
		return true
	}
	return e.nodeMatchesTriggeredNode(nodeID)
}

func (e *workflowExecutor) shouldCollectEdgeInTriggeredRun(targetID string, edge models.AgentWorkflowEdge) bool {
	if !e.hasTriggeredNodes() {
		return true
	}
	sourceID := edge.Source
	if e.nodeMatchesTriggeredNode(sourceID) {
		return true
	}
	if e.nodeHasTriggerScope(sourceID) {
		return false
	}
	node, ok := e.nodes[sourceID]
	if !ok {
		return false
	}
	return workflowNodeSupportsTriggeredRun(node.Type)
}

func workflowNodeSupportsTriggeredRun(nodeType string) bool {
	switch nodeType {
	case workflowNodeTypeText, workflowNodeTypeGroup, workflowNodeTypeTimeWindow:
		return true
	default:
		return false
	}
}
