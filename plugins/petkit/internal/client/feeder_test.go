package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

func TestExecuteFeederFeedOnceUsesTypedEndpoint(t *testing.T) {
	apiClient := newCommandTestClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/d4h/saveDailyFeed" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		body := mustParseFormBody(t, r)
		if got := body.Get("deviceId"); got != "118197" {
			t.Fatalf("unexpected deviceId: %q", got)
		}
		if got := body.Get("amount"); got != "2" {
			t.Fatalf("unexpected amount: %q", got)
		}
		if got := r.Header.Get("X-Session"); got != "session-old" {
			t.Fatalf("unexpected session header: %q", got)
		}
		return jsonResponse(r, http.StatusOK, map[string]any{"result": true})
	}))

	err := apiClient.executeFeeder(context.Background(), DeviceSnapshot{
		Info: PetkitDeviceInfo{
			DeviceID:   118197,
			DeviceType: "d4h",
		},
	}, models.CommandRequest{
		Action: "feed_once",
		Params: map[string]any{"portions": 2},
	})
	if err != nil {
		t.Fatalf("executeFeeder returned error: %v", err)
	}
}

func TestExecuteFeederManualFeedDualUsesTypedEndpoint(t *testing.T) {
	apiClient := newCommandTestClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/d4s/saveDailyFeed" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		body := mustParseFormBody(t, r)
		if got := body.Get("amount1"); got != "20" {
			t.Fatalf("unexpected amount1: %q", got)
		}
		if got := body.Get("amount2"); got != "30" {
			t.Fatalf("unexpected amount2: %q", got)
		}
		return jsonResponse(r, http.StatusOK, map[string]any{"result": true})
	}))

	err := apiClient.executeFeeder(context.Background(), DeviceSnapshot{
		Info: PetkitDeviceInfo{
			DeviceID:   118197,
			DeviceType: "d4s",
		},
	}, models.CommandRequest{
		Action: "manual_feed_dual",
		Params: map[string]any{"amount1": 20, "amount2": 30},
	})
	if err != nil {
		t.Fatalf("executeFeeder returned error: %v", err)
	}
}

func TestExecuteFeederCancelManualFeedUsesManualFeedID(t *testing.T) {
	apiClient := newCommandTestClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/d4s/cancelRealtimeFeed" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		body := mustParseFormBody(t, r)
		if got := body.Get("id"); got != "feed-job-1" {
			t.Fatalf("unexpected manual feed id: %q", got)
		}
		return jsonResponse(r, http.StatusOK, map[string]any{"result": true})
	}))

	err := apiClient.executeFeeder(context.Background(), DeviceSnapshot{
		Info: PetkitDeviceInfo{
			DeviceID:   118197,
			DeviceType: "d4s",
		},
		Detail: map[string]any{
			"manualFeed": map[string]any{"id": "feed-job-1"},
		},
	}, models.CommandRequest{
		Action: "cancel_manual_feed",
	})
	if err != nil {
		t.Fatalf("executeFeeder returned error: %v", err)
	}
}

func TestExecuteLitterUsesTypedControlEndpoint(t *testing.T) {
	apiClient := newCommandTestClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/t5/controlDevice" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		body := mustParseFormBody(t, r)
		if got := body.Get("type"); got != "start_action" {
			t.Fatalf("unexpected command type: %q", got)
		}
		if got := body.Get("id"); got != "42" {
			t.Fatalf("unexpected device id: %q", got)
		}
		return jsonResponse(r, http.StatusOK, map[string]any{"result": true})
	}))

	err := apiClient.executeLitter(context.Background(), DeviceSnapshot{
		Info: PetkitDeviceInfo{
			DeviceID:   42,
			DeviceType: "t5",
		},
	}, models.CommandRequest{
		Action: "clean_now",
	})
	if err != nil {
		t.Fatalf("executeLitter returned error: %v", err)
	}
}

func TestLatestFeederOccurredEventUsesRecordPayload(t *testing.T) {
	event := LatestFeederOccurredEvent(map[string]any{
		"feed": []any{
			map[string]any{
				"items": []any{
					map[string]any{
						"eventId":       "evt-2",
						"enumEventType": "manual_feed_dual",
						"timestamp":     1_710_000_000,
						"amount1":       20,
						"amount2":       30,
						"state": map[string]any{
							"realAmount1": 20,
							"realAmount2": 30,
						},
					},
				},
			},
		},
	})
	if event == nil {
		t.Fatal("expected event")
	}
	if event.Key != "evt-2" {
		t.Fatalf("unexpected event key: %q", event.Key)
	}
	if got := event.Payload["event"]; got != "manual_feed_dual" {
		t.Fatalf("unexpected event name: %v", got)
	}
	if got := event.Payload["amount1"]; got != 20 {
		t.Fatalf("unexpected amount1: %v", got)
	}
	if got := event.Payload["amount2"]; got != 30 {
		t.Fatalf("unexpected amount2: %v", got)
	}
}

func TestBuildStateIncludesFeederHopperLevels(t *testing.T) {
	state := BuildState(PetkitDeviceInfo{DeviceType: "d4sh"}, models.DeviceKindPetFeeder, map[string]any{
		"state": map[string]any{
			"food":    80,
			"food1":   55,
			"food2":   65,
			"feeding": 0,
		},
		"settings": map[string]any{
			"surplusControl":  1,
			"surplusStandard": 3,
			"selectedSound":   8,
		},
	}, nil)

	if got := state["food_level_hopper_1"]; got != 55 {
		t.Fatalf("unexpected hopper 1 level: %v", got)
	}
	if got := state["food_level_hopper_2"]; got != 65 {
		t.Fatalf("unexpected hopper 2 level: %v", got)
	}
	if got := state["surplus_control"]; got != 1 {
		t.Fatalf("unexpected surplus_control: %v", got)
	}
	if got := state["selected_sound"]; got != 8 {
		t.Fatalf("unexpected selected_sound: %v", got)
	}
}

func newCommandTestClient(t *testing.T, transport roundTripFunc) *Client {
	t.Helper()
	compat := DefaultCompatConfig()
	compat.ChinaBaseURL = "https://petkit.test/"
	apiClient := NewClient(AccountConfig{
		Username: "user@example.com",
		Password: "secret",
		Region:   "cn",
		Timezone: "Asia/Shanghai",
	}, compat)
	apiClient.httpClient = &http.Client{Transport: transport}
	apiClient.baseURL = "https://petkit.test/"
	apiClient.session = &SessionInfo{
		ID:        "session-old",
		UserID:    "user-1",
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(time.Hour),
		ExpiresIn: 3600,
	}
	return apiClient
}

func mustParseFormBody(t *testing.T, r *http.Request) url.Values {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	values, err := url.ParseQuery(string(body))
	if err != nil {
		t.Fatalf("parse body: %v", err)
	}
	return values
}

func jsonResponse(r *http.Request, status int, payload map[string]any) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode: status,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body:    io.NopCloser(bytes.NewReader(body)),
		Request: r,
	}, nil
}
