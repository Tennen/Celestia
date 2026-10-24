package garden

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/chentianyu/celestia/internal/core/codexapp"
	"github.com/chentianyu/celestia/internal/models"
)

type memoryStore map[string]models.AgentDocument

func (s memoryStore) GetAgentDocument(_ context.Context, key string) (models.AgentDocument, bool, error) {
	d, ok := s[key]
	return d, ok, nil
}
func (s memoryStore) UpsertAgentDocument(_ context.Context, d models.AgentDocument) error {
	s[d.Key] = d
	return nil
}

type submitterFunc func(context.Context, string, string, string, string) (codexapp.Result, error)

func (f submitterFunc) Submit(c context.Context, id, cwd, prompt, msg string) (codexapp.Result, error) {
	return f(c, id, cwd, prompt, msg)
}

func TestOnlyExactOwnerCanSubmit(t *testing.T) {
	for _, sender := range []string{"", "frypan", "Jellyfish", "Frypan ", "home"} {
		t.Run(sender, func(t *testing.T) {
			s := New(memoryStore{}, submitterFunc(func(context.Context, string, string, string, string) (codexapp.Result, error) {
				t.Fatal("unauthorized RPC")
				return codexapp.Result{}, nil
			}))
			if _, err := s.Submit(context.Background(), sender, CheckAlerts, "message"); err == nil {
				t.Fatal("expected denial")
			}
		})
	}
}

func TestPersistBeforeRPCAndDeduplicateAfterRestart(t *testing.T) {
	payload, _ := json.Marshal(Config{ThreadID: "thread", CWD: "/garden"})
	store := memoryStore{ConfigKey: {Payload: payload}}
	calls := 0
	runner := submitterFunc(func(_ context.Context, id, cwd, prompt, msg string) (codexapp.Result, error) {
		calls++
		var rs []Request
		if err := json.Unmarshal(store[RequestsKey].Payload, &rs); err != nil {
			t.Fatal(err)
		}
		if len(rs) != 1 || rs[0].Status != "submitting" {
			t.Fatal("request not persisted before RPC", rs)
		}
		if id != "thread" || cwd != "/garden" || prompt != "检查花园脚本最近的告警，判断是游戏问题还是脚本问题，需要修复的话请修复。" || msg == "" {
			t.Fatal("incorrect binding or prompt")
		}
		return codexapp.Result{ThreadID: id, TurnID: "turn"}, nil
	})
	s := New(store, runner)
	if _, err := s.Submit(context.Background(), Owner, CheckAlerts, "message"); err != nil {
		t.Fatal(err)
	}
	// A fresh coordinator sees the persisted ledger and cannot replay the event.
	s = New(store, runner)
	if _, err := s.Submit(context.Background(), Owner, CheckAlerts, "message"); err == nil {
		t.Fatal("replayed event")
	}
	if _, err := s.Submit(context.Background(), Owner, CheckAlerts, "another-click"); err == nil {
		t.Fatal("rapid repeat")
	}
	if calls != 1 {
		t.Fatal(calls)
	}
}

func TestAmbiguousFailureIsNotAutomaticallyRetried(t *testing.T) {
	payload, _ := json.Marshal(Config{ThreadID: "thread", CWD: "/garden"})
	store := memoryStore{ConfigKey: {Payload: payload}}
	calls := 0
	s := New(store, submitterFunc(func(context.Context, string, string, string, string) (codexapp.Result, error) {
		calls++
		return codexapp.Result{}, errors.New("lost response")
	}))
	_, _ = s.Submit(context.Background(), Owner, GameUpgrade, "event")
	_, _ = s.Submit(context.Background(), Owner, GameUpgrade, "event")
	if calls != 1 {
		t.Fatal("ambiguous result retried", calls)
	}
	if _, err := s.Submit(context.Background(), Owner, "garden:arbitrary", "new"); err == nil {
		t.Fatal("unrecognized action allowed")
	}
}
