package runtime

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

func (s *Service) RunTerminal(ctx context.Context, req models.AgentTerminalRequest) (models.AgentTerminalResult, error) {
	if err := requireText(req.Command, "command"); err != nil {
		return models.AgentTerminalResult{}, err
	}
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return models.AgentTerminalResult{}, err
	}
	config := snapshot.Settings.Terminal
	if !config.Enabled {
		return models.AgentTerminalResult{}, errors.New("terminal execution is disabled")
	}
	started := time.Now().UTC()
	timeout := time.Duration(maxInt(config.TimeoutMS, 30000)) * time.Millisecond
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cwd := firstNonEmpty(req.CWD, config.CWD)
	cmd := exec.CommandContext(runCtx, "/bin/sh", "-lc", req.Command)
	if strings.TrimSpace(cwd) != "" {
		cmd.Dir = cwd
	}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err = cmd.Run()
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
		Command:    req.Command,
		CWD:        cwd,
		ExitCode:   exitCode,
		Output:     strings.TrimSpace(output.String()),
		StartedAt:  started,
		FinishedAt: finished,
	}
	if runCtx.Err() == context.DeadlineExceeded {
		result.Output = result.Output + "\ncommand timed out after " + strconv.Itoa(config.TimeoutMS) + "ms"
		return result, runCtx.Err()
	}
	return result, err
}

func floatString(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func intString(value int) string {
	return strconv.Itoa(value)
}
