package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

func (s *Server) handleAutomations(w http.ResponseWriter, r *http.Request) {
	items, err := s.gateway.ListAutomations(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateAutomation(w http.ResponseWriter, r *http.Request) {
	automation, err := decodeAutomationRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	item, err := s.gateway.SaveAutomation(r.Context(), automation)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleUpdateAutomation(w http.ResponseWriter, r *http.Request) {
	automation, err := decodeAutomationRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	automation.ID = r.PathValue("id")
	item, err := s.gateway.SaveAutomation(r.Context(), automation)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleDeleteAutomation(w http.ResponseWriter, r *http.Request) {
	if err := s.gateway.DeleteAutomation(r.Context(), r.PathValue("id")); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func decodeAutomationRequest(r *http.Request) (models.Automation, error) {
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return models.Automation{}, err
	}
	for _, key := range []string{"created_at", "updated_at", "last_triggered_at"} {
		if value, ok := payload[key].(string); ok && strings.TrimSpace(value) == "" {
			delete(payload, key)
		}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return models.Automation{}, err
	}
	var automation models.Automation
	if err := json.Unmarshal(raw, &automation); err != nil {
		return models.Automation{}, err
	}
	if triggerPayload, ok := payload["trigger"]; ok {
		legacyTrigger, err := decodeLegacyTriggerCondition(triggerPayload)
		if err != nil {
			return models.Automation{}, err
		}
		automation.Conditions = append([]models.AutomationCondition{legacyTrigger}, automation.Conditions...)
	}
	return automation, nil
}

type legacyAutomationTrigger struct {
	DeviceID string                      `json:"device_id"`
	StateKey string                      `json:"state_key"`
	From     models.AutomationStateMatch `json:"from"`
	To       models.AutomationStateMatch `json:"to"`
}

func decodeLegacyTriggerCondition(value any) (models.AutomationCondition, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return models.AutomationCondition{}, err
	}
	var trigger legacyAutomationTrigger
	if err := json.Unmarshal(raw, &trigger); err != nil {
		return models.AutomationCondition{}, err
	}
	from := trigger.From
	to := trigger.To
	return models.AutomationCondition{
		Scope:    models.AutomationConditionScopeEvent,
		Kind:     models.AutomationConditionKindTransition,
		DeviceID: strings.TrimSpace(trigger.DeviceID),
		StateKey: strings.TrimSpace(trigger.StateKey),
		From:     &from,
		To:       &to,
	}, nil
}
