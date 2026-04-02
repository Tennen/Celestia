package client

import (
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// TestHasCredentials_ValidPair verifies that non-empty clientId + refreshToken returns true.
func TestHasCredentials_ValidPair(t *testing.T) {
	cfg := AccountConfig{ClientID: "abc", RefreshToken: "xyz"}
	if !cfg.HasCredentials() {
		t.Error("expected HasCredentials=true for non-empty clientId and refreshToken")
	}
}

// TestHasCredentials_EmptyClientID verifies that empty clientId returns false.
func TestHasCredentials_EmptyClientID(t *testing.T) {
	cfg := AccountConfig{ClientID: "", RefreshToken: "xyz"}
	if cfg.HasCredentials() {
		t.Error("expected HasCredentials=false for empty clientId")
	}
}

// TestHasCredentials_EmptyRefreshToken verifies that empty refreshToken returns false.
func TestHasCredentials_EmptyRefreshToken(t *testing.T) {
	cfg := AccountConfig{ClientID: "abc", RefreshToken: ""}
	if cfg.HasCredentials() {
		t.Error("expected HasCredentials=false for empty refreshToken")
	}
}

// TestHasCredentials_Property6_BlankCredentialsRejected is Property 6:
// For any AccountConfig where clientId or refreshToken is empty or whitespace-only,
// HasCredentials() must return false.
// Feature: haier-uws-platform-migration, Property 6: 空白凭证拒绝
func TestHasCredentials_Property6_BlankCredentialsRejected(t *testing.T) {
	// Generate whitespace-only strings.
	whitespace := rapid.Custom(func(t *rapid.T) string {
		n := rapid.IntRange(0, 10).Draw(t, "n")
		return strings.Repeat(" ", n)
	})

	rapid.Check(t, func(t *rapid.T) {
		// Case 1: blank clientId, any refreshToken.
		blankClientID := whitespace.Draw(t, "blankClientID")
		anyRefresh := rapid.String().Draw(t, "anyRefresh")
		cfg1 := AccountConfig{ClientID: blankClientID, RefreshToken: anyRefresh}
		if cfg1.HasCredentials() {
			t.Fatalf("HasCredentials should be false when clientId=%q", blankClientID)
		}

		// Case 2: any clientId, blank refreshToken.
		anyClient := rapid.String().Draw(t, "anyClient")
		blankRefresh := whitespace.Draw(t, "blankRefresh")
		cfg2 := AccountConfig{ClientID: anyClient, RefreshToken: blankRefresh}
		if cfg2.HasCredentials() {
			t.Fatalf("HasCredentials should be false when refreshToken=%q", blankRefresh)
		}
	})
}
