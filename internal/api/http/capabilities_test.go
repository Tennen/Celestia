package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleDeleteVisionRuleEvent(t *testing.T) {
	gateway := &stubGateway{}
	server := &Server{gateway: gateway}

	req := httptest.NewRequest(
		http.MethodDelete,
		"/api/v1/capabilities/vision_entity_stay_zone/rules/feeder-zone/events/vision-evt-1",
		nil,
	)
	req.SetPathValue("ruleID", "feeder-zone")
	req.SetPathValue("eventID", "vision-evt-1")
	recorder := httptest.NewRecorder()

	server.handleDeleteVisionRuleEvent(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("handleDeleteVisionRuleEvent() status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if gateway.deletedVisionRuleID != "feeder-zone" {
		t.Fatalf("deleted rule id = %q, want feeder-zone", gateway.deletedVisionRuleID)
	}
	if gateway.deletedVisionEventID != "vision-evt-1" {
		t.Fatalf("deleted event id = %q, want vision-evt-1", gateway.deletedVisionEventID)
	}
}
