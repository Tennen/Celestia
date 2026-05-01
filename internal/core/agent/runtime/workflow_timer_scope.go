package runtime

import "github.com/chentianyu/celestia/internal/models"

func buildWorkflowTimerScopes(nodes map[string]models.AgentWorkflowNode, outgoing map[string][]models.AgentWorkflowEdge) map[string]map[string]struct{} {
	scopes := map[string]map[string]struct{}{}
	for nodeID, node := range nodes {
		if node.Type != workflowNodeTypeTimer {
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

func (e *workflowExecutor) hasTriggeredTimers() bool {
	return len(e.runOptions.TriggeredTimerNode) > 0
}

func (e *workflowExecutor) nodeHasTimerScope(nodeID string) bool {
	return len(e.timerScopes[nodeID]) > 0
}

func (e *workflowExecutor) nodeMatchesTriggeredTimer(nodeID string) bool {
	scope := e.timerScopes[nodeID]
	if len(scope) == 0 {
		return false
	}
	for timerID := range e.runOptions.TriggeredTimerNode {
		if _, ok := scope[timerID]; ok {
			return true
		}
	}
	return false
}

func (e *workflowExecutor) shouldEvaluateNodeInTriggeredRun(nodeID string) bool {
	if !e.hasTriggeredTimers() {
		return true
	}
	return e.nodeMatchesTriggeredTimer(nodeID)
}

func (e *workflowExecutor) shouldCollectEdgeInTriggeredRun(targetID string, edge models.AgentWorkflowEdge) bool {
	if !e.hasTriggeredTimers() {
		return true
	}
	sourceID := edge.Source
	if e.nodeMatchesTriggeredTimer(sourceID) {
		return true
	}
	if e.nodeHasTimerScope(sourceID) {
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
	case workflowNodeTypeText, workflowNodeTypeGroup:
		return true
	default:
		return false
	}
}
