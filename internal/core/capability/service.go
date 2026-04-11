package capability

import (
	"context"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/core/automation"
	"github.com/chentianyu/celestia/internal/core/vision"
	"github.com/chentianyu/celestia/internal/models"
)

type Service struct {
	automation *automation.Service
	vision     *vision.Service
}

func New(automationSvc *automation.Service, visionSvc *vision.Service) *Service {
	return &Service{
		automation: automationSvc,
		vision:     visionSvc,
	}
}

func (s *Service) List(ctx context.Context) ([]models.Capability, error) {
	automationItem, err := s.automationCapability(ctx)
	if err != nil {
		return nil, err
	}
	visionItem, err := s.visionCapability(ctx)
	if err != nil {
		return nil, err
	}
	return []models.Capability{automationItem, visionItem}, nil
}

func (s *Service) Get(ctx context.Context, id string) (models.CapabilityDetail, bool, error) {
	switch strings.TrimSpace(id) {
	case string(models.CapabilityKindAutomation):
		return s.automationDetail(ctx)
	case models.VisionCapabilityID:
		return s.visionDetail(ctx)
	default:
		return models.CapabilityDetail{}, false, nil
	}
}

func (s *Service) automationCapability(ctx context.Context) (models.Capability, error) {
	detail, _, err := s.automationDetail(ctx)
	if err != nil {
		return models.Capability{}, err
	}
	return detail.Capability, nil
}

func (s *Service) automationDetail(ctx context.Context) (models.CapabilityDetail, bool, error) {
	automations, err := s.automation.List(ctx)
	if err != nil {
		return models.CapabilityDetail{}, false, err
	}
	var (
		enabledCount    int
		lastTriggeredAt *time.Time
		status          = models.HealthStateHealthy
	)
	for _, item := range automations {
		if item.Enabled {
			enabledCount++
		}
		if item.LastRunStatus == models.AutomationRunStatusFailed {
			status = models.HealthStateDegraded
		}
		if item.LastTriggeredAt != nil && (lastTriggeredAt == nil || item.LastTriggeredAt.After(*lastTriggeredAt)) {
			lastTriggeredAt = item.LastTriggeredAt
		}
	}
	updatedAt := time.Now().UTC()
	if len(automations) > 0 {
		updatedAt = automations[0].UpdatedAt
	}
	detail := models.CapabilityDetail{
		Capability: models.Capability{
			ID:          string(models.CapabilityKindAutomation),
			Kind:        models.CapabilityKindAutomation,
			Name:        "Automations",
			Description: "Core-owned state-change automations that execute device actions.",
			Enabled:     true,
			Status:      status,
			Summary: map[string]any{
				"total":             len(automations),
				"enabled_count":     enabledCount,
				"last_triggered_at": lastTriggeredAt,
			},
			UpdatedAt: updatedAt,
		},
		Automation: &models.AutomationCapabilityDetail{
			Total:           len(automations),
			EnabledCount:    enabledCount,
			LastTriggeredAt: lastTriggeredAt,
		},
	}
	return detail, true, nil
}

func (s *Service) visionCapability(ctx context.Context) (models.Capability, error) {
	detail, _, err := s.visionDetail(ctx)
	if err != nil {
		return models.Capability{}, err
	}
	return detail.Capability, nil
}

func (s *Service) visionDetail(ctx context.Context) (models.CapabilityDetail, bool, error) {
	visionDetail, err := s.vision.Detail(ctx)
	if err != nil {
		return models.CapabilityDetail{}, false, err
	}
	summary := map[string]any{
		"service_ws_url":     visionDetail.Config.ServiceWSURL,
		"model_name":         visionDetail.Config.ModelName,
		"rule_count":         len(visionDetail.Config.Rules),
		"enabled_rule_count": enabledRuleCount(visionDetail.Config.Rules),
		"last_event_at":      visionDetail.Runtime.LastEventAt,
		"last_synced_at":     visionDetail.Runtime.LastSyncedAt,
	}
	if visionDetail.Catalog != nil {
		summary["entity_count"] = len(visionDetail.Catalog.Entities)
		summary["catalog_fetched_at"] = visionDetail.Catalog.FetchedAt
		summary["catalog_service_ws_url"] = visionDetail.Catalog.ServiceWSURL
		summary["catalog_model_name"] = visionDetail.Catalog.ModelName
	}
	detail := models.CapabilityDetail{
		Capability: models.Capability{
			ID:          models.VisionCapabilityID,
			Kind:        models.CapabilityKindVisionEntityStayZone,
			Name:        "Vision Stay Zone Recognition",
			Description: "Gateway-managed stay-zone control plane for independent vision processing services.",
			Enabled:     visionDetail.Config.RecognitionEnabled,
			Status:      visionDetail.Runtime.Status,
			Summary:     summary,
			UpdatedAt:   latestVisionUpdate(visionDetail),
		},
		Vision: &visionDetail,
	}
	return detail, true, nil
}

func enabledRuleCount(rules []models.VisionRule) int {
	count := 0
	for _, rule := range rules {
		if rule.Enabled {
			count++
		}
	}
	return count
}

func latestVisionUpdate(detail models.VisionCapabilityDetail) time.Time {
	updatedAt := detail.Config.UpdatedAt
	if detail.Runtime.UpdatedAt.After(updatedAt) {
		updatedAt = detail.Runtime.UpdatedAt
	}
	if updatedAt.IsZero() {
		return time.Now().UTC()
	}
	return updatedAt
}
