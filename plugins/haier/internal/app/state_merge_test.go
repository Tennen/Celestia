package app

import "testing"

func TestMergeHaierStateUpdateUpdatesDerivedKeys(t *testing.T) {
	current := map[string]any{
		"machMode":          "1",
		"machine_status":    "running",
		"prPhase":           "4",
		"phase":             "4",
		"remainingTimeMM":   "18",
		"remaining_minutes": 18,
	}

	next := mergeHaierStateUpdate(current, map[string]string{
		"machMode":        "0",
		"prPhase":         "12",
		"remainingTimeMM": "0",
	})

	if got := next["machine_status"]; got != "idle" {
		t.Fatalf("machine_status = %#v, want idle", got)
	}
	if got := next["phase"]; got != "12" {
		t.Fatalf("phase = %#v, want 12", got)
	}
	if got := next["remaining_minutes"]; got != 0 {
		t.Fatalf("remaining_minutes = %#v, want 0", got)
	}
	if got := next["prPhase"]; got != "12" {
		t.Fatalf("prPhase = %#v, want 12", got)
	}
}

func TestMergeHaierStateUpdateClearsDerivedKeysWhenSourceIsEmpty(t *testing.T) {
	current := map[string]any{
		"prCode":      "7",
		"program":     "7",
		"prPhase":     "12",
		"phase":       "12",
		"tempLevel":   "40",
		"temperature": 40,
		"spinSpeed":   "1200",
		"spin_speed":  1200,
	}

	next := mergeHaierStateUpdate(current, map[string]string{
		"prCode":    "",
		"prPhase":   "",
		"tempLevel": "0",
		"spinSpeed": "0",
	})

	if _, ok := next["program"]; ok {
		t.Fatal("program should be removed when prCode becomes empty")
	}
	if _, ok := next["phase"]; ok {
		t.Fatal("phase should be removed when prPhase becomes empty")
	}
	if _, ok := next["temperature"]; ok {
		t.Fatal("temperature should be removed when tempLevel becomes 0")
	}
	if _, ok := next["spin_speed"]; ok {
		t.Fatal("spin_speed should be removed when spinSpeed becomes 0")
	}
}
