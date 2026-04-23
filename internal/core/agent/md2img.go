package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

func (s *Service) RunMarkdownRender(ctx context.Context, req models.AgentMarkdownRenderRequest) (models.AgentMarkdownRenderResult, error) {
	if strings.TrimSpace(req.Markdown) == "" {
		return models.AgentMarkdownRenderResult{}, errors.New("markdown is required")
	}
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return models.AgentMarkdownRenderResult{}, err
	}
	settings := snapshot.Settings.MD2Img
	if !settings.Enabled {
		return models.AgentMarkdownRenderResult{}, errors.New("md2img is disabled in agent settings")
	}
	mode := firstNonEmpty(req.Mode, settings.Mode, "long-image")
	outputDir := firstNonEmpty(req.OutputDir, settings.OutputDir, "data/renderer/md2img")
	payload := map[string]any{
		"markdown":   req.Markdown,
		"mode":       mode,
		"output_dir": outputDir,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return models.AgentMarkdownRenderResult{}, err
	}

	timeout := time.Duration(maxInt(settings.TimeoutMS, 60000)) * time.Millisecond
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(reqCtx, "/bin/sh", "-lc", firstNonEmpty(settings.Command, "node internal/core/renderer/md2img/render.mjs"))
	cmd.Stdin = bytes.NewReader(raw)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if reqCtx.Err() == context.DeadlineExceeded {
			return models.AgentMarkdownRenderResult{}, fmt.Errorf("md2img timeout after %dms", settings.TimeoutMS)
		}
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return models.AgentMarkdownRenderResult{}, fmt.Errorf("md2img render failed: %s", detail)
	}
	var result models.AgentMarkdownRenderResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return models.AgentMarkdownRenderResult{}, fmt.Errorf("md2img returned invalid JSON: %w", err)
	}
	if len(result.Images) == 0 {
		return models.AgentMarkdownRenderResult{}, errors.New("md2img produced no images")
	}
	result.Mode = firstNonEmpty(result.Mode, mode)
	result.OutputDir = firstNonEmpty(result.OutputDir, outputDir)
	result.SourceChars = len([]rune(req.Markdown))
	if result.RenderedAt.IsZero() {
		result.RenderedAt = time.Now().UTC()
	}
	return result, nil
}
