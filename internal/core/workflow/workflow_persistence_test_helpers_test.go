package workflow

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

func newWorkflowPersistenceTestService(t *testing.T) (*Service, *sqlitestore.Store) {
	t.Helper()
	store, err := sqlitestore.New(filepath.Join(t.TempDir(), "workflow.db"))
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

func saveWorkflowTestSettings(t *testing.T, store *sqlitestore.Store, settings models.AgentSettings) {
	t.Helper()
	now := time.Now().UTC()
	if err := writeWorkflowTestDoc(store, workflowSettingsLLMDocumentKey, "agent.settings.llm", workflowSettingsLLMDocument{
		RuntimeMode:          settings.RuntimeMode,
		DefaultLLMProviderID: settings.DefaultLLMProviderID,
		LLMProviders:         settings.LLMProviders,
		UpdatedAt:            now,
	}, now); err != nil {
		t.Fatalf("write llm settings: %v", err)
	}
	if err := writeWorkflowTestDoc(store, workflowSettingsSearchKey, "agent.settings.search", workflowSettingsSearchDocument{
		SearchEngines: settings.SearchEngines,
		UpdatedAt:     now,
	}, now); err != nil {
		t.Fatalf("write search settings: %v", err)
	}
}

func writeWorkflowTestDoc(store *sqlitestore.Store, key string, domain string, payload any, updatedAt time.Time) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return store.UpsertAgentDocument(context.Background(), models.AgentDocument{
		Key:       key,
		Domain:    domain,
		Payload:   raw,
		UpdatedAt: updatedAt,
	})
}
