package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/core/timeschedule"
	"github.com/chentianyu/celestia/internal/models"
)

const (
	workflowMatchAny       = "any"
	workflowMatchEquals    = "equals"
	workflowMatchNotEquals = "not_equals"
	workflowMatchIn        = "in"
	workflowMatchNotIn     = "not_in"
	workflowMatchExists    = "exists"
	workflowMatchMissing   = "missing"
)

type workflowStateMatchConfig struct {
	Operator string `json:"operator"`
	Value    any    `json:"value,omitempty"`
}

type workflowDeviceStateChangedConfig struct {
	DeviceID string                    `json:"device_id"`
	StateKey string                    `json:"state_key"`
	From     *workflowStateMatchConfig `json:"from,omitempty"`
	To       *workflowStateMatchConfig `json:"to,omitempty"`
}

type workflowDeviceStateIsConfig struct {
	DeviceID string                    `json:"device_id"`
	StateKey string                    `json:"state_key"`
	Match    *workflowStateMatchConfig `json:"match,omitempty"`
}

type workflowTimeWindowConfig struct {
	Start    string `json:"start"`
	End      string `json:"end"`
	Timezone string `json:"timezone,omitempty"`
}

func (e *workflowExecutor) executeDeviceStateChangedNode(node models.AgentWorkflowNode) (workflowNodeValue, string, map[string]any, error) {
	config, err := decodeWorkflowNodeData[workflowDeviceStateChangedConfig](node.Data)
	if err != nil {
		return workflowNodeValue{}, "", nil, err
	}
	if _, triggered := e.runOptions.TriggeredNode[node.ID]; !triggered {
		return workflowNodeValue{Blocked: true}, "Device state change trigger inactive for this run", map[string]any{"triggered": false}, nil
	}
	if !matchesWorkflowStateChangedNode(config, e.runOptions.SourceEvent) {
		return workflowNodeValue{Blocked: true}, "Device state change event no longer matches", map[string]any{"triggered": false}, nil
	}
	text := workflowStateEventText("state changed", config.DeviceID, config.StateKey, e.runOptions.SourceEvent)
	return workflowNodeValue{Triggered: true, Text: text}, "Device state change matched", map[string]any{
		"triggered": true,
		"device_id": strings.TrimSpace(config.DeviceID),
		"state_key": strings.TrimSpace(config.StateKey),
	}, nil
}

func (e *workflowExecutor) executeDeviceStateIsNode(node models.AgentWorkflowNode) (workflowNodeValue, string, map[string]any, error) {
	if e.service.workflowDevices == nil {
		return workflowNodeValue{}, "", nil, errors.New("workflow device runtime is not configured")
	}
	config, err := decodeWorkflowNodeData[workflowDeviceStateIsConfig](node.Data)
	if err != nil {
		return workflowNodeValue{}, "", nil, err
	}
	inputs := collectedWorkflowInputs{}
	if len(e.incoming[node.ID]) > 0 {
		collected, inputErr := e.collect(node.ID, "")
		if inputErr != nil {
			return workflowNodeValue{}, "", nil, inputErr
		}
		inputs = collected
		if inputs.hasBlockingWindow() {
			return workflowNodeValue{Blocked: true, BlockedByWindow: true}, "Device state predicate outside time window", map[string]any{
				"triggered":         false,
				"input_count":       inputs.count(),
				"blocked_by_window": true,
			}, nil
		}
		if !inputs.hasActiveContent() {
			return workflowNodeValue{Blocked: true, BlockedByTimer: inputs.blockedByTimer > 0}, "Device state predicate waiting for upstream gate", map[string]any{
				"triggered":        false,
				"input_count":      inputs.count(),
				"blocked_by_timer": inputs.blockedByTimer > 0,
			}, nil
		}
	}
	deviceID := strings.TrimSpace(config.DeviceID)
	stateKey := strings.TrimSpace(config.StateKey)
	if deviceID == "" || stateKey == "" {
		return workflowNodeValue{}, "", nil, errors.New("device state is node requires device_id and state_key")
	}
	snapshot, ok, err := e.service.workflowDevices.DeviceState(e.ctx, deviceID)
	if err != nil {
		return workflowNodeValue{}, "", nil, err
	}
	if !ok {
		return workflowNodeValue{}, "", nil, fmt.Errorf("device %q state not found", deviceID)
	}
	value, exists := snapshot.State[stateKey]
	match := normalizeWorkflowStateMatch(config.Match, false)
	if err := validateWorkflowStateMatch(match, false); err != nil {
		return workflowNodeValue{}, "", nil, err
	}
	if !matchesWorkflowStateValue(value, exists, match) {
		return workflowNodeValue{Blocked: true}, "Device state predicate did not match", map[string]any{
			"triggered": false,
			"device_id": deviceID,
			"state_key": stateKey,
		}, nil
	}
	textParts := orderedWorkflowStrings(append([]string{}, inputs.texts...))
	textParts = append(textParts, fmt.Sprintf("Device %s state %s matched", deviceID, stateKey))
	return workflowNodeValue{Triggered: true, Text: strings.Join(textParts, "\n\n")}, "Device state predicate matched", map[string]any{
		"triggered":   true,
		"device_id":   deviceID,
		"state_key":   stateKey,
		"input_count": inputs.count(),
	}, nil
}

func (e *workflowExecutor) executeTimeWindowNode(node models.AgentWorkflowNode) (workflowNodeValue, string, map[string]any, error) {
	config, err := decodeWorkflowNodeData[workflowTimeWindowConfig](node.Data)
	if err != nil {
		return workflowNodeValue{}, "", nil, err
	}
	inputs, inputErr := e.collect(node.ID, "")
	if inputErr != nil {
		return workflowNodeValue{}, "", nil, inputErr
	}
	match, err := workflowTimeWindowMatches(e.workflowRunTime(), config)
	if err != nil {
		return workflowNodeValue{}, "", nil, err
	}
	metadata := map[string]any{
		"start":    strings.TrimSpace(config.Start),
		"end":      strings.TrimSpace(config.End),
		"timezone": strings.TrimSpace(config.Timezone),
		"matched":  match,
	}
	if !match {
		return workflowNodeValue{Blocked: true, BlockedByWindow: true}, "Outside time window", metadata, nil
	}
	if len(e.incoming[node.ID]) > 0 && !inputs.hasActiveContent() {
		return workflowNodeValue{Blocked: true, BlockedByTimer: inputs.blockedByTimer > 0}, "Time window waiting for trigger input", metadata, nil
	}
	if len(e.incoming[node.ID]) > 0 {
		return workflowNodeValue{Triggered: true, Text: strings.Join(orderedWorkflowStrings(inputs.texts), "\n\n")}, "Inside time window", metadata, nil
	}
	return workflowNodeValue{}, "Inside time window", metadata, nil
}

func (e *workflowExecutor) workflowRunTime() time.Time {
	if !e.runOptions.TriggeredAt.IsZero() {
		return e.runOptions.TriggeredAt.UTC()
	}
	if !e.runOptions.SourceEvent.TS.IsZero() {
		return e.runOptions.SourceEvent.TS.UTC()
	}
	return time.Now().UTC()
}

func matchesWorkflowStateChangedNode(config workflowDeviceStateChangedConfig, event models.Event) bool {
	deviceID := strings.TrimSpace(config.DeviceID)
	stateKey := strings.TrimSpace(config.StateKey)
	if deviceID == "" || stateKey == "" || event.DeviceID != deviceID {
		return false
	}
	previousState := workflowStateMap(event.Payload["previous_state"])
	currentState := workflowStateMap(event.Payload["state"])
	currentValue, hasCurrent := currentState[stateKey]
	if !hasCurrent {
		return false
	}
	previousValue, hasPrevious := previousState[stateKey]
	if !hasPrevious || reflect.DeepEqual(previousValue, currentValue) {
		return false
	}
	from := normalizeWorkflowStateMatch(config.From, true)
	to := normalizeWorkflowStateMatch(config.To, false)
	return validateWorkflowStateMatch(from, true) == nil &&
		validateWorkflowStateMatch(to, false) == nil &&
		matchesWorkflowStateValue(previousValue, hasPrevious, from) &&
		matchesWorkflowStateValue(currentValue, hasCurrent, to)
}

func matchesWorkflowStateIsNode(config workflowDeviceStateIsConfig, event models.Event) bool {
	deviceID := strings.TrimSpace(config.DeviceID)
	stateKey := strings.TrimSpace(config.StateKey)
	if deviceID == "" || stateKey == "" || event.DeviceID != deviceID {
		return false
	}
	currentState := workflowStateMap(event.Payload["state"])
	value, exists := currentState[stateKey]
	match := normalizeWorkflowStateMatch(config.Match, false)
	return validateWorkflowStateMatch(match, false) == nil && matchesWorkflowStateValue(value, exists, match)
}

func workflowStateEventText(kind string, deviceID string, stateKey string, event models.Event) string {
	return fmt.Sprintf("Device %s %s for %s at %s", strings.TrimSpace(deviceID), kind, strings.TrimSpace(stateKey), event.TS.UTC().Format(time.RFC3339))
}

func workflowStateMap(value any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func normalizeWorkflowStateMatch(match *workflowStateMatchConfig, allowAny bool) workflowStateMatchConfig {
	if match == nil {
		if allowAny {
			return workflowStateMatchConfig{Operator: workflowMatchAny}
		}
		return workflowStateMatchConfig{Operator: workflowMatchExists}
	}
	normalized := *match
	normalized.Operator = strings.TrimSpace(normalized.Operator)
	if normalized.Operator == "" {
		if normalized.Value != nil {
			normalized.Operator = workflowMatchEquals
		} else if allowAny {
			normalized.Operator = workflowMatchAny
		} else {
			normalized.Operator = workflowMatchExists
		}
	}
	if normalized.Operator == workflowMatchIn || normalized.Operator == workflowMatchNotIn {
		normalized.Value = workflowMatchValueList(normalized.Value)
	}
	return normalized
}

func validateWorkflowStateMatch(match workflowStateMatchConfig, allowAny bool) error {
	switch match.Operator {
	case workflowMatchAny:
		if !allowAny {
			return errors.New(`operator "any" is only allowed for transition from`)
		}
	case workflowMatchEquals, workflowMatchNotEquals:
		if match.Value == nil {
			return errors.New("state matcher value is required")
		}
	case workflowMatchIn, workflowMatchNotIn:
		if len(workflowMatchValueList(match.Value)) == 0 {
			return errors.New("state matcher value list is required")
		}
	case workflowMatchExists, workflowMatchMissing:
	default:
		return fmt.Errorf("unsupported state matcher operator %q", match.Operator)
	}
	return nil
}

func matchesWorkflowStateValue(value any, exists bool, match workflowStateMatchConfig) bool {
	switch match.Operator {
	case workflowMatchAny, workflowMatchExists:
		return exists
	case workflowMatchMissing:
		return !exists
	case workflowMatchEquals:
		return exists && workflowValueEquals(value, match.Value)
	case workflowMatchNotEquals:
		return !exists || !workflowValueEquals(value, match.Value)
	case workflowMatchIn:
		return exists && workflowValueInList(value, match.Value)
	case workflowMatchNotIn:
		return !exists || !workflowValueInList(value, match.Value)
	default:
		return false
	}
}

func workflowValueInList(value any, list any) bool {
	for _, item := range workflowMatchValueList(list) {
		if workflowValueEquals(value, item) {
			return true
		}
	}
	return false
}

func workflowValueEquals(left any, right any) bool {
	leftRaw, err := json.Marshal(left)
	if err != nil {
		return false
	}
	rightRaw, err := json.Marshal(right)
	if err != nil {
		return false
	}
	return string(leftRaw) == string(rightRaw)
}

func workflowMatchValueList(value any) []any {
	switch typed := value.(type) {
	case nil:
		return nil
	case []any:
		return append([]any{}, typed...)
	case []string:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out
	case []int:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out
	case []float64:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out
	case []bool:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out
	default:
		return []any{typed}
	}
}

func workflowTimeWindowMatches(now time.Time, window workflowTimeWindowConfig) (bool, error) {
	startRaw := strings.TrimSpace(window.Start)
	endRaw := strings.TrimSpace(window.End)
	if startRaw == "" && endRaw == "" {
		return true, nil
	}
	if startRaw == "" || endRaw == "" {
		return false, errors.New("time window requires both start and end")
	}
	start, err := timeschedule.ParseClockHM(startRaw)
	if err != nil {
		return false, fmt.Errorf("invalid time window start: %w", err)
	}
	end, err := timeschedule.ParseClockHM(endRaw)
	if err != nil {
		return false, fmt.Errorf("invalid time window end: %w", err)
	}
	location := time.Local
	if timezone := strings.TrimSpace(window.Timezone); timezone != "" {
		loaded, loadErr := time.LoadLocation(timezone)
		if loadErr != nil {
			return false, fmt.Errorf("invalid time window timezone: %w", loadErr)
		}
		location = loaded
	}
	current := now.In(location)
	currentMinutes := current.Hour()*60 + current.Minute()
	if start == end {
		return true, nil
	}
	if start < end {
		return currentMinutes >= start && currentMinutes < end, nil
	}
	return currentMinutes >= start || currentMinutes < end, nil
}
