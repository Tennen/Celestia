package automation

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/chentianyu/celestia/internal/core/audit"
	"github.com/chentianyu/celestia/internal/core/eventbus"
	"github.com/chentianyu/celestia/internal/core/policy"
	"github.com/chentianyu/celestia/internal/core/registry"
	"github.com/chentianyu/celestia/internal/core/state"
	"github.com/chentianyu/celestia/internal/models"
	sqlitestore "github.com/chentianyu/celestia/internal/storage/sqlite"
)

func newAutomationTestService(t *testing.T) (*Service, *sqlitestore.Store) {
	t.Helper()
	ctx := context.Background()
	store, err := sqlitestore.New(filepath.Join(t.TempDir(), "celestia.db"))
	if err != nil {
		t.Fatalf("sqlite.New() error = %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}
	registrySvc := registry.New(store)
	if err := registrySvc.Upsert(ctx, []models.Device{
		{
			ID:             "haier:washer:test",
			PluginID:       "haier",
			VendorDeviceID: "washer-test",
			Kind:           models.DeviceKindWasher,
			Name:           "Washer",
		},
		{
			ID:             "xiaomi:speaker:test",
			PluginID:       "xiaomi",
			VendorDeviceID: "speaker-test",
			Kind:           models.DeviceKindSpeaker,
			Name:           "Speaker",
		},
	}); err != nil {
		t.Fatalf("registry.Upsert() error = %v", err)
	}
	svc := New(store, eventbus.New(), registrySvc, state.New(store), policy.New(), audit.New(store), nil)
	t.Cleanup(func() {
		svc.Close()
	})
	return svc, store
}

func TestServiceSavePersistsNormalizedAutomation(t *testing.T) {
	ctx := context.Background()
	svc, store := newAutomationTestService(t)

	saved, err := svc.Save(ctx, models.Automation{
		Name:    "Washer done",
		Enabled: true,
		Trigger: models.AutomationTrigger{
			DeviceID: "haier:washer:test",
			StateKey: "phase",
			From:     models.AutomationStateMatch{Operator: models.AutomationMatchNotEquals, Value: "ready"},
			To:       models.AutomationStateMatch{Value: "ready"},
		},
		Actions: []models.AutomationAction{
			{
				DeviceID: "xiaomi:speaker:test",
				Action:   "push_voice_message",
				Params:   map[string]any{"message": "done"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if saved.ID == "" {
		t.Fatal("Save() should assign an automation id")
	}
	if saved.ConditionLogic != models.AutomationLogicAll {
		t.Fatalf("ConditionLogic = %q, want %q", saved.ConditionLogic, models.AutomationLogicAll)
	}
	if saved.Trigger.To.Operator != models.AutomationMatchEquals {
		t.Fatalf("Trigger.To.Operator = %q, want %q", saved.Trigger.To.Operator, models.AutomationMatchEquals)
	}
	if saved.LastRunStatus != models.AutomationRunStatusIdle {
		t.Fatalf("LastRunStatus = %q, want %q", saved.LastRunStatus, models.AutomationRunStatusIdle)
	}

	items, err := store.ListAutomations(ctx)
	if err != nil {
		t.Fatalf("ListAutomations() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("ListAutomations len = %d, want 1", len(items))
	}
	if items[0].Name != "Washer done" {
		t.Fatalf("persisted Name = %q, want Washer done", items[0].Name)
	}
}

func TestMatchesTriggerRequiresStateKeyChange(t *testing.T) {
	trigger := models.AutomationTrigger{
		DeviceID: "haier:washer:test",
		StateKey: "phase",
		From:     models.AutomationStateMatch{Operator: models.AutomationMatchNotEquals, Value: "ready"},
		To:       models.AutomationStateMatch{Operator: models.AutomationMatchEquals, Value: "ready"},
	}
	if !matchesTrigger(trigger, "haier:washer:test", map[string]any{"phase": "rinse"}, map[string]any{"phase": "ready"}) {
		t.Fatal("matchesTrigger() should accept non-ready -> ready")
	}
	if matchesTrigger(trigger, "haier:washer:test", map[string]any{"phase": "ready"}, map[string]any{"phase": "ready"}) {
		t.Fatal("matchesTrigger() should reject unchanged state values")
	}
}

func TestMatchesTriggerSupportsMultiTargetValues(t *testing.T) {
	trigger := models.AutomationTrigger{
		DeviceID: "haier:washer:test",
		StateKey: "phase",
		From:     models.AutomationStateMatch{Operator: models.AutomationMatchEquals, Value: "D"},
		To:       models.AutomationStateMatch{Operator: models.AutomationMatchIn, Value: []any{"A", "B", "C"}},
	}
	if !matchesTrigger(trigger, "haier:washer:test", map[string]any{"phase": "D"}, map[string]any{"phase": "B"}) {
		t.Fatal("matchesTrigger() should accept D -> B when target matcher is in [A, B, C]")
	}
	if matchesTrigger(trigger, "haier:washer:test", map[string]any{"phase": "D"}, map[string]any{"phase": "E"}) {
		t.Fatal("matchesTrigger() should reject D -> E when target matcher is in [A, B, C]")
	}
}

func TestServiceSaveNormalizesListMatchers(t *testing.T) {
	ctx := context.Background()
	svc, _ := newAutomationTestService(t)

	saved, err := svc.Save(ctx, models.Automation{
		Name:    "Washer phase fan-out",
		Enabled: true,
		Trigger: models.AutomationTrigger{
			DeviceID: "haier:washer:test",
			StateKey: "phase",
			From:     models.AutomationStateMatch{Operator: models.AutomationMatchEquals, Value: "D"},
			To:       models.AutomationStateMatch{Operator: models.AutomationMatchIn, Value: "A"},
		},
		Conditions: []models.AutomationCondition{
			{
				DeviceID: "haier:washer:test",
				StateKey: "phase",
				Match:    models.AutomationStateMatch{Operator: models.AutomationMatchNotIn, Value: []string{"X", "Y"}},
			},
		},
		Actions: []models.AutomationAction{
			{
				DeviceID: "xiaomi:speaker:test",
				Action:   "push_voice_message",
			},
		},
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	toValues, ok := saved.Trigger.To.Value.([]any)
	if !ok || len(toValues) != 1 || toValues[0] != "A" {
		t.Fatalf("Trigger.To.Value should normalize to single-item list, got %#v", saved.Trigger.To.Value)
	}
	conditionValues, ok := saved.Conditions[0].Match.Value.([]any)
	if !ok || len(conditionValues) != 2 {
		t.Fatalf("Condition matcher should preserve list values, got %#v", saved.Conditions[0].Match.Value)
	}
}

func TestMatchesTimeWindowSupportsOvernightRanges(t *testing.T) {
	window := &models.AutomationTimeWindow{Start: "22:00", End: "06:00"}
	if !matchesTimeWindow(time.Date(2026, 4, 3, 23, 15, 0, 0, time.Local), window) {
		t.Fatal("matchesTimeWindow() should match 23:15 for overnight range")
	}
	if !matchesTimeWindow(time.Date(2026, 4, 4, 1, 15, 0, 0, time.Local), window) {
		t.Fatal("matchesTimeWindow() should match 01:15 for overnight range")
	}
	if matchesTimeWindow(time.Date(2026, 4, 4, 12, 0, 0, 0, time.Local), window) {
		t.Fatal("matchesTimeWindow() should reject noon for overnight range")
	}
}
