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
