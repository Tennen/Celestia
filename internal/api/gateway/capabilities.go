package gateway

import (
	"context"
	"fmt"
	"net/http"

	"github.com/chentianyu/celestia/internal/models"
)

func (s *HTTPService) ListCapabilities(ctx context.Context) ([]models.Capability, error) {
	var out []models.Capability
	if err := s.get(ctx, "/api/v1/capabilities", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *HTTPService) GetCapability(ctx context.Context, id string) (models.CapabilityDetail, error) {
	var out models.CapabilityDetail
	if err := s.get(ctx, fmt.Sprintf("/api/v1/capabilities/%s", id), nil, &out); err != nil {
		return models.CapabilityDetail{}, err
	}
	return out, nil
}

func (s *HTTPService) SaveVisionCapabilityConfig(ctx context.Context, config models.VisionCapabilityConfig) (models.CapabilityDetail, error) {
	var out models.CapabilityDetail
	path := fmt.Sprintf("/api/v1/capabilities/%s", models.VisionCapabilityID)
	if err := s.request(ctx, http.MethodPut, path, nil, config, &out, ""); err != nil {
		return models.CapabilityDetail{}, err
	}
	return out, nil
}

func (s *HTTPService) RefreshVisionEntityCatalog(ctx context.Context, req models.VisionEntityCatalogRefreshRequest) (models.VisionEntityCatalog, error) {
	var out models.VisionEntityCatalog
	path := fmt.Sprintf("/api/v1/capabilities/%s/entities/refresh", models.VisionCapabilityID)
	if err := s.request(ctx, http.MethodPost, path, nil, req, &out, ""); err != nil {
		return models.VisionEntityCatalog{}, err
	}
	return out, nil
}

func (s *HTTPService) ReportVisionCapabilityStatus(ctx context.Context, report models.VisionServiceStatusReport) (models.VisionCapabilityStatus, error) {
	var out models.VisionCapabilityStatus
	path := fmt.Sprintf("/api/v1/capabilities/%s/status", models.VisionCapabilityID)
	if err := s.request(ctx, http.MethodPost, path, nil, report, &out, ""); err != nil {
		return models.VisionCapabilityStatus{}, err
	}
	return out, nil
}

func (s *HTTPService) ReportVisionCapabilityEvents(ctx context.Context, batch models.VisionServiceEventBatch) error {
	path := fmt.Sprintf("/api/v1/capabilities/%s/events", models.VisionCapabilityID)
	return s.request(ctx, http.MethodPost, path, nil, batch, nil, "")
}
