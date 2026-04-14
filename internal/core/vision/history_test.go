package vision

import (
	"context"
	"errors"
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
				"event_status":  "threshold_met",
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

	ruleEvents, err := service.RuleEvents(ctx, "feeder-zone", EventHistoryFilter{Limit: 10})
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

func TestDeleteRuleEventRemovesPersistedEventAndCaptures(t *testing.T) {
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
		},
	}); err != nil {
		t.Fatalf("UpsertVisionConfig() error = %v", err)
	}

	for _, event := range []models.Event{
		{
			ID:       "vision-delete",
			Type:     models.EventDeviceOccurred,
			PluginID: "hikvision",
			DeviceID: cameraID,
			TS:       now.Add(-2 * time.Minute),
			Payload: map[string]any{
				"capability_id": models.VisionCapabilityID,
				"rule_id":       "feeder-zone",
				"rule_name":     "Feeder Zone",
				"event_status":  "threshold_met",
				"entity_value":  "cat",
			},
		},
		{
			ID:       "vision-keep",
			Type:     models.EventDeviceOccurred,
			PluginID: "hikvision",
			DeviceID: cameraID,
			TS:       now.Add(-1 * time.Minute),
			Payload: map[string]any{
				"capability_id": models.VisionCapabilityID,
				"rule_id":       "feeder-zone",
				"rule_name":     "Feeder Zone",
				"event_status":  "threshold_met",
				"entity_value":  "dog",
			},
		},
	} {
		if err := store.AppendEvent(ctx, event); err != nil {
			t.Fatalf("AppendEvent(%s) error = %v", event.ID, err)
		}
	}

	for _, asset := range []models.VisionEventCaptureAsset{
		{
			Capture: models.VisionEventCapture{
				CaptureID:      "vision-delete:start",
				EventID:        "vision-delete",
				RuleID:         "feeder-zone",
				CameraDeviceID: cameraID,
				Phase:          models.VisionEventCapturePhaseStart,
				CapturedAt:     now.Add(-2 * time.Minute),
				ContentType:    "image/jpeg",
				SizeBytes:      3,
			},
			Data: []byte("one"),
		},
		{
			Capture: models.VisionEventCapture{
				CaptureID:      "vision-keep:start",
				EventID:        "vision-keep",
				RuleID:         "feeder-zone",
				CameraDeviceID: cameraID,
				Phase:          models.VisionEventCapturePhaseStart,
				CapturedAt:     now.Add(-1 * time.Minute),
				ContentType:    "image/jpeg",
				SizeBytes:      3,
			},
			Data: []byte("two"),
		},
	} {
		if err := store.UpsertVisionEventCapture(ctx, asset); err != nil {
			t.Fatalf("UpsertVisionEventCapture(%s) error = %v", asset.Capture.CaptureID, err)
		}
	}

	if err := service.DeleteRuleEvent(ctx, "feeder-zone", "vision-delete"); err != nil {
		t.Fatalf("DeleteRuleEvent() error = %v", err)
	}

	if _, ok, err := store.GetEvent(ctx, "vision-delete"); err != nil {
		t.Fatalf("GetEvent(vision-delete) error = %v", err)
	} else if ok {
		t.Fatal("deleted event still exists")
	}
	if _, ok, err := store.GetVisionEventCapture(ctx, "vision-delete:start"); err != nil {
		t.Fatalf("GetVisionEventCapture(vision-delete:start) error = %v", err)
	} else if ok {
		t.Fatal("deleted capture still exists")
	}
	if _, ok, err := store.GetEvent(ctx, "vision-keep"); err != nil {
		t.Fatalf("GetEvent(vision-keep) error = %v", err)
	} else if !ok {
		t.Fatal("kept event missing after delete")
	}

	events, err := service.RuleEvents(ctx, "feeder-zone", EventHistoryFilter{Limit: 10})
	if err != nil {
		t.Fatalf("RuleEvents() error = %v", err)
	}
	if len(events) != 1 || events[0].ID != "vision-keep" {
		t.Fatalf("RuleEvents() after delete = %#v, want only vision-keep", events)
	}
}

func TestDeleteRuleEventRejectsEventsOutsideRule(t *testing.T) {
	ctx := context.Background()
	service, _, _, store := newVisionTestService(t)

	now := time.Now().UTC().Truncate(time.Second)
	cameraID := visionTestCamera().ID
	if err := store.UpsertVisionConfig(ctx, models.VisionCapabilityConfig{
		RecognitionEnabled: true,
		UpdatedAt:          now,
		Rules: []models.VisionRule{
			{ID: "feeder-zone", Name: "Feeder Zone", CameraDeviceID: cameraID},
			{ID: "water-zone", Name: "Water Zone", CameraDeviceID: cameraID},
		},
	}); err != nil {
		t.Fatalf("UpsertVisionConfig() error = %v", err)
	}
	if err := store.AppendEvent(ctx, models.Event{
		ID:       "vision-other-rule",
		Type:     models.EventDeviceOccurred,
		PluginID: "hikvision",
		DeviceID: cameraID,
		TS:       now,
		Payload: map[string]any{
			"capability_id": models.VisionCapabilityID,
			"rule_id":       "water-zone",
			"rule_name":     "Water Zone",
			"event_status":  "threshold_met",
			"entity_value":  "cat",
		},
	}); err != nil {
		t.Fatalf("AppendEvent(vision-other-rule) error = %v", err)
	}

	err := service.DeleteRuleEvent(ctx, "feeder-zone", "vision-other-rule")
	if !errors.Is(err, ErrVisionEventNotFound) {
		t.Fatalf("DeleteRuleEvent() error = %v, want ErrVisionEventNotFound", err)
	}
}

func TestRuleEventsSupportsDateRangeAndCursor(t *testing.T) {
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
		},
	}); err != nil {
		t.Fatalf("UpsertVisionConfig() error = %v", err)
	}

	for idx, eventTS := range []time.Time{
		now.Add(-15 * time.Minute),
		now.Add(-45 * time.Minute),
		now.Add(-75 * time.Minute),
	} {
		if err := store.AppendEvent(ctx, models.Event{
			ID:       fmt.Sprintf("vision-range-%d", idx+1),
			Type:     models.EventDeviceOccurred,
			PluginID: "hikvision",
			DeviceID: cameraID,
			TS:       eventTS,
			Payload: map[string]any{
				"capability_id": models.VisionCapabilityID,
				"rule_id":       "feeder-zone",
				"rule_name":     "Feeder Zone",
				"event_status":  "threshold_met",
				"entity_value":  "cat",
			},
		}); err != nil {
			t.Fatalf("AppendEvent(%d) error = %v", idx, err)
		}
	}

	fromTS := now.Add(-80 * time.Minute)
	toTS := now.Add(-20 * time.Minute)
	items, err := service.RuleEvents(ctx, "feeder-zone", EventHistoryFilter{
		FromTS: &fromTS,
		ToTS:   &toTS,
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("RuleEvents(range) error = %v", err)
	}
	if len(items) != 2 || items[0].ID != "vision-range-2" || items[1].ID != "vision-range-3" {
		t.Fatalf("RuleEvents(range) = %#v, want [vision-range-2 vision-range-3]", items)
	}

	beforeTS := items[0].TS
	items, err = service.RuleEvents(ctx, "feeder-zone", EventHistoryFilter{
		FromTS:   &fromTS,
		ToTS:     &toTS,
		BeforeTS: &beforeTS,
		BeforeID: items[0].ID,
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("RuleEvents(cursor) error = %v", err)
	}
	if len(items) != 1 || items[0].ID != "vision-range-3" {
		t.Fatalf("RuleEvents(cursor) = %#v, want [vision-range-3]", items)
	}
}
