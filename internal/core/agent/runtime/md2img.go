package runtime

import (
	"context"
	"errors"

	"github.com/chentianyu/celestia/internal/core/agent/workflows/renderer"
	"github.com/chentianyu/celestia/internal/models"
)

func (s *Service) RunMarkdownRender(ctx context.Context, req models.AgentMarkdownRenderRequest) (models.AgentMarkdownRenderResult, error) {
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return models.AgentMarkdownRenderResult{}, err
	}
	settings := snapshot.Settings.MD2Img
	if !settings.Enabled {
		return models.AgentMarkdownRenderResult{}, errors.New("md2img is disabled in agent settings")
	}
	return renderer.RenderMarkdown(ctx, req, settings)
}
