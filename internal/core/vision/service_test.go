package vision

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chentianyu/celestia/internal/core/eventbus"
	"github.com/chentianyu/celestia/internal/core/registry"
	"github.com/chentianyu/celestia/internal/core/state"
	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/storage"
	"github.com/chentianyu/celestia/internal/storage/sqlite"
)

func TestSaveConfigSeedsCameraStateAndSyncs(t *testing.T) {
	ctx := context.Background()
	store := newVisionTestStore(t)
	registrySvc := registry.New(store)
	stateSvc := state.New(store)
	bus := eventbus.New()
	service := New(store, registrySvc, stateSvc, bus)

	camera := visionTestCamera()
	if err := registrySvc.Upsert(ctx, []models.Device{camera}); err != nil {
		t.Fatalf("registry.Upsert() error = %v", err)
	}
	if err := stateSvc.Upsert(ctx, []models.DeviceStateSnapshot{{
		DeviceID: camera.ID,
		PluginID: camera.PluginID,
		TS:       time.Now().UTC(),
		State: map[string]any{
			"connected": true,
		},
	}}); err != nil {
		t.Fatalf("state.Upsert() error = %v", err)
	}

	var synced models.VisionServiceSyncPayload
	service.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodPut {
				t.Fatalf("sync method = %s, want PUT", req.Method)
			}
			if req.URL.Path != visionConfigSyncPath {
				t.Fatalf("sync path = %s, want %s", req.URL.Path, visionConfigSyncPath)
			}
			if err := json.NewDecoder(req.Body).Decode(&synced); err != nil {
				t.Fatalf("decode sync payload error = %v", err)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("{}")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	detail, err := service.SaveConfig(ctx, models.VisionCapabilityConfig{
		ServiceURL:         "http://vision.example",
		RecognitionEnabled: true,
		Rules: []models.VisionRule{
			{
				ID:                   "Feeder Zone",
				Name:                 "Feeder Zone",
				Enabled:              true,
				CameraDeviceID:       camera.ID,
				RecognitionEnabled:   true,
				RTSPSource:           models.VisionRTSPSource{URL: "rtsp://user:pass@camera/stream"},
				EntitySelector:       models.VisionEntitySelector{Kind: "label", Value: "cat"},
				Zone:                 models.VisionZoneBox{X: 0.1, Y: 0.2, Width: 0.3, Height: 0.4},
				StayThresholdSeconds: 7,
			},
		},
	})
	if err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	if detail.Runtime.Status != models.HealthStateHealthy {
		t.Fatalf("runtime status = %s, want healthy", detail.Runtime.Status)
	}
	if synced.SchemaVersion != "celestia.vision.control.v1" {
		t.Fatalf("schema_version = %q", synced.SchemaVersion)
	}
	if len(synced.Rules) != 1 {
		t.Fatalf("synced rules len = %d, want 1", len(synced.Rules))
	}
	if synced.Callbacks.EvidencePath != visionEvidenceCallbackPath {
		t.Fatalf("evidence path = %q, want %q", synced.Callbacks.EvidencePath, visionEvidenceCallbackPath)
	}
	if synced.Rules[0].Camera.EntryID != "entry-1" {
		t.Fatalf("camera entry_id = %q, want entry-1", synced.Rules[0].Camera.EntryID)
	}
	if synced.Rules[0].ID != "feeder-zone" {
		t.Fatalf("synced rule id = %q, want feeder-zone", synced.Rules[0].ID)
	}

	snapshot, ok, err := stateSvc.Get(ctx, camera.ID)
	if err != nil {
		t.Fatalf("state.Get() error = %v", err)
	}
	if !ok {
		t.Fatal("camera state missing after SaveConfig()")
	}
	if snapshot.State["connected"] != true {
		t.Fatalf("connected state missing after reconcile: %#v", snapshot.State["connected"])
	}
	if intValue(snapshot.State["vision_rule_feeder-zone_match_count"]) != 0 {
		t.Fatalf("match_count = %#v, want 0", snapshot.State["vision_rule_feeder-zone_match_count"])
	}
}

func TestSaveConfigDerivesRTSPSourceFromCameraState(t *testing.T) {
	ctx := context.Background()
	store := newVisionTestStore(t)
	registrySvc := registry.New(store)
	stateSvc := state.New(store)
	bus := eventbus.New()
	service := New(store, registrySvc, stateSvc, bus)

	camera := visionTestCamera()
	if err := registrySvc.Upsert(ctx, []models.Device{camera}); err != nil {
		t.Fatalf("registry.Upsert() error = %v", err)
	}
	const cameraRTSP = "rtsp://viewer:secret@camera/live"
	if err := stateSvc.Upsert(ctx, []models.DeviceStateSnapshot{{
		DeviceID: camera.ID,
		PluginID: camera.PluginID,
		TS:       time.Now().UTC(),
		State: map[string]any{
			"rtsp_url": cameraRTSP,
		},
	}}); err != nil {
		t.Fatalf("state.Upsert() error = %v", err)
	}

	var synced models.VisionServiceSyncPayload
	service.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if err := json.NewDecoder(req.Body).Decode(&synced); err != nil {
				t.Fatalf("decode sync payload error = %v", err)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("{}")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	detail, err := service.SaveConfig(ctx, models.VisionCapabilityConfig{
		ServiceURL:         "http://vision.example",
		RecognitionEnabled: true,
		Rules: []models.VisionRule{
			{
				ID:                 "Feeder Zone",
				Name:               "Feeder Zone",
				Enabled:            true,
				CameraDeviceID:     camera.ID,
				RecognitionEnabled: true,
				EntitySelector:     models.VisionEntitySelector{Kind: "label", Value: "cat"},
				Zone:               models.VisionZoneBox{X: 0.1, Y: 0.2, Width: 0.3, Height: 0.4},
			},
		},
	})
	if err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	if detail.Config.Rules[0].RTSPSource.URL != cameraRTSP {
		t.Fatalf("detail rtsp_source.url = %q, want %q", detail.Config.Rules[0].RTSPSource.URL, cameraRTSP)
	}
	if synced.Rules[0].RTSPSource.URL != cameraRTSP {
		t.Fatalf("synced rtsp_source.url = %q, want %q", synced.Rules[0].RTSPSource.URL, cameraRTSP)
	}
}

func TestSaveConfigRejectsMissingRTSPSourceWhenCameraHasNoRTSP(t *testing.T) {
	ctx := context.Background()
	store := newVisionTestStore(t)
	registrySvc := registry.New(store)
	stateSvc := state.New(store)
	bus := eventbus.New()
	service := New(store, registrySvc, stateSvc, bus)

	camera := visionTestCamera()
	if err := registrySvc.Upsert(ctx, []models.Device{camera}); err != nil {
		t.Fatalf("registry.Upsert() error = %v", err)
	}

	_, err := service.SaveConfig(ctx, models.VisionCapabilityConfig{
		ServiceURL:         "http://vision.example",
		RecognitionEnabled: true,
		Rules: []models.VisionRule{
			{
				ID:                 "Feeder Zone",
				Name:               "Feeder Zone",
				Enabled:            true,
				CameraDeviceID:     camera.ID,
				RecognitionEnabled: true,
				EntitySelector:     models.VisionEntitySelector{Kind: "label", Value: "cat"},
				Zone:               models.VisionZoneBox{X: 0.1, Y: 0.2, Width: 0.3, Height: 0.4},
			},
		},
	})
	if err == nil {
		t.Fatal("SaveConfig() error = nil, want rtsp_source validation error")
	}
	if !strings.Contains(err.Error(), "rtsp_source.url is required") {
		t.Fatalf("SaveConfig() error = %v, want rtsp_source validation error", err)
	}
}

func TestRefreshCatalogPersistsSupportedEntities(t *testing.T) {
	ctx := context.Background()
	store := newVisionTestStore(t)
	registrySvc := registry.New(store)
	stateSvc := state.New(store)
	bus := eventbus.New()
	service := New(store, registrySvc, stateSvc, bus)

	service.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodGet {
				t.Fatalf("catalog method = %s, want GET", req.Method)
			}
			if req.URL.Path != visionEntityCatalogPath {
				t.Fatalf("catalog path = %s, want %s", req.URL.Path, visionEntityCatalogPath)
			}
			body := `{
				"schema_version":"celestia.vision.catalog.v1",
				"service_version":"1.2.0",
				"model_name":"yolo11m-coco",
				"fetched_at":"2026-04-08T09:15:00Z",
				"entities":[
					{"kind":"label","value":"cat","display_name":"Cat"},
					{"kind":"label","value":"dog","display_name":"Dog"}
				]
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	catalog, err := service.RefreshCatalog(ctx, models.VisionEntityCatalogRefreshRequest{
		ServiceURL: "http://vision.example/",
	})
	if err != nil {
		t.Fatalf("RefreshCatalog() error = %v", err)
	}
	if catalog.ServiceURL != "http://vision.example" {
		t.Fatalf("catalog service_url = %q, want http://vision.example", catalog.ServiceURL)
	}
	if catalog.ModelName != "yolo11m-coco" {
		t.Fatalf("catalog model_name = %q, want yolo11m-coco", catalog.ModelName)
	}
	if len(catalog.Entities) != 2 {
		t.Fatalf("catalog entities len = %d, want 2", len(catalog.Entities))
	}

	persisted, ok, err := service.GetCatalog(ctx)
	if err != nil {
		t.Fatalf("GetCatalog() error = %v", err)
	}
	if !ok {
		t.Fatal("catalog missing after RefreshCatalog()")
	}
	if persisted.Entities[0].Value != "cat" {
		t.Fatalf("first entity value = %q, want cat", persisted.Entities[0].Value)
	}
}

func TestSaveConfigRejectsEntityNotInCatalog(t *testing.T) {
	ctx := context.Background()
	store := newVisionTestStore(t)
	registrySvc := registry.New(store)
	stateSvc := state.New(store)
	bus := eventbus.New()
	service := New(store, registrySvc, stateSvc, bus)

	camera := visionTestCamera()
	if err := registrySvc.Upsert(ctx, []models.Device{camera}); err != nil {
		t.Fatalf("registry.Upsert() error = %v", err)
	}
	if err := store.UpsertVisionCatalog(ctx, models.VisionEntityCatalog{
		ServiceURL:    "http://vision.example",
		SchemaVersion: "celestia.vision.catalog.v1",
		FetchedAt:     time.Now().UTC(),
		Entities: []models.VisionEntityDescriptor{
			{Kind: "label", Value: "cat", DisplayName: "Cat"},
		},
	}); err != nil {
		t.Fatalf("UpsertVisionCatalog() error = %v", err)
	}

	_, err := service.SaveConfig(ctx, models.VisionCapabilityConfig{
		ServiceURL:         "http://vision.example",
		RecognitionEnabled: true,
		Rules: []models.VisionRule{
			{
				ID:                 "feeder-zone",
				Name:               "Feeder Zone",
				Enabled:            true,
				CameraDeviceID:     camera.ID,
				RecognitionEnabled: true,
				RTSPSource:         models.VisionRTSPSource{URL: "rtsp://user:pass@camera/stream"},
				EntitySelector:     models.VisionEntitySelector{Kind: "label", Value: "dog"},
				Zone:               models.VisionZoneBox{X: 0.1, Y: 0.2, Width: 0.3, Height: 0.4},
			},
		},
	})
	if err == nil {
		t.Fatal("SaveConfig() error = nil, want unsupported entity error")
	}
	if !strings.Contains(err.Error(), "not advertised") {
		t.Fatalf("SaveConfig() error = %v, want unsupported entity validation", err)
	}
}

func TestReportEventsProjectsDeviceStateChange(t *testing.T) {
	ctx := context.Background()
	store := newVisionTestStore(t)
	registrySvc := registry.New(store)
	stateSvc := state.New(store)
	bus := eventbus.New()
	service := New(store, registrySvc, stateSvc, bus)

	camera := visionTestCamera()
	if err := registrySvc.Upsert(ctx, []models.Device{camera}); err != nil {
		t.Fatalf("registry.Upsert() error = %v", err)
	}

	service.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("{}")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	if _, err := service.SaveConfig(ctx, models.VisionCapabilityConfig{
		ServiceURL:         "http://vision.example",
		RecognitionEnabled: true,
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

	subID, ch := bus.Subscribe(8)
	defer bus.Unsubscribe(subID)

	observedAt := time.Now().UTC()
	if err := service.ReportEvents(ctx, models.VisionServiceEventBatch{
		Events: []models.VisionServiceEvent{
			{
				EventID:      "evt-1",
				RuleID:       "feeder-zone",
				Status:       models.VisionServiceEventStatusThresholdMet,
				ObservedAt:   observedAt,
				DwellSeconds: 6,
				EntityValue:  "cat",
			},
		},
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
	if snapshot.State["vision_rule_feeder-zone_active"] != true {
		t.Fatalf("active = %#v, want true", snapshot.State["vision_rule_feeder-zone_active"])
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

	seenDeviceOccurred := false
	seenStateChanged := false
	timeout := time.After(2 * time.Second)
	for !seenDeviceOccurred || !seenStateChanged {
		select {
		case event := <-ch:
			if event.Type == models.EventDeviceOccurred {
				seenDeviceOccurred = true
			}
			if event.Type == models.EventDeviceStateChanged {
				seenStateChanged = true
			}
		case <-timeout:
			t.Fatalf("timed out waiting for projected vision events: occurred=%v state_changed=%v", seenDeviceOccurred, seenStateChanged)
		}
	}
}

func newVisionTestStore(t *testing.T) *sqlite.Store {
	t.Helper()
	store, err := sqlite.New(filepath.Join(t.TempDir(), "vision.db"))
	if err != nil {
		t.Fatalf("sqlite.New() error = %v", err)
	}
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store
}

func visionTestCamera() models.Device {
	return models.Device{
		ID:             "hikvision:camera:entry-1",
		PluginID:       "hikvision",
		VendorDeviceID: "192.0.2.10:8000:ch1",
		Kind:           models.DeviceKindCameraLike,
		Name:           "Patio Camera",
		Online:         true,
		Capabilities:   []string{"stream"},
		Metadata: map[string]any{
			"entry_id": "entry-1",
		},
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
