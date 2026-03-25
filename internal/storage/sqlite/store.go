package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/storage"
	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) EnsureSchema(ctx context.Context) error {
	stmts := []string{
		`create table if not exists plugin_installations (
			plugin_id text primary key,
			version text not null,
			status text not null,
			binary_path text not null,
			config_json text not null,
			config_ref text not null default '',
			installed_at text not null,
			updated_at text not null,
			last_heartbeat_at text,
			last_health_status text not null,
			metadata_json text not null default '{}'
		)`,
		`create table if not exists devices (
			device_id text primary key,
			plugin_id text not null,
			vendor_device_id text not null,
			kind text not null,
			name text not null,
			room text not null default '',
			online integer not null,
			capabilities_json text not null,
			metadata_json text not null default '{}'
		)`,
		`create table if not exists device_states (
			device_id text primary key,
			plugin_id text not null,
			ts text not null,
			state_json text not null
		)`,
		`create table if not exists events (
			id text primary key,
			type text not null,
			plugin_id text not null default '',
			device_id text not null default '',
			ts text not null,
			payload_json text not null default '{}'
		)`,
		`create table if not exists audits (
			id text primary key,
			actor text not null,
			device_id text not null,
			action text not null,
			params_json text not null default '{}',
			result text not null,
			risk_level text not null,
			allowed integer not null,
			created_at text not null
		)`,
		`create index if not exists idx_devices_plugin on devices(plugin_id)`,
		`create index if not exists idx_events_plugin_ts on events(plugin_id, ts desc)`,
		`create index if not exists idx_audits_created_at on audits(created_at desc)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

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

func (s *Store) UpsertDevice(ctx context.Context, device models.Device) error {
	capabilitiesJSON, err := marshalJSON(device.Capabilities)
	if err != nil {
		return err
	}
	metadataJSON, err := marshalJSON(device.Metadata)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		insert into devices(device_id, plugin_id, vendor_device_id, kind, name, room, online, capabilities_json, metadata_json)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?)
		on conflict(device_id) do update set
			plugin_id=excluded.plugin_id,
			vendor_device_id=excluded.vendor_device_id,
			kind=excluded.kind,
			name=excluded.name,
			room=excluded.room,
			online=excluded.online,
			capabilities_json=excluded.capabilities_json,
			metadata_json=excluded.metadata_json
	`, device.ID, device.PluginID, device.VendorDeviceID, device.Kind, device.Name, device.Room, boolToInt(device.Online), capabilitiesJSON, metadataJSON)
	return err
}

func (s *Store) GetDevice(ctx context.Context, deviceID string) (models.Device, bool, error) {
	rows, err := s.db.QueryContext(ctx, `
		select device_id, plugin_id, vendor_device_id, kind, name, room, online, capabilities_json, metadata_json
		from devices where device_id = ?
	`, deviceID)
	if err != nil {
		return models.Device{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return models.Device{}, false, nil
	}
	device, err := scanDevice(rows)
	return device, err == nil, err
}

func (s *Store) ListDevices(ctx context.Context, filter storage.DeviceFilter) ([]models.Device, error) {
	var (
		clauses []string
		args    []any
	)
	if filter.PluginID != "" {
		clauses = append(clauses, "plugin_id = ?")
		args = append(args, filter.PluginID)
	}
	if filter.Kind != "" {
		clauses = append(clauses, "kind = ?")
		args = append(args, filter.Kind)
	}
	if filter.Query != "" {
		clauses = append(clauses, "(device_id like ? or name like ? or room like ?)")
		pattern := "%" + filter.Query + "%"
		args = append(args, pattern, pattern, pattern)
	}
	query := `
		select device_id, plugin_id, vendor_device_id, kind, name, room, online, capabilities_json, metadata_json
		from devices
	`
	if len(clauses) > 0 {
		query += " where " + strings.Join(clauses, " and ")
	}
	query += " order by plugin_id, name"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Device
	for rows.Next() {
		device, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, device)
	}
	return out, rows.Err()
}

func (s *Store) DeleteDevicesByPlugin(ctx context.Context, pluginID string) error {
	if _, err := s.db.ExecContext(ctx, `delete from device_states where plugin_id = ?`, pluginID); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `delete from devices where plugin_id = ?`, pluginID)
	return err
}

func (s *Store) UpsertDeviceState(ctx context.Context, snapshot models.DeviceStateSnapshot) error {
	stateJSON, err := marshalJSON(snapshot.State)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		insert into device_states(device_id, plugin_id, ts, state_json)
		values (?, ?, ?, ?)
		on conflict(device_id) do update set
			plugin_id=excluded.plugin_id,
			ts=excluded.ts,
			state_json=excluded.state_json
	`, snapshot.DeviceID, snapshot.PluginID, snapshot.TS.UTC().Format(time.RFC3339Nano), stateJSON)
	return err
}

func (s *Store) GetDeviceState(ctx context.Context, deviceID string) (models.DeviceStateSnapshot, bool, error) {
	rows, err := s.db.QueryContext(ctx, `
		select device_id, plugin_id, ts, state_json
		from device_states where device_id = ?
	`, deviceID)
	if err != nil {
		return models.DeviceStateSnapshot{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return models.DeviceStateSnapshot{}, false, nil
	}
	state, err := scanState(rows)
	return state, err == nil, err
}

func (s *Store) ListDeviceStates(ctx context.Context, filter storage.StateFilter) ([]models.DeviceStateSnapshot, error) {
	var (
		clauses []string
		args    []any
	)
	if filter.PluginID != "" {
		clauses = append(clauses, "plugin_id = ?")
		args = append(args, filter.PluginID)
	}
	if filter.DeviceID != "" {
		clauses = append(clauses, "device_id = ?")
		args = append(args, filter.DeviceID)
	}
	query := `select device_id, plugin_id, ts, state_json from device_states`
	if len(clauses) > 0 {
		query += " where " + strings.Join(clauses, " and ")
	}
	query += " order by ts desc"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.DeviceStateSnapshot
	for rows.Next() {
		state, err := scanState(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, state)
	}
	return out, rows.Err()
}

func (s *Store) AppendEvent(ctx context.Context, event models.Event) error {
	payloadJSON, err := marshalJSON(event.Payload)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		insert into events(id, type, plugin_id, device_id, ts, payload_json)
		values (?, ?, ?, ?, ?, ?)
	`, event.ID, event.Type, event.PluginID, event.DeviceID, event.TS.UTC().Format(time.RFC3339Nano), payloadJSON)
	return err
}

func (s *Store) ListEvents(ctx context.Context, filter storage.EventFilter) ([]models.Event, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	var (
		clauses []string
		args    []any
	)
	if filter.PluginID != "" {
		clauses = append(clauses, "plugin_id = ?")
		args = append(args, filter.PluginID)
	}
	if filter.DeviceID != "" {
		clauses = append(clauses, "device_id = ?")
		args = append(args, filter.DeviceID)
	}
	query := `select id, type, plugin_id, device_id, ts, payload_json from events`
	if len(clauses) > 0 {
		query += " where " + strings.Join(clauses, " and ")
	}
	query += " order by ts desc limit ?"
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Event
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, event)
	}
	return out, rows.Err()
}

func (s *Store) CountEvents(ctx context.Context) (int, error) {
	return count(ctx, s.db, `select count(*) from events`)
}

func (s *Store) AppendAudit(ctx context.Context, audit models.AuditRecord) error {
	paramsJSON, err := marshalJSON(audit.Params)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		insert into audits(id, actor, device_id, action, params_json, result, risk_level, allowed, created_at)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, audit.ID, audit.Actor, audit.DeviceID, audit.Action, paramsJSON, audit.Result, audit.RiskLevel, boolToInt(audit.Allowed), audit.CreatedAt.UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) ListAudits(ctx context.Context, filter storage.AuditFilter) ([]models.AuditRecord, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	var (
		clauses []string
		args    []any
	)
	if filter.DeviceID != "" {
		clauses = append(clauses, "device_id = ?")
		args = append(args, filter.DeviceID)
	}
	query := `select id, actor, device_id, action, params_json, result, risk_level, allowed, created_at from audits`
	if len(clauses) > 0 {
		query += " where " + strings.Join(clauses, " and ")
	}
	query += " order by created_at desc limit ?"
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.AuditRecord
	for rows.Next() {
		audit, err := scanAudit(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, audit)
	}
	return out, rows.Err()
}

func (s *Store) CountAudits(ctx context.Context) (int, error) {
	return count(ctx, s.db, `select count(*) from audits`)
}

func scanPluginRecord(scanner interface{ Scan(...any) error }) (models.PluginInstallRecord, error) {
	var (
		record       models.PluginInstallRecord
		configJSON   string
		metadataJSON string
		installedAt  string
		updatedAt    string
		lastBeat     sql.NullString
	)
	if err := scanner.Scan(&record.PluginID, &record.Version, &record.Status, &record.BinaryPath, &configJSON,
		&record.ConfigRef, &installedAt, &updatedAt, &lastBeat, &record.LastHealthStatus, &metadataJSON); err != nil {
		return models.PluginInstallRecord{}, err
	}
	if err := parseJSON(configJSON, &record.Config); err != nil {
		return models.PluginInstallRecord{}, err
	}
	if err := parseJSON(metadataJSON, &record.Metadata); err != nil {
		return models.PluginInstallRecord{}, err
	}
	var err error
	record.InstalledAt, err = time.Parse(time.RFC3339Nano, installedAt)
	if err != nil {
		return models.PluginInstallRecord{}, err
	}
	record.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return models.PluginInstallRecord{}, err
	}
	if lastBeat.Valid {
		t, err := time.Parse(time.RFC3339Nano, lastBeat.String)
		if err != nil {
			return models.PluginInstallRecord{}, err
		}
		record.LastHeartbeatAt = &t
	}
	return record, nil
}

func scanDevice(scanner interface{ Scan(...any) error }) (models.Device, error) {
	var (
		device           models.Device
		capabilitiesJSON string
		metadataJSON     string
		online           int
	)
	if err := scanner.Scan(&device.ID, &device.PluginID, &device.VendorDeviceID, &device.Kind, &device.Name, &device.Room, &online, &capabilitiesJSON, &metadataJSON); err != nil {
		return models.Device{}, err
	}
	device.Online = online == 1
	if err := parseJSON(capabilitiesJSON, &device.Capabilities); err != nil {
		return models.Device{}, err
	}
	if err := parseJSON(metadataJSON, &device.Metadata); err != nil {
		return models.Device{}, err
	}
	return device, nil
}

func scanState(scanner interface{ Scan(...any) error }) (models.DeviceStateSnapshot, error) {
	var (
		state     models.DeviceStateSnapshot
		ts        string
		stateJSON string
	)
	if err := scanner.Scan(&state.DeviceID, &state.PluginID, &ts, &stateJSON); err != nil {
		return models.DeviceStateSnapshot{}, err
	}
	parsed, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return models.DeviceStateSnapshot{}, err
	}
	state.TS = parsed
	if err := parseJSON(stateJSON, &state.State); err != nil {
		return models.DeviceStateSnapshot{}, err
	}
	return state, nil
}

func scanEvent(scanner interface{ Scan(...any) error }) (models.Event, error) {
	var (
		event       models.Event
		ts          string
		payloadJSON string
	)
	if err := scanner.Scan(&event.ID, &event.Type, &event.PluginID, &event.DeviceID, &ts, &payloadJSON); err != nil {
		return models.Event{}, err
	}
	parsed, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return models.Event{}, err
	}
	event.TS = parsed
	if err := parseJSON(payloadJSON, &event.Payload); err != nil {
		return models.Event{}, err
	}
	return event, nil
}

func scanAudit(scanner interface{ Scan(...any) error }) (models.AuditRecord, error) {
	var (
		audit      models.AuditRecord
		paramsJSON string
		allowed    int
		createdAt  string
	)
	if err := scanner.Scan(&audit.ID, &audit.Actor, &audit.DeviceID, &audit.Action, &paramsJSON, &audit.Result, &audit.RiskLevel, &allowed, &createdAt); err != nil {
		return models.AuditRecord{}, err
	}
	parsed, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return models.AuditRecord{}, err
	}
	audit.CreatedAt = parsed
	audit.Allowed = allowed == 1
	if err := parseJSON(paramsJSON, &audit.Params); err != nil {
		return models.AuditRecord{}, err
	}
	return audit, nil
}

func marshalJSON(v any) (string, error) {
	if v == nil {
		v = map[string]any{}
	}
	bytes, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func parseJSON(data string, out any) error {
	if data == "" {
		data = "{}"
	}
	return json.Unmarshal([]byte(data), out)
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func count(ctx context.Context, db *sql.DB, query string) (int, error) {
	var value int
	if err := db.QueryRowContext(ctx, query).Scan(&value); err != nil {
		return 0, err
	}
	return value, nil
}

