package sqlite

import (
	"context"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/storage"
)

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

func (s *Store) UpsertDeviceControlPreference(ctx context.Context, pref models.DeviceControlPreference) error {
	alias := strings.TrimSpace(pref.Alias)
	if alias == "" && pref.Visible {
		_, err := s.db.ExecContext(ctx, `delete from device_control_preferences where device_id = ? and control_id = ?`, pref.DeviceID, pref.ControlID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		insert into device_control_preferences(device_id, control_id, alias, visible, updated_at)
		values (?, ?, ?, ?, ?)
		on conflict(device_id, control_id) do update set
			alias=excluded.alias,
			visible=excluded.visible,
			updated_at=excluded.updated_at
	`, pref.DeviceID, pref.ControlID, alias, boolToInt(pref.Visible), pref.UpdatedAt.UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) ListDeviceControlPreferences(ctx context.Context, deviceID string) ([]models.DeviceControlPreference, error) {
	rows, err := s.db.QueryContext(ctx, `
		select device_id, control_id, alias, visible, updated_at
		from device_control_preferences
		where device_id = ?
		order by control_id
	`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.DeviceControlPreference
	for rows.Next() {
		var (
			pref      models.DeviceControlPreference
			visible   int
			updatedAt string
		)
		if err := rows.Scan(&pref.DeviceID, &pref.ControlID, &pref.Alias, &visible, &updatedAt); err != nil {
			return nil, err
		}
		pref.Visible = visible != 0
		if updatedAt != "" {
			parsed, err := time.Parse(time.RFC3339Nano, updatedAt)
			if err != nil {
				return nil, err
			}
			pref.UpdatedAt = parsed.UTC()
		}
		out = append(out, pref)
	}
	return out, rows.Err()
}
