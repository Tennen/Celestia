package vision

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"nhooyr.io/websocket"
)

func TestReportEvidenceAttachesCapturesToVisionEvents(t *testing.T) {
	ctx := context.Background()
	service, registrySvc, _, store := newVisionTestService(t)

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
		ServiceWSURL:               wsURLFromHTTP(server.URL),
		RecognitionEnabled:         true,
		EventCaptureRetentionHours: 72,
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
	if err := service.ReportEvents(ctx, models.VisionServiceEventBatch{
		Events: []models.VisionServiceEvent{{
			EventID:      "evt-1",
			RuleID:       "feeder-zone",
			Status:       models.VisionServiceEventStatusThresholdMet,
			ObservedAt:   time.Now().UTC(),
			DwellSeconds: 7,
			EntityValue:  "cat",
		}},
	}); err != nil {
		t.Fatalf("ReportEvents() error = %v", err)
	}

	extraMiddle := testVisionCapture("evt-1", "feeder-zone", camera.ID, models.VisionEventCapturePhaseMiddle, "middle-extra")
	extraMiddle.CaptureID = "evt-1:middle:extra"
	extraSample := testVisionCapture("evt-1", "feeder-zone", camera.ID, models.VisionEventCapturePhase("sample_002"), "sample")
	extraSample.CaptureID = "evt-1:sample_002"
	if err := service.ReportEvidence(ctx, models.VisionServiceEventCaptureBatch{
		Captures: []models.VisionServiceEventCapture{
			testVisionCapture("evt-1", "feeder-zone", camera.ID, models.VisionEventCapturePhaseMiddle, "middle"),
			testVisionCapture("evt-1", "feeder-zone", camera.ID, models.VisionEventCapturePhaseEnd, "end"),
			testVisionCapture("evt-1", "feeder-zone", camera.ID, models.VisionEventCapturePhaseStart, "start"),
			extraMiddle,
			extraSample,
		},
	}); err != nil {
		t.Fatalf("ReportEvidence() error = %v", err)
	}

	events, err := service.RecentEvents(ctx, 5)
	if err != nil {
		t.Fatalf("RecentEvents() error = %v", err)
	}
	if len(events) == 0 {
		t.Fatal("RecentEvents() returned no events")
	}
	captures, ok := events[0].Payload["captures"].([]models.VisionEventCapture)
	if !ok {
		t.Fatalf("captures payload type = %T, want []models.VisionEventCapture", events[0].Payload["captures"])
	}
	if len(captures) != 5 {
		t.Fatalf("captures len = %d, want 5", len(captures))
	}
	if captures[0].Phase != models.VisionEventCapturePhaseStart || captures[len(captures)-1].Phase != models.VisionEventCapturePhaseEnd {
		t.Fatalf("captures order = %#v, want start first and end last", captures)
	}
	sampleIndex := -1
	for index, capture := range captures {
		if capture.Phase == models.VisionEventCapturePhase("sample_002") {
			sampleIndex = index
			break
		}
	}
	if sampleIndex < 0 || sampleIndex >= len(captures)-1 {
		t.Fatalf("captures order = %#v, want sample_002 before end", captures)
	}

	asset, ok, err := service.GetCaptureAsset(ctx, "evt-1:start")
	if err != nil {
		t.Fatalf("GetCaptureAsset() error = %v", err)
	}
	if !ok {
		t.Fatal("capture asset missing after ReportEvidence()")
	}
	if string(asset.Data) != "start" {
		t.Fatalf("capture data = %q, want start", string(asset.Data))
	}
	extraAsset, ok, err := service.GetCaptureAsset(ctx, "evt-1:middle:extra")
	if err != nil {
		t.Fatalf("GetCaptureAsset(extra middle) error = %v", err)
	}
	if !ok {
		t.Fatal("extra middle capture asset missing after ReportEvidence()")
	}
	if string(extraAsset.Data) != "middle-extra" {
		t.Fatalf("extra middle capture data = %q, want middle-extra", string(extraAsset.Data))
	}

	persisted, ok, err := store.GetVisionEventCapture(ctx, "evt-1:start")
	if err != nil {
		t.Fatalf("store.GetVisionEventCapture() error = %v", err)
	}
	if !ok || persisted.Capture.EventID != "evt-1" {
		t.Fatalf("persisted capture = %#v, want event evt-1", persisted.Capture)
	}
}

func TestGetCaptureAssetDropsExpiredEvidence(t *testing.T) {
	ctx := context.Background()
	service, registrySvc, _, store := newVisionTestService(t)

	camera := visionTestCamera()
	if err := registrySvc.Upsert(ctx, []models.Device{camera}); err != nil {
		t.Fatalf("registry.Upsert() error = %v", err)
	}
	if err := store.UpsertVisionConfig(ctx, models.VisionCapabilityConfig{
		ServiceWSURL:               "ws://vision.example/api/v1/capabilities/vision_entity_stay_zone",
		RecognitionEnabled:         true,
		EventCaptureRetentionHours: 1,
		Rules: []models.VisionRule{{
			ID:             "feeder-zone",
			Name:           "Feeder Zone",
			CameraDeviceID: camera.ID,
		}},
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpsertVisionConfig() error = %v", err)
	}
	if err := service.ReportEvents(ctx, models.VisionServiceEventBatch{
		Events: []models.VisionServiceEvent{{
			EventID:      "evt-2",
			RuleID:       "feeder-zone",
			Status:       models.VisionServiceEventStatusThresholdMet,
			ObservedAt:   time.Now().UTC(),
			DwellSeconds: 6,
			EntityValue:  "cat",
		}},
	}); err != nil {
		t.Fatalf("ReportEvents() error = %v", err)
	}
	if err := store.UpsertVisionEventCapture(ctx, models.VisionEventCaptureAsset{
		Capture: models.VisionEventCapture{
			CaptureID:      "evt-2:start",
			EventID:        "evt-2",
			RuleID:         "feeder-zone",
			CameraDeviceID: camera.ID,
			Phase:          models.VisionEventCapturePhaseStart,
			CapturedAt:     time.Now().UTC().Add(-2 * time.Hour),
			ContentType:    "image/jpeg",
			SizeBytes:      len([]byte("stale")),
		},
		Data: []byte("stale"),
	}); err != nil {
		t.Fatalf("UpsertVisionEventCapture() error = %v", err)
	}

	if _, ok, err := service.GetCaptureAsset(ctx, "evt-2:start"); err != nil {
		t.Fatalf("GetCaptureAsset() error = %v", err)
	} else if ok {
		t.Fatal("GetCaptureAsset() ok = true, want expired capture removed")
	}
	if _, ok, err := store.GetVisionEventCapture(ctx, "evt-2:start"); err != nil {
		t.Fatalf("store.GetVisionEventCapture() error = %v", err)
	} else if ok {
		t.Fatal("expired capture still present in store")
	}
}

func TestReportEvidenceNormalizesCaptureAnnotations(t *testing.T) {
	ctx := context.Background()
	service, registrySvc, _, store := newVisionTestService(t)

	camera := visionTestCamera()
	if err := registrySvc.Upsert(ctx, []models.Device{camera}); err != nil {
		t.Fatalf("registry.Upsert() error = %v", err)
	}
	if err := store.UpsertVisionConfig(ctx, models.VisionCapabilityConfig{
		ServiceWSURL:               "ws://vision.example/api/v1/capabilities/vision_entity_stay_zone",
		RecognitionEnabled:         true,
		EventCaptureRetentionHours: 72,
		Rules: []models.VisionRule{{
			ID:             "feeder-zone",
			Name:           "Feeder Zone",
			CameraDeviceID: camera.ID,
		}},
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpsertVisionConfig() error = %v", err)
	}
	if err := service.ReportEvents(ctx, models.VisionServiceEventBatch{
		Events: []models.VisionServiceEvent{{
			EventID:      "evt-3",
			RuleID:       "feeder-zone",
			Status:       models.VisionServiceEventStatusThresholdMet,
			ObservedAt:   time.Now().UTC(),
			DwellSeconds: 5,
			EntityValue:  "cat",
		}},
	}); err != nil {
		t.Fatalf("ReportEvents() error = %v", err)
	}

	if err := service.ReportEvidence(ctx, models.VisionServiceEventCaptureBatch{
		Captures: []models.VisionServiceEventCapture{{
			EventID:        "evt-3",
			RuleID:         "feeder-zone",
			CameraDeviceID: camera.ID,
			Phase:          models.VisionEventCapturePhaseStart,
			CapturedAt:     time.Now().UTC(),
			ContentType:    "image/jpeg",
			ImageBase64:    base64.StdEncoding.EncodeToString([]byte("annotated")),
			Metadata: map[string]any{
				"annotations": map[string]any{
					"image_kind": "annotated",
					"source":     "ultralytics.plot",
					"detections": []any{
						map[string]any{
							"kind":         "label",
							"value":        "cat",
							"display_name": "Cat",
							"confidence":   0.93,
							"track_id":     "7",
							"box": map[string]any{
								"x":      0.125,
								"y":      0.2,
								"width":  0.5,
								"height": 0.4,
							},
						},
					},
				},
			},
		}},
	}); err != nil {
		t.Fatalf("ReportEvidence() error = %v", err)
	}

	persisted, ok, err := store.GetVisionEventCapture(ctx, "evt-3:start")
	if err != nil {
		t.Fatalf("GetVisionEventCapture() error = %v", err)
	}
	if !ok {
		t.Fatal("persisted capture missing")
	}
	annotations, ok := persisted.Capture.Metadata["annotations"].(map[string]any)
	if !ok {
		t.Fatalf("capture metadata annotations type = %T, want map[string]any", persisted.Capture.Metadata["annotations"])
	}
	if annotations["image_kind"] != "annotated" || annotations["coordinate_space"] != "normalized_xywh" {
		t.Fatalf("annotations = %#v, want normalized annotation metadata", annotations)
	}
	detections, ok := annotations["detections"].([]any)
	if !ok || len(detections) != 1 {
		t.Fatalf("detections = %#v, want 1 detection", annotations["detections"])
	}
	first, ok := detections[0].(map[string]any)
	if !ok {
		t.Fatalf("first detection type = %T, want map[string]any", detections[0])
	}
	box, ok := first["box"].(map[string]any)
	if !ok {
		t.Fatalf("first box type = %T, want map[string]any", first["box"])
	}
	if first["display_name"] != "Cat" || box["x"] != 0.125 || box["height"] != 0.4 {
		t.Fatalf("first detection = %#v, want normalized cat annotation", first)
	}
}

func testVisionCapture(eventID, ruleID, cameraID string, phase models.VisionEventCapturePhase, raw string) models.VisionServiceEventCapture {
	return models.VisionServiceEventCapture{
		EventID:        eventID,
		RuleID:         ruleID,
		CameraDeviceID: cameraID,
		Phase:          phase,
		CapturedAt:     time.Now().UTC(),
		ContentType:    "image/jpeg",
		ImageBase64:    base64.StdEncoding.EncodeToString([]byte(raw)),
	}
}
