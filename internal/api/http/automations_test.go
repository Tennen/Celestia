package httpapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleCreateAutomation_AllowsBlankDerivedTimestamps(t *testing.T) {
	gw := &stubGateway{}
	s := &Server{gateway: gw}

	body := bytes.NewBufferString(`{
		"name":"Washer done",
		"enabled":true,
		"trigger":{
			"device_id":"haier:washer:test",
			"state_key":"phase",
			"from":{"operator":"not_equals","value":"running"},
			"to":{"operator":"equals","value":"done"}
		},
		"condition_logic":"all",
		"conditions":[],
		"actions":[{"device_id":"xiaomi:speaker:test","action":"push_voice_message","params":{"message":"done"}}],
		"last_triggered_at":"",
		"created_at":"",
		"updated_at":""
	}`)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/automations", body)
	w := httptest.NewRecorder()

	s.handleCreateAutomation(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !gw.savedAutomation.CreatedAt.IsZero() {
		t.Fatalf("expected CreatedAt to remain zero before service normalization, got %v", gw.savedAutomation.CreatedAt)
	}
	if !gw.savedAutomation.UpdatedAt.IsZero() {
		t.Fatalf("expected UpdatedAt to remain zero before service normalization, got %v", gw.savedAutomation.UpdatedAt)
	}
	if gw.savedAutomation.LastTriggeredAt != nil {
		t.Fatalf("expected LastTriggeredAt to be nil, got %v", gw.savedAutomation.LastTriggeredAt)
	}
	if len(gw.savedAutomation.Conditions) != 1 {
		t.Fatalf("expected legacy trigger to be converted into one condition, got %d", len(gw.savedAutomation.Conditions))
	}
	if gw.savedAutomation.Conditions[0].Scope != "event" || gw.savedAutomation.Conditions[0].Kind != "transition" {
		t.Fatalf("expected converted condition to be event/transition, got %#v", gw.savedAutomation.Conditions[0])
	}
}
