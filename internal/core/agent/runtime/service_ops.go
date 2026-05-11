package runtime

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

type ServiceOperationRequest struct {
	Action string `json:"action"`
	Lines  int    `json:"lines,omitempty"`
}

func (s *Service) RunServiceOperation(ctx context.Context, req ServiceOperationRequest) (models.AgentTerminalResult, error) {
	action := normalizeServiceOperation(req.Action)
	if action == "" {
		return models.AgentTerminalResult{}, errors.New("unsupported service operation")
	}
	script := findServiceScript()
	if script == "" {
		return models.AgentTerminalResult{}, errors.New("tool/celestia-service.sh not found")
	}
	command := shellServiceCommand(script, action, req.Lines)
	return runServiceShell(ctx, command, filepath.Dir(filepath.Dir(script)))
}

func shellServiceCommand(script string, action string, lines int) string {
	if action == "logs" {
		return evolutionShellQuote(script) + " logs " + strconv.Itoa(maxInt(lines, 120))
	}
	return evolutionShellQuote(script) + " " + action
}

func normalizeServiceOperation(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "start", "stop", "restart", "status", "logs":
		return strings.ToLower(strings.TrimSpace(action))
	default:
		return ""
	}
}

func runServiceShell(ctx context.Context, command string, cwd string) (models.AgentTerminalResult, error) {
	started := time.Now().UTC()
	runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(runCtx, "/bin/sh", "-lc", command)
	cmd.Dir = cwd
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	finished := time.Now().UTC()
	exitCode := 0
	if err != nil {
		exitCode = 1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
	}
	result := models.AgentTerminalResult{
		Command:    command,
		CWD:        cwd,
		ExitCode:   exitCode,
		Output:     strings.TrimSpace(output.String()),
		StartedAt:  started,
		FinishedAt: finished,
	}
	if runCtx.Err() == context.DeadlineExceeded {
		result.Output = strings.TrimSpace(result.Output + "\nservice operation timed out")
		return result, runCtx.Err()
	}
	return result, err
}

func findServiceScript() string {
	if cwd, err := os.Getwd(); err == nil {
		if path := findFileFromParents(cwd, "tool/celestia-service.sh"); path != "" {
			return path
		}
	}
	if exe, err := os.Executable(); err == nil {
		return findFileFromParents(filepath.Dir(exe), "tool/celestia-service.sh")
	}
	return ""
}

func findFileFromParents(start string, rel string) string {
	for dir := filepath.Clean(start); ; dir = filepath.Dir(dir) {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
	}
}
