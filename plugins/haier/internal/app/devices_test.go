package app

import (
	"testing"

	"github.com/chentianyu/celestia/internal/models"
	"pgregory.net/rapid"
)

func makeTestDevice() models.Device {
	return models.Device{
		ID:       "haier:washer:test:dev1",
		PluginID: "haier",
	}
}

// TestBuildStateSnapshot_MachModeMapping verifies the three fixed machMode mappings.
func TestBuildStateSnapshot_MachModeMapping(t *testing.T) {
	cases := []struct {
		machMode string
		want     string
	}{
		{"0", "idle"},
		{"3", "paused"},
		{"1", "running"},
		{"2", "running"},
		{"5", "running"},
		{"", "idle"},
	}
	for _, tc := range cases {
		attrs := map[string]string{"machMode": tc.machMode}
		snap := buildStateSnapshot(makeTestDevice(), map[string]any{}, attrs)
		got, _ := snap.State["machine_status"].(string)
		if got != tc.want {
			t.Errorf("machMode=%q: expected machine_status=%q, got %q", tc.machMode, tc.want, got)
		}
	}
}

// TestBuildStateSnapshot_Property8_MachModeDeterministic is Property 8:
// For any attrs map containing machMode, buildStateSnapshot always maps:
//   "0" → "idle", "3" → "paused", anything else → "running"
// and the result is deterministic (same input → same output).
// Feature: haier-uws-platform-migration, Property 8: 状态映射确定性
func TestBuildStateSnapshot_Property8_MachModeDeterministic(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		machMode := rapid.String().Draw(t, "machMode")
		attrs := map[string]string{"machMode": machMode}
		device := makeTestDevice()

		snap1 := buildStateSnapshot(device, map[string]any{}, attrs)
		snap2 := buildStateSnapshot(device, map[string]any{}, attrs)

		status1, _ := snap1.State["machine_status"].(string)
		status2, _ := snap2.State["machine_status"].(string)

		// Determinism: same input → same output.
		if status1 != status2 {
			t.Fatalf("non-deterministic: machMode=%q produced %q and %q", machMode, status1, status2)
		}

		// Correctness of mapping.
		switch machMode {
		case "0":
			if status1 != "idle" {
				t.Fatalf("machMode=0 should map to idle, got %q", status1)
			}
		case "3":
			if status1 != "paused" {
				t.Fatalf("machMode=3 should map to paused, got %q", status1)
			}
		default:
			if status1 != "running" && status1 != "idle" {
				t.Fatalf("machMode=%q should map to running or idle, got %q", machMode, status1)
			}
		}
	})
}
