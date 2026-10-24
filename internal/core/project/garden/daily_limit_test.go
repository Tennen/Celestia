package garden

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chentianyu/celestia/internal/core/codexapp"
	"github.com/chentianyu/celestia/internal/models"
	sqlitestore "github.com/chentianyu/celestia/internal/storage/sqlite"
)

func dailyTestService(runner Submitter) *Service {
	payload, _ := json.Marshal(Config{ThreadID: "thread", CWD: "/garden"})
	s := New(memoryStore{ConfigKey: {Payload: payload}}, runner)
	s.now = func() time.Time { return time.Date(2026, 10, 24, 8, 0, 0, 0, time.UTC) }
	return s
}

func TestActionsHaveIndependentDailyLimits(t *testing.T) {
	calls := 0
	s := dailyTestService(submitterFunc(func(context.Context, string, string, string, string) (codexapp.Result, error) {
		calls++
		return codexapp.Result{TurnID: "turn"}, nil
	}))
	for _, action := range []string{CheckAlerts, GameUpgrade} {
		first, err := s.Submit(context.Background(), Owner, action, action+"-1")
		if err != nil || first.Day != "2026-10-24" || first.DailyCount != 1 {
			t.Fatalf("first=%+v err=%v", first, err)
		}
		limited, err := s.Submit(context.Background(), Owner, action, action+"-2")
		if err == nil || limited.DailyCount != 1 || !strings.Contains(err.Error(), "1/1") {
			t.Fatalf("limited=%+v err=%v", limited, err)
		}
	}
	if calls != 2 {
		t.Fatalf("calls=%d, want one per action", calls)
	}
}

func TestQuotaResetsAtBeijingMidnightNotRolling24Hours(t *testing.T) {
	calls := 0
	s := dailyTestService(submitterFunc(func(context.Context, string, string, string, string) (codexapp.Result, error) {
		calls++
		return codexapp.Result{TurnID: "turn"}, nil
	}))
	clock := time.Date(2026, 10, 24, 15, 59, 59, 0, time.UTC)
	s.now = func() time.Time { return clock }
	first, err := s.Submit(context.Background(), Owner, CheckAlerts, "before-midnight")
	if err != nil || first.Day != "2026-10-24" {
		t.Fatal(first, err)
	}
	clock = clock.Add(time.Second)
	second, err := s.Submit(context.Background(), Owner, CheckAlerts, "after-midnight")
	if err != nil || second.Day != "2026-10-25" || second.DailyCount != 1 {
		t.Fatal(second, err)
	}
	if _, err := s.Submit(context.Background(), Owner, CheckAlerts, "same-new-day"); err == nil {
		t.Fatal("second trigger in same Beijing day")
	}
	if calls != 2 {
		t.Fatal(calls)
	}
}

func TestConcurrentClicksReserveOnlyOneDailyAttempt(t *testing.T) {
	calls := 0
	s := dailyTestService(submitterFunc(func(context.Context, string, string, string, string) (codexapp.Result, error) {
		calls++
		return codexapp.Result{TurnID: "turn"}, nil
	}))
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = s.Submit(context.Background(), Owner, GameUpgrade, fmt.Sprintf("click-%d", i))
		}(i)
	}
	wg.Wait()
	if calls != 1 {
		t.Fatalf("concurrent calls=%d", calls)
	}
}

func TestFailedSubmissionStillConsumesDailyLimit(t *testing.T) {
	calls := 0
	s := dailyTestService(submitterFunc(func(context.Context, string, string, string, string) (codexapp.Result, error) {
		calls++
		return codexapp.Result{}, errors.New("lost response")
	}))
	first, err := s.Submit(context.Background(), Owner, CheckAlerts, "first")
	if err == nil || first.DailyCount != 1 || !strings.Contains(err.Error(), "今日次数已使用") {
		t.Fatal(first, err)
	}
	if _, err = s.Submit(context.Background(), Owner, CheckAlerts, "different-click"); err == nil {
		t.Fatal("failure refunded quota")
	}
	if calls != 1 {
		t.Fatal(calls)
	}
}

func TestPreUpgradeRecordsCountTowardDailyQuota(t *testing.T) {
	now := time.Date(2026, 10, 24, 16, 0, 0, 0, time.UTC)
	day, count := dailyUsage([]Request{{Action: CheckAlerts, Status: "failed", CreatedAt: now.Add(-time.Second)}, {Action: CheckAlerts, Status: "submitting", CreatedAt: now}, {Action: GameUpgrade, CreatedAt: now}}, CheckAlerts, now)
	if day != "2026-10-25" || count != 1 {
		t.Fatal(day, count)
	}
}

func TestDailyQuotaSurvivesSQLiteReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "garden.db")
	store, err := sqlitestore.New(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(Config{ThreadID: "thread", CWD: "/garden"})
	if err = store.UpsertAgentDocument(ctx, models.AgentDocument{Key: ConfigKey, Domain: "project.garden", Payload: payload, UpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	runner := submitterFunc(func(context.Context, string, string, string, string) (codexapp.Result, error) {
		return codexapp.Result{TurnID: "turn"}, nil
	})
	s := New(store, runner)
	s.now = func() time.Time { return time.Date(2026, 10, 24, 8, 0, 0, 0, time.UTC) }
	if _, err = s.Submit(ctx, Owner, GameUpgrade, "first"); err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = sqlitestore.New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	s = New(store, submitterFunc(func(context.Context, string, string, string, string) (codexapp.Result, error) {
		t.Fatal("restarted service bypassed quota")
		return codexapp.Result{}, nil
	}))
	s.now = func() time.Time { return time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC) }
	r, err := s.Submit(ctx, Owner, GameUpgrade, "after-restart")
	if err == nil || r.DailyCount != 1 {
		t.Fatal(r, err)
	}
}
