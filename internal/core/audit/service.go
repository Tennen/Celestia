package audit

import (
	"context"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/storage"
)

type Service struct {
	store storage.Store
}

func New(store storage.Store) *Service {
	return &Service{store: store}
}

func (s *Service) Append(ctx context.Context, record models.AuditRecord) error {
	return s.store.AppendAudit(ctx, record)
}

func (s *Service) List(ctx context.Context, filter storage.AuditFilter) ([]models.AuditRecord, error) {
	return s.store.ListAudits(ctx, filter)
}

func (s *Service) Count(ctx context.Context) (int, error) {
	return s.store.CountAudits(ctx)
}

