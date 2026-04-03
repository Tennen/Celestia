package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

func (s *Store) UpsertAutomation(ctx context.Context, automation models.Automation) error {
	triggerJSON, err := marshalJSON(automation.Trigger)
	if err != nil {
		return err
	}
	conditionsJSON, err := marshalJSON(automation.Conditions)
	if err != nil {
		return err
	}
	timeWindowJSON, err := marshalJSON(automation.TimeWindow)
	if err != nil {
		return err
	}
	actionsJSON, err := marshalJSON(automation.Actions)
	if err != nil {
		return err
	}
	var lastTriggeredAt any
	if automation.LastTriggeredAt != nil {
		lastTriggeredAt = automation.LastTriggeredAt.UTC().Format(time.RFC3339Nano)
	}
	_, err = s.db.ExecContext(ctx, `
		insert into automations(
			id, name, enabled, trigger_json, condition_logic, conditions_json, time_window_json,
			actions_json, last_triggered_at, last_run_status, last_error, created_at, updated_at
		) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		on conflict(id) do update set
			name=excluded.name,
			enabled=excluded.enabled,
			trigger_json=excluded.trigger_json,
			condition_logic=excluded.condition_logic,
			conditions_json=excluded.conditions_json,
			time_window_json=excluded.time_window_json,
			actions_json=excluded.actions_json,
			last_triggered_at=excluded.last_triggered_at,
			last_run_status=excluded.last_run_status,
			last_error=excluded.last_error,
			updated_at=excluded.updated_at
	`, automation.ID, strings.TrimSpace(automation.Name), boolToInt(automation.Enabled), triggerJSON, automation.ConditionLogic,
		conditionsJSON, timeWindowJSON, actionsJSON, lastTriggeredAt, automation.LastRunStatus, automation.LastError,
		automation.CreatedAt.UTC().Format(time.RFC3339Nano), automation.UpdatedAt.UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) GetAutomation(ctx context.Context, id string) (models.Automation, bool, error) {
	rows, err := s.db.QueryContext(ctx, `
		select id, name, enabled, trigger_json, condition_logic, conditions_json, time_window_json,
		       actions_json, last_triggered_at, last_run_status, last_error, created_at, updated_at
		from automations where id = ?
	`, id)
	if err != nil {
		return models.Automation{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return models.Automation{}, false, nil
	}
	automation, err := scanAutomation(rows)
	return automation, err == nil, err
}

func (s *Store) ListAutomations(ctx context.Context) ([]models.Automation, error) {
	rows, err := s.db.QueryContext(ctx, `
		select id, name, enabled, trigger_json, condition_logic, conditions_json, time_window_json,
		       actions_json, last_triggered_at, last_run_status, last_error, created_at, updated_at
		from automations
		order by updated_at desc, id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Automation
	for rows.Next() {
		automation, err := scanAutomation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, automation)
	}
	return out, rows.Err()
}

func (s *Store) DeleteAutomation(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `delete from automations where id = ?`, id)
	return err
}

func scanAutomation(scanner interface{ Scan(...any) error }) (models.Automation, error) {
	var (
		automation      models.Automation
		enabled         int
		triggerJSON     string
		conditionsJSON  string
		timeWindowJSON  string
		actionsJSON     string
		lastTriggeredAt sql.NullString
		createdAt       string
		updatedAt       string
	)
	if err := scanner.Scan(
		&automation.ID,
		&automation.Name,
		&enabled,
		&triggerJSON,
		&automation.ConditionLogic,
		&conditionsJSON,
		&timeWindowJSON,
		&actionsJSON,
		&lastTriggeredAt,
		&automation.LastRunStatus,
		&automation.LastError,
		&createdAt,
		&updatedAt,
	); err != nil {
		return models.Automation{}, err
	}
	automation.Enabled = enabled != 0
	if err := parseJSON(triggerJSON, &automation.Trigger); err != nil {
		return models.Automation{}, err
	}
	if conditionsJSON == "" {
		conditionsJSON = "[]"
	}
	if err := json.Unmarshal([]byte(conditionsJSON), &automation.Conditions); err != nil {
		return models.Automation{}, err
	}
	if timeWindowJSON != "" && strings.TrimSpace(timeWindowJSON) != "{}" && strings.TrimSpace(timeWindowJSON) != "null" {
		var window models.AutomationTimeWindow
		if err := json.Unmarshal([]byte(timeWindowJSON), &window); err != nil {
			return models.Automation{}, err
		}
		if strings.TrimSpace(window.Start) != "" || strings.TrimSpace(window.End) != "" {
			automation.TimeWindow = &window
		}
	}
	if actionsJSON == "" {
		actionsJSON = "[]"
	}
	if err := json.Unmarshal([]byte(actionsJSON), &automation.Actions); err != nil {
		return models.Automation{}, err
	}
	if lastTriggeredAt.Valid && strings.TrimSpace(lastTriggeredAt.String) != "" {
		parsed, err := time.Parse(time.RFC3339Nano, lastTriggeredAt.String)
		if err != nil {
			return models.Automation{}, err
		}
		parsed = parsed.UTC()
		automation.LastTriggeredAt = &parsed
	}
	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return models.Automation{}, err
	}
	parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return models.Automation{}, err
	}
	automation.CreatedAt = parsedCreatedAt.UTC()
	automation.UpdatedAt = parsedUpdatedAt.UTC()
	return automation, nil
}
