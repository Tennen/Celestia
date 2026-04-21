package sqlite

import (
	"context"
	"strings"
)

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
		`create table if not exists automations (
			id text primary key,
			name text not null,
			enabled integer not null default 1,
			condition_logic text not null default 'all',
			conditions_json text not null default '[]',
			time_window_json text not null default '{}',
			actions_json text not null default '[]',
			last_triggered_at text,
			last_run_status text not null default 'idle',
			last_error text not null default '',
			created_at text not null,
			updated_at text not null
		)`,
		`create table if not exists agent_documents (
			key text primary key,
			domain text not null,
			payload_json text not null default '{}',
			updated_at text not null
		)`,
		`create table if not exists vision_capability_config (
			capability_id text primary key,
			service_url text not null default '',
			recognition_enabled integer not null default 0,
			event_capture_retention_hours integer not null default 168,
			rules_json text not null default '[]',
			updated_at text not null
		)`,
		`create table if not exists vision_capability_catalog (
			capability_id text primary key,
			service_url text not null default '',
			schema_version text not null default '',
			service_version text not null default '',
			model_name text not null default '',
			entities_json text not null default '[]',
			fetched_at text not null
		)`,
		`create table if not exists vision_capability_status (
			capability_id text primary key,
			status text not null,
			message text not null default '',
			service_version text not null default '',
			last_synced_at text,
			last_reported_at text,
			last_event_at text,
			runtime_json text not null default '{}',
			sync_error text not null default '',
			updated_at text not null
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
		`create table if not exists device_preferences (
			device_id text primary key,
			alias text not null default '',
			updated_at text not null
		)`,
		`create table if not exists device_control_preferences (
			device_id text not null,
			control_id text not null,
			alias text not null default '',
			visible integer not null default 1,
			updated_at text not null,
			primary key(device_id, control_id)
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
		`create table if not exists vision_event_captures (
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
		)`,
		`create table if not exists oauth_sessions (
			id text primary key,
			provider text not null,
			plugin_id text not null default '',
			account_name text not null default '',
			region text not null default '',
			client_id text not null default '',
			redirect_url text not null default '',
			device_id text not null default '',
			state text not null,
			auth_url text not null default '',
			status text not null,
			error_text text not null default '',
			account_config_json text not null default '{}',
			created_at text not null,
			updated_at text not null,
			completed_at text,
			state_expires_at text,
			token_expires_at text
		)`,
		`create index if not exists idx_devices_plugin on devices(plugin_id)`,
		`create index if not exists idx_device_preferences_alias on device_preferences(alias)`,
		`create index if not exists idx_device_control_preferences_device on device_control_preferences(device_id)`,
		`create index if not exists idx_automations_enabled on automations(enabled, updated_at desc)`,
		`create index if not exists idx_agent_documents_domain on agent_documents(domain, updated_at desc)`,
		`create index if not exists idx_events_plugin_ts on events(plugin_id, ts desc)`,
		`drop index if exists idx_vision_event_captures_event_phase`,
		`create index if not exists idx_vision_event_captures_event_captured_at on vision_event_captures(event_id, captured_at asc, capture_id)`,
		`create index if not exists idx_vision_event_captures_captured_at on vision_event_captures(captured_at desc)`,
		`create index if not exists idx_audits_created_at on audits(created_at desc)`,
		`create unique index if not exists idx_oauth_sessions_provider_state on oauth_sessions(provider, state)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	if err := s.addColumnIfMissing(ctx, "alter table vision_capability_config add column event_capture_retention_hours integer not null default 168"); err != nil {
		return err
	}
	return nil
}

func (s *Store) addColumnIfMissing(ctx context.Context, stmt string) error {
	if _, err := s.db.ExecContext(ctx, stmt); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		return err
	}
	return nil
}
