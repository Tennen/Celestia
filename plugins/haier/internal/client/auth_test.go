package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"accessToken":  "test-access-token",
			"refreshToken": "test-refresh-token-new",
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
			"refreshToken": "some-token",
		})
	}))
	defer srv.Close()

	c := newTestUWSClient(AccountConfig{ClientID: "client1", RefreshToken: "old-refresh"}, srv)
	if err := c.Authenticate(context.Background()); err == nil {
		t.Fatal("expected error for missing accessToken, got nil")
	}
}

// TestAuthenticate_Property2_TokenParseRoundTrip is Property 2:
// For any valid refresh response JSON with accessToken and refreshToken fields,
// parsing extracts both fields correctly.
// Feature: haier-uws-platform-migration, Property 2: token 刷新响应解析往返
func TestAuthenticate_Property2_TokenParseRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		accessToken := rapid.StringMatching(`[a-zA-Z0-9_-]{8,32}`).Draw(t, "accessToken")
		refreshToken := rapid.StringMatching(`[a-zA-Z0-9_-]{8,32}`).Draw(t, "refreshToken")

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"accessToken":  accessToken,
				"refreshToken": refreshToken,
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
