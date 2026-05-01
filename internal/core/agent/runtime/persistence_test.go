package runtime

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/chentianyu/celestia/internal/core/eventbus"
	"github.com/chentianyu/celestia/internal/core/project/touchpoint"
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
	svc := New(store, eventbus.New())
	t.Cleanup(func() {
		svc.Close()
	})
	return svc, store
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
	settings.AgentProviders = []models.AgentProvider{{
		ID:              "codex-main",
		Name:            "Codex",
		Type:            "codex",
		Model:           "gpt-5.5",
		ReasoningEffort: "high",
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
	touchpoints := touchpoint.New(svc, svc)
	if _, err := touchpoints.SaveWeComUsers(ctx, models.AgentPushSnapshot{Users: []models.AgentPushUser{{
		Name:      "Alice",
		WeComUser: "alice",
		Enabled:   true,
	}}}); err != nil {
		t.Fatalf("SaveWeComUsers() error = %v", err)
	}

	assertNoAgentDocument(t, store, agentLegacyStateDocumentKey)

	var llmDoc agentSettingsLLMDocument
	readAgentDocument(t, store, agentSettingsLLMDocumentKey, &llmDoc)
	if len(llmDoc.LLMProviders) != 1 || llmDoc.LLMProviders[0].Type != "openai" {
		t.Fatalf("LLM settings doc was not persisted by business domain: %+v", llmDoc)
	}

	var agentProviderDoc agentSettingsAgentProvidersDocument
	readAgentDocument(t, store, agentSettingsAgentProvidersDocumentKey, &agentProviderDoc)
	if len(agentProviderDoc.AgentProviders) != 1 || agentProviderDoc.AgentProviders[0].Type != "codex" {
		t.Fatalf("agent provider settings doc was not persisted by business domain: %+v", agentProviderDoc)
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

func TestAgentPersistenceIgnoresLegacyWorkflowDocuments(t *testing.T) {
	ctx := context.Background()
	svc, store := newAgentPersistenceTestService(t)
	currentAt := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	legacyAt := currentAt.Add(time.Minute)
	currentDefinitions := agentWorkflowDefinitionsDocument{
		ActiveWorkflowID: "workflow-current",
		Workflows: []models.AgentWorkflow{{
			ID:   "workflow-current",
			Name: "Current Workflow",
			Nodes: []models.AgentWorkflowNode{{
				ID:   "rss-main",
				Type: "rss_sources",
				Data: map[string]any{"sources": []models.AgentWorkflowSource{{ID: "feed-main", FeedURL: "https://rss.test/feed"}}},
			}},
		}},
		UpdatedAt: currentAt,
	}
	legacyDefinitionsRaw := []byte(`{"active_profile_id":"legacy-profile","profiles":[{"id":"legacy-profile","name":"Legacy Workflow"}]}`)
	currentRuns := agentWorkflowRunsDocument{
		SourceStates: []models.AgentWorkflowSourceState{{
			WorkflowID:       "workflow-current",
			NodeID:           "rss-main",
			SourceID:         "feed-main",
			FeedURL:          "https://rss.test/feed",
			LastRequestedAt:  currentAt,
			LastResponseBody: "<rss/>",
			UpdatedAt:        currentAt,
		}},
		UpdatedAt: currentAt,
	}
	legacyRunsRaw := []byte(`{"source_states":[],"timer_states":[{"workflow_id":"legacy","node_id":"timer-legacy"}]}`)
	writeAgentDoc(t, store, agentWorkflowDefinitionsDocumentKey, "workflow", currentDefinitions, currentAt)
	writeAgentRawDoc(t, store, "agent/topic/profiles", "workflow", legacyDefinitionsRaw, legacyAt)
	writeAgentDoc(t, store, agentWorkflowRunsDocumentKey, "workflow", currentRuns, currentAt)
	writeAgentRawDoc(t, store, "agent/topic/runs", "workflow", legacyRunsRaw, legacyAt)

	snapshot, err := svc.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if snapshot.Workflow.ActiveWorkflowID != "workflow-current" {
		t.Fatalf("active workflow = %q, want workflow-current", snapshot.Workflow.ActiveWorkflowID)
	}
	if len(snapshot.Workflow.Workflows) != 1 || snapshot.Workflow.Workflows[0].ID != "workflow-current" {
		t.Fatalf("workflows = %+v, want only current workflow", snapshot.Workflow.Workflows)
	}
	if len(snapshot.Workflow.SourceStates) != 1 || snapshot.Workflow.SourceStates[0].WorkflowID != "workflow-current" {
		t.Fatalf("source states = %+v, want current workflow source state", snapshot.Workflow.SourceStates)
	}
	if len(snapshot.Workflow.TimerStates) != 0 {
		t.Fatalf("timer states = %+v, want legacy timer states ignored", snapshot.Workflow.TimerStates)
	}
}

func writeAgentDoc(t *testing.T, store *sqlitestore.Store, key string, domain string, payload any, updatedAt time.Time) {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal(%q) error = %v", key, err)
	}
	writeAgentRawDoc(t, store, key, domain, raw, updatedAt)
}

func writeAgentRawDoc(t *testing.T, store *sqlitestore.Store, key string, domain string, raw []byte, updatedAt time.Time) {
	t.Helper()
	if err := store.UpsertAgentDocument(context.Background(), models.AgentDocument{
		Key:       key,
		Domain:    domain,
		Payload:   raw,
		UpdatedAt: updatedAt,
	}); err != nil {
		t.Fatalf("UpsertAgentDocument(%q) error = %v", key, err)
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
