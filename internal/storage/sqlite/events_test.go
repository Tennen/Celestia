package sqlite

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/storage"
)

func TestListEvents_AppliesTimeRangeAndCursor(t *testing.T) {
	ctx := context.Background()
	store, err := New(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer store.Close()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}

	base := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)
	for idx := 0; idx < 4; idx++ {
		event := models.Event{
			ID:       fmt.Sprintf("evt-%d", idx+1),
			Type:     models.EventDeviceStateChanged,
			PluginID: "xiaomi",
			DeviceID: "device-1",
			TS:       base.Add(time.Duration(idx) * time.Hour),
			Payload:  map[string]any{"index": idx},
		}
		if err := store.AppendEvent(ctx, event); err != nil {
			t.Fatalf("AppendEvent(%d) error = %v", idx, err)
		}
	}

	fromTS := base.Add(90 * time.Minute)
	toTS := base.Add(4 * time.Hour)
	items, err := store.ListEvents(ctx, storage.EventFilter{
		PluginID: "xiaomi",
		FromTS:   &fromTS,
		ToTS:     &toTS,
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("ListEvents(range) error = %v", err)
	}
	if len(items) != 2 || items[0].ID != "evt-4" || items[1].ID != "evt-3" {
		t.Fatalf("ListEvents(range) = %#v, want [evt-4 evt-3]", items)
	}

	beforeTS := base.Add(3 * time.Hour)
	items, err = store.ListEvents(ctx, storage.EventFilter{
		PluginID: "xiaomi",
		FromTS:   &fromTS,
		ToTS:     &toTS,
		BeforeTS: &beforeTS,
		BeforeID: "evt-4",
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("ListEvents(cursor) error = %v", err)
	}
	if len(items) != 1 || items[0].ID != "evt-3" {
		t.Fatalf("ListEvents(cursor) = %#v, want [evt-3]", items)
	}
}
