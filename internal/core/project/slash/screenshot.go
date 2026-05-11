package slash

import (
	"context"
	"errors"
	"fmt"

	"github.com/chentianyu/celestia/internal/models"
)

func (s *Service) runScreenshot(ctx context.Context, args []string) (string, map[string]any, error) {
	if s.agent == nil {
		return "", nil, errors.New("agent runtime is not configured")
	}
	params, values, err := parseSlashParams(args)
	if err != nil {
		return "", nil, err
	}
	if len(values) == 0 {
		return "", nil, errors.New("screenshot url is required")
	}
	result, err := s.agent.RunScreenshot(ctx, models.AgentScreenshotRequest{
		URL:       values[0],
		Width:     intParam(params, "width"),
		Height:    intParam(params, "height"),
		WaitMS:    intParam(params, "wait_ms"),
		FullPage:  boolParam(params, "full_page", "full"),
		OutputDir: stringParam(params, "output_dir"),
	})
	if err != nil {
		return "", nil, err
	}
	image := models.ProjectOutputImage{
		Path:        result.Image.Path,
		ContentType: result.Image.ContentType,
		Filename:    "screenshot.png",
	}
	return fmt.Sprintf("Screenshot captured: %s", result.Image.Path), map[string]any{
		"domain": "screenshot",
		"url":    result.URL,
		"images": []models.ProjectOutputImage{image},
	}, nil
}

func intParam(params map[string]any, key string) int {
	value, ok := params[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func boolParam(params map[string]any, keys ...string) bool {
	for _, key := range keys {
		if value, ok := params[key]; ok {
			typed, _ := value.(bool)
			return typed
		}
	}
	return false
}
