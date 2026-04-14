package vision

import (
	"context"

	"github.com/chentianyu/celestia/internal/core/eventbus"
	"github.com/chentianyu/celestia/internal/core/registry"
	"github.com/chentianyu/celestia/internal/core/state"
	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/storage"
)

const (
	defaultVisionThresholdSeconds = 5
	visionStatePrefix             = "vision_rule_"
)

type Service struct {
	store    storage.Store
	registry *registry.Service
	state    *state.Service
	bus      *eventbus.Bus
	session  *sessionManager
}

func New(store storage.Store, registrySvc *registry.Service, stateSvc *state.Service, bus *eventbus.Bus) *Service {
	service := &Service{
		store:    store,
		registry: registrySvc,
		state:    stateSvc,
		bus:      bus,
	}
	service.session = newSessionManager(service)
	return service
}

func (s *Service) Init(ctx context.Context) error {
	return s.session.Init(ctx)
}

func (s *Service) Close() {
	if s.session != nil {
		s.session.Close()
	}
}

func (s *Service) GetConfig(ctx context.Context) (models.VisionCapabilityConfig, error) {
	config, ok, err := s.store.GetVisionConfig(ctx)
	if err != nil {
		return models.VisionCapabilityConfig{}, err
	}
	if !ok {
		return defaultConfig(), nil
	}
	return hydrateVisionConfigKeyEntities(config), nil
}

func (s *Service) GetStatus(ctx context.Context) (models.VisionCapabilityStatus, error) {
	status, ok, err := s.store.GetVisionStatus(ctx)
	if err != nil {
		return models.VisionCapabilityStatus{}, err
	}
	if !ok {
		return defaultStatus(defaultConfig()), nil
	}
	return status, nil
}

func (s *Service) Detail(ctx context.Context) (models.VisionCapabilityDetail, error) {
	config, err := s.GetConfig(ctx)
	if err != nil {
		return models.VisionCapabilityDetail{}, err
	}
	catalog, ok, err := s.GetCatalog(ctx)
	if err != nil {
		return models.VisionCapabilityDetail{}, err
	}
	status, err := s.GetStatus(ctx)
	if err != nil {
		return models.VisionCapabilityDetail{}, err
	}
	events, err := s.RecentEvents(ctx, 12)
	if err != nil {
		return models.VisionCapabilityDetail{}, err
	}
	detail := models.VisionCapabilityDetail{
		Config:       config,
		Runtime:      status,
		RecentEvents: events,
	}
	if ok {
		detail.Catalog = &catalog
	}
	return detail, nil
}
