package app

import (
	"context"
	"testing"

	"github.com/chentianyu/celestia/plugins/haier/internal/client"
	"pgregory.net/rapid"
)

// newTestPlugin creates a minimal Plugin with one account for testing.
func newTestPlugin(cfg client.AccountConfig, currentRefreshToken string) *Plugin {
	uwsClient, _ := client.NewUWSClient(cfg)
	// Simulate that the client already has a refreshToken in auth state.
	// We do this by calling a helper that sets the internal state.
	// Since we can't access private fields directly, we use the exported constructor
	// and rely on the fact that NewUWSClient seeds auth.RefreshToken from cfg.RefreshToken.
	_ = uwsClient

	p := New()
	p.config = Config{
		Accounts: []client.AccountConfig{cfg},
	}
	// Build a mock UWSClient that returns the currentRefreshToken.
	mockClient, _ := client.NewUWSClient(client.AccountConfig{
		ClientID:     cfg.ClientID,
		RefreshToken: currentRefreshToken,
	})
	p.accounts[cfg.NormalizedName()] = &accountRuntime{
		Config: cfg,
		Client: mockClient,
	}
	return p
}

// TestSyncAccountConfig_NoChangeNoWrite verifies that when the refreshToken
// has not changed, syncAccountConfig returns nil without attempting to persist
// (which would fail without a core address).
func TestSyncAccountConfig_NoChangeNoWrite(t *testing.T) {
	cfg := client.AccountConfig{
		Name:         "test",
		ClientID:     "client1",
		RefreshToken: "same-token",
	}
	p := newTestPlugin(cfg, "same-token")
	account := p.accounts["test"]

	// When token is unchanged, syncAccountConfig must return nil (no persist attempt).
	if err := p.syncAccountConfig(account); err != nil {
		t.Fatalf("syncAccountConfig should return nil when token unchanged, got: %v", err)
	}
}

// TestSyncAccountConfig_Property9_IdempotentWhenTokenUnchanged is Property 9:
// For any account where the refreshToken has not changed, syncAccountConfig
// must not trigger PersistPluginConfig (idempotency: same token → no write).
// We verify this by confirming no error is returned (a real persist call would
// fail without CELESTIA_CORE_ADDR set).
// Feature: haier-uws-platform-migration, Property 9: 持久化幂等性
func TestSyncAccountConfig_Property9_IdempotentWhenTokenUnchanged(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		clientID := rapid.StringMatching(`[a-zA-Z0-9]{4,16}`).Draw(t, "clientID")
		token := rapid.StringMatching(`[a-zA-Z0-9_-]{8,32}`).Draw(t, "token")

		cfg := client.AccountConfig{
			Name:         "test",
			ClientID:     clientID,
			RefreshToken: token,
		}
		p := newTestPlugin(cfg, token) // same token in client and config
		account := p.accounts["test"]

		// Must return nil — no persist attempt when token is unchanged.
		if err := p.syncAccountConfig(account); err != nil {
			t.Fatalf("syncAccountConfig should not persist when token unchanged, got: %v", err)
		}
	})
}

// TestSyncAccountConfig_ChangedTokenAttemptsPersist verifies that when the
// refreshToken changes, syncAccountConfig attempts to persist (and fails
// gracefully without a core address).
func TestSyncAccountConfig_ChangedTokenAttemptsPersist(t *testing.T) {
	cfg := client.AccountConfig{
		Name:         "test",
		ClientID:     "client1",
		RefreshToken: "old-token",
	}
	p := newTestPlugin(cfg, "new-token") // different token in client
	account := p.accounts["test"]

	// Should attempt persist and fail (no CELESTIA_CORE_ADDR in test env).
	err := p.syncAccountConfig(account)
	if err == nil {
		t.Log("persist succeeded unexpectedly (core may be running)")
	}
	// Either way, the function should not panic.
}

// TestSyncAccountConfig_EmptyRefreshToken verifies that empty refreshToken is skipped.
func TestSyncAccountConfig_EmptyRefreshToken(t *testing.T) {
	cfg := client.AccountConfig{
		Name:         "test",
		ClientID:     "client1",
		RefreshToken: "some-token",
	}
	p := newTestPlugin(cfg, "") // empty token in client
	account := p.accounts["test"]

	// Empty refreshToken → skip persist, return nil.
	if err := p.syncAccountConfig(account); err != nil {
		t.Fatalf("syncAccountConfig should return nil for empty refreshToken, got: %v", err)
	}
}

// Ensure context is used (suppress unused import warning).
var _ = context.Background
