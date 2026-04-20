package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

func TestEnsureSchemaAllowsMultipleVisionCapturesPerEventPhase(t *testing.T) {
	ctx := context.Background()
	store, err := New(filepath.Join(t.TempDir(), "vision-captures.db"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer store.Close()

	if _, err := store.db.ExecContext(ctx, `
		create table vision_event_captures (
			capture_id text primary key,
			event_id text not null,
			rule_id text not null default '',
			camera_device_id text not null default '',
			phase text not null,
			captured_at text not null,
			content_type text not null,
			size_bytes integer not null default 0,
			metadata_json text not null default '{}',
			image_data blob not null
		)
	`); err != nil {
		t.Fatalf("create legacy vision_event_captures table error = %v", err)
	}
	if _, err := store.db.ExecContext(ctx, `
		create unique index idx_vision_event_captures_event_phase on vision_event_captures(event_id, phase)
	`); err != nil {
		t.Fatalf("create legacy unique index error = %v", err)
	}
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}

	base := time.Date(2026, 4, 20, 8, 0, 0, 0, time.UTC)
	for idx, captureID := range []string{"evt-1:middle:1", "evt-1:middle:2"} {
		if err := store.UpsertVisionEventCapture(ctx, models.VisionEventCaptureAsset{
			Capture: models.VisionEventCapture{
				CaptureID:      captureID,
				EventID:        "evt-1",
				RuleID:         "feeder-zone",
				CameraDeviceID: "hikvision:camera:entry-1",
				Phase:          models.VisionEventCapturePhaseMiddle,
				CapturedAt:     base.Add(time.Duration(idx) * time.Second),
				ContentType:    "image/jpeg",
				SizeBytes:      5,
			},
			Data: []byte("image"),
		}); err != nil {
			t.Fatalf("UpsertVisionEventCapture(%s) error = %v", captureID, err)
		}
	}

	captures, err := store.ListVisionEventCaptures(ctx, []string{"evt-1"})
	if err != nil {
		t.Fatalf("ListVisionEventCaptures() error = %v", err)
	}
	if len(captures["evt-1"]) != 2 {
		t.Fatalf("captures len = %d, want 2", len(captures["evt-1"]))
	}
}
