package audit

import (
	"context"
	"sync"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/storage"
)

type Service struct {
	store storage.Store

	mu          sync.RWMutex
	subscribers map[int]chan models.AuditRecord
	nextID      int
}

func New(store storage.Store) *Service {
	return &Service{
		store:       store,
		subscribers: make(map[int]chan models.AuditRecord),
	}
}

func (s *Service) Append(ctx context.Context, record models.AuditRecord) error {
	if err := s.store.AppendAudit(ctx, record); err != nil {
		return err
	}
	s.publish(record)
	return nil
}

func (s *Service) List(ctx context.Context, filter storage.AuditFilter) ([]models.AuditRecord, error) {
	return s.store.ListAudits(ctx, filter)
}

func (s *Service) Count(ctx context.Context) (int, error) {
	return s.store.CountAudits(ctx)
}

func (s *Service) Subscribe(buffer int) (int, <-chan models.AuditRecord) {
	if buffer <= 0 {
		buffer = 32
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.nextID
	s.nextID++
	ch := make(chan models.AuditRecord, buffer)
	s.subscribers[id] = ch
	return id, ch
}

func (s *Service) Unsubscribe(id int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch, ok := s.subscribers[id]
	if !ok {
		return
	}
	delete(s.subscribers, id)
	close(ch)
}

func (s *Service) publish(record models.AuditRecord) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, ch := range s.subscribers {
		select {
		case ch <- record:
		default:
		}
	}
}
