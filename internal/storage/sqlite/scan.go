package sqlite

import (
	"database/sql"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

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

func scanOAuthSession(scanner interface{ Scan(...any) error }) (models.OAuthSession, error) {
	var (
		session           models.OAuthSession
		accountConfigJSON string
		createdAt         string
		updatedAt         string
		completedAt       sql.NullString
		stateExpiresAt    sql.NullString
		tokenExpiresAt    sql.NullString
	)
	if err := scanner.Scan(
		&session.ID,
		&session.Provider,
		&session.PluginID,
		&session.AccountName,
		&session.Region,
		&session.ClientID,
		&session.RedirectURL,
		&session.DeviceID,
		&session.State,
		&session.AuthURL,
		&session.Status,
		&session.Error,
		&accountConfigJSON,
		&createdAt,
		&updatedAt,
		&completedAt,
		&stateExpiresAt,
		&tokenExpiresAt,
	); err != nil {
		return models.OAuthSession{}, err
	}
	if err := parseJSON(accountConfigJSON, &session.AccountConfig); err != nil {
		return models.OAuthSession{}, err
	}
	var err error
	session.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return models.OAuthSession{}, err
	}
	session.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return models.OAuthSession{}, err
	}
	if session.CompletedAt, err = parseNullableTime(completedAt); err != nil {
		return models.OAuthSession{}, err
	}
	if session.StateExpiresAt, err = parseNullableTime(stateExpiresAt); err != nil {
		return models.OAuthSession{}, err
	}
	if session.TokenExpiresAt, err = parseNullableTime(tokenExpiresAt); err != nil {
		return models.OAuthSession{}, err
	}
	return session, nil
}

func scanVisionConfig(scanner interface{ Scan(...any) error }) (models.VisionCapabilityConfig, error) {
	var (
		capabilityID       string
		config             models.VisionCapabilityConfig
		recognitionEnabled int
		retentionHours     int
		rulesJSON          string
		updatedAt          string
	)
	if err := scanner.Scan(&capabilityID, &config.ServiceWSURL, &recognitionEnabled, &retentionHours, &rulesJSON, &updatedAt); err != nil {
		return models.VisionCapabilityConfig{}, err
	}
	config.RecognitionEnabled = recognitionEnabled == 1
	config.EventCaptureRetentionHours = retentionHours
	if err := parseJSON(rulesJSON, &config.Rules); err != nil {
		return models.VisionCapabilityConfig{}, err
	}
	parsed, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return models.VisionCapabilityConfig{}, err
	}
	config.UpdatedAt = parsed.UTC()
	return config, nil
}

func scanVisionCatalog(scanner interface{ Scan(...any) error }) (models.VisionEntityCatalog, error) {
	var (
		capabilityID string
		catalog      models.VisionEntityCatalog
		entitiesJSON string
		fetchedAt    string
	)
	if err := scanner.Scan(
		&capabilityID,
		&catalog.ServiceWSURL,
		&catalog.SchemaVersion,
		&catalog.ServiceVersion,
		&catalog.ModelName,
		&entitiesJSON,
		&fetchedAt,
	); err != nil {
		return models.VisionEntityCatalog{}, err
	}
	if err := parseJSON(entitiesJSON, &catalog.Entities); err != nil {
		return models.VisionEntityCatalog{}, err
	}
	parsed, err := time.Parse(time.RFC3339Nano, fetchedAt)
	if err != nil {
		return models.VisionEntityCatalog{}, err
	}
	catalog.FetchedAt = parsed.UTC()
	return catalog, nil
}

func scanVisionStatus(scanner interface{ Scan(...any) error }) (models.VisionCapabilityStatus, error) {
	var (
		capabilityID   string
		status         models.VisionCapabilityStatus
		lastSyncedAt   sql.NullString
		lastReportedAt sql.NullString
		lastEventAt    sql.NullString
		runtimeJSON    string
		updatedAt      string
	)
	if err := scanner.Scan(
		&capabilityID,
		&status.Status,
		&status.Message,
		&status.ServiceVersion,
		&lastSyncedAt,
		&lastReportedAt,
		&lastEventAt,
		&runtimeJSON,
		&status.SyncError,
		&updatedAt,
	); err != nil {
		return models.VisionCapabilityStatus{}, err
	}
	if err := parseJSON(runtimeJSON, &status.Runtime); err != nil {
		return models.VisionCapabilityStatus{}, err
	}
	var err error
	if status.LastSyncedAt, err = parseNullableTime(lastSyncedAt); err != nil {
		return models.VisionCapabilityStatus{}, err
	}
	if status.LastReportedAt, err = parseNullableTime(lastReportedAt); err != nil {
		return models.VisionCapabilityStatus{}, err
	}
	if status.LastEventAt, err = parseNullableTime(lastEventAt); err != nil {
		return models.VisionCapabilityStatus{}, err
	}
	parsed, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return models.VisionCapabilityStatus{}, err
	}
	status.UpdatedAt = parsed.UTC()
	return status, nil
}
