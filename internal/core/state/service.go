package state

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

func (s *Service) Upsert(ctx context.Context, snapshots []models.DeviceStateSnapshot) error {
	for _, snapshot := range snapshots {
		if err := s.store.UpsertDeviceState(ctx, snapshot); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) Get(ctx context.Context, deviceID string) (models.DeviceStateSnapshot, bool, error) {
	return s.store.GetDeviceState(ctx, deviceID)
}

func (s *Service) List(ctx context.Context, filter storage.StateFilter) ([]models.DeviceStateSnapshot, error) {
	return s.store.ListDeviceStates(ctx, filter)
}

