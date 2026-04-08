package vision

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/chentianyu/celestia/internal/core/eventbus"
	"github.com/chentianyu/celestia/internal/core/registry"
	"github.com/chentianyu/celestia/internal/core/state"
	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/storage/sqlite"
)

func TestReportEvidenceAttachesCapturesToVisionEvents(t *testing.T) {
	ctx := context.Background()
	service, registrySvc, _, store := newVisionEvidenceTestService(t)
	camera := visionTestCamera()
	if err := registrySvc.Upsert(ctx, []models.Device{camera}); err != nil {
		t.Fatalf("registry.Upsert() error = %v", err)
	}
	if _, err := service.SaveConfig(ctx, models.VisionCapabilityConfig{
		ServiceURL:                 "http://vision.example",
		RecognitionEnabled:         true,
		EventCaptureRetentionHours: 72,
		Rules: []models.VisionRule{
			{
				ID:                   "feeder-zone",
				Name:                 "Feeder Zone",
				Enabled:              true,
				CameraDeviceID:       camera.ID,
				RecognitionEnabled:   true,
				RTSPSource:           models.VisionRTSPSource{URL: "rtsp://user:pass@camera/stream"},
				EntitySelector:       models.VisionEntitySelector{Kind: "label", Value: "cat"},
				Zone:                 models.VisionZoneBox{X: 0.1, Y: 0.2, Width: 0.3, Height: 0.4},
				StayThresholdSeconds: 5,
			},
		},
	}); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
	if err := service.ReportEvents(ctx, models.VisionServiceEventBatch{
		Events: []models.VisionServiceEvent{
			{
				EventID:      "evt-1",
				RuleID:       "feeder-zone",
				Status:       models.VisionServiceEventStatusThresholdMet,
				ObservedAt:   time.Now().UTC(),
				DwellSeconds: 7,
				EntityValue:  "cat",
			},
		},
	}); err != nil {
		t.Fatalf("ReportEvents() error = %v", err)
	}

	if err := service.ReportEvidence(ctx, models.VisionServiceEventCaptureBatch{
		Captures: []models.VisionServiceEventCapture{
			testVisionCapture("evt-1", "feeder-zone", camera.ID, models.VisionEventCapturePhaseMiddle, "middle"),
			testVisionCapture("evt-1", "feeder-zone", camera.ID, models.VisionEventCapturePhaseEnd, "end"),
			testVisionCapture("evt-1", "feeder-zone", camera.ID, models.VisionEventCapturePhaseStart, "start"),
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
	if len(captures) != 3 {
		t.Fatalf("captures len = %d, want 3", len(captures))
	}
	if captures[0].Phase != models.VisionEventCapturePhaseStart || captures[1].Phase != models.VisionEventCapturePhaseMiddle || captures[2].Phase != models.VisionEventCapturePhaseEnd {
		t.Fatalf("capture phases = %#v, want start/middle/end", []models.VisionEventCapturePhase{captures[0].Phase, captures[1].Phase, captures[2].Phase})
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
	service, registrySvc, _, store := newVisionEvidenceTestService(t)
	camera := visionTestCamera()
	if err := registrySvc.Upsert(ctx, []models.Device{camera}); err != nil {
		t.Fatalf("registry.Upsert() error = %v", err)
	}
	if _, err := service.SaveConfig(ctx, models.VisionCapabilityConfig{
		ServiceURL:                 "http://vision.example",
		RecognitionEnabled:         true,
		EventCaptureRetentionHours: 1,
		Rules: []models.VisionRule{
			{
				ID:                   "feeder-zone",
				Name:                 "Feeder Zone",
				Enabled:              true,
				CameraDeviceID:       camera.ID,
				RecognitionEnabled:   true,
				RTSPSource:           models.VisionRTSPSource{URL: "rtsp://user:pass@camera/stream"},
				EntitySelector:       models.VisionEntitySelector{Kind: "label", Value: "cat"},
				Zone:                 models.VisionZoneBox{X: 0.1, Y: 0.2, Width: 0.3, Height: 0.4},
				StayThresholdSeconds: 5,
			},
		},
	}); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
	if err := service.ReportEvents(ctx, models.VisionServiceEventBatch{
		Events: []models.VisionServiceEvent{
			{
				EventID:      "evt-2",
				RuleID:       "feeder-zone",
				Status:       models.VisionServiceEventStatusThresholdMet,
				ObservedAt:   time.Now().UTC(),
				DwellSeconds: 6,
				EntityValue:  "cat",
			},
		},
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

func newVisionEvidenceTestService(t *testing.T) (*Service, *registry.Service, *state.Service, *sqlite.Store) {
	t.Helper()
	store := newVisionTestStore(t)
	registrySvc := registry.New(store)
	stateSvc := state.New(store)
	bus := eventbus.New()
	service := New(store, registrySvc, stateSvc, bus)
	service.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("{}")),
				Header:     make(http.Header),
			}, nil
		}),
	}
	return service, registrySvc, stateSvc, store
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
