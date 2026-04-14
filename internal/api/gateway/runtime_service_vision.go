package gateway

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/chentianyu/celestia/internal/core/vision"
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
	if detail.Vision != nil {
		item.Catalog = detail.Vision.Catalog
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

func (s *RuntimeService) ListVisionRuleEvents(ctx context.Context, ruleID string, filter VisionRuleEventFilter) ([]models.Event, error) {
	if filter.FromTS != nil && filter.ToTS != nil && !filter.FromTS.Before(*filter.ToTS) {
		return nil, statusError(http.StatusBadRequest, errors.New("from_ts must be before to_ts"))
	}
	items, err := s.runtime.Vision.RuleEvents(ctx, ruleID, vision.EventHistoryFilter{
		FromTS:   filter.FromTS,
		ToTS:     filter.ToTS,
		BeforeTS: filter.BeforeTS,
		BeforeID: filter.BeforeID,
		Limit:    filter.Limit,
	})
	if err != nil {
		if errors.Is(err, vision.ErrVisionRuleNotFound) {
			return nil, statusError(http.StatusNotFound, err)
		}
		return nil, statusError(http.StatusBadRequest, err)
	}
	return items, nil
}

func (s *RuntimeService) DeleteVisionRuleEvent(ctx context.Context, ruleID string, eventID string) error {
	if strings.TrimSpace(ruleID) == "" {
		return statusError(http.StatusBadRequest, errors.New("vision rule id is required"))
	}
	if strings.TrimSpace(eventID) == "" {
		return statusError(http.StatusBadRequest, errors.New("vision event id is required"))
	}
	if err := s.runtime.Vision.DeleteRuleEvent(ctx, ruleID, eventID); err != nil {
		switch {
		case errors.Is(err, vision.ErrVisionRuleNotFound), errors.Is(err, vision.ErrVisionEventNotFound):
			return statusError(http.StatusNotFound, err)
		default:
			return statusError(http.StatusInternalServerError, err)
		}
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
