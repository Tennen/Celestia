package automation

import (
	"testing"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

func TestMatchesTimeConditionSupportsIntervalSchedule(t *testing.T) {
	condition := &models.AutomationTimeCondition{
		Schedule:        "interval",
		WindowStart:     "08:00",
		WindowEnd:       "18:00",
		IntervalSeconds: 600,
		Timezone:        "UTC",
	}
	windowStart := time.Date(2026, 4, 28, 8, 0, 0, 0, time.UTC)
	if !matchesTimeCondition(windowStart, condition, nil) {
		t.Fatal("matchesTimeCondition() should accept the first interval slot")
	}
	last := windowStart
	if matchesTimeCondition(windowStart.Add(5*time.Minute), condition, &last) {
		t.Fatal("matchesTimeCondition() should reject duplicate trigger inside the same slot")
	}
	if !matchesTimeCondition(windowStart.Add(10*time.Minute), condition, &last) {
		t.Fatal("matchesTimeCondition() should accept the next interval slot")
	}
}
