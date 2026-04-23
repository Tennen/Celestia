package touchpoint

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chentianyu/celestia/internal/core/agent"
	"github.com/chentianyu/celestia/internal/core/eventbus"
	"github.com/chentianyu/celestia/internal/models"
	sqlitestore "github.com/chentianyu/celestia/internal/storage/sqlite"
)

func newTouchpointPersistenceTestService(t *testing.T) (*Service, *agent.Service) {
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
	agentSvc := agent.New(store, eventbus.New())
	return New(agentSvc, agentSvc), agentSvc
}

func TestResolveWeComRecipientRequiresConfiguredEnabledUser(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTouchpointPersistenceTestService(t)
	if _, err := svc.SaveWeComUsers(ctx, models.AgentPushSnapshot{Users: []models.AgentPushUser{
		{ID: "user-alice", Name: "Alice", WeComUser: "alice", Enabled: true},
		{ID: "user-disabled", Name: "Disabled", WeComUser: "disabled", Enabled: false},
	}}); err != nil {
		t.Fatalf("SaveWeComUsers() error = %v", err)
	}

	byID, err := svc.ResolveWeComRecipient(ctx, "user-alice")
	if err != nil {
		t.Fatalf("ResolveWeComRecipient(id) error = %v", err)
	}
	if byID.WeComUser != "alice" {
		t.Fatalf("ResolveWeComRecipient(id) = %+v, want alice", byID)
	}
	byWeComUser, err := svc.ResolveWeComRecipient(ctx, "alice")
	if err != nil {
		t.Fatalf("ResolveWeComRecipient(wecom_user) error = %v", err)
	}
	if byWeComUser.ID != "user-alice" {
		t.Fatalf("ResolveWeComRecipient(wecom_user) = %+v, want user-alice", byWeComUser)
	}
	if _, err := svc.ResolveWeComRecipient(ctx, "disabled"); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("ResolveWeComRecipient(disabled) error = %v, want disabled rejection", err)
	}
	if _, err := svc.ResolveWeComRecipient(ctx, "missing"); err == nil || !strings.Contains(err.Error(), "not a configured user") {
		t.Fatalf("ResolveWeComRecipient(missing) error = %v, want configured-user rejection", err)
	}
}

func TestSavePushValidatesWeComUsers(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTouchpointPersistenceTestService(t)

	if _, err := svc.SaveWeComUsers(ctx, models.AgentPushSnapshot{Users: []models.AgentPushUser{{Name: "Missing WeCom"}}}); err == nil || !strings.Contains(err.Error(), "wecom user is required") {
		t.Fatalf("SaveWeComUsers(missing wecom_user) error = %v, want required rejection", err)
	}
	if _, err := svc.SaveWeComUsers(ctx, models.AgentPushSnapshot{Users: []models.AgentPushUser{
		{Name: "One", WeComUser: "alice", Enabled: true},
		{Name: "Two", WeComUser: "alice", Enabled: true},
	}}); err == nil || !strings.Contains(err.Error(), "must be unique") {
		t.Fatalf("SaveWeComUsers(duplicate wecom_user) error = %v, want duplicate rejection", err)
	}
}

func TestSendWeComMessageRejectsUnconfiguredTargetBeforeDelivery(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTouchpointPersistenceTestService(t)

	err := svc.SendWeComMessage(ctx, WeComSendRequest{ToUser: "missing", Text: "hello"})
	if err == nil || !strings.Contains(err.Error(), "not a configured user") {
		t.Fatalf("SendWeComMessage() error = %v, want configured-user rejection", err)
	}
}
