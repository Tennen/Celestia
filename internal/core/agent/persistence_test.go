package agent

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/chentianyu/celestia/internal/core/eventbus"
	"github.com/chentianyu/celestia/internal/models"
	sqlitestore "github.com/chentianyu/celestia/internal/storage/sqlite"
)

func newAgentPersistenceTestService(t *testing.T) (*Service, *sqlitestore.Store) {
	t.Helper()
	store, err := sqlitestore.New(filepath.Join(t.TempDir(), "agent.db"))
	if err != nil {
		t.Fatalf("sqlite.New() error = %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}
	return New(store, eventbus.New()), store
}

func TestAgentPersistenceWritesBusinessDocuments(t *testing.T) {
	ctx := context.Background()
	svc, store := newAgentPersistenceTestService(t)

	snapshot, err := svc.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	settings := snapshot.Settings
	settings.LLMProviders = []models.AgentLLMProvider{{
		ID:    "openai-default",
		Name:  "OpenAI",
		Type:  "openai",
		Model: "gpt-5.4",
	}}
	settings.SearchEngines = []models.AgentSearchProvider{{
		ID:      "bing-main",
		Name:    "Bing",
		Type:    "bing",
		Enabled: true,
	}}
	if _, err := svc.SaveSettings(ctx, settings); err != nil {
		t.Fatalf("SaveSettings() error = %v", err)
	}
	if _, err := svc.SaveDirectInput(ctx, models.AgentDirectInputConfig{Rules: []models.AgentDirectInputRule{{
		Name:       "Apple Notes",
		Pattern:    "note",
		TargetText: "/apple-note append inbox",
		Enabled:    true,
	}}}); err != nil {
		t.Fatalf("SaveDirectInput() error = %v", err)
	}
	if _, err := svc.SavePush(ctx, models.AgentPushSnapshot{Users: []models.AgentPushUser{{
		Name:      "Alice",
		WeComUser: "alice",
		Enabled:   true,
	}}}); err != nil {
		t.Fatalf("SavePush() error = %v", err)
	}

	assertNoAgentDocument(t, store, agentLegacyStateDocumentKey)

	var llmDoc agentSettingsLLMDocument
	readAgentDocument(t, store, agentSettingsLLMDocumentKey, &llmDoc)
	if len(llmDoc.LLMProviders) != 1 || llmDoc.LLMProviders[0].Type != "openai" {
		t.Fatalf("LLM settings doc was not persisted by business domain: %+v", llmDoc)
	}

	var searchDoc agentSettingsSearchDocument
	readAgentDocument(t, store, agentSettingsSearchDocumentKey, &searchDoc)
	if len(searchDoc.SearchEngines) != 1 || searchDoc.SearchEngines[0].Type != "bing" {
		t.Fatalf("search settings doc was not persisted by business domain: %+v", searchDoc)
	}
	assertDocumentLacksKey(t, store, agentSettingsSearchDocumentKey, "llm_providers")

	var directDoc models.AgentDirectInputConfig
	readAgentDocument(t, store, agentDirectInputDocumentKey, &directDoc)
	if len(directDoc.Rules) != 1 || directDoc.Rules[0].ID == "" {
		t.Fatalf("direct input doc was not normalized and persisted: %+v", directDoc)
	}
	assertDocumentLacksKey(t, store, agentDirectInputDocumentKey, "settings")

	var usersDoc agentWeComUsersDocument
	readAgentDocument(t, store, agentWeComUsersDocumentKey, &usersDoc)
	if len(usersDoc.Users) != 1 || usersDoc.Users[0].WeComUser != "alice" {
		t.Fatalf("wecom user doc was not persisted by business domain: %+v", usersDoc)
	}
}

func TestAgentPersistenceMigratesLegacySnapshotToSplitDocuments(t *testing.T) {
	ctx := context.Background()
	svc, store := newAgentPersistenceTestService(t)
	legacyAt := time.Date(2026, 4, 23, 10, 0, 0, 0, time.UTC)
	legacy := normalizeSnapshot(defaultSnapshot())
	legacy.DirectInput = models.AgentDirectInputConfig{
		Version: 1,
		Rules: []models.AgentDirectInputRule{{
			ID:         "legacy-rule",
			Name:       "Legacy note",
			Pattern:    "legacy note",
			TargetText: "/apple-note append legacy",
			Enabled:    true,
		}},
		UpdatedAt: legacyAt,
	}
	legacy.Push = models.AgentPushSnapshot{
		Users: []models.AgentPushUser{{
			ID:        "legacy-user",
			Name:      "Legacy User",
			WeComUser: "legacy",
			Enabled:   true,
			UpdatedAt: legacyAt,
		}},
		UpdatedAt: legacyAt,
	}
	legacy.Market = models.AgentMarketSnapshot{
		Portfolio: models.AgentMarketPortfolio{Funds: []models.AgentMarketHolding{{
			Code:     "510300",
			Name:     "CSI 300 ETF",
			Quantity: 10,
		}}},
		UpdatedAt: legacyAt,
	}
	legacy.UpdatedAt = legacyAt
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if err := store.UpsertAgentDocument(ctx, models.AgentDocument{
		Key:       agentLegacyStateDocumentKey,
		Domain:    "agent",
		Payload:   raw,
		UpdatedAt: legacyAt,
	}); err != nil {
		t.Fatalf("UpsertAgentDocument() error = %v", err)
	}

	got, err := svc.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if len(got.DirectInput.Rules) != 1 || got.DirectInput.Rules[0].TargetText != "/apple-note append legacy" {
		t.Fatalf("legacy direct input was not migrated: %+v", got.DirectInput)
	}
	if len(got.Push.Users) != 1 || got.Push.Users[0].WeComUser != "legacy" {
		t.Fatalf("legacy wecom users were not migrated: %+v", got.Push)
	}
	if len(got.Market.Portfolio.Funds) != 1 || got.Market.Portfolio.Funds[0].Code != "510300" {
		t.Fatalf("legacy market portfolio was not migrated: %+v", got.Market.Portfolio)
	}
	assertNoAgentDocument(t, store, agentLegacyStateDocumentKey)

	var directDoc models.AgentDirectInputConfig
	readAgentDocument(t, store, agentDirectInputDocumentKey, &directDoc)
	if len(directDoc.Rules) != 1 || directDoc.Rules[0].ID != "legacy-rule" {
		t.Fatalf("direct input split doc was not created from legacy state: %+v", directDoc)
	}
	var portfolioDoc agentMarketPortfolioDocument
	readAgentDocument(t, store, agentMarketPortfolioDocumentKey, &portfolioDoc)
	if len(portfolioDoc.Portfolio.Funds) != 1 || portfolioDoc.Portfolio.Funds[0].Code != "510300" {
		t.Fatalf("market portfolio split doc was not created from legacy state: %+v", portfolioDoc)
	}
}

func readAgentDocument(t *testing.T, store *sqlitestore.Store, key string, target any) {
	t.Helper()
	doc, ok, err := store.GetAgentDocument(context.Background(), key)
	if err != nil {
		t.Fatalf("GetAgentDocument(%q) error = %v", key, err)
	}
	if !ok {
		t.Fatalf("GetAgentDocument(%q) missing", key)
	}
	if err := json.Unmarshal(doc.Payload, target); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v", key, err)
	}
}

func assertNoAgentDocument(t *testing.T, store *sqlitestore.Store, key string) {
	t.Helper()
	if _, ok, err := store.GetAgentDocument(context.Background(), key); err != nil {
		t.Fatalf("GetAgentDocument(%q) error = %v", key, err)
	} else if ok {
		t.Fatalf("GetAgentDocument(%q) should be absent", key)
	}
}

func assertDocumentLacksKey(t *testing.T, store *sqlitestore.Store, key string, forbidden string) {
	t.Helper()
	var payload map[string]json.RawMessage
	readAgentDocument(t, store, key, &payload)
	if _, ok := payload[forbidden]; ok {
		t.Fatalf("document %q unexpectedly contains %q: %+v", key, forbidden, payload)
	}
}
