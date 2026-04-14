package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHandleEvents_ForwardsPaginationAndDateFilters(t *testing.T) {
	gw := &stubGateway{}
	s := &Server{gateway: gw}

	r := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/events?plugin_id=xiaomi&device_id=device-1&from_ts=2026-04-10T00:00:00Z&to_ts=2026-04-12T00:00:00Z&before_ts=2026-04-11T12:00:00Z&before_id=evt-9&limit=25",
		nil,
	)
	w := httptest.NewRecorder()

	s.handleEvents(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if gw.listedEventsFilter.PluginID != "xiaomi" {
		t.Fatalf("plugin_id = %q, want xiaomi", gw.listedEventsFilter.PluginID)
	}
	if gw.listedEventsFilter.DeviceID != "device-1" {
		t.Fatalf("device_id = %q, want device-1", gw.listedEventsFilter.DeviceID)
	}
	if gw.listedEventsFilter.BeforeID != "evt-9" {
		t.Fatalf("before_id = %q, want evt-9", gw.listedEventsFilter.BeforeID)
	}
	if gw.listedEventsFilter.Limit != 25 {
		t.Fatalf("limit = %d, want 25", gw.listedEventsFilter.Limit)
	}
	assertFilterTime(t, "from_ts", gw.listedEventsFilter.FromTS, "2026-04-10T00:00:00Z")
	assertFilterTime(t, "to_ts", gw.listedEventsFilter.ToTS, "2026-04-12T00:00:00Z")
	assertFilterTime(t, "before_ts", gw.listedEventsFilter.BeforeTS, "2026-04-11T12:00:00Z")
}

func TestHandleEvents_RejectsInvalidTimestamp(t *testing.T) {
	gw := &stubGateway{}
	s := &Server{gateway: gw}

	r := httptest.NewRequest(http.MethodGet, "/api/v1/events?from_ts=not-a-timestamp", nil)
	w := httptest.NewRecorder()

	s.handleEvents(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func assertFilterTime(t *testing.T, field string, value *time.Time, want string) {
	t.Helper()
	if value == nil {
		t.Fatalf("%s was nil", field)
	}
	if value.UTC().Format(time.RFC3339Nano) != want {
		t.Fatalf("%s = %s, want %s", field, value.UTC().Format(time.RFC3339Nano), want)
	}
}
