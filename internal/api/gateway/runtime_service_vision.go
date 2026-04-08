package gateway

import (
	"context"
	"errors"
	"net/http"

	"github.com/chentianyu/celestia/internal/models"
)

func (s *RuntimeService) SaveVisionCapabilityConfig(ctx context.Context, config models.VisionCapabilityConfig) (models.CapabilityDetail, error) {
	item, err := s.runtime.Vision.SaveConfig(ctx, config)
	if err != nil {
		return models.CapabilityDetail{}, statusError(http.StatusBadRequest, err)
	}
	detail, ok, err := s.runtime.Capability.Get(ctx, models.VisionCapabilityID)
	if err != nil {
		return models.CapabilityDetail{}, statusError(http.StatusInternalServerError, err)
	}
	if !ok {
		return models.CapabilityDetail{}, statusError(http.StatusNotFound, errors.New("capability not found"))
	}
	detail.Vision = &item
	return detail, nil
}

func (s *RuntimeService) RefreshVisionEntityCatalog(ctx context.Context, req models.VisionEntityCatalogRefreshRequest) (models.VisionEntityCatalog, error) {
	item, err := s.runtime.Vision.RefreshCatalog(ctx, req)
	if err != nil {
		return models.VisionEntityCatalog{}, statusError(http.StatusBadRequest, err)
	}
	return item, nil
}

func (s *RuntimeService) ReportVisionCapabilityStatus(ctx context.Context, report models.VisionServiceStatusReport) (models.VisionCapabilityStatus, error) {
	item, err := s.runtime.Vision.ReportStatus(ctx, report)
	if err != nil {
		return models.VisionCapabilityStatus{}, statusError(http.StatusBadRequest, err)
	}
	return item, nil
}

func (s *RuntimeService) ReportVisionCapabilityEvents(ctx context.Context, batch models.VisionServiceEventBatch) error {
	if err := s.runtime.Vision.ReportEvents(ctx, batch); err != nil {
		return statusError(http.StatusBadRequest, err)
	}
	return nil
}

func (s *RuntimeService) ReportVisionCapabilityEvidence(ctx context.Context, batch models.VisionServiceEventCaptureBatch) error {
	if err := s.runtime.Vision.ReportEvidence(ctx, batch); err != nil {
		return statusError(http.StatusBadRequest, err)
	}
	return nil
}

func (s *RuntimeService) GetVisionEventCapture(ctx context.Context, captureID string) (models.VisionEventCaptureAsset, error) {
	item, ok, err := s.runtime.Vision.GetCaptureAsset(ctx, captureID)
	if err != nil {
		return models.VisionEventCaptureAsset{}, statusError(http.StatusInternalServerError, err)
	}
	if !ok {
		return models.VisionEventCaptureAsset{}, statusError(http.StatusNotFound, errors.New("vision capture not found"))
	}
	return item, nil
}
