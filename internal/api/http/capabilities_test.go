package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleVisionRuleEvents_ForwardsDateFiltersAndCursor(t *testing.T) {
	gateway := &stubGateway{}
	server := &Server{gateway: gateway}

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/capabilities/vision_entity_stay_zone/rules/feeder-zone/events?from_ts=2026-04-10T00:00:00Z&to_ts=2026-04-12T00:00:00Z&before_ts=2026-04-11T12:00:00Z&before_id=vision-evt-9&limit=25",
		nil,
	)
	req.SetPathValue("ruleID", "feeder-zone")
	recorder := httptest.NewRecorder()

	server.handleVisionRuleEvents(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("handleVisionRuleEvents() status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if gateway.listedVisionRuleID != "feeder-zone" {
		t.Fatalf("rule id = %q, want feeder-zone", gateway.listedVisionRuleID)
	}
	if gateway.listedVisionFilter.BeforeID != "vision-evt-9" {
		t.Fatalf("before_id = %q, want vision-evt-9", gateway.listedVisionFilter.BeforeID)
	}
	if gateway.listedVisionFilter.Limit != 25 {
		t.Fatalf("limit = %d, want 25", gateway.listedVisionFilter.Limit)
	}
	assertFilterTime(t, "from_ts", gateway.listedVisionFilter.FromTS, "2026-04-10T00:00:00Z")
	assertFilterTime(t, "to_ts", gateway.listedVisionFilter.ToTS, "2026-04-12T00:00:00Z")
	assertFilterTime(t, "before_ts", gateway.listedVisionFilter.BeforeTS, "2026-04-11T12:00:00Z")
}

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
