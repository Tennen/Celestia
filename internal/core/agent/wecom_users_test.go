package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/chentianyu/celestia/internal/models"
)

func TestResolveWeComRecipientRequiresConfiguredEnabledUser(t *testing.T) {
	ctx := context.Background()
	svc, _ := newAgentPersistenceTestService(t)
	if _, err := svc.SavePush(ctx, models.AgentPushSnapshot{Users: []models.AgentPushUser{
		{ID: "user-alice", Name: "Alice", WeComUser: "alice", Enabled: true},
		{ID: "user-disabled", Name: "Disabled", WeComUser: "disabled", Enabled: false},
	}}); err != nil {
		t.Fatalf("SavePush() error = %v", err)
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
	svc, _ := newAgentPersistenceTestService(t)

	if _, err := svc.SavePush(ctx, models.AgentPushSnapshot{Users: []models.AgentPushUser{{Name: "Missing WeCom"}}}); err == nil || !strings.Contains(err.Error(), "wecom user is required") {
		t.Fatalf("SavePush(missing wecom_user) error = %v, want required rejection", err)
	}
	if _, err := svc.SavePush(ctx, models.AgentPushSnapshot{Users: []models.AgentPushUser{
		{Name: "One", WeComUser: "alice", Enabled: true},
		{Name: "Two", WeComUser: "alice", Enabled: true},
	}}); err == nil || !strings.Contains(err.Error(), "must be unique") {
		t.Fatalf("SavePush(duplicate wecom_user) error = %v, want duplicate rejection", err)
	}
}

func TestSendWeComMessageRejectsUnconfiguredTargetBeforeDelivery(t *testing.T) {
	ctx := context.Background()
	svc, _ := newAgentPersistenceTestService(t)

	err := svc.SendWeComMessage(ctx, WeComSendRequest{ToUser: "missing", Text: "hello"})
	if err == nil || !strings.Contains(err.Error(), "not a configured user") {
		t.Fatalf("SendWeComMessage() error = %v, want configured-user rejection", err)
	}
}
