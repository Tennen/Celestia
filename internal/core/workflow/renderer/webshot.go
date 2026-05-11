package renderer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

const webshotScriptRel = "internal/core/workflow/renderer/webshot/capture.mjs"

func CaptureWebPage(ctx context.Context, req models.AgentScreenshotRequest) (models.AgentScreenshotResult, error) {
	if err := validateScreenshotURL(req.URL); err != nil {
		return models.AgentScreenshotResult{}, err
	}
	outputDir := firstNonEmpty(req.OutputDir, "data/agent/screenshots")
	payload := map[string]any{
		"url":        strings.TrimSpace(req.URL),
		"output_dir": outputDir,
		"width":      maxInt(req.Width, 1440),
		"height":     maxInt(req.Height, 1000),
		"full_page":  req.FullPage,
		"wait_ms":    maxInt(req.WaitMS, 500),
		"timeout_ms": maxInt(req.TimeoutMS, 30000),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return models.AgentScreenshotResult{}, err
	}
	timeout := time.Duration(maxInt(req.TimeoutMS, 30000)+maxInt(req.WaitMS, 500)+5000) * time.Millisecond
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	scriptPath := bundledWebshotScript()
	cmd := exec.CommandContext(runCtx, "/bin/sh", "-lc", bundledRendererCommand(scriptPath))
	if root := repositoryRoot(scriptPath); root != "" {
		cmd.Dir = root
	}
	cmd.Stdin = bytes.NewReader(raw)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if runCtx.Err() == context.DeadlineExceeded {
			return models.AgentScreenshotResult{}, errors.New("screenshot capture timed out")
		}
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return models.AgentScreenshotResult{}, fmt.Errorf("screenshot capture failed: %s", detail)
	}
	var result models.AgentScreenshotResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return models.AgentScreenshotResult{}, fmt.Errorf("screenshot returned invalid JSON: %w", err)
	}
	if strings.TrimSpace(result.Image.Path) == "" {
		return models.AgentScreenshotResult{}, errors.New("screenshot produced no image")
	}
	return result, nil
}

func validateScreenshotURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return errors.New("valid http url is required")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("screenshot url must use http or https")
	}
	host := parsed.Hostname()
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("screenshot url must target localhost or loopback")
	}
	return nil
}

func bundledWebshotScript() string {
	if _, file, _, ok := runtime.Caller(0); ok {
		if path := filepath.Join(filepath.Dir(file), "webshot", "capture.mjs"); fileExists(path) {
			return path
		}
	}
	return findFromParents(".", webshotScriptRel)
}
