package gateway

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

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

func (s *HTTPService) GetVisionEventCapture(ctx context.Context, captureID string) (models.VisionEventCaptureAsset, error) {
	path := fmt.Sprintf("/api/v1/capabilities/%s/captures/%s", models.VisionCapabilityID, url.PathEscape(captureID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+path, nil)
	if err != nil {
		return models.VisionEventCaptureAsset{}, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return models.VisionEventCaptureAsset{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return models.VisionEventCaptureAsset{}, fmt.Errorf("request failed with %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return models.VisionEventCaptureAsset{}, err
	}
	return models.VisionEventCaptureAsset{
		Capture: models.VisionEventCapture{
			CaptureID:   captureID,
			ContentType: resp.Header.Get("Content-Type"),
			SizeBytes:   len(data),
		},
		Data: data,
	}, nil
}
