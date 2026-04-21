package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

func (s *Service) Transcribe(ctx context.Context, req models.AgentSpeechRequest) (models.AgentSpeechResult, error) {
	if err := requireText(req.AudioPath, "audio_path"); err != nil {
		return models.AgentSpeechResult{}, err
	}
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return models.AgentSpeechResult{}, err
	}
	config := snapshot.Settings.STT
	if !config.Enabled {
		return models.AgentSpeechResult{}, errors.New("STT is disabled")
	}
	if strings.TrimSpace(config.Provider) != "" && strings.TrimSpace(config.Provider) != "fast-whisper" {
		return models.AgentSpeechResult{}, errors.New("unsupported STT provider: " + config.Provider)
	}
	if _, err := os.Stat(req.AudioPath); err != nil {
		return models.AgentSpeechResult{}, err
	}
	timeout := time.Duration(maxInt(config.TimeoutMS, 180000)) * time.Millisecond
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	command := firstNonEmpty(config.Command, "python3 tools/fast-whisper-transcribe.py --audio "+shellQuote(req.AudioPath))
	cmd := exec.CommandContext(runCtx, "/bin/sh", "-lc", command)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return models.AgentSpeechResult{}, errors.New(strings.TrimSpace(firstNonEmpty(stderr.String(), stdout.String(), err.Error())))
	}
	text := parseSTTText(stdout.String())
	return models.AgentSpeechResult{Text: text, Provider: "fast-whisper", At: time.Now().UTC()}, nil
}

func parseSTTText(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for idx := len(lines) - 1; idx >= 0; idx-- {
		line := strings.TrimSpace(lines[idx])
		if line == "" {
			continue
		}
		var payload struct {
			Text string `json:"text"`
		}
		if json.Unmarshal([]byte(line), &payload) == nil && strings.TrimSpace(payload.Text) != "" {
			return strings.TrimSpace(payload.Text)
		}
		return line
	}
	return ""
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
