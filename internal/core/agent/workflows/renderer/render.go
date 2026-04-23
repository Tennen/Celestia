package renderer

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

func RenderMarkdown(ctx context.Context, req models.AgentMarkdownRenderRequest, settings models.AgentMD2ImgConfig) (models.AgentMarkdownRenderResult, error) {
	if strings.TrimSpace(req.Markdown) == "" {
		return models.AgentMarkdownRenderResult{}, errors.New("markdown is required")
	}
	mode := firstNonEmpty(req.Mode, settings.Mode, "long-image")
	outputDir := firstNonEmpty(req.OutputDir, settings.OutputDir, "data/agent/renderer/md2img")
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
	cmd := exec.CommandContext(reqCtx, "/bin/sh", "-lc", firstNonEmpty(settings.Command, "node internal/core/agent/workflows/renderer/md2img/render.mjs"))
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func maxInt(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}
