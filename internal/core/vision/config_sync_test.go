package vision

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"nhooyr.io/websocket"
)

func TestSaveConfigSeedsCameraStateAndSyncsOverWebsocket(t *testing.T) {
	ctx := context.Background()
	service, registrySvc, stateSvc, _ := newVisionTestService(t)

	camera := visionTestCamera()
	if err := registrySvc.Upsert(ctx, []models.Device{camera}); err != nil {
		t.Fatalf("registry.Upsert() error = %v", err)
	}
	if err := stateSvc.Upsert(ctx, []models.DeviceStateSnapshot{{
		DeviceID: camera.ID,
		PluginID: camera.PluginID,
		TS:       time.Now().UTC(),
		State:    map[string]any{"connected": true},
	}}); err != nil {
		t.Fatalf("state.Upsert() error = %v", err)
	}

	modelSelections := make(chan wsSelectModelPayload, 1)
	syncPayloads := make(chan models.VisionServiceSyncPayload, 1)
	server := newVisionWSTestServer(t, func(serverCtx context.Context, conn *websocket.Conn) {
		sendTestEnvelope(t, serverCtx, conn, visionMessageTypeHello, "", wsHelloPayload{
			SchemaVersion:  visionWSSchemaVersion,
			ServiceVersion: "0.1.0",
			ConnectedAt:    time.Now().UTC(),
		})
		sendTestEnvelope(t, serverCtx, conn, visionMessageTypeRuntimeStatus, "", models.VisionServiceStatusReport{
			Status:     models.HealthStateUnknown,
			Message:    "awaiting sync",
			ReportedAt: time.Now().UTC(),
		})

		selectModel := readTestEnvelope(t, serverCtx, conn)
		if selectModel.Type != visionMessageTypeSelectModel {
			t.Fatalf("first request type = %q, want %q", selectModel.Type, visionMessageTypeSelectModel)
		}
		var modelPayload wsSelectModelPayload
		if err := decodeWSPayload(selectModel, &modelPayload); err != nil {
			t.Fatalf("decode select_model: %v", err)
		}
		modelSelections <- modelPayload
		sendTestEnvelope(t, serverCtx, conn, visionMessageTypeModelSelected, selectModel.RequestID, wsModelSelectedPayload{
			OK:        true,
			ModelName: "custom-pets.pt",
			ChangedAt: time.Now().UTC(),
		})

		syncConfig := readTestEnvelope(t, serverCtx, conn)
		if syncConfig.Type != visionMessageTypeSyncConfig {
			t.Fatalf("second request type = %q, want %q", syncConfig.Type, visionMessageTypeSyncConfig)
		}
		var payload models.VisionServiceSyncPayload
		if err := decodeWSPayload(syncConfig, &payload); err != nil {
			t.Fatalf("decode sync_config: %v", err)
		}
		syncPayloads <- payload
		sendTestEnvelope(t, serverCtx, conn, visionMessageTypeSyncApplied, syncConfig.RequestID, wsSyncAppliedPayload{
			OK:        true,
			AppliedAt: time.Now().UTC(),
		})
	})

	detail, err := service.SaveConfig(ctx, models.VisionCapabilityConfig{
		ServiceWSURL:       wsURLFromHTTP(server.URL),
		ModelName:          "custom-pets.pt",
		RecognitionEnabled: true,
		Rules: []models.VisionRule{{
			ID:                 "Feeder Zone",
			Name:               "Feeder Zone",
			Enabled:            true,
			CameraDeviceID:     camera.ID,
			RecognitionEnabled: true,
			RTSPSource:         models.VisionRTSPSource{URL: "rtsp://user:pass@camera/stream"},
			EntitySelector:     models.VisionEntitySelector{Kind: "label", Value: "cat"},
			Behavior:           " eating ",
			KeyEntities: []models.VisionRuleKeyEntity{
				{
					ID:          101,
					Name:        " Feeder Cat ",
					Description: " orange tabby with a blue collar ",
				},
				{
					ID:   102,
					Name: " Midnight ",
					Image: &models.VisionRuleKeyEntityImage{
						Base64:      "ZmFrZS1pbWFnZQ==",
						ContentType: " image/png ",
					},
				},
			},
			Zone:                 models.VisionZoneBox{X: 0.1, Y: 0.2, Width: 0.3, Height: 0.4},
			StayThresholdSeconds: 7,
		}},
	})
	if err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	modelSelection := waitFor(t, modelSelections)
	if modelSelection.ModelName == nil || *modelSelection.ModelName != "custom-pets.pt" {
		t.Fatalf("select_model payload = %#v, want custom-pets.pt", modelSelection)
	}

	synced := waitFor(t, syncPayloads)
	if detail.Runtime.Status != models.HealthStateHealthy {
		t.Fatalf("runtime status = %s, want healthy", detail.Runtime.Status)
	}
	if synced.SchemaVersion != visionControlSchemaVersion {
		t.Fatalf("schema_version = %q, want %q", synced.SchemaVersion, visionControlSchemaVersion)
	}
	if len(synced.Rules) != 1 {
		t.Fatalf("synced rules len = %d, want 1", len(synced.Rules))
	}
	if synced.Rules[0].Camera.EntryID != "entry-1" {
		t.Fatalf("camera entry_id = %q, want entry-1", synced.Rules[0].Camera.EntryID)
	}
	if synced.Rules[0].ID != "feeder-zone" {
		t.Fatalf("synced rule id = %q, want feeder-zone", synced.Rules[0].ID)
	}
	if detail.Config.Rules[0].Behavior != "eating" {
		t.Fatalf("detail behavior = %q, want eating", detail.Config.Rules[0].Behavior)
	}
	if synced.Rules[0].Behavior != "eating" {
		t.Fatalf("synced behavior = %q, want eating", synced.Rules[0].Behavior)
	}
	if len(synced.Rules[0].KeyEntities) != 2 {
		t.Fatalf("synced key_entities len = %d, want 2", len(synced.Rules[0].KeyEntities))
	}
	if detail.Config.Rules[0].KeyEntities[0].Name != "Feeder Cat" {
		t.Fatalf("detail key_entities[0].name = %q, want Feeder Cat", detail.Config.Rules[0].KeyEntities[0].Name)
	}
	if detail.Config.Rules[0].KeyEntities[0].Description != "orange tabby with a blue collar" {
		t.Fatalf("detail key_entities[0].description = %q, want trimmed description", detail.Config.Rules[0].KeyEntities[0].Description)
	}
	if synced.Rules[0].KeyEntities[1].Image == nil || synced.Rules[0].KeyEntities[1].Image.ContentType != "image/png" {
		t.Fatalf("synced key_entities[1].image = %#v, want trimmed image/png", synced.Rules[0].KeyEntities[1].Image)
	}
	encodedKeyEntities, err := json.Marshal(synced.Rules[0].KeyEntities)
	if err != nil {
		t.Fatalf("marshal synced key_entities: %v", err)
	}
	if strings.Contains(string(encodedKeyEntities), "\"name\"") {
		t.Fatalf("synced key_entities leaked Gateway-only name: %s", encodedKeyEntities)
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
	service, registrySvc, stateSvc, _ := newVisionTestService(t)

	camera := visionTestCamera()
	if err := registrySvc.Upsert(ctx, []models.Device{camera}); err != nil {
		t.Fatalf("registry.Upsert() error = %v", err)
	}
	const cameraRTSP = "rtsp://viewer:secret@camera/live"
	if err := stateSvc.Upsert(ctx, []models.DeviceStateSnapshot{{
		DeviceID: camera.ID,
		PluginID: camera.PluginID,
		TS:       time.Now().UTC(),
		State:    map[string]any{"rtsp_url": cameraRTSP},
	}}); err != nil {
		t.Fatalf("state.Upsert() error = %v", err)
	}

	syncPayloads := make(chan models.VisionServiceSyncPayload, 1)
	server := newVisionWSTestServer(t, func(serverCtx context.Context, conn *websocket.Conn) {
		sendTestEnvelope(t, serverCtx, conn, visionMessageTypeHello, "", wsHelloPayload{SchemaVersion: visionWSSchemaVersion, ConnectedAt: time.Now().UTC()})
		sendTestEnvelope(t, serverCtx, conn, visionMessageTypeRuntimeStatus, "", models.VisionServiceStatusReport{Status: models.HealthStateUnknown, ReportedAt: time.Now().UTC()})

		selectModel := readTestEnvelope(t, serverCtx, conn)
		sendTestEnvelope(t, serverCtx, conn, visionMessageTypeModelSelected, selectModel.RequestID, wsModelSelectedPayload{OK: true, ChangedAt: time.Now().UTC()})

		syncConfig := readTestEnvelope(t, serverCtx, conn)
		var payload models.VisionServiceSyncPayload
		if err := decodeWSPayload(syncConfig, &payload); err != nil {
			t.Fatalf("decode sync_config: %v", err)
		}
		syncPayloads <- payload
		sendTestEnvelope(t, serverCtx, conn, visionMessageTypeSyncApplied, syncConfig.RequestID, wsSyncAppliedPayload{OK: true, AppliedAt: time.Now().UTC()})
	})

	detail, err := service.SaveConfig(ctx, models.VisionCapabilityConfig{
		ServiceWSURL:       wsURLFromHTTP(server.URL),
		RecognitionEnabled: true,
		Rules: []models.VisionRule{{
			ID:                 "Feeder Zone",
			Name:               "Feeder Zone",
			Enabled:            true,
			CameraDeviceID:     camera.ID,
			RecognitionEnabled: true,
			EntitySelector:     models.VisionEntitySelector{Kind: "label", Value: "cat"},
			Zone:               models.VisionZoneBox{X: 0.1, Y: 0.2, Width: 0.3, Height: 0.4},
		}},
	})
	if err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	synced := waitFor(t, syncPayloads)
	if detail.Config.Rules[0].RTSPSource.URL != cameraRTSP {
		t.Fatalf("detail rtsp_source.url = %q, want %q", detail.Config.Rules[0].RTSPSource.URL, cameraRTSP)
	}
	if synced.Rules[0].RTSPSource.URL != cameraRTSP {
		t.Fatalf("synced rtsp_source.url = %q, want %q", synced.Rules[0].RTSPSource.URL, cameraRTSP)
	}
}

func TestSaveConfigRejectsMissingRTSPSourceWhenCameraHasNoRTSP(t *testing.T) {
	ctx := context.Background()
	service, registrySvc, _, _ := newVisionTestService(t)

	camera := visionTestCamera()
	if err := registrySvc.Upsert(ctx, []models.Device{camera}); err != nil {
		t.Fatalf("registry.Upsert() error = %v", err)
	}

	_, err := service.SaveConfig(ctx, models.VisionCapabilityConfig{
		ServiceWSURL:       "ws://vision.example/api/v1/capabilities/vision_entity_stay_zone",
		RecognitionEnabled: true,
		Rules: []models.VisionRule{{
			ID:                 "Feeder Zone",
			Name:               "Feeder Zone",
			Enabled:            true,
			CameraDeviceID:     camera.ID,
			RecognitionEnabled: true,
			EntitySelector:     models.VisionEntitySelector{Kind: "label", Value: "cat"},
			Zone:               models.VisionZoneBox{X: 0.1, Y: 0.2, Width: 0.3, Height: 0.4},
		}},
	})
	if err == nil {
		t.Fatal("SaveConfig() error = nil, want rtsp_source validation error")
	}
}

func TestInitConnectsUsingSavedConfig(t *testing.T) {
	ctx := context.Background()
	service, registrySvc, _, store := newVisionTestService(t)

	camera := visionTestCamera()
	if err := registrySvc.Upsert(ctx, []models.Device{camera}); err != nil {
		t.Fatalf("registry.Upsert() error = %v", err)
	}

	syncedCh := make(chan models.VisionServiceSyncPayload, 1)
	server := newVisionWSTestServer(t, func(serverCtx context.Context, conn *websocket.Conn) {
		sendTestEnvelope(t, serverCtx, conn, visionMessageTypeHello, "", wsHelloPayload{SchemaVersion: visionWSSchemaVersion, ConnectedAt: time.Now().UTC()})
		sendTestEnvelope(t, serverCtx, conn, visionMessageTypeRuntimeStatus, "", models.VisionServiceStatusReport{Status: models.HealthStateUnknown, ReportedAt: time.Now().UTC()})

		selectModel := readTestEnvelope(t, serverCtx, conn)
		sendTestEnvelope(t, serverCtx, conn, visionMessageTypeModelSelected, selectModel.RequestID, wsModelSelectedPayload{OK: true, ChangedAt: time.Now().UTC()})

		syncConfig := readTestEnvelope(t, serverCtx, conn)
		var payload models.VisionServiceSyncPayload
		if err := decodeWSPayload(syncConfig, &payload); err != nil {
			t.Fatalf("decode sync_config: %v", err)
		}
		syncedCh <- payload
		sendTestEnvelope(t, serverCtx, conn, visionMessageTypeSyncApplied, syncConfig.RequestID, wsSyncAppliedPayload{OK: true, AppliedAt: time.Now().UTC()})
	})

	if err := store.UpsertVisionConfig(ctx, models.VisionCapabilityConfig{
		ServiceWSURL:               wsURLFromHTTP(server.URL),
		ModelName:                  "startup-model.pt",
		RecognitionEnabled:         true,
		EventCaptureRetentionHours: models.DefaultVisionEventCaptureRetentionHours,
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
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpsertVisionConfig() error = %v", err)
	}

	if err := service.Init(ctx); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	synced := waitFor(t, syncedCh)
	if synced.Rules[0].ID != "feeder-zone" {
		t.Fatalf("synced rule id = %q, want feeder-zone", synced.Rules[0].ID)
	}
}
