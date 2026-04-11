package vision

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"nhooyr.io/websocket"
)

func TestRefreshCatalogPersistsSupportedEntitiesOverWebsocket(t *testing.T) {
	ctx := context.Background()
	service, _, _, _ := newVisionTestService(t)

	requests := make(chan wsGetEntitiesPayload, 1)
	server := newVisionWSTestServer(t, func(serverCtx context.Context, conn *websocket.Conn) {
		sendTestEnvelope(t, serverCtx, conn, visionMessageTypeHello, "", wsHelloPayload{SchemaVersion: visionWSSchemaVersion, ConnectedAt: time.Now().UTC()})
		sendTestEnvelope(t, serverCtx, conn, visionMessageTypeRuntimeStatus, "", models.VisionServiceStatusReport{Status: models.HealthStateUnknown, ReportedAt: time.Now().UTC()})

		request := readTestEnvelope(t, serverCtx, conn)
		if request.Type != visionMessageTypeGetEntities {
			t.Fatalf("request type = %q, want %q", request.Type, visionMessageTypeGetEntities)
		}
		var payload wsGetEntitiesPayload
		if err := decodeWSPayload(request, &payload); err != nil {
			t.Fatalf("decode get_entities: %v", err)
		}
		requests <- payload
		sendTestEnvelope(t, serverCtx, conn, visionMessageTypeEntityCatalog, request.RequestID, models.VisionServiceEntityCatalog{
			SchemaVersion:  visionEntityCatalogSchemaVersion,
			ServiceVersion: "1.2.0",
			ModelName:      "yolo11m-coco",
			FetchedAt:      time.Date(2026, 4, 8, 9, 15, 0, 0, time.UTC),
			Entities: []models.VisionEntityDescriptor{
				{Kind: "label", Value: "cat", DisplayName: "Cat"},
				{Kind: "label", Value: "dog", DisplayName: "Dog"},
			},
		})
	})

	catalog, err := service.RefreshCatalog(ctx, models.VisionEntityCatalogRefreshRequest{
		ServiceWSURL: wsURLFromHTTP(server.URL),
		ModelName:    "yolo11m-coco",
	})
	if err != nil {
		t.Fatalf("RefreshCatalog() error = %v", err)
	}

	request := waitFor(t, requests)
	if request.ModelName != "yolo11m-coco" {
		t.Fatalf("requested model_name = %q, want yolo11m-coco", request.ModelName)
	}
	if catalog.ServiceWSURL != wsURLFromHTTP(server.URL) {
		t.Fatalf("catalog service_ws_url = %q, want %q", catalog.ServiceWSURL, wsURLFromHTTP(server.URL))
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
	service, registrySvc, _, store := newVisionTestService(t)

	camera := visionTestCamera()
	if err := registrySvc.Upsert(ctx, []models.Device{camera}); err != nil {
		t.Fatalf("registry.Upsert() error = %v", err)
	}
	if err := store.UpsertVisionCatalog(ctx, models.VisionEntityCatalog{
		ServiceWSURL:  wsURLFromHTTP("http://vision.example"),
		SchemaVersion: visionEntityCatalogSchemaVersion,
		ModelName:     "custom-pets.pt",
		FetchedAt:     time.Now().UTC(),
		Entities: []models.VisionEntityDescriptor{
			{Kind: "label", Value: "cat", DisplayName: "Cat"},
		},
	}); err != nil {
		t.Fatalf("UpsertVisionCatalog() error = %v", err)
	}

	_, err := service.SaveConfig(ctx, models.VisionCapabilityConfig{
		ServiceWSURL:       wsURLFromHTTP("http://vision.example"),
		ModelName:          "custom-pets.pt",
		RecognitionEnabled: true,
		Rules: []models.VisionRule{{
			ID:                 "feeder-zone",
			Name:               "Feeder Zone",
			Enabled:            true,
			CameraDeviceID:     camera.ID,
			RecognitionEnabled: true,
			RTSPSource:         models.VisionRTSPSource{URL: "rtsp://user:pass@camera/stream"},
			EntitySelector:     models.VisionEntitySelector{Kind: "label", Value: "dog"},
			Zone:               models.VisionZoneBox{X: 0.1, Y: 0.2, Width: 0.3, Height: 0.4},
		}},
	})
	if err == nil {
		t.Fatal("SaveConfig() error = nil, want unsupported entity error")
	}
	if !strings.Contains(err.Error(), "not advertised") {
		t.Fatalf("SaveConfig() error = %v, want unsupported entity validation", err)
	}
}
