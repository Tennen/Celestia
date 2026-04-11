package vision

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

func TestVisionHistoryQueriesUsePersistedEventsBeyondInitialWindow(t *testing.T) {
	ctx := context.Background()
	service, _, _, store := newVisionTestService(t)

	now := time.Now().UTC().Truncate(time.Second)
	cameraID := visionTestCamera().ID
	if err := store.UpsertVisionConfig(ctx, models.VisionCapabilityConfig{
		RecognitionEnabled:         true,
		EventCaptureRetentionHours: 168,
		UpdatedAt:                  now,
		Rules: []models.VisionRule{
			{ID: "feeder-zone", Name: "Feeder Zone", CameraDeviceID: cameraID},
			{ID: "water-zone", Name: "Water Zone", CameraDeviceID: cameraID},
		},
	}); err != nil {
		t.Fatalf("UpsertVisionConfig() error = %v", err)
	}

	for idx := range 120 {
		if err := store.AppendEvent(ctx, models.Event{
			ID:       fmt.Sprintf("noise-event-%03d", idx),
			Type:     models.EventDeviceStateChanged,
			PluginID: "xiaomi",
			DeviceID: "xiaomi:cn:noise",
			TS:       now.Add(-time.Duration(idx+6) * time.Minute),
			Payload: map[string]any{
				"state": map[string]any{"power": idx%2 == 0},
			},
		}); err != nil {
			t.Fatalf("AppendEvent(noise %d) error = %v", idx, err)
		}
	}

	for _, event := range []models.Event{
		{
			ID:       "vision-recent",
			Type:     models.EventDeviceOccurred,
			PluginID: "hikvision",
			DeviceID: cameraID,
			TS:       now.Add(-5 * time.Minute),
			Payload: map[string]any{
				"capability_id": models.VisionCapabilityID,
				"rule_id":       "feeder-zone",
				"rule_name":     "Feeder Zone",
				"event_status":  "threshold_met",
				"entity_value":  "cat",
			},
		},
		{
			ID:       "vision-other-rule",
			Type:     models.EventDeviceOccurred,
			PluginID: "hikvision",
			DeviceID: cameraID,
			TS:       now.Add(-30 * time.Minute),
			Payload: map[string]any{
				"capability_id": models.VisionCapabilityID,
				"rule_id":       "water-zone",
				"rule_name":     "Water Zone",
				"event_status":  "threshold_met",
				"entity_value":  "cat",
			},
		},
		{
			ID:       "vision-older",
			Type:     models.EventDeviceOccurred,
			PluginID: "hikvision",
			DeviceID: cameraID,
			TS:       now.Add(-4 * time.Hour),
			Payload: map[string]any{
				"capability_id": models.VisionCapabilityID,
				"rule_id":       "feeder-zone",
				"rule_name":     "Feeder Zone",
				"event_status":  "cleared",
				"entity_value":  "cat",
			},
		},
		{
			ID:       "vision-expired",
			Type:     models.EventDeviceOccurred,
			PluginID: "hikvision",
			DeviceID: cameraID,
			TS:       now.Add(-(8 * 24 * time.Hour)),
			Payload: map[string]any{
				"capability_id": models.VisionCapabilityID,
				"rule_id":       "feeder-zone",
				"rule_name":     "Feeder Zone",
				"event_status":  "threshold_met",
				"entity_value":  "cat",
			},
		},
	} {
		if err := store.AppendEvent(ctx, event); err != nil {
			t.Fatalf("AppendEvent(%s) error = %v", event.ID, err)
		}
	}

	recentEvents, err := service.RecentEvents(ctx, 10)
	if err != nil {
		t.Fatalf("RecentEvents() error = %v", err)
	}
	if len(recentEvents) != 3 {
		t.Fatalf("RecentEvents() len = %d, want 3", len(recentEvents))
	}
	if recentEvents[0].ID != "vision-recent" || recentEvents[1].ID != "vision-other-rule" || recentEvents[2].ID != "vision-older" {
		t.Fatalf("RecentEvents() ids = [%s %s %s], want [vision-recent vision-other-rule vision-older]", recentEvents[0].ID, recentEvents[1].ID, recentEvents[2].ID)
	}

	ruleEvents, err := service.RuleEvents(ctx, "feeder-zone", 10)
	if err != nil {
		t.Fatalf("RuleEvents() error = %v", err)
	}
	if len(ruleEvents) != 2 {
		t.Fatalf("RuleEvents() len = %d, want 2", len(ruleEvents))
	}
	if ruleEvents[0].ID != "vision-recent" || ruleEvents[1].ID != "vision-older" {
		t.Fatalf("RuleEvents() ids = [%s %s], want [vision-recent vision-older]", ruleEvents[0].ID, ruleEvents[1].ID)
	}
}
