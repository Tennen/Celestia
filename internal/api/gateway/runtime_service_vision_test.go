package gateway

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	runtimepkg "github.com/chentianyu/celestia/internal/core/runtime"
	"github.com/chentianyu/celestia/internal/models"
	sqlitestore "github.com/chentianyu/celestia/internal/storage/sqlite"
)

const visionTestCatalogSchemaVersion = "celestia.vision.catalog.v1"

func newRuntimeServiceVisionTest(t *testing.T) (*RuntimeService, *runtimepkg.Runtime, *sqlitestore.Store) {
	t.Helper()

	store, err := sqlitestore.New(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatalf("sqlite.New() error = %v", err)
	}
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}
	runtime := runtimepkg.New(store)
	service := NewRuntimeService(runtime)
	t.Cleanup(func() {
		_ = store.Close()
	})
	return service, runtime, store
}

func TestSaveVisionCapabilityConfigPreservesCatalogInResponse(t *testing.T) {
	ctx := context.Background()
	service, runtime, store := newRuntimeServiceVisionTest(t)

	camera := models.Device{
		ID:             "hikvision:camera:entry-1",
		PluginID:       "hikvision",
		VendorDeviceID: "192.0.2.10:8000:ch1",
		Kind:           models.DeviceKindCameraLike,
		Name:           "Patio Camera",
		Online:         true,
		Capabilities:   []string{"stream"},
	}
	if err := runtime.Registry.Upsert(ctx, []models.Device{camera}); err != nil {
		t.Fatalf("registry.Upsert() error = %v", err)
	}

	const serviceWSURL = "ws://vision.example/api/v1/capabilities/vision_entity_stay_zone"
	if err := store.UpsertVisionCatalog(ctx, models.VisionEntityCatalog{
		ServiceWSURL:  serviceWSURL,
		SchemaVersion: visionTestCatalogSchemaVersion,
		ModelName:     "custom-pets.pt",
		FetchedAt:     time.Date(2026, 4, 12, 4, 0, 0, 0, time.UTC),
		Entities: []models.VisionEntityDescriptor{
			{Kind: "label", Value: "cat", DisplayName: "Cat"},
		},
	}); err != nil {
		t.Fatalf("UpsertVisionCatalog() error = %v", err)
	}

	detail, err := service.SaveVisionCapabilityConfig(ctx, models.VisionCapabilityConfig{
		ServiceWSURL:       serviceWSURL,
		ModelName:          "custom-pets.pt",
		RecognitionEnabled: false,
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
	})
	if err != nil {
		t.Fatalf("SaveVisionCapabilityConfig() error = %v", err)
	}
	if detail.Vision == nil || detail.Vision.Catalog == nil {
		t.Fatal("SaveVisionCapabilityConfig() response dropped the persisted entity catalog")
	}
	if len(detail.Vision.Catalog.Entities) != 1 {
		t.Fatalf("catalog entities len = %d, want 1", len(detail.Vision.Catalog.Entities))
	}
	if detail.Vision.Catalog.Entities[0].Value != "cat" {
		t.Fatalf("catalog entity value = %q, want cat", detail.Vision.Catalog.Entities[0].Value)
	}
}
