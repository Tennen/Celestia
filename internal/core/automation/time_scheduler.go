package automation

import (
	"context"
	"time"

	"github.com/chentianyu/celestia/internal/core/timeschedule"
	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
)

func (s *Service) runTimeScheduler() {
	timeschedule.RunLoop(s.stop, 30*time.Second, s.handleTimeTick)
}

func (s *Service) handleTimeTick(now time.Time) {
	automations := s.timeAutomations()
	if len(automations) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	for _, automation := range automations {
		condition, ok := timeCondition(automation.Conditions)
		if !ok || !matchesTimeCondition(now, condition.Time, automation.LastTriggeredAt) {
			continue
		}
		if !matchesTimeWindow(now, automation.TimeWindow) {
			continue
		}
		sourceEvent := models.Event{
			ID:   uuid.NewString(),
			Type: "automation.time",
			TS:   now.UTC(),
			Payload: map[string]any{
				"trigger":       "time",
				"schedule":      condition.Time.Schedule,
				"at":            condition.Time.At,
				"timezone":      condition.Time.Timezone,
				"automation_id": automation.ID,
			},
		}
		ok, err := s.matchesStateConditions(ctx, automation)
		if err != nil {
			s.updateRunResult(ctx, automation, models.AutomationRunStatusFailed, err.Error())
			s.publishAutomationEvent(models.EventAutomationFailed, automation, sourceEvent, err.Error(), nil)
			continue
		}
		if !ok {
			continue
		}
		if err := s.executeAutomation(ctx, automation, sourceEvent); err != nil {
			s.updateRunResult(ctx, automation, models.AutomationRunStatusFailed, err.Error())
			s.publishAutomationEvent(models.EventAutomationFailed, automation, sourceEvent, err.Error(), nil)
			continue
		}
		s.updateRunResult(ctx, automation, models.AutomationRunStatusSucceeded, "")
		s.publishAutomationEvent(models.EventAutomationTriggered, automation, sourceEvent, "", nil)
	}
}

func normalizeTimeCondition(condition *models.AutomationTimeCondition) (models.AutomationTimeCondition, error) {
	normalized, err := timeschedule.Normalize(&timeschedule.Spec{
		Schedule: condition.Schedule,
		At:       condition.At,
		Timezone: condition.Timezone,
	})
	if err != nil {
		return models.AutomationTimeCondition{}, err
	}
	return models.AutomationTimeCondition{
		Schedule: normalized.Schedule,
		At:       normalized.At,
		Timezone: normalized.Timezone,
	}, nil
}

func matchesTimeCondition(now time.Time, condition *models.AutomationTimeCondition, lastTriggeredAt *time.Time) bool {
	if condition == nil {
		return false
	}
	return timeschedule.Matches(now, &timeschedule.Spec{
		Schedule: condition.Schedule,
		At:       condition.At,
		Timezone: condition.Timezone,
	}, lastTriggeredAt)
}
