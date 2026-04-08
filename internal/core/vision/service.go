package vision

import (
	"context"
	"net/http"
	"time"

	"github.com/chentianyu/celestia/internal/core/eventbus"
	"github.com/chentianyu/celestia/internal/core/registry"
	"github.com/chentianyu/celestia/internal/core/state"
	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/storage"
)

const (
	defaultVisionThresholdSeconds = 5
	visionStatePrefix             = "vision_rule_"
	visionConfigSyncPath          = "/api/v1/capabilities/" + models.VisionCapabilityID
	visionStatusCallbackPath      = "/api/v1/capabilities/" + models.VisionCapabilityID + "/status"
	visionEventCallbackPath       = "/api/v1/capabilities/" + models.VisionCapabilityID + "/events"
	visionEvidenceCallbackPath    = "/api/v1/capabilities/" + models.VisionCapabilityID + "/evidence"
)

type Service struct {
	store    storage.Store
	registry *registry.Service
	state    *state.Service
	bus      *eventbus.Bus
	client   *http.Client
}

func New(store storage.Store, registrySvc *registry.Service, stateSvc *state.Service, bus *eventbus.Bus) *Service {
	return &Service{
		store:    store,
		registry: registrySvc,
		state:    stateSvc,
		bus:      bus,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
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
	return config, nil
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

func (s *Service) RecentEvents(ctx context.Context, limit int) ([]models.Event, error) {
	items, err := s.store.ListEvents(ctx, storage.EventFilter{Limit: max(limit*6, 60)})
	if err != nil {
		return nil, err
	}
	out := make([]models.Event, 0, min(limit, len(items)))
	for _, item := range items {
		if item.Type != models.EventDeviceOccurred {
			continue
		}
		if capabilityID, _ := item.Payload["capability_id"].(string); capabilityID != models.VisionCapabilityID {
			continue
		}
		out = append(out, item)
		if len(out) >= limit {
			break
		}
	}
	return s.EnrichEvents(ctx, out)
}
