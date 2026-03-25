package registry

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

func (s *Service) Upsert(ctx context.Context, devices []models.Device) error {
	for _, device := range devices {
		if err := s.store.UpsertDevice(ctx, device); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) List(ctx context.Context, filter storage.DeviceFilter) ([]models.Device, error) {
	return s.store.ListDevices(ctx, filter)
}

func (s *Service) Get(ctx context.Context, deviceID string) (models.Device, bool, error) {
	return s.store.GetDevice(ctx, deviceID)
}

func (s *Service) DeleteByPlugin(ctx context.Context, pluginID string) error {
	return s.store.DeleteDevicesByPlugin(ctx, pluginID)
}

