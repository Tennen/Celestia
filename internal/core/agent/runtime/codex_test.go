package runtime

import (
	"slices"
	"testing"

	"github.com/chentianyu/celestia/internal/models"
)

func TestBuildCodexArgsPlacesResumeOptionsBeforeSessionID(t *testing.T) {
	args := buildCodexArgs(models.AgentCodexRequest{
		Model:            "gpt-test",
		ReasoningEffort:  "high",
		ResumeSessionID:  "session-123",
		SkipGitRepoCheck: true,
	}, "/kb", "/tmp/out.txt")

	sessionIdx := slices.Index(args, "session-123")
	modelIdx := slices.Index(args, "--model")
	if sessionIdx < 0 || modelIdx < 0 || modelIdx > sessionIdx {
		t.Fatalf("args = %#v, want model option before resume session id", args)
	}
	if !slices.Contains(args, "--skip-git-repo-check") {
		t.Fatalf("args = %#v, want --skip-git-repo-check", args)
	}
}

func TestExtractCodexSessionIDFindsNestedJSONLValue(t *testing.T) {
	output := `{"type":"started"}
{"msg":{"session_id":"abc-123"}}`

	if got := extractCodexSessionID(output); got != "abc-123" {
		t.Fatalf("extractCodexSessionID() = %q, want abc-123", got)
	}
}

func TestResolveCodexProviderRequiresCodexType(t *testing.T) {
	settings := models.AgentSettings{LLMProviders: []models.AgentLLMProvider{
		{ID: "chat", Type: "openai"},
		{ID: "codex-main", Type: "codex", Model: "gpt-test", ChatPath: "high"},
	}}

	provider, err := resolveCodexProvider(settings, "codex-main")
	if err != nil {
		t.Fatalf("resolveCodexProvider() error = %v", err)
	}
	if provider.Model != "gpt-test" || provider.ChatPath != "high" {
		t.Fatalf("provider = %+v", provider)
	}
	if _, err := resolveCodexProvider(settings, "chat"); err == nil {
		t.Fatal("resolveCodexProvider(openai) error = nil, want type rejection")
	}
}
