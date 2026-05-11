package runtime

import (
	"context"

	"github.com/chentianyu/celestia/internal/core/workflow/renderer"
	"github.com/chentianyu/celestia/internal/models"
)

func (s *Service) RunScreenshot(ctx context.Context, req models.AgentScreenshotRequest) (models.AgentScreenshotResult, error) {
	return renderer.CaptureWebPage(ctx, req)
}
