package automation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
)

type automationTouchpoint struct {
	Type     string
	ToUser   string
	DeviceID string
	Action   string
	Params   map[string]any
}

func (s *Service) executeAgentAction(ctx context.Context, automation models.Automation, action models.AutomationAction) error {
	s.mu.RLock()
	agent := s.agent
	s.mu.RUnlock()
	if agent == nil {
		return errors.New("agent runtime is not available")
	}
	input := strings.TrimSpace(stringParam(action.Params["input"]))
	if input == "" {
		return errors.New("agent input is required")
	}
	sessionID := strings.TrimSpace(stringParam(action.Params["session_id"]))
	if sessionID == "" {
		sessionID = "automation:" + automation.ID
	}
	result, err := agent.HandleInput(ctx, models.ProjectInputRequest{
		SessionID: sessionID,
		Input:     input,
		Actor:     "automation:" + automation.ID,
		Source:    "automation",
	})
	if err != nil {
		return err
	}
	message := strings.TrimSpace(result.ResponseText)
	if message == "" {
		return errors.New("agent returned empty response")
	}
	var failures []string
	for _, touchpoint := range parseTouchpoints(action.Params) {
		if err := s.deliverTouchpoint(ctx, automation, touchpoint, message); err != nil {
			failures = append(failures, err.Error())
		}
	}
	if len(failures) > 0 {
		return errors.New(strings.Join(failures, "; "))
	}
	return nil
}

func (s *Service) validateAgentTouchpoints(ctx context.Context, params map[string]any) error {
	for _, touchpoint := range parseTouchpoints(params) {
		switch touchpoint.Type {
		case "", "none":
			continue
		case "wecom":
			if strings.TrimSpace(touchpoint.ToUser) == "" {
				return errors.New("wecom touchpoint requires to_user")
			}
			s.mu.RLock()
			wecom := s.wecom
			s.mu.RUnlock()
			if wecom == nil {
				return errors.New("wecom runtime is not available")
			}
			if _, err := wecom.ResolveWeComRecipient(ctx, touchpoint.ToUser); err != nil {
				return err
			}
		case "device":
			if strings.TrimSpace(touchpoint.DeviceID) == "" {
				return errors.New("device touchpoint requires device_id")
			}
			if _, ok, err := s.registry.Get(ctx, touchpoint.DeviceID); err != nil {
				return err
			} else if !ok {
				return fmt.Errorf("device touchpoint %q not found", touchpoint.DeviceID)
			}
		default:
			return fmt.Errorf("unsupported touchpoint type %q", touchpoint.Type)
		}
	}
	return nil
}

func (s *Service) deliverTouchpoint(ctx context.Context, automation models.Automation, touchpoint automationTouchpoint, message string) error {
	switch touchpoint.Type {
	case "", "none":
		return nil
	case "wecom":
		s.mu.RLock()
		wecom := s.wecom
		s.mu.RUnlock()
		if wecom == nil {
			return errors.New("wecom runtime is not available")
		}
		return wecom.SendWeComText(ctx, touchpoint.ToUser, message)
	case "device":
		params := cloneParams(touchpoint.Params)
		if _, ok := params["message"]; !ok {
			params["message"] = message
		}
		return s.executeDeviceAction(ctx, "automation:"+automation.ID, models.AutomationAction{
			Kind:     models.AutomationActionKindDevice,
			DeviceID: touchpoint.DeviceID,
			Action:   firstNonEmpty(touchpoint.Action, "push_voice_message"),
			Params:   params,
		})
	default:
		return fmt.Errorf("unsupported touchpoint type %q", touchpoint.Type)
	}
}

func parseTouchpoints(params map[string]any) []automationTouchpoint {
	raw, ok := params["touchpoints"]
	if !ok {
		raw = params["output_touchpoints"]
	}
	switch typed := raw.(type) {
	case nil:
		return nil
	case []any:
		out := make([]automationTouchpoint, 0, len(typed))
		for _, item := range typed {
			if parsed, ok := parseTouchpoint(item); ok {
				out = append(out, parsed)
			}
		}
		return out
	case map[string]any:
		if parsed, ok := parseTouchpoint(typed); ok {
			return []automationTouchpoint{parsed}
		}
	}
	return nil
}

func parseTouchpoint(raw any) (automationTouchpoint, bool) {
	item, ok := raw.(map[string]any)
	if !ok {
		return automationTouchpoint{}, false
	}
	touchpoint := automationTouchpoint{
		Type:     strings.TrimSpace(stringParam(item["type"])),
		ToUser:   strings.TrimSpace(firstNonEmpty(stringParam(item["to_user"]), stringParam(item["toUser"]))),
		DeviceID: strings.TrimSpace(firstNonEmpty(stringParam(item["device_id"]), stringParam(item["deviceId"]))),
		Action:   strings.TrimSpace(stringParam(item["action"])),
		Params:   mapParam(item["params"]),
	}
	if touchpoint.Type == "" && touchpoint.ToUser != "" {
		touchpoint.Type = "wecom"
	}
	if touchpoint.Type == "" && touchpoint.DeviceID != "" {
		touchpoint.Type = "device"
	}
	return touchpoint, true
}

func (s *Service) executeDeviceAction(ctx context.Context, actor string, action models.AutomationAction) error {
	device, ok, err := s.registry.Get(ctx, action.DeviceID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("device %q not found", action.DeviceID)
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
		return fmt.Errorf("action %q denied: %s", action.Action, decision.Reason)
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
		return fmt.Errorf("action %q failed: %v", action.Action, err)
	}
	if !resp.Accepted {
		auditRecord.Result = "failed"
		_ = s.audit.Append(ctx, auditRecord)
		return fmt.Errorf("action %q rejected: %s", action.Action, strings.TrimSpace(resp.Message))
	}
	auditRecord.Result = "accepted"
	_ = s.audit.Append(ctx, auditRecord)
	return nil
}

func stringParam(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return ""
	}
}

func mapParam(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return map[string]any{}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
