package runtime

import (
	"errors"
	"slices"
	"strings"
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

func TestEvolutionCodexErrorIncludesOutput(t *testing.T) {
	err := evolutionCodexError(models.AgentCodexResult{Error: "exit status 1", Output: `{"steps":["inspect code"]}`}, errors.New("exit status 1"))
	if err == nil || !strings.Contains(err.Error(), `{"steps":["inspect code"]}`) {
		t.Fatalf("evolutionCodexError() = %v, want output detail", err)
	}
}

func TestResolveCodexAgentProviderRequiresCodexType(t *testing.T) {
	settings := models.AgentSettings{AgentProviders: []models.AgentProvider{
		{ID: "browser", Type: "browser"},
		{ID: "codex-main", Type: "codex", Model: "gpt-test", ReasoningEffort: "high"},
	}}

	provider, err := resolveCodexAgentProvider(settings, "codex-main")
	if err != nil {
		t.Fatalf("resolveCodexAgentProvider() error = %v", err)
	}
	if provider.Model != "gpt-test" || provider.ReasoningEffort != "high" {
		t.Fatalf("provider = %+v", provider)
	}
	if _, err := resolveCodexAgentProvider(settings, "browser"); err == nil {
		t.Fatal("resolveCodexAgentProvider(browser) error = nil, want type rejection")
	}
}
