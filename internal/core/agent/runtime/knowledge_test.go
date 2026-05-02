package runtime

import (
	"os"
	"path/filepath"
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

func TestListKnowledgeAnswerFilesReturnsMarkdownAnswers(t *testing.T) {
	baseDir := t.TempDir()
	answerDir := filepath.Join(baseDir, ".answers")
	if err := os.MkdirAll(answerDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	path := filepath.Join(answerDir, "20260502-090000-answer.md")
	if err := os.WriteFile(path, []byte("# Answer Title\n\nBody"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(answerDir, "ignore.txt"), []byte("ignore"), 0o644); err != nil {
		t.Fatalf("WriteFile(ignore) error = %v", err)
	}

	answers, err := listKnowledgeAnswerFiles(models.AgentKnowledgeBase{ID: "ops"}, baseDir)
	if err != nil {
		t.Fatalf("listKnowledgeAnswerFiles() error = %v", err)
	}
	if len(answers) != 1 {
		t.Fatalf("answers = %+v, want one markdown answer", answers)
	}
	if answers[0].ID != "20260502-090000-answer" || answers[0].Title != "Answer Title" || answers[0].KnowledgeBaseID != "ops" {
		t.Fatalf("answer = %+v", answers[0])
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
