package runtime

import (
	"testing"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

func TestKnowledgeSessionIsScopedByBase(t *testing.T) {
	now := time.Now().UTC()
	ops := newKnowledgeSession("alice", "wecom", "ops", now)
	design := newKnowledgeSession("alice", "wecom", "design", now)
	sessions := activateKnowledgeSession(nil, ops)
	sessions = activateKnowledgeSession(sessions, design)

	if session, ok := activeKnowledgeSession(sessions, "alice", "ops"); !ok || !session.Active {
		t.Fatalf("ops session = %+v, %v; want active", session, ok)
	}
	if session, ok := activeKnowledgeSession(sessions, "alice", "design"); !ok || !session.Active {
		t.Fatalf("design session = %+v, %v; want active", session, ok)
	}
}

func TestResolveKnowledgeBaseRequiresConfiguredBase(t *testing.T) {
	config := models.AgentKnowledgeConfig{
		Enabled:       true,
		DefaultBaseID: "ops",
		Bases: []models.AgentKnowledgeBase{{
			ID:      "ops",
			Name:    "Operations",
			BaseDir: t.TempDir(),
			Enabled: true,
		}},
	}

	base, _, err := resolveKnowledgeBase(config, "")
	if err != nil {
		t.Fatalf("resolveKnowledgeBase() error = %v", err)
	}
	if base.ID != "ops" {
		t.Fatalf("base id = %q, want ops", base.ID)
	}
	if _, _, err := resolveKnowledgeBase(config, "missing"); err == nil {
		t.Fatal("resolveKnowledgeBase(missing) error = nil, want error")
	}
}
