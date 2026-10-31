package accounts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/chentianyu/celestia/internal/core/project/garden"
	"github.com/chentianyu/celestia/internal/models"
)

type testStore struct {
	mu      sync.Mutex
	docs    map[string]models.AgentDocument
	failKey string
}

func (s *testStore) GetAgentDocument(_ context.Context, k string) (models.AgentDocument, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.docs[k]
	return d, ok, nil
}
func (s *testStore) UpsertAgentDocument(_ context.Context, d models.AgentDocument) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d.Key == s.failKey {
		return errors.New("storage failed")
	}
	s.docs[d.Key] = d
	return nil
}

type testAPI struct {
	list              []Account
	reads, writes     int
	readErr, writeErr error
}

func (a *testAPI) Accounts(context.Context, string) ([]Account, error) {
	a.reads++
	return append([]Account{}, a.list...), a.readErr
}
func (a *testAPI) SetExecution(_ context.Context, _ string, id string, on bool) (Account, error) {
	a.writes++
	for i := range a.list {
		if a.list[i].ID == id {
			a.list[i].AutomationEnabled = on
			return a.list[i], a.writeErr
		}
	}
	return Account{}, errors.New("missing account")
}

type testMenus struct {
	config        models.AgentWeComMenuConfig
	reads, writes int
	fail          bool
}

func (m *testMenus) GetWeComMenu(context.Context) (models.AgentWeComMenuSnapshot, error) {
	m.reads++
	return models.AgentWeComMenuSnapshot{Config: m.config}, nil
}
func (m *testMenus) SaveWeComMenu(_ context.Context, c models.AgentWeComMenuConfig) (models.AgentWeComMenuSnapshot, error) {
	m.writes++
	if m.fail {
		return models.AgentWeComMenuSnapshot{}, errors.New("publish failed")
	}
	m.config = c
	return models.AgentWeComMenuSnapshot{Config: c}, nil
}

func testAccount(i int) Account {
	return Account{ID: fmt.Sprintf("%08x-0000-4000-8000-%012x", i, i), Name: fmt.Sprintf("账号%d", i), AutomationEnabled: true}
}
func testModule(n int) (*Service, *testStore, *testAPI, *testMenus) {
	p, _ := json.Marshal(garden.Config{CWD: "/garden"})
	store := &testStore{docs: map[string]models.AgentDocument{garden.ConfigKey: {Payload: p}}}
	api := &testAPI{}
	for i := 0; i < n; i++ {
		api.list = append(api.list, testAccount(i))
	}
	menus := &testMenus{config: models.AgentWeComMenuConfig{Version: 1, Buttons: []models.AgentWeComButton{{Name: "花园", Enabled: true, SubButtons: []models.AgentWeComButton{{Name: "检查告警", Key: garden.CheckAlerts, Enabled: true}, {Name: "处理游戏升级", Key: garden.GameUpgrade, Enabled: true}}}}}}
	return New(store, api, menus), store, api, menus
}

func TestTogglePermissionsReplayAndRestart(t *testing.T) {
	s, store, api, _ := testModule(1)
	ctx := context.Background()
	key := TogglePrefix + api.list[0].ID
	for _, sender := range []string{"", "Home", "Jellyfish2", " jellyfish", "jellyfish "} {
		if _, err := s.Toggle(ctx, sender, key, "denied"); err == nil {
			t.Fatal("unauthorized sender", sender)
		}
	}
	if api.reads != 0 || api.writes != 0 {
		t.Fatal("unauthorized request reached garden")
	}
	first, err := s.Toggle(ctx, "jellyfish", key, "click-1")
	if err != nil || first.Account.AutomationEnabled {
		t.Fatal(first, err)
	}
	s = New(store, api, nil)
	replay, err := s.Toggle(ctx, "Jellyfish", key, "click-1")
	if err != nil || !replay.Duplicate || api.writes != 1 {
		t.Fatal(replay, err, api.writes)
	}
	second, err := s.Toggle(ctx, "frypan", key, "click-2")
	if err != nil || !second.Account.AutomationEnabled || api.writes != 2 {
		t.Fatal(second, err)
	}
}

func TestToggleDoesNotReplayAmbiguousOrUnpersistedWrites(t *testing.T) {
	s, store, api, _ := testModule(1)
	ctx := context.Background()
	key := TogglePrefix + api.list[0].ID
	store.failKey = TogglesKey
	if _, err := s.Toggle(ctx, "Frypan", key, "not-saved"); err == nil || api.writes != 0 {
		t.Fatal("write without durable reservation", err)
	}
	store.failKey = ""
	api.writeErr = errors.New("lost acknowledgement")
	if _, err := s.Toggle(ctx, "Frypan", key, "ambiguous"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := s.Toggle(ctx, "Frypan", key, "ambiguous"); err == nil || api.writes != 1 {
		t.Fatal("ambiguous toggle replayed", err)
	}
}

func TestDeletedAndMalformedAccountNeverWrites(t *testing.T) {
	s, _, api, _ := testModule(1)
	for _, key := range []string{TogglePrefix + "../settings", TogglePrefix + testAccount(9).ID} {
		if _, err := s.Toggle(context.Background(), "Jellyfish", key, "click"); err == nil {
			t.Fatal("invalid account accepted")
		}
	}
	if api.writes != 0 {
		t.Fatal(api.writes)
	}
}

func TestConcurrentSSEReplayTogglesOnlyOnce(t *testing.T) {
	s, _, api, _ := testModule(1)
	key := TogglePrefix + api.list[0].ID
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Toggle(context.Background(), "Jellyfish", key, "same-click"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if api.writes != 1 {
		t.Fatal(api.writes)
	}
}

func TestRosterSyncPublishesOnlyChangesAndSurvivesRestart(t *testing.T) {
	s, store, api, menus := testModule(10)
	ctx := context.Background()
	if changed, err := s.Sync(ctx); err != nil || !changed {
		t.Fatal(changed, err)
	}
	if len(menus.config.Buttons) != 3 || menus.config.Buttons[0].Name != "花园" {
		t.Fatal("existing menu lost", menus.config)
	}
	for _, b := range menus.config.Buttons[1:] {
		if len(b.SubButtons) != 5 {
			t.Fatal(b)
		}
	}
	api.list[0], api.list[9] = api.list[9], api.list[0]
	api.list[0].AutomationEnabled = false
	s = New(store, api, menus)
	if changed, err := s.Sync(ctx); err != nil || changed || menus.reads != 1 || menus.writes != 1 {
		t.Fatal("unchanged roster refreshed menu", changed, err, menus)
	}
	api.list[0] = testAccount(99) // Same count, different account must be detected.
	if changed, err := s.Sync(ctx); err != nil || !changed || menus.writes != 2 {
		t.Fatal(changed, err, menus.writes)
	}
	api.list = api.list[:4]
	if changed, err := s.Sync(ctx); err != nil || !changed || len(menus.config.Buttons) != 2 {
		t.Fatal(changed, err, menus.config)
	}
	api.list = nil
	if changed, err := s.Sync(ctx); err != nil || !changed || len(menus.config.Buttons) != 1 {
		t.Fatal(changed, err, menus.config)
	}
}

func TestMenuCapacityAndPublishFailuresPreserveLastGoodState(t *testing.T) {
	s, store, api, menus := testModule(10)
	ctx := context.Background()
	if _, err := s.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	api.list = append(api.list, testAccount(10))
	if _, err := s.Sync(ctx); err == nil || menus.writes != 1 {
		t.Fatal("capacity limit silently truncated accounts", err)
	}
	var state MenuState
	if err := json.Unmarshal(store.docs[MenuStateKey].Payload, &state); err != nil {
		t.Fatal(err)
	}
	if len(state.Accounts) != 10 || state.Error == "" {
		t.Fatal(state)
	}
	api.list = api.list[:9]
	menus.fail = true
	if _, err := s.Sync(ctx); err == nil {
		t.Fatal("publish failure hidden")
	}
	menus.fail = false
	if changed, err := s.Sync(ctx); err != nil || !changed {
		t.Fatal("failed roster not retried", changed, err)
	}
}
