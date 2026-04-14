package vision

import (
	"context"
	"strings"
	"testing"

	"github.com/chentianyu/celestia/internal/models"
)

func TestSaveConfigRejectsKeyEntityWithoutImageOrDescription(t *testing.T) {
	ctx := context.Background()
	service, registrySvc, _, _ := newVisionTestService(t)

	camera := visionTestCamera()
	if err := registrySvc.Upsert(ctx, []models.Device{camera}); err != nil {
		t.Fatalf("registry.Upsert() error = %v", err)
	}

	_, err := service.SaveConfig(ctx, models.VisionCapabilityConfig{
		ServiceWSURL:       "ws://vision.example/ws/control",
		RecognitionEnabled: false,
		Rules: []models.VisionRule{{
			ID:                 "feeder-zone",
			Name:               "Feeder Zone",
			Enabled:            true,
			CameraDeviceID:     camera.ID,
			RecognitionEnabled: true,
			RTSPSource:         models.VisionRTSPSource{URL: "rtsp://user:pass@camera/stream"},
			EntitySelector:     models.VisionEntitySelector{Kind: "label", Value: "cat"},
			KeyEntities: []models.VisionRuleKeyEntity{
				{ID: 101},
			},
			Zone:                 models.VisionZoneBox{X: 0.1, Y: 0.2, Width: 0.3, Height: 0.4},
			StayThresholdSeconds: 5,
		}},
	})
	if err == nil {
		t.Fatal("SaveConfig() error = nil, want key entity validation error")
	}
	if !strings.Contains(err.Error(), "requires image or description") {
		t.Fatalf("SaveConfig() error = %v, want missing image/description validation", err)
	}
}

func TestNormalizeRuleKeyEntityDerivesGatewayDisplayName(t *testing.T) {
	item, err := normalizeRuleKeyEntity("feeder-zone", 0, models.VisionRuleKeyEntity{
		ID:          101,
		Description: " orange tabby with a blue collar ",
	})
	if err != nil {
		t.Fatalf("normalizeRuleKeyEntity() error = %v", err)
	}
	if item.Name != "orange tabby with a blue collar" {
		t.Fatalf("normalizeRuleKeyEntity().Name = %q, want description fallback", item.Name)
	}

	item, err = normalizeRuleKeyEntity("feeder-zone", 0, models.VisionRuleKeyEntity{
		ID:   102,
		Name: " Midnight ",
		Image: &models.VisionRuleKeyEntityImage{
			Base64: "ZmFrZS1pbWFnZQ==",
		},
	})
	if err != nil {
		t.Fatalf("normalizeRuleKeyEntity() with explicit name error = %v", err)
	}
	if item.Name != "Midnight" {
		t.Fatalf("normalizeRuleKeyEntity().Name = %q, want trimmed explicit name", item.Name)
	}
}

func TestGetConfigHydratesLegacyKeyEntityName(t *testing.T) {
	ctx := context.Background()
	service, registrySvc, _, store := newVisionTestService(t)

	camera := visionTestCamera()
	if err := registrySvc.Upsert(ctx, []models.Device{camera}); err != nil {
		t.Fatalf("registry.Upsert() error = %v", err)
	}

	if err := store.UpsertVisionConfig(ctx, models.VisionCapabilityConfig{
		ServiceWSURL:       "ws://vision.example/ws/control",
		RecognitionEnabled: false,
		Rules: []models.VisionRule{{
			ID:             "feeder-zone",
			Name:           "Feeder Zone",
			CameraDeviceID: camera.ID,
			KeyEntities: []models.VisionRuleKeyEntity{{
				ID:          101,
				Description: "orange tabby with a blue collar",
			}},
		}},
	}); err != nil {
		t.Fatalf("UpsertVisionConfig() error = %v", err)
	}

	config, err := service.GetConfig(ctx)
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}
	if config.Rules[0].KeyEntities[0].Name != "orange tabby with a blue collar" {
		t.Fatalf("GetConfig().Rules[0].KeyEntities[0].Name = %q, want description fallback", config.Rules[0].KeyEntities[0].Name)
	}
}
