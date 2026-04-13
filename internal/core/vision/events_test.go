package vision

import (
	"context"
	"testing"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/storage"
	"nhooyr.io/websocket"
)

func TestReportEventsProjectsDeviceStateChange(t *testing.T) {
	ctx := context.Background()
	service, registrySvc, stateSvc, store := newVisionTestService(t)

	camera := visionTestCamera()
	if err := registrySvc.Upsert(ctx, []models.Device{camera}); err != nil {
		t.Fatalf("registry.Upsert() error = %v", err)
	}

	server := newVisionWSTestServer(t, func(serverCtx context.Context, conn *websocket.Conn) {
		sendTestEnvelope(t, serverCtx, conn, visionMessageTypeHello, "", wsHelloPayload{SchemaVersion: visionWSSchemaVersion, ConnectedAt: time.Now().UTC()})
		sendTestEnvelope(t, serverCtx, conn, visionMessageTypeRuntimeStatus, "", models.VisionServiceStatusReport{Status: models.HealthStateUnknown, ReportedAt: time.Now().UTC()})

		selectModel := readTestEnvelope(t, serverCtx, conn)
		sendTestEnvelope(t, serverCtx, conn, visionMessageTypeModelSelected, selectModel.RequestID, wsModelSelectedPayload{OK: true, ChangedAt: time.Now().UTC()})

		syncConfig := readTestEnvelope(t, serverCtx, conn)
		sendTestEnvelope(t, serverCtx, conn, visionMessageTypeSyncApplied, syncConfig.RequestID, wsSyncAppliedPayload{OK: true, AppliedAt: time.Now().UTC()})
	})

	if _, err := service.SaveConfig(ctx, models.VisionCapabilityConfig{
		ServiceWSURL:       wsURLFromHTTP(server.URL),
		RecognitionEnabled: true,
		Rules: []models.VisionRule{{
			ID:                   "feeder-zone",
			Name:                 "Feeder Zone",
			Enabled:              true,
			CameraDeviceID:       camera.ID,
			RecognitionEnabled:   true,
			RTSPSource:           models.VisionRTSPSource{URL: "rtsp://user:pass@camera/stream"},
			EntitySelector:       models.VisionEntitySelector{Kind: "label", Value: "cat"},
			Zone:                 models.VisionZoneBox{X: 0.1, Y: 0.2, Width: 0.3, Height: 0.4},
			StayThresholdSeconds: 5,
		}},
	}); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	subID, ch := service.bus.Subscribe(8)
	defer service.bus.Unsubscribe(subID)

	observedAt := time.Now().UTC()
	if err := service.ReportEvents(ctx, models.VisionServiceEventBatch{
		Events: []models.VisionServiceEvent{{
			EventID:      "evt-1",
			RuleID:       "feeder-zone",
			Status:       models.VisionServiceEventStatusThresholdMet,
			ObservedAt:   observedAt,
			DwellSeconds: 6,
			EntityValue:  "cat",
		}},
	}); err != nil {
		t.Fatalf("ReportEvents() error = %v", err)
	}

	snapshot, ok, err := stateSvc.Get(ctx, camera.ID)
	if err != nil {
		t.Fatalf("state.Get() error = %v", err)
	}
	if !ok {
		t.Fatal("camera state missing after ReportEvents()")
	}
	if intValue(snapshot.State["vision_rule_feeder-zone_match_count"]) != 1 {
		t.Fatalf("match_count = %#v, want 1", snapshot.State["vision_rule_feeder-zone_match_count"])
	}
	if snapshot.State["vision_rule_feeder-zone_active"] != false {
		t.Fatalf("active = %#v, want false", snapshot.State["vision_rule_feeder-zone_active"])
	}
	if snapshot.State["vision_rule_feeder-zone_last_status"] != "threshold_met" {
		t.Fatalf("last_status = %#v, want threshold_met", snapshot.State["vision_rule_feeder-zone_last_status"])
	}

	events, err := store.ListEvents(ctx, storage.EventFilter{Limit: 10})
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	if len(events) < 3 {
		t.Fatalf("events len = %d, want at least 3", len(events))
	}

	seenOccurred := false
	seenStateChanged := false
	timeout := time.After(2 * time.Second)
	for !seenOccurred || !seenStateChanged {
		select {
		case event := <-ch:
			if event.Type == models.EventDeviceOccurred {
				seenOccurred = true
			}
			if event.Type == models.EventDeviceStateChanged {
				seenStateChanged = true
			}
		case <-timeout:
			t.Fatalf("timed out waiting for projected events: occurred=%v state_changed=%v", seenOccurred, seenStateChanged)
		}
	}
}

func TestReportEventsPersistsReportedEntities(t *testing.T) {
	ctx := context.Background()
	service, registrySvc, stateSvc, store := newVisionTestService(t)

	camera := visionTestCamera()
	if err := registrySvc.Upsert(ctx, []models.Device{camera}); err != nil {
		t.Fatalf("registry.Upsert() error = %v", err)
	}
	if err := store.UpsertVisionConfig(ctx, models.VisionCapabilityConfig{
		RecognitionEnabled:         true,
		EventCaptureRetentionHours: 168,
		UpdatedAt:                  time.Now().UTC(),
		Rules: []models.VisionRule{{
			ID:                   "feeder-zone",
			Name:                 "Feeder Zone",
			Enabled:              true,
			CameraDeviceID:       camera.ID,
			RecognitionEnabled:   true,
			RTSPSource:           models.VisionRTSPSource{URL: "rtsp://user:pass@camera/stream"},
			EntitySelector:       models.VisionEntitySelector{Kind: "label", Value: ""},
			Zone:                 models.VisionZoneBox{X: 0.1, Y: 0.2, Width: 0.3, Height: 0.4},
			StayThresholdSeconds: 5,
		}},
	}); err != nil {
		t.Fatalf("UpsertVisionConfig() error = %v", err)
	}

	observedAt := time.Now().UTC()
	if err := service.ReportEvents(ctx, models.VisionServiceEventBatch{
		Events: []models.VisionServiceEvent{{
			EventID:      "evt-multi",
			RuleID:       "feeder-zone",
			Status:       models.VisionServiceEventStatusThresholdMet,
			ObservedAt:   observedAt,
			DwellSeconds: 8,
			Entities: []models.VisionEntityDescriptor{
				{Kind: "label", Value: "cat", DisplayName: "Cat"},
				{Kind: "label", Value: "dog", DisplayName: "Dog"},
			},
		}},
	}); err != nil {
		t.Fatalf("ReportEvents() error = %v", err)
	}

	snapshot, ok, err := stateSvc.Get(ctx, camera.ID)
	if err != nil {
		t.Fatalf("state.Get() error = %v", err)
	}
	if !ok {
		t.Fatal("camera state missing after ReportEvents()")
	}
	if snapshot.State["vision_rule_feeder-zone_last_entity_value"] != "Cat, Dog" {
		t.Fatalf("last_entity_value = %#v, want \"Cat, Dog\"", snapshot.State["vision_rule_feeder-zone_last_entity_value"])
	}

	event, ok, err := store.GetEvent(ctx, "evt-multi")
	if err != nil {
		t.Fatalf("GetEvent() error = %v", err)
	}
	if !ok {
		t.Fatal("persisted event missing after ReportEvents()")
	}
	if event.Payload["entity_value"] != "cat" {
		t.Fatalf("payload.entity_value = %#v, want cat", event.Payload["entity_value"])
	}
	entities, ok := event.Payload["entities"].([]any)
	if !ok {
		t.Fatalf("payload.entities type = %T, want []any", event.Payload["entities"])
	}
	if len(entities) != 2 {
		t.Fatalf("payload.entities len = %d, want 2", len(entities))
	}
	first, ok := entities[0].(map[string]any)
	if !ok {
		t.Fatalf("first payload entity type = %T, want map[string]any", entities[0])
	}
	if first["value"] != "cat" || first["display_name"] != "Cat" {
		t.Fatalf("first payload entity = %#v, want cat/Cat", first)
	}
}

func TestReportEventsPersistsDecisionMetadata(t *testing.T) {
	ctx := context.Background()
	service, registrySvc, _, store := newVisionTestService(t)

	camera := visionTestCamera()
	if err := registrySvc.Upsert(ctx, []models.Device{camera}); err != nil {
		t.Fatalf("registry.Upsert() error = %v", err)
	}
	if err := store.UpsertVisionConfig(ctx, models.VisionCapabilityConfig{
		RecognitionEnabled: true,
		UpdatedAt:          time.Now().UTC(),
		Rules: []models.VisionRule{{
			ID:                   "feeder-zone",
			Name:                 "Feeder Zone",
			Enabled:              true,
			CameraDeviceID:       camera.ID,
			RecognitionEnabled:   true,
			RTSPSource:           models.VisionRTSPSource{URL: "rtsp://user:pass@camera/stream"},
			EntitySelector:       models.VisionEntitySelector{Kind: "label", Value: "cat"},
			Behavior:             "eating",
			Zone:                 models.VisionZoneBox{X: 0.1, Y: 0.2, Width: 0.3, Height: 0.4},
			StayThresholdSeconds: 5,
		}},
	}); err != nil {
		t.Fatalf("UpsertVisionConfig() error = %v", err)
	}

	if err := service.ReportEvents(ctx, models.VisionServiceEventBatch{
		Events: []models.VisionServiceEvent{{
			EventID:      "evt-decision",
			RuleID:       "feeder-zone",
			Status:       models.VisionServiceEventStatusThresholdMet,
			ObservedAt:   time.Now().UTC(),
			DwellSeconds: 9,
			EntityValue:  "cat",
			Metadata: map[string]any{
				"decision": map[string]any{
					"source":           "roi_vlm_fallback",
					"confidence_score": 0.91,
					"confidence_breakdown": map[string]any{
						"detector": 0.52,
						"semantic": 0.96,
					},
					"semantic_checker": map[string]any{
						"verdict": "pass",
					},
				},
			},
		}},
	}); err != nil {
		t.Fatalf("ReportEvents() error = %v", err)
	}

	event, ok, err := store.GetEvent(ctx, "evt-decision")
	if err != nil {
		t.Fatalf("GetEvent() error = %v", err)
	}
	if !ok {
		t.Fatal("persisted event missing after ReportEvents()")
	}
	metadata, ok := event.Payload["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("payload.metadata type = %T, want map[string]any", event.Payload["metadata"])
	}
	decision, ok := metadata["decision"].(map[string]any)
	if !ok {
		t.Fatalf("payload.metadata.decision type = %T, want map[string]any", metadata["decision"])
	}
	if decision["source"] != "roi_vlm_fallback" {
		t.Fatalf("payload.metadata.decision.source = %#v, want roi_vlm_fallback", decision["source"])
	}
	if decision["confidence_score"] != 0.91 {
		t.Fatalf("payload.metadata.decision.confidence_score = %#v, want 0.91", decision["confidence_score"])
	}
}

func TestReportEventsRejectsClearedStatus(t *testing.T) {
	ctx := context.Background()
	service, registrySvc, _, store := newVisionTestService(t)

	camera := visionTestCamera()
	if err := registrySvc.Upsert(ctx, []models.Device{camera}); err != nil {
		t.Fatalf("registry.Upsert() error = %v", err)
	}
	if err := store.UpsertVisionConfig(ctx, models.VisionCapabilityConfig{
		RecognitionEnabled: true,
		UpdatedAt:          time.Now().UTC(),
		Rules: []models.VisionRule{{
			ID:                   "feeder-zone",
			Name:                 "Feeder Zone",
			Enabled:              true,
			CameraDeviceID:       camera.ID,
			RecognitionEnabled:   true,
			RTSPSource:           models.VisionRTSPSource{URL: "rtsp://user:pass@camera/stream"},
			EntitySelector:       models.VisionEntitySelector{Kind: "label", Value: "cat"},
			Zone:                 models.VisionZoneBox{X: 0.1, Y: 0.2, Width: 0.3, Height: 0.4},
			StayThresholdSeconds: 5,
		}},
	}); err != nil {
		t.Fatalf("UpsertVisionConfig() error = %v", err)
	}

	err := service.ReportEvents(ctx, models.VisionServiceEventBatch{
		Events: []models.VisionServiceEvent{{
			EventID:      "evt-cleared",
			RuleID:       "feeder-zone",
			Status:       models.VisionServiceEventStatus("cleared"),
			ObservedAt:   time.Now().UTC(),
			DwellSeconds: 6,
			EntityValue:  "cat",
		}},
	})
	if err == nil {
		t.Fatal("ReportEvents() error = nil, want unsupported status error")
	}
}
