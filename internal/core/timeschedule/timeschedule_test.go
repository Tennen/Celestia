package timeschedule

import (
	"testing"
	"time"
)

func TestNormalizeDefaultsDailySchedule(t *testing.T) {
	spec, err := Normalize(&Spec{At: "08:30", Timezone: "UTC"})
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if spec.Schedule != ScheduleDaily {
		t.Fatalf("Schedule = %q, want %q", spec.Schedule, ScheduleDaily)
	}
}

func TestNormalizeIntervalSchedule(t *testing.T) {
	spec, err := Normalize(&Spec{
		Schedule:        ScheduleInterval,
		WindowStart:     "08:00",
		WindowEnd:       "18:00",
		IntervalSeconds: 600,
		Timezone:        "UTC",
	})
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if spec.Schedule != ScheduleInterval {
		t.Fatalf("Schedule = %q, want %q", spec.Schedule, ScheduleInterval)
	}
	if spec.WindowStart != "08:00" || spec.WindowEnd != "18:00" {
		t.Fatalf("window = %s -> %s, want 08:00 -> 18:00", spec.WindowStart, spec.WindowEnd)
	}
	if spec.IntervalSeconds != 600 {
		t.Fatalf("IntervalSeconds = %d, want 600", spec.IntervalSeconds)
	}
}

func TestMatchesRejectsDuplicateMinuteTriggers(t *testing.T) {
	now := time.Date(2026, 4, 28, 8, 30, 0, 0, time.UTC)
	last := now
	if Matches(now, &Spec{Schedule: ScheduleDaily, At: "08:30", Timezone: "UTC"}, &last) {
		t.Fatal("Matches() should reject duplicate trigger in the same minute")
	}
	if !Matches(now.Add(24*time.Hour), &Spec{Schedule: ScheduleDaily, At: "08:30", Timezone: "UTC"}, &last) {
		t.Fatal("Matches() should accept the next day's trigger")
	}
}

func TestMatchesIntervalTriggersPerSlotWithinWindow(t *testing.T) {
	spec := &Spec{
		Schedule:        ScheduleInterval,
		WindowStart:     "08:00",
		WindowEnd:       "18:00",
		IntervalSeconds: 600,
		Timezone:        "UTC",
	}
	windowStart := time.Date(2026, 4, 28, 8, 0, 0, 0, time.UTC)
	if Matches(windowStart.Add(-time.Minute), spec, nil) {
		t.Fatal("Matches() should reject times before the interval window")
	}
	if !Matches(windowStart, spec, nil) {
		t.Fatal("Matches() should accept the first interval slot")
	}
	last := windowStart
	if Matches(windowStart.Add(5*time.Minute), spec, &last) {
		t.Fatal("Matches() should reject duplicate trigger inside the same interval slot")
	}
	nextSlot := windowStart.Add(10 * time.Minute)
	if !Matches(nextSlot, spec, &last) {
		t.Fatal("Matches() should accept the next interval slot")
	}
	last = nextSlot
	if Matches(time.Date(2026, 4, 28, 18, 0, 0, 0, time.UTC), spec, &last) {
		t.Fatal("Matches() should reject the interval window end boundary")
	}
}

func TestMatchesIntervalSupportsOvernightWindows(t *testing.T) {
	spec := &Spec{
		Schedule:        ScheduleInterval,
		WindowStart:     "22:00",
		WindowEnd:       "02:00",
		IntervalSeconds: 3600,
		Timezone:        "UTC",
	}
	now := time.Date(2026, 4, 29, 1, 0, 0, 0, time.UTC)
	if !Matches(now, spec, nil) {
		t.Fatal("Matches() should accept interval slots inside an overnight window")
	}
}
