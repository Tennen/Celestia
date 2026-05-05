package timeschedule

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const ScheduleDaily = "daily"
const ScheduleInterval = "interval"
const DefaultTickInterval = 5 * time.Second

type Spec struct {
	Schedule        string
	At              string
	Timezone        string
	IntervalStart   string
	IntervalEnd     string
	IntervalSeconds int
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
	timezone := strings.TrimSpace(spec.Timezone)
	if timezone != "" {
		if _, err := time.LoadLocation(timezone); err != nil {
			return Spec{}, fmt.Errorf("invalid timezone: %w", err)
		}
	}
	switch schedule {
	case ScheduleDaily:
		at := strings.TrimSpace(spec.At)
		if _, err := ParseClockHM(at); err != nil {
			return Spec{}, fmt.Errorf("invalid at: %w", err)
		}
		return Spec{Schedule: schedule, At: at, Timezone: timezone}, nil
	case ScheduleInterval:
		intervalStart := strings.TrimSpace(spec.IntervalStart)
		intervalEnd := strings.TrimSpace(spec.IntervalEnd)
		if _, err := ParseClockHM(intervalStart); err != nil {
			return Spec{}, fmt.Errorf("invalid interval start: %w", err)
		}
		if _, err := ParseClockHM(intervalEnd); err != nil {
			return Spec{}, fmt.Errorf("invalid interval end: %w", err)
		}
		if spec.IntervalSeconds <= 0 {
			return Spec{}, errors.New("interval_seconds must be greater than 0")
		}
		return Spec{
			Schedule:        schedule,
			Timezone:        timezone,
			IntervalStart:   intervalStart,
			IntervalEnd:     intervalEnd,
			IntervalSeconds: spec.IntervalSeconds,
		}, nil
	default:
		return Spec{}, fmt.Errorf("unsupported schedule %q", schedule)
	}
}

func Matches(now time.Time, spec *Spec, lastTriggeredAt *time.Time) bool {
	if spec == nil {
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
	switch spec.Schedule {
	case ScheduleDaily:
		return matchesDaily(now, spec, lastTriggeredAt, location)
	case ScheduleInterval:
		return matchesInterval(now, spec, lastTriggeredAt, location)
	default:
		return false
	}
}

func matchesDaily(now time.Time, spec *Spec, lastTriggeredAt *time.Time, location *time.Location) bool {
	minutes, err := ParseClockHM(spec.At)
	if err != nil {
		return false
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

func matchesInterval(now time.Time, spec *Spec, lastTriggeredAt *time.Time, location *time.Location) bool {
	windowStart, windowEnd, ok := intervalWindowBounds(now.In(location), spec.IntervalStart, spec.IntervalEnd, location)
	if !ok {
		return false
	}
	localNow := now.In(location)
	if localNow.Before(windowStart) || !localNow.Before(windowEnd) {
		return false
	}
	slotStart := currentIntervalSlotStart(localNow, windowStart, time.Duration(spec.IntervalSeconds)*time.Second)
	if !slotStart.Before(windowEnd) {
		return false
	}
	if lastTriggeredAt == nil {
		return true
	}
	return lastTriggeredAt.In(location).Before(slotStart)
}

func intervalWindowBounds(localNow time.Time, rawStart string, rawEnd string, location *time.Location) (time.Time, time.Time, bool) {
	startMinutes, err := ParseClockHM(rawStart)
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	endMinutes, err := ParseClockHM(rawEnd)
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	start := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), startMinutes/60, startMinutes%60, 0, 0, location)
	end := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), endMinutes/60, endMinutes%60, 0, 0, location)
	currentMinutes := localNow.Hour()*60 + localNow.Minute()
	switch {
	case startMinutes == endMinutes:
		if localNow.Before(start) {
			start = start.Add(-24 * time.Hour)
		}
		end = start.Add(24 * time.Hour)
	case startMinutes < endMinutes:
		// Same-day window.
	default:
		if currentMinutes < endMinutes {
			start = start.Add(-24 * time.Hour)
		} else {
			end = end.Add(24 * time.Hour)
		}
	}
	return start, end, true
}

func currentIntervalSlotStart(localNow time.Time, windowStart time.Time, interval time.Duration) time.Time {
	if interval <= 0 || localNow.Before(windowStart) {
		return windowStart
	}
	elapsed := localNow.Sub(windowStart)
	return windowStart.Add((elapsed / interval) * interval)
}

func ParseClockHM(value string) (int, error) {
	parsed, err := time.Parse("15:04", strings.TrimSpace(value))
	if err != nil {
		return 0, err
	}
	return parsed.Hour()*60 + parsed.Minute(), nil
}
