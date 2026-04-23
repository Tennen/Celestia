package runtime

import (
	"bytes"
	"context"
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
	outputDir := filepath.Join(cwd, "data", "agent", "codex")
	_ = os.MkdirAll(outputDir, 0o755)
	outputFile := filepath.Join(outputDir, taskID+".txt")
	timeout := time.Duration(maxInt(req.TimeoutMS, snapshot.Settings.Evolution.TimeoutMS)) * time.Millisecond
	if timeout <= 0 {
		timeout = 15 * time.Minute
	}
	args := []string{"-a", "never", "exec", "--json", "--sandbox", "workspace-write", "-o", outputFile}
	if strings.TrimSpace(req.Model) != "" {
		args = append(args, "--model", strings.TrimSpace(req.Model))
	}
	if strings.TrimSpace(req.ReasoningEffort) != "" {
		args = append(args, "--config", "model_reasoning_effort="+quoteCodexConfig(req.ReasoningEffort))
	}
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

func quoteCodexConfig(value string) string {
	escaped := strings.ReplaceAll(strings.TrimSpace(value), `"`, `\"`)
	return `"` + escaped + `"`
}
