package sqlite

import (
	"context"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/storage"
)

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

func (s *Store) GetEvent(ctx context.Context, id string) (models.Event, bool, error) {
	rows, err := s.db.QueryContext(ctx, `
		select id, type, plugin_id, device_id, ts, payload_json from events where id = ?
	`, strings.TrimSpace(id))
	if err != nil {
		return models.Event{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return models.Event{}, false, nil
	}
	event, err := scanEvent(rows)
	return event, err == nil, err
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
