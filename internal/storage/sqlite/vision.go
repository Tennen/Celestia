package sqlite

import (
	"context"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

func (s *Store) UpsertVisionConfig(ctx context.Context, config models.VisionCapabilityConfig) error {
	rulesJSON, err := marshalJSON(config.Rules)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		insert into vision_capability_config(
			capability_id, service_url, recognition_enabled, rules_json, updated_at
		) values (?, ?, ?, ?, ?)
		on conflict(capability_id) do update set
			service_url=excluded.service_url,
			recognition_enabled=excluded.recognition_enabled,
			rules_json=excluded.rules_json,
			updated_at=excluded.updated_at
	`,
		models.VisionCapabilityID,
		strings.TrimSpace(config.ServiceURL),
		boolToInt(config.RecognitionEnabled),
		rulesJSON,
		config.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	return err
}

func (s *Store) GetVisionConfig(ctx context.Context) (models.VisionCapabilityConfig, bool, error) {
	rows, err := s.db.QueryContext(ctx, `
		select capability_id, service_url, recognition_enabled, rules_json, updated_at
		from vision_capability_config where capability_id = ?
	`, models.VisionCapabilityID)
	if err != nil {
		return models.VisionCapabilityConfig{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return models.VisionCapabilityConfig{}, false, nil
	}
	config, err := scanVisionConfig(rows)
	return config, err == nil, err
}

func (s *Store) UpsertVisionCatalog(ctx context.Context, catalog models.VisionEntityCatalog) error {
	entitiesJSON, err := marshalJSON(catalog.Entities)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		insert into vision_capability_catalog(
			capability_id, service_url, schema_version, service_version, model_name, entities_json, fetched_at
		) values (?, ?, ?, ?, ?, ?, ?)
		on conflict(capability_id) do update set
			service_url=excluded.service_url,
			schema_version=excluded.schema_version,
			service_version=excluded.service_version,
			model_name=excluded.model_name,
			entities_json=excluded.entities_json,
			fetched_at=excluded.fetched_at
	`,
		models.VisionCapabilityID,
		strings.TrimSpace(catalog.ServiceURL),
		strings.TrimSpace(catalog.SchemaVersion),
		strings.TrimSpace(catalog.ServiceVersion),
		strings.TrimSpace(catalog.ModelName),
		entitiesJSON,
		catalog.FetchedAt.UTC().Format(time.RFC3339Nano),
	)
	return err
}

func (s *Store) GetVisionCatalog(ctx context.Context) (models.VisionEntityCatalog, bool, error) {
	rows, err := s.db.QueryContext(ctx, `
		select capability_id, service_url, schema_version, service_version, model_name, entities_json, fetched_at
		from vision_capability_catalog where capability_id = ?
	`, models.VisionCapabilityID)
	if err != nil {
		return models.VisionEntityCatalog{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return models.VisionEntityCatalog{}, false, nil
	}
	catalog, err := scanVisionCatalog(rows)
	return catalog, err == nil, err
}

func (s *Store) UpsertVisionStatus(ctx context.Context, status models.VisionCapabilityStatus) error {
	runtimeJSON, err := marshalJSON(status.Runtime)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		insert into vision_capability_status(
			capability_id, status, message, service_version, last_synced_at, last_reported_at,
			last_event_at, runtime_json, sync_error, updated_at
		) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		on conflict(capability_id) do update set
			status=excluded.status,
			message=excluded.message,
			service_version=excluded.service_version,
			last_synced_at=excluded.last_synced_at,
			last_reported_at=excluded.last_reported_at,
			last_event_at=excluded.last_event_at,
			runtime_json=excluded.runtime_json,
			sync_error=excluded.sync_error,
			updated_at=excluded.updated_at
	`,
		models.VisionCapabilityID,
		status.Status,
		strings.TrimSpace(status.Message),
		strings.TrimSpace(status.ServiceVersion),
		nullableTime(status.LastSyncedAt),
		nullableTime(status.LastReportedAt),
		nullableTime(status.LastEventAt),
		runtimeJSON,
		strings.TrimSpace(status.SyncError),
		status.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	return err
}

func (s *Store) GetVisionStatus(ctx context.Context) (models.VisionCapabilityStatus, bool, error) {
	rows, err := s.db.QueryContext(ctx, `
		select capability_id, status, message, service_version, last_synced_at, last_reported_at,
		       last_event_at, runtime_json, sync_error, updated_at
		from vision_capability_status where capability_id = ?
	`, models.VisionCapabilityID)
	if err != nil {
		return models.VisionCapabilityStatus{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return models.VisionCapabilityStatus{}, false, nil
	}
	status, err := scanVisionStatus(rows)
	return status, err == nil, err
}

func nullableTime(ts *time.Time) any {
	if ts == nil {
		return nil
	}
	return ts.UTC().Format(time.RFC3339Nano)
}
