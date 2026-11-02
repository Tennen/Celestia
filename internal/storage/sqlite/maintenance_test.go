package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func pragmaInt(t *testing.T, db *sql.DB, name string) int64 {
	t.Helper()
	var n int64
	if err := db.QueryRow("pragma " + name).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestNewEnablesIncrementalVacuumAndCloseStopsMaintenance(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.db")
	s, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := pragmaInt(t, s.db, "auto_vacuum"); mode != 2 {
		t.Fatalf("mode = %d, want incremental", mode)
	}
	if err := s.EnsureSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-s.maintenanceDone:
	case <-time.After(time.Second):
		t.Fatal("Close left maintenance running")
	}
	if err := s.Close(); err != nil {
		t.Fatal("repeated Close", err)
	}
	s, err = New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if mode := pragmaInt(t, s.db, "auto_vacuum"); mode != 2 {
		t.Fatalf("reopened mode = %d", mode)
	}
}

func addMaintenanceRows(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec("create table retained(id integer primary key, note text, body blob)"); err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	for i := 1; i <= 128; i++ {
		if _, err := tx.Exec("insert into retained values (?, ?, zeroblob(32768))", i, "saved content"); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("delete from retained where id > 1"); err != nil {
		t.Fatal(err)
	}
}

func checkMaintenanceData(t *testing.T, db *sql.DB) {
	t.Helper()
	var count, id int
	var note string
	var body []byte
	if err := db.QueryRow("select count(*) from retained").Scan(&count); err != nil || count != 1 {
		t.Fatalf("retained count = %d: %v", count, err)
	}
	if err := db.QueryRow("select id,note,body from retained").Scan(&id, &note, &body); err != nil {
		t.Fatal(err)
	}
	if id != 1 || note != "saved content" || len(body) != 32768 {
		t.Fatal("retained row changed")
	}
	for _, b := range body {
		if b != 0 {
			t.Fatal("retained blob changed")
		}
	}
	var result string
	if err := db.QueryRow("pragma integrity_check").Scan(&result); err != nil || result != "ok" {
		t.Fatalf("integrity_check = %q: %v", result, err)
	}
}

func TestNewMigratesLegacyDatabaseWithoutLosingRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	addMaintenanceRows(t, db)
	if pragmaInt(t, db, "auto_vacuum") != 0 || pragmaInt(t, db, "freelist_count") == 0 {
		t.Fatal("legacy test database must have NONE mode and free pages")
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(path)
	s, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if pragmaInt(t, s.db, "auto_vacuum") != 2 || pragmaInt(t, s.db, "freelist_count") != 0 {
		t.Fatal("legacy database was not rebuilt in incremental mode")
	}
	after, _ := os.Stat(path)
	if after.Size() >= before.Size() {
		t.Fatal("migration did not reclaim existing empty pages")
	}
	checkMaintenanceData(t, s.db)
}

func TestIncrementalVacuumRespectsThresholdAndBatchBudget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reclaim.db")
	s, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	addMaintenanceRows(t, s.db)
	ctx := context.Background()
	pageSize, free, err := s.freePages(ctx)
	if err != nil || free < 16 {
		t.Fatalf("insufficient free pages: %d, %v", free, err)
	}
	before, _ := os.Stat(path)
	if err := s.reclaimFreePages(ctx, free*pageSize+1, 8*pageSize); err != nil {
		t.Fatal(err)
	}
	if n := pragmaInt(t, s.db, "freelist_count"); n != free {
		t.Fatal("reclaimed below threshold")
	}
	if err := s.reclaimFreePages(ctx, 0, 8*pageSize); err != nil {
		t.Fatal(err)
	}
	if n := pragmaInt(t, s.db, "freelist_count"); free-n != 8 {
		t.Fatalf("reclaimed %d pages, want full batch of 8", free-n)
	}
	after, _ := os.Stat(path)
	if after.Size() >= before.Size() {
		t.Fatal("incremental vacuum did not truncate database")
	}
	if err := s.reclaimFreePages(ctx, 0, free*pageSize); err != nil {
		t.Fatal(err)
	}
	if n := pragmaInt(t, s.db, "freelist_count"); n != 0 {
		t.Fatalf("remaining free pages = %d", n)
	}
	checkMaintenanceData(t, s.db)
}

func TestIncrementalVacuumHonorsCancellation(t *testing.T) {
	s, err := New(filepath.Join(t.TempDir(), "cancel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.reclaimFreePages(ctx, 0, spaceReclaimBudget); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled reclaim = %v", err)
	}
}
