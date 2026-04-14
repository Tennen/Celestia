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

var (
	ErrVisionRuleNotFound  = errors.New("vision rule not found")
	ErrVisionEventNotFound = errors.New("vision event not found")
)

const (
	defaultVisionHistoryLimit = 12
	maxVisionHistoryPageSize  = 200
)

type EventHistoryFilter struct {
	FromTS   *time.Time
	ToTS     *time.Time
	BeforeTS *time.Time
	BeforeID string
	Limit    int
}

func (s *Service) RecentEvents(ctx context.Context, limit int) ([]models.Event, error) {
	config, err := s.GetConfig(ctx)
	if err != nil {
		return nil, err
	}
	return s.listHistoryEvents(ctx, config, "", EventHistoryFilter{Limit: limit})
}

func (s *Service) RuleEvents(ctx context.Context, ruleID string, filter EventHistoryFilter) ([]models.Event, error) {
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
	return s.listHistoryEvents(ctx, config, ruleID, filter)
}

func (s *Service) DeleteRuleEvent(ctx context.Context, ruleID, eventID string) error {
	ruleID = strings.TrimSpace(ruleID)
	if ruleID == "" {
		return errors.New("vision rule id is required")
	}
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return errors.New("vision event id is required")
	}
	config, err := s.GetConfig(ctx)
	if err != nil {
		return err
	}
	if !hasVisionRule(config.Rules, ruleID) {
		return fmt.Errorf("%w: %s", ErrVisionRuleNotFound, ruleID)
	}

	event, ok, err := s.store.GetEvent(ctx, eventID)
	if err != nil {
		return err
	}
	if !ok || !isVisionOccurredEvent(event) || visionEventRuleID(event) != ruleID {
		return fmt.Errorf("%w: %s", ErrVisionEventNotFound, eventID)
	}
	return s.store.DeleteVisionEvent(ctx, eventID)
}

func (s *Service) listHistoryEvents(
	ctx context.Context,
	config models.VisionCapabilityConfig,
	ruleID string,
	filter EventHistoryFilter,
) ([]models.Event, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = defaultVisionHistoryLimit
	}

	pageSize := max(limit*4, 100)
	if pageSize > maxVisionHistoryPageSize {
		pageSize = maxVisionHistoryPageSize
	}

	cutoff := time.Now().UTC().Add(-time.Duration(normalizeCaptureRetentionHours(config.EventCaptureRetentionHours)) * time.Hour)
	fromTS := cutoff
	if filter.FromTS != nil && filter.FromTS.After(fromTS) {
		fromTS = filter.FromTS.UTC()
	}
	storeFilter := storage.EventFilter{
		Type:     models.EventDeviceOccurred,
		FromTS:   &fromTS,
		ToTS:     filter.ToTS,
		BeforeTS: filter.BeforeTS,
		BeforeID: filter.BeforeID,
		Limit:    pageSize,
	}
	out := make([]models.Event, 0, min(limit, pageSize))

	for len(out) < limit {
		page, err := s.store.ListEvents(ctx, storeFilter)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}

		stop := false
		for _, item := range page {
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
		if stop || len(page) < storeFilter.Limit {
			break
		}

		last := page[len(page)-1]
		beforeTS := last.TS.UTC()
		storeFilter.BeforeTS = &beforeTS
		storeFilter.BeforeID = last.ID
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
