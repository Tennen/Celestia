package sqlite

import (
	"context"
	"fmt"
	"log"
	"time"
)

const (
	spaceCheckInterval = time.Hour
	spaceCheckTimeout  = 5 * time.Second
	spaceReclaimMin    = int64(64 << 20)
	spaceReclaimBudget = int64(256 << 20)
	spaceReclaimBatch  = int64(4 << 20)
)

func (s *Store) enableIncrementalVacuum() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	var mode int
	if err := s.db.QueryRowContext(ctx, "pragma auto_vacuum").Scan(&mode); err != nil {
		return fmt.Errorf("read SQLite auto-vacuum mode: %w", err)
	}
	if mode == 2 {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, "pragma auto_vacuum = incremental"); err != nil {
		return fmt.Errorf("set SQLite incremental auto-vacuum: %w", err)
	}
	if err := s.db.QueryRowContext(ctx, "pragma auto_vacuum").Scan(&mode); err != nil {
		return err
	}
	// Existing NONE databases require one rebuild to add page back-pointers.
	// New databases and FULL databases can switch without a rebuild.
	if mode != 2 {
		log.Print("[sqlite] enabling incremental auto-vacuum; rebuilding existing database once")
		if _, err := s.db.ExecContext(ctx, "vacuum"); err != nil {
			return fmt.Errorf("migrate SQLite incremental auto-vacuum: %w", err)
		}
	}
	if err := s.db.QueryRowContext(ctx, "pragma auto_vacuum").Scan(&mode); err != nil {
		return err
	}
	if mode != 2 {
		return fmt.Errorf("SQLite incremental auto-vacuum was not enabled (mode=%d)", mode)
	}
	return nil
}

func (s *Store) runSpaceMaintenance(ctx context.Context) {
	ticker := time.NewTicker(spaceCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			checkCtx, cancel := context.WithTimeout(ctx, spaceCheckTimeout)
			err := s.reclaimFreePages(checkCtx, spaceReclaimMin, spaceReclaimBudget)
			cancel()
			if err != nil && ctx.Err() == nil {
				log.Printf("[sqlite] space maintenance deferred until next check: %v", err)
			}
		}
	}
}

func (s *Store) freePages(ctx context.Context) (pageSize, free int64, err error) {
	err = s.db.QueryRowContext(ctx, `
		select page_size, freelist_count from pragma_page_size(), pragma_freelist_count()
	`).Scan(&pageSize, &free)
	return
}

// Each batch releases the connection so foreground work can run between batches.
// The byte budget and caller deadline both bound a maintenance pass.
func (s *Store) reclaimFreePages(ctx context.Context, minBytes, budgetBytes int64) error {
	pageSize, free, err := s.freePages(ctx)
	if err != nil {
		return err
	}
	if free*pageSize < minBytes || budgetBytes < pageSize {
		return nil
	}
	remaining := budgetBytes / pageSize
	for free > 0 && remaining > 0 {
		pages := min(free, remaining, max(int64(1), spaceReclaimBatch/pageSize))
		rows, err := s.db.QueryContext(ctx, fmt.Sprintf("pragma incremental_vacuum(%d)", pages))
		if err != nil {
			return err
		}
		// SQLite yields during incremental_vacuum. Exec only steps once in our
		// driver; consume the statement fully to reclaim the requested batch.
		for rows.Next() {
		}
		err = rows.Err()
		closeErr := rows.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		remaining -= pages
		_, next, err := s.freePages(ctx)
		if err != nil {
			return err
		}
		if next >= free {
			return nil
		}
		free = next
	}
	return nil
}
