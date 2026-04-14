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
