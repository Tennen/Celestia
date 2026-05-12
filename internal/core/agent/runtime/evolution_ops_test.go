package runtime

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chentianyu/celestia/internal/models"
)

func TestNormalizeEvolutionOperationIncludesPull(t *testing.T) {
	if got := normalizeEvolutionOperation("pull"); got != "pull" {
		t.Fatalf("normalizeEvolutionOperation(pull) = %q, want pull", got)
	}
}

func TestRunEvolutionPullCommandFastForwards(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	remote := filepath.Join(tmp, "remote.git")
	work := filepath.Join(tmp, "work")
	other := filepath.Join(tmp, "other")

	runGitTestCommand(t, "", "git", "init", "--bare", remote)
	runGitTestCommand(t, remote, "git", "symbolic-ref", "HEAD", "refs/heads/main")
	runGitTestCommand(t, "", "git", "clone", remote, work)
	configureGitUser(t, work)
	if err := os.WriteFile(filepath.Join(work, "README.md"), []byte("one\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(work) error = %v", err)
	}
	runGitTestCommand(t, work, "git", "add", "README.md")
	runGitTestCommand(t, work, "git", "commit", "-m", "initial")
	runGitTestCommand(t, work, "git", "push", "origin", "HEAD:main")

	runGitTestCommand(t, "", "git", "clone", remote, other)
	configureGitUser(t, other)
	if err := os.WriteFile(filepath.Join(other, "README.md"), []byte("two\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(other) error = %v", err)
	}
	runGitTestCommand(t, other, "git", "commit", "-am", "update")
	runGitTestCommand(t, other, "git", "push", "origin", "main")

	result, err := runEvolutionPullCommand(ctx, models.AgentEvolutionConfig{
		CWD:        work,
		PushRemote: "origin",
		PushBranch: "main",
	})
	if err != nil {
		t.Fatalf("runEvolutionPullCommand() error = %v; output=%s", err, result.Output)
	}
	if !result.OK || result.Name != "git pull" || !strings.Contains(result.Command, "--ff-only") {
		t.Fatalf("result = %+v, want successful ff-only pull", result)
	}
	raw, err := os.ReadFile(filepath.Join(work, "README.md"))
	if err != nil {
		t.Fatalf("ReadFile(work) error = %v", err)
	}
	if string(raw) != "two\n" {
		t.Fatalf("README.md = %q, want pulled content", string(raw))
	}
}

func configureGitUser(t *testing.T, cwd string) {
	t.Helper()
	runGitTestCommand(t, cwd, "git", "config", "user.email", "celestia-test@example.invalid")
	runGitTestCommand(t, cwd, "git", "config", "user.name", "Celestia Test")
}

func runGitTestCommand(t *testing.T, cwd string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	if strings.TrimSpace(cwd) != "" {
		cmd.Dir = cwd
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
}
