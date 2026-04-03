package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// newTestUWSClient creates a UWSClient whose HTTP requests are routed through
// the provided httptest.Server by rewriting the host.
func newTestUWSClient(cfg AccountConfig, srv *httptest.Server) *UWSClient {
	return &UWSClient{
		cfg:  cfg,
		auth: uwsAuthState{RefreshToken: cfg.RefreshToken},
		client: &http.Client{
			Transport: &rewriteHostTransport{target: srv.URL},
		},
	}
}

// rewriteHostTransport rewrites every request's host to the test server.
type rewriteHostTransport struct {
	target string // e.g. "http://127.0.0.1:PORT"
}

func (t *rewriteHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = "http"
	clone.URL.Host = strings.TrimPrefix(t.target, "http://")
	return http.DefaultTransport.RoundTrip(clone)
}

func TestAuthenticate_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawBody, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if !strings.Contains(string(rawBody), `"refreshToken":"old-refresh"`) {
			t.Fatalf("expected refreshToken in body, got %s", string(rawBody))
		}
		if strings.Contains(string(rawBody), `"clientId"`) {
			t.Fatalf("did not expect clientId in refresh body, got %s", string(rawBody))
		}
		if got := r.Header.Get("appId"); got != uwsAppID {
			t.Fatalf("expected appId=%s, got %q", uwsAppID, got)
		}
		if got := r.Header.Get("appKey"); got != uwsAppKey {
			t.Fatalf("expected appKey=%s, got %q", uwsAppKey, got)
		}
		if got := r.Header.Get("clientId"); got != "client1" {
			t.Fatalf("expected clientId=client1, got %q", got)
		}
		if got := r.Header.Get("timezone"); got != uwsTimezone {
			t.Fatalf("expected timezone=%s, got %q", uwsTimezone, got)
		}
		if got := r.Header.Get("language"); got != uwsLanguage {
			t.Fatalf("expected language=%s, got %q", uwsLanguage, got)
		}
		if got := r.Header.Get("sequenceId"); len(got) != 20 {
			t.Fatalf("expected 20-digit sequenceId, got %q", got)
		}
		if ok, _ := regexp.MatchString(`^[0-9]{20}$`, r.Header.Get("sequenceId")); !ok {
			t.Fatalf("sequenceId should be numeric, got %q", r.Header.Get("sequenceId"))
		}
		timestamp := r.Header.Get("timestamp")
		expectedSign := Sign("/api-gw/oauthserver/account/v1/refreshToken", string(rawBody), timestamp)
		if got := r.Header.Get("sign"); got != expectedSign {
			t.Fatalf("expected sign=%s, got %q", expectedSign, got)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"retCode": "00000",
			"data": map[string]any{
				"tokenInfo": map[string]any{
					"accountToken": "test-access-token",
					"refreshToken": "test-refresh-token-new",
					"expiresIn":    7200,
				},
			},
		})
	}))
	defer srv.Close()

	c := newTestUWSClient(AccountConfig{ClientID: "client1", RefreshToken: "old-refresh"}, srv)
	if err := c.Authenticate(context.Background()); err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}
	if c.auth.AccessToken != "test-access-token" {
		t.Errorf("expected accessToken=test-access-token, got %q", c.auth.AccessToken)
	}
	if c.auth.RefreshToken != "test-refresh-token-new" {
		t.Errorf("expected refreshToken=test-refresh-token-new, got %q", c.auth.RefreshToken)
	}
}

func TestAuthenticate_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := newTestUWSClient(AccountConfig{ClientID: "client1", RefreshToken: "bad-token"}, srv)
	if err := c.Authenticate(context.Background()); err == nil {
		t.Fatal("expected error for HTTP 401, got nil")
	}
}

func TestAuthenticate_MissingAccessToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"retCode": "00000",
			"data": map[string]any{
				"tokenInfo": map[string]any{
					"refreshToken": "some-token",
				},
			},
		})
	}))
	defer srv.Close()

	c := newTestUWSClient(AccountConfig{ClientID: "client1", RefreshToken: "old-refresh"}, srv)
	if err := c.Authenticate(context.Background()); err == nil {
		t.Fatal("expected error for missing accessToken, got nil")
	}
}

// TestAuthenticate_Property2_TokenParseRoundTrip is Property 2:
// For any valid HAier refresh response JSON with accountToken and refreshToken fields,
// parsing extracts both fields correctly.
// Feature: haier-uws-platform-migration, Property 2: token 刷新响应解析往返
func TestAuthenticate_Property2_TokenParseRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		accessToken := rapid.StringMatching(`[a-zA-Z0-9_-]{8,32}`).Draw(t, "accessToken")
		refreshToken := rapid.StringMatching(`[a-zA-Z0-9_-]{8,32}`).Draw(t, "refreshToken")

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"retCode": "00000",
				"data": map[string]any{
					"tokenInfo": map[string]any{
						"accountToken": accessToken,
						"refreshToken": refreshToken,
						"expiresIn":    7200,
					},
				},
			})
		}))
		defer srv.Close()

		c := newTestUWSClient(AccountConfig{ClientID: "client1", RefreshToken: "seed"}, srv)
		if err := c.Authenticate(context.Background()); err != nil {
			t.Fatalf("Authenticate failed: %v", err)
		}
		if c.auth.AccessToken != accessToken {
			t.Fatalf("accessToken mismatch: want %q got %q", accessToken, c.auth.AccessToken)
		}
		if c.auth.RefreshToken != refreshToken {
			t.Fatalf("refreshToken mismatch: want %q got %q", refreshToken, c.auth.RefreshToken)
		}
	})
}
