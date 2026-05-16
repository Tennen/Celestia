package workflow

import (
	"errors"
	"fmt"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

type deviceCommandNodeConfig struct {
	DeviceID string         `json:"device_id"`
	Action   string         `json:"action"`
	Params   map[string]any `json:"params,omitempty"`
}

type agentFunctionNodeConfig struct {
	Input       string             `json:"input"`
	SessionID   string             `json:"session_id,omitempty"`
	Touchpoints []touchpointConfig `json:"touchpoints,omitempty"`
}

type touchpointConfig struct {
	Type     string         `json:"type,omitempty"`
	ToUser   string         `json:"to_user,omitempty"`
	DeviceID string         `json:"device_id,omitempty"`
	Action   string         `json:"action,omitempty"`
	Params   map[string]any `json:"params,omitempty"`
}

func (e *workflowExecutor) executeDeviceCommandNode(node models.AgentWorkflowNode) (workflowNodeValue, string, map[string]any, error) {
	if e.service.workflowDevices == nil {
		return workflowNodeValue{}, "", nil, errors.New("workflow device runtime is not configured")
	}
	inputs, inputErr := e.collect(node.ID, "")
	if inputErr != nil {
		return workflowNodeValue{}, "", nil, inputErr
	}
	if shouldBlockExecutionNode(e.incoming[node.ID], inputs) {
		return workflowNodeValue{Blocked: true, BlockedByTimer: inputs.blockedByTimer > 0, BlockedByWindow: inputs.blockedByWindow > 0}, "Device command waiting for trigger input", map[string]any{
			"input_count":       inputs.count(),
			"blocked_by_timer":  inputs.blockedByTimer > 0,
			"blocked_by_window": inputs.blockedByWindow > 0,
		}, nil
	}
	config, err := decodeWorkflowNodeData[deviceCommandNodeConfig](node.Data)
	if err != nil {
		return workflowNodeValue{}, "", nil, err
	}
	deviceID := strings.TrimSpace(config.DeviceID)
	action := strings.TrimSpace(config.Action)
	if deviceID == "" || action == "" {
		return workflowNodeValue{}, "", nil, errors.New("device command node requires device_id and action")
	}
	inputText := strings.Join(orderedWorkflowStrings(inputs.texts), "\n\n")
	params := renderWorkflowParamsInput(config.Params, inputText)
	actor := workflowActor(e.workflow.ID, node.ID)
	if err := e.service.workflowDevices.ExecuteDeviceCommand(e.ctx, actor, deviceID, action, params); err != nil {
		return workflowNodeValue{}, "", nil, err
	}
	return workflowNodeValue{Triggered: true, Text: inputText}, "Device command accepted", map[string]any{
		"device_id":   deviceID,
		"action":      action,
		"input_count": inputs.count(),
		"input_chars": len(inputText),
	}, nil
}

func (e *workflowExecutor) executeAgentFunctionNode(node models.AgentWorkflowNode) (workflowNodeValue, string, map[string]any, error) {
	if e.service.workflowInput == nil {
		return workflowNodeValue{}, "", nil, errors.New("workflow input runtime is not configured")
	}
	inputs, inputErr := e.collect(node.ID, "")
	if inputErr != nil {
		return workflowNodeValue{}, "", nil, inputErr
	}
	if shouldBlockExecutionNode(e.incoming[node.ID], inputs) {
		return workflowNodeValue{Blocked: true, BlockedByTimer: inputs.blockedByTimer > 0, BlockedByWindow: inputs.blockedByWindow > 0}, "Agent function waiting for trigger input", map[string]any{
			"input_count":       inputs.count(),
			"blocked_by_timer":  inputs.blockedByTimer > 0,
			"blocked_by_window": inputs.blockedByWindow > 0,
		}, nil
	}
	config, err := decodeWorkflowNodeData[agentFunctionNodeConfig](node.Data)
	if err != nil {
		return workflowNodeValue{}, "", nil, err
	}
	input := strings.TrimSpace(config.Input)
	if input == "" {
		input = strings.Join(orderedWorkflowStrings(append(inputs.prompts, inputs.texts...)), "\n\n")
	}
	if input == "" {
		return workflowNodeValue{}, "", nil, errors.New("agent function node requires input or upstream text")
	}
	sessionID := strings.TrimSpace(config.SessionID)
	if sessionID == "" {
		sessionID = workflowActor(e.workflow.ID, node.ID)
	}
	result, err := e.service.workflowInput.HandleInput(e.ctx, models.ProjectInputRequest{
		SessionID: sessionID,
		Input:     input,
		Actor:     workflowActor(e.workflow.ID, node.ID),
		Source:    "workflow",
	})
	if err != nil {
		return workflowNodeValue{}, "", nil, err
	}
	message := strings.TrimSpace(result.ResponseText)
	if message == "" {
		return workflowNodeValue{}, "", nil, errors.New("agent function returned empty response")
	}
	if err := e.deliverAgentFunctionTouchpoints(config.Touchpoints, message, node.ID); err != nil {
		return workflowNodeValue{}, "", nil, err
	}
	return workflowNodeValue{Text: message, Items: append([]models.AgentWorkflowItem{}, inputs.items...)}, "Agent function completed", map[string]any{
		"chars":            len(message),
		"touchpoint_count": len(config.Touchpoints),
	}, nil
}

func (e *workflowExecutor) deliverAgentFunctionTouchpoints(touchpoints []touchpointConfig, message string, nodeID string) error {
	var failures []string
	for _, touchpoint := range touchpoints {
		if err := e.deliverAgentFunctionTouchpoint(touchpoint, message, nodeID); err != nil {
			failures = append(failures, err.Error())
		}
	}
	if len(failures) > 0 {
		return errors.New(strings.Join(failures, "; "))
	}
	return nil
}

func (e *workflowExecutor) deliverAgentFunctionTouchpoint(touchpoint touchpointConfig, message string, nodeID string) error {
	touchpoint.Type = strings.TrimSpace(touchpoint.Type)
	switch touchpoint.Type {
	case "", "none":
		return nil
	case "wecom":
		if e.service.workflowOutput == nil {
			return errors.New("wecom output runtime is not configured")
		}
		toUser := strings.TrimSpace(touchpoint.ToUser)
		if toUser == "" {
			return errors.New("wecom touchpoint requires to_user")
		}
		return e.service.workflowOutput.SendWeComText(e.ctx, toUser, message)
	case "device":
		if e.service.workflowDevices == nil {
			return errors.New("workflow device runtime is not configured")
		}
		deviceID := strings.TrimSpace(touchpoint.DeviceID)
		if deviceID == "" {
			return errors.New("device touchpoint requires device_id")
		}
		params := cloneWorkflowParams(touchpoint.Params)
		if _, ok := params["message"]; !ok {
			params["message"] = message
		}
		action := firstNonEmpty(strings.TrimSpace(touchpoint.Action), "push_voice_message")
		return e.service.workflowDevices.ExecuteDeviceCommand(e.ctx, workflowActor(e.workflow.ID, nodeID), deviceID, action, params)
	default:
		return fmt.Errorf("unsupported touchpoint type %q", touchpoint.Type)
	}
}

func shouldBlockExecutionNode(incoming []models.AgentWorkflowEdge, inputs collectedWorkflowInputs) bool {
	if len(incoming) == 0 {
		return false
	}
	if inputs.hasBlockingWindow() {
		return true
	}
	return !inputs.hasActiveContent()
}

func workflowActor(workflowID string, nodeID string) string {
	return "workflow:" + strings.TrimSpace(workflowID) + ":" + strings.TrimSpace(nodeID)
}

func renderWorkflowParamsInput(params map[string]any, input string) map[string]any {
	return renderWorkflowParamInputValue(cloneWorkflowParams(params), input).(map[string]any)
}

func renderWorkflowParamInputValue(value any, input string) any {
	switch typed := value.(type) {
	case string:
		return strings.ReplaceAll(typed, "${input}", input)
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = renderWorkflowParamInputValue(item, input)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = renderWorkflowParamInputValue(item, input)
		}
		return out
	default:
		return value
	}
}
