package runtime

import (
	"context"

	corellm "github.com/chentianyu/celestia/internal/core/llm"
)

func (s *Service) GenerateText(ctx context.Context, prompt string) (string, error) {
	return s.GenerateTextWithProvider(ctx, "", prompt)
}

func (s *Service) GenerateTextWithProvider(ctx context.Context, providerID string, prompt string) (string, error) {
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return "", err
	}
	return corellm.GenerateTextWithProvider(ctx, snapshot.Settings, providerID, prompt)
}
