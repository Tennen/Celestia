package automation

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
)

func (s *Service) handleStateChange(event models.Event) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	automations := s.indexedAutomationsForDevice(event.DeviceID)
	if len(automations) == 0 {
		return
	}
	previousState := stateMap(event.Payload["previous_state"])
	currentState := stateMap(event.Payload["state"])
	matchedStateChanges := 0
	triggeredCount := 0
	for _, automation := range automations {
		trigger, ok := stateChangedCondition(automation.Conditions)
		if !ok {
			continue
		}
		if !matchesStateChangedCondition(trigger, event.DeviceID, previousState, currentState) {
			continue
		}
		matchedStateChanges++
		if !matchesTimeWindow(event.TS, automation.TimeWindow) {
			continue
		}
		ok, err := s.matchesStateConditions(ctx, automation)
		if err != nil {
			s.updateRunResult(ctx, automation, models.AutomationRunStatusFailed, err.Error())
			s.publishAutomationEvent(models.EventAutomationFailed, automation, event, err.Error(), nil)
			continue
		}
		if !ok {
			if event.PluginID == "haier" {
				log.Printf(
					"automation: state event matched trigger but gate rejected automation=%s device=%s source=%v changed_keys=%v",
					automation.ID,
					event.DeviceID,
					event.Payload["source"],
					event.Payload["changed_keys"],
				)
			}
			continue
		}
		if err := s.executeAutomation(ctx, automation, event); err != nil {
			s.updateRunResult(ctx, automation, models.AutomationRunStatusFailed, err.Error())
			s.publishAutomationEvent(models.EventAutomationFailed, automation, event, err.Error(), nil)
			continue
		}
		triggeredCount++
		s.updateRunResult(ctx, automation, models.AutomationRunStatusSucceeded, "")
		s.publishAutomationEvent(models.EventAutomationTriggered, automation, event, "", nil)
	}
	if event.PluginID == "haier" {
		log.Printf(
			"automation: state event evaluated plugin=%s device=%s source=%v changed_keys=%v matched_state_changes=%d triggered=%d",
			event.PluginID,
			event.DeviceID,
			event.Payload["source"],
			event.Payload["changed_keys"],
			matchedStateChanges,
			triggeredCount,
		)
	}
}

func matchesStateChangedCondition(
	condition models.AutomationCondition,
	deviceID string,
	previousState map[string]any,
	currentState map[string]any,
) bool {
	if condition.DeviceID != deviceID {
		return false
	}
	currentValue, hasCurrent := currentState[condition.StateKey]
	if !hasCurrent {
		return false
	}
	previousValue, hasPrevious := previousState[condition.StateKey]
	if !hasPrevious {
		return false
	}
	if reflect.DeepEqual(previousValue, currentValue) {
		return false
	}
	return condition.From != nil &&
		condition.To != nil &&
		matchesStateValue(previousValue, hasPrevious, *condition.From) &&
		matchesStateValue(currentValue, hasCurrent, *condition.To)
}

func matchesTimeWindow(ts time.Time, window *models.AutomationTimeWindow) bool {
	if window == nil {
		return true
	}
	start, err := parseClockHM(window.Start)
	if err != nil {
		return false
	}
	end, err := parseClockHM(window.End)
	if err != nil {
		return false
	}
	current := ts.In(time.Local)
	currentMinutes := current.Hour()*60 + current.Minute()
	if start == end {
		return true
	}
	if start < end {
		return currentMinutes >= start && currentMinutes < end
	}
	return currentMinutes >= start || currentMinutes < end
}

func parseClockHM(value string) (int, error) {
	parsed, err := time.Parse("15:04", strings.TrimSpace(value))
	if err != nil {
		return 0, err
	}
	return parsed.Hour()*60 + parsed.Minute(), nil
}

func (s *Service) matchesStateConditions(ctx context.Context, automation models.Automation) (bool, error) {
	stateConditions := make([]models.AutomationCondition, 0, len(automation.Conditions))
	for _, condition := range automation.Conditions {
		if condition.Type != models.AutomationConditionTypeCurrentState {
			continue
		}
		stateConditions = append(stateConditions, condition)
	}
	if len(stateConditions) == 0 {
		return true, nil
	}
	results := make([]bool, 0, len(stateConditions))
	for _, condition := range stateConditions {
		snapshot, ok, err := s.state.Get(ctx, condition.DeviceID)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, fmt.Errorf("condition device %q state not found", condition.DeviceID)
		}
		value, exists := snapshot.State[condition.StateKey]
		results = append(results, condition.Match != nil && matchesStateValue(value, exists, *condition.Match))
	}
	if automation.ConditionLogic == models.AutomationLogicAny {
		for _, result := range results {
			if result {
				return true, nil
			}
		}
		return false, nil
	}
	for _, result := range results {
		if !result {
			return false, nil
		}
	}
	return true, nil
}

func (s *Service) executeAutomation(ctx context.Context, automation models.Automation, sourceEvent models.Event) error {
	actor := "automation:" + automation.ID
	var failures []string
	for _, action := range automation.Actions {
		device, ok, err := s.registry.Get(ctx, action.DeviceID)
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		if !ok {
			failures = append(failures, fmt.Sprintf("device %q not found", action.DeviceID))
			continue
		}
		decision := s.policy.Evaluate(actor, action.Action)
		auditRecord := models.AuditRecord{
			ID:        uuid.NewString(),
			Actor:     actor,
			DeviceID:  device.ID,
			Action:    action.Action,
			Params:    cloneParams(action.Params),
			Allowed:   decision.Allowed,
			RiskLevel: decision.RiskLevel,
			CreatedAt: time.Now().UTC(),
		}
		if !decision.Allowed {
			auditRecord.Result = "denied"
			_ = s.audit.Append(ctx, auditRecord)
			failures = append(failures, fmt.Sprintf("action %q denied: %s", action.Action, decision.Reason))
			continue
		}
		resp, err := s.pluginMgr.ExecuteCommand(ctx, device, models.CommandRequest{
			DeviceID:  device.ID,
			Action:    action.Action,
			Params:    cloneParams(action.Params),
			RequestID: uuid.NewString(),
		})
		if err != nil {
			auditRecord.Result = "failed"
			_ = s.audit.Append(ctx, auditRecord)
			failures = append(failures, fmt.Sprintf("action %q failed: %v", action.Action, err))
			continue
		}
		if !resp.Accepted {
			auditRecord.Result = "failed"
			_ = s.audit.Append(ctx, auditRecord)
			failures = append(failures, fmt.Sprintf("action %q rejected: %s", action.Action, strings.TrimSpace(resp.Message)))
			continue
		}
		auditRecord.Result = "accepted"
		_ = s.audit.Append(ctx, auditRecord)
	}
	if len(failures) > 0 {
		return fmt.Errorf(strings.Join(failures, "; "))
	}
	log.Printf("automation: triggered id=%s name=%q source_event=%s device=%s", automation.ID, automation.Name, sourceEvent.ID, sourceEvent.DeviceID)
	return nil
}

func (s *Service) updateRunResult(ctx context.Context, automation models.Automation, status models.AutomationRunStatus, message string) {
	automation.LastRunStatus = status
	automation.LastError = strings.TrimSpace(message)
	now := time.Now().UTC()
	automation.LastTriggeredAt = &now
	automation.UpdatedAt = now
	s.updateIndexedAutomation(automation)
	if err := s.store.UpsertAutomation(ctx, automation); err != nil {
		log.Printf("automation: persist run result id=%s failed: %v", automation.ID, err)
	}
}

func (s *Service) publishAutomationEvent(eventType models.EventType, automation models.Automation, sourceEvent models.Event, message string, payload map[string]any) {
	if payload == nil {
		payload = map[string]any{}
	}
	payload["automation_id"] = automation.ID
	payload["automation_name"] = automation.Name
	payload["source_event_id"] = sourceEvent.ID
	payload["source_device_id"] = sourceEvent.DeviceID
	payload["enabled"] = automation.Enabled
	if strings.TrimSpace(message) != "" {
		payload["message"] = strings.TrimSpace(message)
	}
	event := models.Event{
		ID:       uuid.NewString(),
		Type:     eventType,
		TS:       time.Now().UTC(),
		DeviceID: sourceEvent.DeviceID,
		Payload:  payload,
	}
	if err := s.store.AppendEvent(context.Background(), event); err != nil {
		log.Printf("automation: append event failed: %v", err)
	}
	s.bus.Publish(event)
}

func stateMap(value any) map[string]any {
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

func matchesStateValue(value any, exists bool, match models.AutomationStateMatch) bool {
	switch match.Operator {
	case models.AutomationMatchAny:
		return exists
	case models.AutomationMatchExists:
		return exists
	case models.AutomationMatchMissing:
		return !exists
	case models.AutomationMatchEquals:
		if !exists {
			return false
		}
		return valueEquals(value, match.Value)
	case models.AutomationMatchNotEquals:
		if !exists {
			return true
		}
		return !valueEquals(value, match.Value)
	case models.AutomationMatchIn:
		if !exists {
			return false
		}
		return valueInList(value, match.Value)
	case models.AutomationMatchNotIn:
		if !exists {
			return true
		}
		return !valueInList(value, match.Value)
	default:
		return false
	}
}

func valueInList(value any, list any) bool {
	for _, item := range matchValueList(list) {
		if valueEquals(value, item) {
			return true
		}
	}
	return false
}

func valueEquals(left any, right any) bool {
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

func cloneParams(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	raw, err := json.Marshal(src)
	if err != nil {
		out := make(map[string]any, len(src))
		for key, value := range src {
			out[key] = value
		}
		return out
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{}
	}
	return out
}
