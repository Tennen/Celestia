package sqlite

import (
	"context"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

func (s *Store) UpsertPluginRecord(ctx context.Context, record models.PluginInstallRecord) error {
	configJSON, err := marshalJSON(record.Config)
	if err != nil {
		return err
	}
	metadataJSON, err := marshalJSON(record.Metadata)
	if err != nil {
		return err
	}
	var heartbeat any
	if record.LastHeartbeatAt != nil {
		heartbeat = record.LastHeartbeatAt.UTC().Format(time.RFC3339Nano)
	}
	_, err = s.db.ExecContext(ctx, `
		insert into plugin_installations(
			plugin_id, version, status, binary_path, config_json, config_ref,
			installed_at, updated_at, last_heartbeat_at, last_health_status, metadata_json
		) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		on conflict(plugin_id) do update set
			version=excluded.version,
			status=excluded.status,
			binary_path=excluded.binary_path,
			config_json=excluded.config_json,
			config_ref=excluded.config_ref,
			updated_at=excluded.updated_at,
			last_heartbeat_at=excluded.last_heartbeat_at,
			last_health_status=excluded.last_health_status,
			metadata_json=excluded.metadata_json
	`, record.PluginID, record.Version, record.Status, record.BinaryPath, configJSON,
		record.ConfigRef, record.InstalledAt.UTC().Format(time.RFC3339Nano),
		record.UpdatedAt.UTC().Format(time.RFC3339Nano), heartbeat, record.LastHealthStatus, metadataJSON)
	return err
}

func (s *Store) GetPluginRecord(ctx context.Context, pluginID string) (models.PluginInstallRecord, bool, error) {
	rows, err := s.db.QueryContext(ctx, `
		select plugin_id, version, status, binary_path, config_json, config_ref,
		       installed_at, updated_at, last_heartbeat_at, last_health_status, metadata_json
		from plugin_installations where plugin_id = ?
	`, pluginID)
	if err != nil {
		return models.PluginInstallRecord{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return models.PluginInstallRecord{}, false, nil
	}
	record, err := scanPluginRecord(rows)
	return record, err == nil, err
}

func (s *Store) ListPluginRecords(ctx context.Context) ([]models.PluginInstallRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		select plugin_id, version, status, binary_path, config_json, config_ref,
		       installed_at, updated_at, last_heartbeat_at, last_health_status, metadata_json
		from plugin_installations order by plugin_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.PluginInstallRecord
	for rows.Next() {
		record, err := scanPluginRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	return out, rows.Err()
}

func (s *Store) DeletePluginRecord(ctx context.Context, pluginID string) error {
	_, err := s.db.ExecContext(ctx, `delete from plugin_installations where plugin_id = ?`, pluginID)
	return err
}
