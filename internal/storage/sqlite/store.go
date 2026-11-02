package sqlite

import (
	"context"
	"database/sql"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db                *sql.DB
	maintenanceCancel context.CancelFunc
	maintenanceDone   chan struct{}
	closeOnce         sync.Once
	closeErr          error
}

func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.enableIncrementalVacuum(); err != nil {
		_ = db.Close()
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.maintenanceCancel = cancel
	s.maintenanceDone = make(chan struct{})
	go func() {
		defer close(s.maintenanceDone)
		s.runSpaceMaintenance(ctx)
	}()
	return s, nil
}

func (s *Store) Close() error {
	s.closeOnce.Do(func() {
		s.maintenanceCancel()
		<-s.maintenanceDone
		s.closeErr = s.db.Close()
	})
	return s.closeErr
}
