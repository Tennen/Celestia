package agent

import (
	"context"
	"testing"

	"github.com/chentianyu/celestia/internal/models"
)

func TestRunSearchRecordsRecentQueries(t *testing.T) {
	ctx := context.Background()
	svc, _ := newAgentPersistenceTestService(t)
	snapshot, err := svc.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	settings := snapshot.Settings
	settings.SearchEngines = []models.AgentSearchProvider{{
		ID:      "test-engine",
		Name:    "Test Engine",
		Type:    "unsupported",
		Enabled: true,
	}}
	if _, err := svc.SaveSettings(ctx, settings); err != nil {
		t.Fatalf("SaveSettings() error = %v", err)
	}

	result, err := svc.RunSearch(ctx, models.AgentSearchRequest{
		EngineSelector: "test-engine",
		MaxItems:       3,
		LogContext:     "test",
		Plans:          []models.AgentSearchPlan{{Label: "manual", Query: "celestia agent search log", Recency: "week"}},
	})
	if err != nil {
		t.Fatalf("RunSearch() error = %v", err)
	}
	if len(result.Errors) == 0 {
		t.Fatalf("RunSearch() result errors = nil, want unsupported provider error")
	}

	snapshot, err = svc.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if len(snapshot.Search.RecentQueries) != 1 {
		t.Fatalf("recent query count = %d, want 1", len(snapshot.Search.RecentQueries))
	}
	log := snapshot.Search.RecentQueries[0]
	if log.Query != "celestia agent search log" || log.EngineID != "test-engine" || log.Status != "degraded" {
		t.Fatalf("unexpected search log: %+v", log)
	}
}

func TestRunSearchKeepsLatestFiftyQueries(t *testing.T) {
	ctx := context.Background()
	svc, _ := newAgentPersistenceTestService(t)
	snapshot, err := svc.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	settings := snapshot.Settings
	settings.SearchEngines = []models.AgentSearchProvider{{ID: "test-engine", Type: "unsupported", Enabled: true}}
	if _, err := svc.SaveSettings(ctx, settings); err != nil {
		t.Fatalf("SaveSettings() error = %v", err)
	}

	for i := 0; i < 55; i++ {
		_, err := svc.RunSearch(ctx, models.AgentSearchRequest{
			EngineSelector: "test-engine",
			Plans:          []models.AgentSearchPlan{{Label: "manual", Query: "query-" + intString(i)}},
		})
		if err != nil {
			t.Fatalf("RunSearch(%d) error = %v", i, err)
		}
	}
	snapshot, err = svc.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if len(snapshot.Search.RecentQueries) != 50 {
		t.Fatalf("recent query count = %d, want 50", len(snapshot.Search.RecentQueries))
	}
	if snapshot.Search.RecentQueries[0].Query != "query-54" {
		t.Fatalf("newest query = %q, want query-54", snapshot.Search.RecentQueries[0].Query)
	}
	if snapshot.Search.RecentQueries[49].Query != "query-5" {
		t.Fatalf("oldest retained query = %q, want query-5", snapshot.Search.RecentQueries[49].Query)
	}
}
