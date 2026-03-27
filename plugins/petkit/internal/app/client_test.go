package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestParseRegionServerList(t *testing.T) {
	t.Run("map with list", func(t *testing.T) {
		value := map[string]any{
			"list": []any{
				map[string]any{"id": "US"},
			},
			"pref": "CN",
		}
		list, ok := parseRegionServerList(value)
		if !ok {
			t.Fatal("expected list to parse from map payload")
		}
		if len(list) != 1 {
			t.Fatalf("expected 1 region entry, got %d", len(list))
		}
	})

	t.Run("bare list", func(t *testing.T) {
		value := []any{
			map[string]any{"id": "EU"},
		}
		list, ok := parseRegionServerList(value)
		if !ok {
			t.Fatal("expected list to parse from bare array payload")
		}
		if len(list) != 1 {
			t.Fatalf("expected 1 region entry, got %d", len(list))
		}
	})

	t.Run("invalid shape", func(t *testing.T) {
		if _, ok := parseRegionServerList(map[string]any{"pref": "CN"}); ok {
			t.Fatal("expected invalid payload to fail parsing")
		}
	})
}

func TestCompatClientPayloadMatchesUpstreamFormat(t *testing.T) {
	client := NewClient(
		AccountConfig{
			Username: "user@example.com",
			Password: "secret",
			Region:   "us",
			Timezone: "Asia/Shanghai",
		},
		defaultCompatConfig(),
	)
	got := client.compatClientPayload("Asia/Shanghai")
	want := "{'locale': 'en-US', 'name': '23127PN0CG', 'osVersion': '15.1', 'phoneBrand': 'Xiaomi', 'platform': 'android', 'source': 'app.petkit-android', 'version': '12.4.9', 'timezoneId': 'Asia/Shanghai'}"
	if got != want {
		t.Fatalf("unexpected client payload:\nwant: %s\ngot:  %s", want, got)
	}
}

func TestSanitizeSessionBaseURLDropsLegacyPassportPath(t *testing.T) {
	compat := defaultCompatConfig()
	got := sanitizeSessionBaseURL("https://passport.petkt.com/6/", "us", compat)
	if got != "" {
		t.Fatalf("expected legacy passport path to be dropped, got %q", got)
	}
}

func TestLoadDeviceDetailFallsBackToTypedDeviceDataOnCode97(t *testing.T) {
	var loginCalls atomic.Int32
	var detailCalls atomic.Int32
	var deviceDataCalls atomic.Int32

	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			var (
				status  int
				payload map[string]any
			)

			switch {
			case r.URL.Path == "/user/login":
				loginCalls.Add(1)
				status = http.StatusOK
				payload = map[string]any{
					"result": map[string]any{
						"session": map[string]any{
							"id":        "session-new",
							"userId":    "user-1",
							"expiresIn": 3600,
						},
					},
				}
			case r.URL.Path == "/device_detail":
				detailCalls.Add(1)
				status = http.StatusNotFound
				payload = map[string]any{
					"error": map[string]any{
						"code": 97,
						"msg":  "App is out of date, please upgrade",
					},
				}
			case r.URL.Path == "/ctw3/deviceData":
				deviceDataCalls.Add(1)
				if r.Method != http.MethodPost {
					t.Fatalf("expected typed device data fallback to use POST, got %s", r.Method)
				}
				if got := r.URL.Query().Get("id"); got != "118197" {
					t.Fatalf("expected fallback id query param, got %q", got)
				}
				if got := r.Header.Get("X-Session"); got != "session-new" {
					t.Fatalf("expected refreshed session on fallback request, got %q", got)
				}
				status = http.StatusOK
				payload = map[string]any{
					"result": map[string]any{
						"mac":              "AA:BB:CC:DD:EE:FF",
						"filterPercent":    88,
						"waterPumpRunTime": 12,
						"status": map[string]any{
							"powerStatus": 1,
							"runStatus":   1,
						},
					},
				}
			default:
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
			}
			body, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("marshal test payload: %v", err)
			}
			return &http.Response{
				StatusCode: status,
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body:    io.NopCloser(bytes.NewReader(body)),
				Request: r,
			}, nil
		}),
	}

	compat := defaultCompatConfig()
	compat.ChinaBaseURL = "https://petkit.test/"
	apiClient := NewClient(AccountConfig{
		Username: "user@example.com",
		Password: "secret",
		Region:   "cn",
		Timezone: "Asia/Shanghai",
	}, compat)
	apiClient.httpClient = client
	apiClient.baseURL = "https://petkit.test/"
	apiClient.session = &sessionInfo{
		ID:        "session-old",
		UserID:    "user-1",
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(time.Hour),
		ExpiresIn: 3600,
	}

	detail, err := apiClient.loadDeviceDetail(context.Background(), petkitDeviceInfo{
		DeviceID:   118197,
		DeviceType: "ctw3",
	})
	if err != nil {
		t.Fatalf("loadDeviceDetail returned error: %v", err)
	}
	if got := stringFromAny(detail["mac"], ""); got != "AA:BB:CC:DD:EE:FF" {
		t.Fatalf("unexpected fallback detail mac: %q", got)
	}
	if got := intFromAny(detail["filterPercent"], 0); got != 88 {
		t.Fatalf("unexpected fallback filter percent: %d", got)
	}
	if detailCalls.Load() != 2 {
		t.Fatalf("expected 2 device_detail attempts (before/after re-login), got %d", detailCalls.Load())
	}
	if loginCalls.Load() != 1 {
		t.Fatalf("expected 1 login retry, got %d", loginCalls.Load())
	}
	if deviceDataCalls.Load() != 1 {
		t.Fatalf("expected 1 typed deviceData fallback, got %d", deviceDataCalls.Load())
	}
}

func TestNewPetkitRequestErrorIncludesRequestContextAndRedactsSecrets(t *testing.T) {
	err := newPetkitRequestError(
		http.StatusNotFound,
		http.MethodPost,
		"https://api.petkit.cn/6/user/login?username=user@example.com&region=us",
		url.Values{
			"username": {"user@example.com"},
			"password": {"secret"},
			"region":   {"us"},
			"client":   {"client-payload"},
		},
		[]byte(`{"error":{"code":97,"msg":"App is out of date, please upgrade"}}`),
	)
	message := err.Error()
	if !strings.Contains(message, "method=POST") {
		t.Fatalf("expected method in error, got %q", message)
	}
	if !strings.Contains(message, "url=https://api.petkit.cn/6/user/login?region=us&username=%5BREDACTED%5D") {
		t.Fatalf("expected sanitized url in error, got %q", message)
	}
	if !strings.Contains(message, "form=client=client-payload&password=%5BREDACTED%5D&region=us&username=%5BREDACTED%5D") {
		t.Fatalf("expected sanitized form in error, got %q", message)
	}
	if !strings.Contains(message, `code=97`) || !strings.Contains(message, `message="App is out of date, please upgrade"`) {
		t.Fatalf("expected parsed upstream error details, got %q", message)
	}
	if strings.Contains(message, "secret") || strings.Contains(message, "user@example.com") {
		t.Fatalf("expected secrets to be redacted, got %q", message)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}
