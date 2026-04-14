package audit

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/storage/sqlite"
)

func TestAppendPublishesToSubscribers(t *testing.T) {
	store, err := sqlite.New(filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatalf("sqlite.New() error = %v", err)
	}
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}
	service := New(store)

	subID, ch := service.Subscribe(1)
	defer service.Unsubscribe(subID)

	record := models.AuditRecord{
		ID:        "audit-1",
		Actor:     "admin",
		DeviceID:  "device-1",
		Action:    "toggle",
		Result:    "accepted",
		RiskLevel: models.RiskLevelLow,
		Allowed:   true,
		CreatedAt: time.Now().UTC(),
	}
	if err := service.Append(context.Background(), record); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	select {
	case published := <-ch:
		if published.ID != record.ID {
			t.Fatalf("published audit id = %q, want %q", published.ID, record.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for audit publish")
	}
}
