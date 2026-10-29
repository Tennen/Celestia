package garden

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
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

func TestGameUpgradeSubmitsVersionAndLiveAdvertisingCheck(t *testing.T) {
	payload, _ := json.Marshal(Config{ThreadID: "thread", CWD: "/garden"})
	store := memoryStore{ConfigKey: {Payload: payload}}
	calls := 0
	s := New(store, submitterFunc(func(_ context.Context, id, cwd, prompt, msg string) (codexapp.Result, error) {
		calls++
		if id != "thread" || cwd != "/garden" || msg == "" {
			t.Fatal("incorrect existing conversation binding")
		}
		for _, requirement := range []string{"核对接口协议", "即使游戏版本未变", "现有广告队列和账号运行配置", "登录切换", "角色绑定", "原生广告加载播放及奖励领取", "模拟器释放后通过独立查询", "本次检查产生的证据", "历史成功", "修复并复测", "无可用广告", "广告未验收", "不能报告全部正常"} {
			if !strings.Contains(prompt, requirement) {
				t.Fatalf("submitted upgrade task omits %q", requirement)
			}
		}
		return codexapp.Result{ThreadID: id, TurnID: "upgrade-turn"}, nil
	}))
	request, err := s.Submit(context.Background(), Owner, GameUpgrade, "upgrade-click")
	if err != nil || request.Status != "submitted" || request.TurnID != "upgrade-turn" || calls != 1 {
		t.Fatalf("upgrade task not submitted once: %+v, %v, calls=%d", request, err, calls)
	}
}
