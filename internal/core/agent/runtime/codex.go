package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
)

func (s *Service) RunCodex(ctx context.Context, req models.AgentCodexRequest) (models.AgentCodexResult, error) {
	if err := requireText(req.Prompt, "prompt"); err != nil {
		return models.AgentCodexResult{}, err
	}
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return models.AgentCodexResult{}, err
	}
	taskID := firstNonEmpty(req.TaskID, uuid.NewString())
	cwd := firstNonEmpty(req.CWD, snapshot.Settings.Evolution.CWD)
	if cwd == "" {
		cwd = "."
	}
	outputDir := resolveCodexOutputDir(cwd, req.OutputDir)
	_ = os.MkdirAll(outputDir, 0o755)
	outputFile := filepath.Join(outputDir, taskID+".txt")
	timeout := time.Duration(maxInt(req.TimeoutMS, snapshot.Settings.Evolution.TimeoutMS)) * time.Millisecond
	if timeout <= 0 {
		timeout = 15 * time.Minute
	}
	args := buildCodexArgs(req, cwd, outputFile)
	args = append(args, req.Prompt)

	started := time.Now().UTC()
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, "codex", args...)
	cmd.Dir = cwd
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err = cmd.Run()
	finished := time.Now().UTC()
	exitCode := 0
	if err != nil {
		exitCode = 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}
	fileBytes, _ := os.ReadFile(outputFile)
	result := models.AgentCodexResult{
		TaskID:     taskID,
		OK:         err == nil,
		SessionID:  extractCodexSessionID(output.String()),
		OutputFile: outputFile,
		Output:     strings.TrimSpace(firstNonEmpty(string(fileBytes), output.String())),
		ExitCode:   exitCode,
		StartedAt:  started,
		FinishedAt: finished,
	}
	if err != nil {
		result.Error = err.Error()
	}
	return result, err
}

func resolveCodexOutputDir(cwd string, outputDir string) string {
	if strings.TrimSpace(outputDir) == "" {
		return filepath.Join(cwd, "data", "agent", "codex")
	}
	if filepath.IsAbs(outputDir) {
		return filepath.Clean(outputDir)
	}
	return filepath.Join(cwd, outputDir)
}

func buildCodexArgs(req models.AgentCodexRequest, cwd string, outputFile string) []string {
	args := []string{"-a", "never", "exec"}
	if strings.TrimSpace(req.ResumeSessionID) != "" {
		args = append(args, "resume", "--json", "-o", outputFile)
		args = appendCodexConfigArgs(args, req)
		if req.SkipGitRepoCheck {
			args = append(args, "--skip-git-repo-check")
		}
		args = append(args, strings.TrimSpace(req.ResumeSessionID))
		return args
	}
	sandbox := strings.TrimSpace(req.Sandbox)
	if sandbox == "" {
		sandbox = "workspace-write"
	}
	args = append(args, "--json", "--sandbox", sandbox, "-o", outputFile, "--cd", cwd)
	args = appendCodexConfigArgs(args, req)
	if req.SkipGitRepoCheck {
		args = append(args, "--skip-git-repo-check")
	}
	return args
}

func appendCodexConfigArgs(args []string, req models.AgentCodexRequest) []string {
	if strings.TrimSpace(req.Model) != "" {
		args = append(args, "--model", strings.TrimSpace(req.Model))
	}
	if strings.TrimSpace(req.ReasoningEffort) != "" {
		args = append(args, "--config", "model_reasoning_effort="+quoteCodexConfig(req.ReasoningEffort))
	}
	return args
}

func extractCodexSessionID(output string) string {
	for _, line := range strings.Split(output, "\n") {
		var payload any
		if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &payload); err != nil {
			continue
		}
		if sessionID := findCodexSessionID(payload); sessionID != "" {
			return sessionID
		}
	}
	return ""
}

func findCodexSessionID(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range []string{"session_id", "sessionId"} {
			if sessionID, ok := typed[key].(string); ok && strings.TrimSpace(sessionID) != "" {
				return strings.TrimSpace(sessionID)
			}
		}
		for _, child := range typed {
			if sessionID := findCodexSessionID(child); sessionID != "" {
				return sessionID
			}
		}
	case []any:
		for _, child := range typed {
			if sessionID := findCodexSessionID(child); sessionID != "" {
				return sessionID
			}
		}
	}
	return ""
}

func quoteCodexConfig(value string) string {
	escaped := strings.ReplaceAll(strings.TrimSpace(value), `"`, `\"`)
	return `"` + escaped + `"`
}
