package renderer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	scriptPath := bundledRendererScript()
	cmd := exec.CommandContext(reqCtx, "/bin/sh", "-lc", rendererCommand(settings.Command, scriptPath))
	if root := repositoryRoot(scriptPath); root != "" {
		cmd.Dir = root
	}
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

const rendererScriptRel = "internal/core/workflow/renderer/md2img/render.mjs"

func bundledRendererCommand(scriptPath string) string {
	if strings.TrimSpace(scriptPath) == "" {
		return "node " + rendererScriptRel
	}
	return "node " + shellQuote(scriptPath)
}

func rendererCommand(configured string, scriptPath string) string {
	command := strings.TrimSpace(configured)
	if command == "" || isBundledRendererCommand(command) {
		return bundledRendererCommand(scriptPath)
	}
	return command
}

func isBundledRendererCommand(command string) bool {
	normalized := filepath.ToSlash(command)
	return strings.Contains(normalized, rendererScriptRel) ||
		strings.Contains(normalized, "internal/core/agent/workflows/renderer/md2img/render.mjs") ||
		strings.Contains(normalized, "internal/core/agent/md2img/render.mjs")
}

func bundledRendererScript() string {
	if _, file, _, ok := runtime.Caller(0); ok {
		if path := filepath.Join(filepath.Dir(file), "md2img", "render.mjs"); fileExists(path) {
			return path
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		if path := findFromParents(cwd, rendererScriptRel); path != "" {
			return path
		}
	}
	if exe, err := os.Executable(); err == nil {
		if path := findFromParents(filepath.Dir(exe), rendererScriptRel); path != "" {
			return path
		}
	}
	return ""
}

func repositoryRoot(scriptPath string) string {
	if strings.TrimSpace(scriptPath) != "" {
		if root := findGoModRoot(filepath.Dir(scriptPath)); root != "" {
			return root
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		return findGoModRoot(cwd)
	}
	return ""
}

func findFromParents(start string, rel string) string {
	for dir := filepath.Clean(start); ; dir = filepath.Dir(dir) {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if fileExists(path) {
			return path
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
	}
}

func findGoModRoot(start string) string {
	for dir := filepath.Clean(start); ; dir = filepath.Dir(dir) {
		if fileExists(filepath.Join(dir, "go.mod")) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
