package automation

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/chentianyu/celestia/internal/core/audit"
	"github.com/chentianyu/celestia/internal/core/eventbus"
	"github.com/chentianyu/celestia/internal/core/policy"
	"github.com/chentianyu/celestia/internal/core/registry"
	"github.com/chentianyu/celestia/internal/core/state"
	"github.com/chentianyu/celestia/internal/models"
	sqlitestore "github.com/chentianyu/celestia/internal/storage/sqlite"
)

type countingAutomationStore struct {
	*sqlitestore.Store
	mu                  sync.Mutex
	listAutomationsCall int
}

func (s *countingAutomationStore) ListAutomations(ctx context.Context) ([]models.Automation, error) {
	s.mu.Lock()
	s.listAutomationsCall++
	s.mu.Unlock()
	return s.Store.ListAutomations(ctx)
}

func (s *countingAutomationStore) listAutomationCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listAutomationsCall
}

func newCountingAutomationTestService(t *testing.T) (*Service, *countingAutomationStore) {
	t.Helper()
	ctx := context.Background()
	baseStore, err := sqlitestore.New(filepath.Join(t.TempDir(), "celestia.db"))
	if err != nil {
		t.Fatalf("sqlite.New() error = %v", err)
	}
	t.Cleanup(func() {
		_ = baseStore.Close()
	})
	if err := baseStore.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}

	store := &countingAutomationStore{Store: baseStore}
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

func TestServiceLoadsEnabledAutomationIndexOnStart(t *testing.T) {
	ctx := context.Background()
	baseStore, err := sqlitestore.New(filepath.Join(t.TempDir(), "celestia.db"))
	if err != nil {
		t.Fatalf("sqlite.New() error = %v", err)
	}
	t.Cleanup(func() {
		_ = baseStore.Close()
	})
	if err := baseStore.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}

	store := &countingAutomationStore{Store: baseStore}
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
	if err := store.UpsertAutomation(ctx, models.Automation{
		ID:             "automation-1",
		Name:           "Washer done",
		Enabled:        true,
		ConditionLogic: models.AutomationLogicAll,
		Conditions: []models.AutomationCondition{
			{
				Type:     models.AutomationConditionTypeStateChanged,
				DeviceID: "haier:washer:test",
				StateKey: "phase",
				From:     &models.AutomationStateMatch{Operator: models.AutomationMatchAny},
				To:       &models.AutomationStateMatch{Operator: models.AutomationMatchEquals, Value: "ready"},
			},
		},
		Actions: []models.AutomationAction{
			{
				DeviceID: "xiaomi:speaker:test",
				Action:   "push_voice_message",
			},
		},
	}); err != nil {
		t.Fatalf("UpsertAutomation() error = %v", err)
	}

	svc := New(store, eventbus.New(), registrySvc, state.New(store), policy.New(), audit.New(store), nil)
	t.Cleanup(func() {
		svc.Close()
	})

	indexed := svc.indexedAutomationsForDevice("haier:washer:test")
	if len(indexed) != 1 || indexed[0].ID != "automation-1" {
		t.Fatalf("indexedAutomationsForDevice() = %#v, want automation-1", indexed)
	}
	if store.listAutomationCalls() != 1 {
		t.Fatalf("ListAutomations() calls = %d, want 1 startup load", store.listAutomationCalls())
	}
}

func TestHandleStateChangeUsesDeviceIndex(t *testing.T) {
	ctx := context.Background()
	svc, store := newCountingAutomationTestService(t)

	if _, err := svc.Save(ctx, models.Automation{
		Name:    "Washer done",
		Enabled: true,
		Conditions: []models.AutomationCondition{
			{
				Type:     models.AutomationConditionTypeStateChanged,
				DeviceID: "haier:washer:test",
				StateKey: "phase",
				From:     &models.AutomationStateMatch{Operator: models.AutomationMatchAny},
				To:       &models.AutomationStateMatch{Operator: models.AutomationMatchEquals, Value: "ready"},
			},
			{
				Type:     models.AutomationConditionTypeCurrentState,
				DeviceID: "haier:washer:test",
				StateKey: "machine_status",
				Match:    &models.AutomationStateMatch{Operator: models.AutomationMatchEquals, Value: "idle"},
			},
		},
		Actions: []models.AutomationAction{
			{
				DeviceID: "xiaomi:speaker:test",
				Action:   "push_voice_message",
			},
		},
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := svc.state.Upsert(ctx, []models.DeviceStateSnapshot{{
		DeviceID: "haier:washer:test",
		PluginID: "haier",
		State: map[string]any{
			"machine_status": "running",
		},
	}}); err != nil {
		t.Fatalf("state.Upsert() error = %v", err)
	}

	initialCalls := store.listAutomationCalls()
	svc.handleStateChange(models.Event{
		ID:       "event-1",
		Type:     models.EventDeviceStateChanged,
		PluginID: "haier",
		DeviceID: "haier:washer:test",
		Payload: map[string]any{
			"previous_state": map[string]any{"phase": "rinse"},
			"state":          map[string]any{"phase": "ready"},
		},
	})

	if store.listAutomationCalls() != initialCalls {
		t.Fatalf("handleStateChange() should use in-memory index, list calls %d -> %d", initialCalls, store.listAutomationCalls())
	}
}
