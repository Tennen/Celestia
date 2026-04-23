package automation

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
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

type fakeAgentRuntime struct {
	requests []models.AgentConversationRequest
	sends    []struct {
		to   string
		text string
	}
	users map[string]models.AgentPushUser
}

func (f *fakeAgentRuntime) Converse(_ context.Context, req models.AgentConversationRequest) (models.AgentConversation, error) {
	f.requests = append(f.requests, req)
	return models.AgentConversation{Response: "agent output"}, nil
}

func (f *fakeAgentRuntime) ResolveWeComRecipient(_ context.Context, target string) (models.AgentPushUser, error) {
	if f.users == nil {
		return models.AgentPushUser{}, errors.New("wecom target not configured")
	}
	if user, ok := f.users[target]; ok && user.Enabled && user.WeComUser != "" {
		return user, nil
	}
	for _, user := range f.users {
		if user.WeComUser == target && user.Enabled {
			return user, nil
		}
	}
	return models.AgentPushUser{}, errors.New("wecom target not configured")
}

func (f *fakeAgentRuntime) SendWeComText(ctx context.Context, toUser string, text string) error {
	user, err := f.ResolveWeComRecipient(ctx, toUser)
	if err != nil {
		return err
	}
	f.sends = append(f.sends, struct {
		to   string
		text string
	}{to: user.WeComUser, text: text})
	return nil
}

func fakeAgentWithWeComUsers(users ...models.AgentPushUser) *fakeAgentRuntime {
	index := map[string]models.AgentPushUser{}
	for _, user := range users {
		index[user.ID] = user
	}
	return &fakeAgentRuntime{users: index}
}

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
		Conditions: []models.AutomationCondition{
			{
				Type:     models.AutomationConditionTypeStateChanged,
				DeviceID: "haier:washer:test",
				StateKey: "phase",
				From:     &models.AutomationStateMatch{Operator: models.AutomationMatchNotEquals, Value: "ready"},
				To:       &models.AutomationStateMatch{Value: "ready"},
			},
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
	if saved.Conditions[0].To == nil || saved.Conditions[0].To.Operator != models.AutomationMatchEquals {
		t.Fatalf("Conditions[0].To.Operator = %#v, want %q", saved.Conditions[0].To, models.AutomationMatchEquals)
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

func TestMatchesEventConditionRequiresStateKeyChange(t *testing.T) {
	condition := models.AutomationCondition{
		Type:     models.AutomationConditionTypeStateChanged,
		DeviceID: "haier:washer:test",
		StateKey: "phase",
		From:     &models.AutomationStateMatch{Operator: models.AutomationMatchNotEquals, Value: "ready"},
		To:       &models.AutomationStateMatch{Operator: models.AutomationMatchEquals, Value: "ready"},
	}
	if !matchesStateChangedCondition(condition, "haier:washer:test", map[string]any{"phase": "rinse"}, map[string]any{"phase": "ready"}) {
		t.Fatal("matchesStateChangedCondition() should accept non-ready -> ready")
	}
	if matchesStateChangedCondition(condition, "haier:washer:test", map[string]any{"phase": "ready"}, map[string]any{"phase": "ready"}) {
		t.Fatal("matchesStateChangedCondition() should reject unchanged state values")
	}
}

func TestMatchesEventConditionSupportsMultiTargetValues(t *testing.T) {
	condition := models.AutomationCondition{
		Type:     models.AutomationConditionTypeStateChanged,
		DeviceID: "haier:washer:test",
		StateKey: "phase",
		From:     &models.AutomationStateMatch{Operator: models.AutomationMatchEquals, Value: "D"},
		To:       &models.AutomationStateMatch{Operator: models.AutomationMatchIn, Value: []any{"A", "B", "C"}},
	}
	if !matchesStateChangedCondition(condition, "haier:washer:test", map[string]any{"phase": "D"}, map[string]any{"phase": "B"}) {
		t.Fatal("matchesStateChangedCondition() should accept D -> B when target matcher is in [A, B, C]")
	}
	if matchesStateChangedCondition(condition, "haier:washer:test", map[string]any{"phase": "D"}, map[string]any{"phase": "E"}) {
		t.Fatal("matchesStateChangedCondition() should reject D -> E when target matcher is in [A, B, C]")
	}
}

func TestServiceSaveNormalizesListMatchers(t *testing.T) {
	ctx := context.Background()
	svc, _ := newAutomationTestService(t)

	saved, err := svc.Save(ctx, models.Automation{
		Name:    "Washer phase fan-out",
		Enabled: true,
		Conditions: []models.AutomationCondition{
			{
				Type:     models.AutomationConditionTypeStateChanged,
				DeviceID: "haier:washer:test",
				StateKey: "phase",
				From:     &models.AutomationStateMatch{Operator: models.AutomationMatchEquals, Value: "D"},
				To:       &models.AutomationStateMatch{Operator: models.AutomationMatchIn, Value: "A"},
			},
			{
				Type:     models.AutomationConditionTypeCurrentState,
				DeviceID: "haier:washer:test",
				StateKey: "phase",
				Match:    &models.AutomationStateMatch{Operator: models.AutomationMatchNotIn, Value: []string{"X", "Y"}},
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
	toValues, ok := saved.Conditions[0].To.Value.([]any)
	if !ok || len(toValues) != 1 || toValues[0] != "A" {
		t.Fatalf("Conditions[0].To.Value should normalize to single-item list, got %#v", saved.Conditions[0].To.Value)
	}
	conditionValues, ok := saved.Conditions[1].Match.Value.([]any)
	if !ok || len(conditionValues) != 2 {
		t.Fatalf("Condition matcher should preserve list values, got %#v", saved.Conditions[1].Match.Value)
	}
}

func TestServiceSaveRequiresExactlyOneTriggerCondition(t *testing.T) {
	ctx := context.Background()
	svc, _ := newAutomationTestService(t)

	tests := []struct {
		name       string
		conditions []models.AutomationCondition
	}{
		{
			name: "missing trigger condition",
			conditions: []models.AutomationCondition{
				{
					Type:     models.AutomationConditionTypeCurrentState,
					DeviceID: "haier:washer:test",
					StateKey: "phase",
					Match:    &models.AutomationStateMatch{Operator: models.AutomationMatchEquals, Value: "ready"},
				},
			},
		},
		{
			name: "multiple trigger conditions",
			conditions: []models.AutomationCondition{
				{
					Type:     models.AutomationConditionTypeStateChanged,
					DeviceID: "haier:washer:test",
					StateKey: "phase",
					From:     &models.AutomationStateMatch{Operator: models.AutomationMatchAny},
					To:       &models.AutomationStateMatch{Operator: models.AutomationMatchEquals, Value: "ready"},
				},
				{
					Type:     models.AutomationConditionTypeStateChanged,
					DeviceID: "xiaomi:speaker:test",
					StateKey: "play_status",
					From:     &models.AutomationStateMatch{Operator: models.AutomationMatchAny},
					To:       &models.AutomationStateMatch{Operator: models.AutomationMatchEquals, Value: "idle"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Save(ctx, models.Automation{
				Name:       tt.name,
				Enabled:    true,
				Conditions: tt.conditions,
				Actions: []models.AutomationAction{
					{
						DeviceID: "xiaomi:speaker:test",
						Action:   "push_voice_message",
					},
				},
			})
			if err == nil {
				t.Fatal("Save() should reject automations without exactly one trigger condition")
			}
			if got := err.Error(); got != "automation requires exactly one trigger condition" {
				t.Fatalf("Save() error = %q, want exact single-trigger error", got)
			}
		})
	}
}

func TestServiceSaveAllowsDailyTimeTrigger(t *testing.T) {
	ctx := context.Background()
	svc, _ := newAutomationTestService(t)
	svc.SetAgentRuntime(fakeAgentWithWeComUsers(models.AgentPushUser{ID: "user-chentianyu", Name: "Chentianyu", WeComUser: "chentianyu", Enabled: true}))

	saved, err := svc.Save(ctx, models.Automation{
		Name:    "Daily digest",
		Enabled: true,
		Conditions: []models.AutomationCondition{
			{
				Type: models.AutomationConditionTypeTime,
				Time: &models.AutomationTimeCondition{
					Schedule: "daily",
					At:       "08:30",
					Timezone: "Asia/Shanghai",
				},
			},
		},
		Actions: []models.AutomationAction{
			{
				Kind:   models.AutomationActionKindAgent,
				Action: "agent.run",
				Params: map[string]any{
					"input": "生成每日摘要",
					"touchpoints": []any{
						map[string]any{"type": "wecom", "to_user": "user-chentianyu"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if saved.Conditions[0].Time == nil || saved.Conditions[0].Time.At != "08:30" {
		t.Fatalf("time trigger not normalized: %#v", saved.Conditions[0].Time)
	}
	if saved.Actions[0].DeviceID != "" || saved.Actions[0].Kind != models.AutomationActionKindAgent {
		t.Fatalf("agent action not normalized: %#v", saved.Actions[0])
	}
}

func TestServiceSaveRejectsUnknownWeComTouchpoint(t *testing.T) {
	ctx := context.Background()
	svc, _ := newAutomationTestService(t)
	svc.SetAgentRuntime(fakeAgentWithWeComUsers(models.AgentPushUser{ID: "user-known", Name: "Known", WeComUser: "known", Enabled: true}))

	_, err := svc.Save(ctx, models.Automation{
		Name:    "Unknown wecom user",
		Enabled: true,
		Conditions: []models.AutomationCondition{
			{
				Type: models.AutomationConditionTypeTime,
				Time: &models.AutomationTimeCondition{Schedule: "daily", At: "08:30"},
			},
		},
		Actions: []models.AutomationAction{
			{
				Kind:   models.AutomationActionKindAgent,
				Action: "agent.run",
				Params: map[string]any{
					"input": "run digest",
					"touchpoints": []any{
						map[string]any{"type": "wecom", "to_user": "missing"},
					},
				},
			},
		},
	})
	if err == nil {
		t.Fatal("Save() error = nil, want unknown wecom user rejection")
	}
	if got := err.Error(); !strings.Contains(got, "wecom target not configured") {
		t.Fatalf("Save() error = %q, want unknown wecom user rejection", got)
	}
}

func TestExecuteAgentActionSendsWeComTouchpoint(t *testing.T) {
	ctx := context.Background()
	svc, _ := newAutomationTestService(t)
	agent := fakeAgentWithWeComUsers(models.AgentPushUser{ID: "user-chentianyu", Name: "Chentianyu", WeComUser: "chentianyu", Enabled: true})
	svc.SetAgentRuntime(agent)

	automation, err := svc.Save(ctx, models.Automation{
		Name:    "Daily agent output",
		Enabled: true,
		Conditions: []models.AutomationCondition{
			{
				Type: models.AutomationConditionTypeTime,
				Time: &models.AutomationTimeCondition{Schedule: "daily", At: "08:30"},
			},
		},
		Actions: []models.AutomationAction{
			{
				Kind:   models.AutomationActionKindAgent,
				Action: "agent.run",
				Params: map[string]any{
					"input": "run digest",
					"touchpoints": []any{
						map[string]any{"type": "wecom", "to_user": "user-chentianyu"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := svc.executeAutomation(ctx, automation, models.Event{ID: "source", Type: "automation.time"}); err != nil {
		t.Fatalf("executeAutomation() error = %v", err)
	}
	if len(agent.requests) != 1 || agent.requests[0].Input != "run digest" || agent.requests[0].Actor != "automation:"+automation.ID {
		t.Fatalf("agent requests = %#v", agent.requests)
	}
	if len(agent.sends) != 1 || agent.sends[0].to != "chentianyu" || agent.sends[0].text != "agent output" {
		t.Fatalf("wecom sends = %#v", agent.sends)
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
