package timeschedule

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const ScheduleDaily = "daily"

type Spec struct {
	Schedule string
	At       string
	Timezone string
}

func RunLoop(stop <-chan struct{}, interval time.Duration, tick func(time.Time)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	tick(time.Now())
	for {
		select {
		case <-stop:
			return
		case now := <-ticker.C:
			tick(now)
		}
	}
}

func Normalize(spec *Spec) (Spec, error) {
	if spec == nil {
		return Spec{}, errors.New("time condition is required")
	}
	schedule := strings.TrimSpace(spec.Schedule)
	if schedule == "" {
		schedule = ScheduleDaily
	}
	if schedule != ScheduleDaily {
		return Spec{}, fmt.Errorf("unsupported schedule %q", schedule)
	}
	at := strings.TrimSpace(spec.At)
	if _, err := ParseClockHM(at); err != nil {
		return Spec{}, fmt.Errorf("invalid at: %w", err)
	}
	timezone := strings.TrimSpace(spec.Timezone)
	if timezone != "" {
		if _, err := time.LoadLocation(timezone); err != nil {
			return Spec{}, fmt.Errorf("invalid timezone: %w", err)
		}
	}
	return Spec{Schedule: schedule, At: at, Timezone: timezone}, nil
}

func Matches(now time.Time, spec *Spec, lastTriggeredAt *time.Time) bool {
	if spec == nil || spec.Schedule != ScheduleDaily {
		return false
	}
	minutes, err := ParseClockHM(spec.At)
	if err != nil {
		return false
	}
	location := time.Local
	if strings.TrimSpace(spec.Timezone) != "" {
		loaded, loadErr := time.LoadLocation(spec.Timezone)
		if loadErr != nil {
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

func ParseClockHM(value string) (int, error) {
	parsed, err := time.Parse("15:04", strings.TrimSpace(value))
	if err != nil {
		return 0, err
	}
	return parsed.Hour()*60 + parsed.Minute(), nil
}
