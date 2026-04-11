package vision

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/storage"
)

var ErrVisionRuleNotFound = errors.New("vision rule not found")

const (
	defaultVisionHistoryLimit = 12
	maxVisionHistoryPageSize  = 200
)

func (s *Service) RecentEvents(ctx context.Context, limit int) ([]models.Event, error) {
	config, err := s.GetConfig(ctx)
	if err != nil {
		return nil, err
	}
	return s.listHistoryEvents(ctx, config, "", limit)
}

func (s *Service) RuleEvents(ctx context.Context, ruleID string, limit int) ([]models.Event, error) {
	ruleID = strings.TrimSpace(ruleID)
	if ruleID == "" {
		return nil, errors.New("vision rule id is required")
	}
	config, err := s.GetConfig(ctx)
	if err != nil {
		return nil, err
	}
	if !hasVisionRule(config.Rules, ruleID) {
		return nil, fmt.Errorf("%w: %s", ErrVisionRuleNotFound, ruleID)
	}
	return s.listHistoryEvents(ctx, config, ruleID, limit)
}

func (s *Service) listHistoryEvents(
	ctx context.Context,
	config models.VisionCapabilityConfig,
	ruleID string,
	limit int,
) ([]models.Event, error) {
	if limit <= 0 {
		limit = defaultVisionHistoryLimit
	}

	pageSize := max(limit*4, 100)
	if pageSize > maxVisionHistoryPageSize {
		pageSize = maxVisionHistoryPageSize
	}

	cutoff := time.Now().UTC().Add(-time.Duration(normalizeCaptureRetentionHours(config.EventCaptureRetentionHours)) * time.Hour)
	filter := storage.EventFilter{
		Type:  models.EventDeviceOccurred,
		Limit: pageSize,
	}
	out := make([]models.Event, 0, min(limit, pageSize))

	for len(out) < limit {
		page, err := s.store.ListEvents(ctx, filter)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}

		stop := false
		for _, item := range page {
			if item.TS.Before(cutoff) {
				stop = true
				break
			}
			if !isVisionOccurredEvent(item) {
				continue
			}
			if ruleID != "" && visionEventRuleID(item) != ruleID {
				continue
			}
			out = append(out, item)
			if len(out) >= limit {
				stop = true
				break
			}
		}
		if stop || len(page) < filter.Limit {
			break
		}

		last := page[len(page)-1]
		beforeTS := last.TS.UTC()
		filter.BeforeTS = &beforeTS
		filter.BeforeID = last.ID
	}

	return s.EnrichEvents(ctx, out)
}

func hasVisionRule(rules []models.VisionRule, ruleID string) bool {
	for _, rule := range rules {
		if strings.TrimSpace(rule.ID) == ruleID {
			return true
		}
	}
	return false
}

func visionEventRuleID(event models.Event) string {
	ruleID, _ := event.Payload["rule_id"].(string)
	return strings.TrimSpace(ruleID)
}
