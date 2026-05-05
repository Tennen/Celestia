package runtime

import (
	"errors"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

func (e *workflowExecutor) executeWeComOutputNode(node models.AgentWorkflowNode) (workflowNodeValue, string, map[string]any, error) {
	config, err := decodeWorkflowNodeData[wecomOutputConfig](node.Data)
	if err != nil {
		return workflowNodeValue{}, "", nil, err
	}
	if e.service.workflowOutput == nil {
		return workflowNodeValue{}, "", nil, errors.New("wecom output runtime is not configured")
	}
	inputs, inputErr := e.collect(node.ID, "")
	if inputErr != nil {
		return workflowNodeValue{}, "", nil, inputErr
	}
	if inputs.hasBlockingWindow() {
		return workflowNodeValue{Blocked: true, BlockedByWindow: true}, "WeCom outside time window", map[string]any{
			"blocked_by_window": true,
			"input_count":       inputs.count(),
		}, nil
	}
	text := strings.Join(orderedWorkflowStrings(inputs.texts), "\n\n")
	if strings.TrimSpace(text) == "" {
		if inputs.onlyBlockedByTimer() {
			return workflowNodeValue{Blocked: true, BlockedByTimer: true}, "WeCom waiting for timer upstream", map[string]any{
				"blocked_by_timer": true,
				"input_count":      inputs.count(),
			}, nil
		}
		if inputs.onlyBlocked() {
			return workflowNodeValue{Blocked: true}, "WeCom waiting for upstream text", map[string]any{
				"blocked_by_upstream": true,
				"input_count":         inputs.count(),
			}, nil
		}
		return workflowNodeValue{}, "", nil, errors.New("wecom output node requires text input")
	}
	toUser := strings.TrimSpace(config.ToUser)
	if toUser == "" {
		return workflowNodeValue{}, "", nil, errors.New("wecom output node requires to_user")
	}
	if sendErr := e.service.workflowOutput.SendWeComText(e.ctx, toUser, text); sendErr != nil {
		e.deliveryErrors = append(e.deliveryErrors, models.AgentRunError{Target: toUser, Error: sendErr.Error()})
		return workflowNodeValue{}, "", nil, sendErr
	}
	for _, item := range inputs.items {
		key := normalizeWorkflowURL(item.URL)
		if key == "" {
			continue
		}
		e.sentItems[key] = item
	}
	return workflowNodeValue{Text: text, Items: append([]models.AgentWorkflowItem{}, inputs.items...)}, "Delivered to WeCom", map[string]any{
		"to_user":    toUser,
		"text_chars": len(text),
		"item_count": len(inputs.items),
	}, nil
}
