package automation

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
)

const automationTimeScheduleDaily = "daily"

func (s *Service) runTimeScheduler() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	s.handleTimeTick(time.Now())
	for {
		select {
		case <-s.stop:
			return
		case now := <-ticker.C:
			s.handleTimeTick(now)
		}
	}
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
	if condition == nil {
		return models.AutomationTimeCondition{}, errors.New("time condition is required")
	}
	schedule := strings.TrimSpace(condition.Schedule)
	if schedule == "" {
		schedule = automationTimeScheduleDaily
	}
	if schedule != automationTimeScheduleDaily {
		return models.AutomationTimeCondition{}, fmt.Errorf("unsupported schedule %q", schedule)
	}
	at := strings.TrimSpace(condition.At)
	if _, err := parseClockHM(at); err != nil {
		return models.AutomationTimeCondition{}, fmt.Errorf("invalid at: %w", err)
	}
	timezone := strings.TrimSpace(condition.Timezone)
	if timezone != "" {
		if _, err := time.LoadLocation(timezone); err != nil {
			return models.AutomationTimeCondition{}, fmt.Errorf("invalid timezone: %w", err)
		}
	}
	return models.AutomationTimeCondition{Schedule: schedule, At: at, Timezone: timezone}, nil
}

func matchesTimeCondition(now time.Time, condition *models.AutomationTimeCondition, lastTriggeredAt *time.Time) bool {
	if condition == nil || condition.Schedule != automationTimeScheduleDaily {
		return false
	}
	minutes, err := parseClockHM(condition.At)
	if err != nil {
		return false
	}
	location := time.Local
	if strings.TrimSpace(condition.Timezone) != "" {
		loaded, loadErr := time.LoadLocation(condition.Timezone)
		if loadErr != nil {
			log.Printf("automation: invalid timezone %q: %v", condition.Timezone, loadErr)
			return false
		}
		location = loaded
	}
	localNow := now.In(location)
	currentMinutes := localNow.Hour()*60 + localNow.Minute()
	if currentMinutes != minutes {
		return false
	}
	if lastTriggeredAt == nil {
		return true
	}
	last := lastTriggeredAt.In(location)
	return last.Year() != localNow.Year() ||
		last.Month() != localNow.Month() ||
		last.Day() != localNow.Day() ||
		last.Hour()*60+last.Minute() != minutes
}
