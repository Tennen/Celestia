package vision

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
)

func (s *Service) ReportStatus(ctx context.Context, report models.VisionServiceStatusReport) (models.VisionCapabilityStatus, error) {
	config, err := s.GetConfig(ctx)
	if err != nil {
		return models.VisionCapabilityStatus{}, err
	}
	status, err := s.GetStatus(ctx)
	if err != nil {
		return models.VisionCapabilityStatus{}, err
	}
	now := time.Now().UTC()
	reportedAt := report.ReportedAt.UTC()
	if reportedAt.IsZero() {
		reportedAt = now
	}
	status.Status = report.Status
	status.Message = strings.TrimSpace(report.Message)
	status.ServiceVersion = strings.TrimSpace(report.ServiceVersion)
	status.LastReportedAt = &reportedAt
	status.Runtime = cloneMap(report.Runtime)
	status.SyncError = ""
	status.UpdatedAt = now
	if !config.RecognitionEnabled {
		status.Status = models.HealthStateStopped
	}
	if err := s.store.UpsertVisionStatus(ctx, status); err != nil {
		return models.VisionCapabilityStatus{}, err
	}
	s.publishStatusEvent(status)
	return status, nil
}

func (s *Service) ReportEvents(ctx context.Context, batch models.VisionServiceEventBatch) error {
	config, err := s.GetConfig(ctx)
	if err != nil {
		return err
	}
	if len(batch.Events) == 0 {
		return errors.New("events are required")
	}
	ruleIndex := make(map[string]models.VisionRule, len(config.Rules))
	for _, rule := range config.Rules {
		ruleIndex[rule.ID] = rule
	}
	status, err := s.GetStatus(ctx)
	if err != nil {
		return err
	}

	var lastEventAt *time.Time
	for _, item := range batch.Events {
		rule, ok := ruleIndex[strings.TrimSpace(item.RuleID)]
		if !ok {
			return fmt.Errorf("vision rule %q not found", item.RuleID)
		}
		if err := s.reportEvent(ctx, rule, item); err != nil {
			return err
		}
		observedAt := item.ObservedAt.UTC()
		if observedAt.IsZero() {
			observedAt = time.Now().UTC()
		}
		if lastEventAt == nil || observedAt.After(*lastEventAt) {
			lastEventAt = &observedAt
		}
	}

	if lastEventAt != nil {
		status.LastEventAt = lastEventAt
		status.UpdatedAt = time.Now().UTC()
		if status.Status == "" || status.Status == models.HealthStateUnknown {
			status.Status = models.HealthStateHealthy
		}
		if err := s.store.UpsertVisionStatus(ctx, status); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) reportEvent(ctx context.Context, rule models.VisionRule, item models.VisionServiceEvent) error {
	deviceID := strings.TrimSpace(item.CameraDeviceID)
	if deviceID == "" {
		deviceID = rule.CameraDeviceID
	}
	if deviceID != rule.CameraDeviceID {
		return fmt.Errorf("vision rule %q event camera %q does not match configured camera %q", rule.ID, deviceID, rule.CameraDeviceID)
	}
	device, ok, err := s.registry.Get(ctx, deviceID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("vision camera %q not found", deviceID)
	}
	previous, ok, err := s.state.Get(ctx, device.ID)
	if err != nil {
		return err
	}
	if !ok {
		previous = models.DeviceStateSnapshot{
			DeviceID: device.ID,
			PluginID: device.PluginID,
			State:    map[string]any{},
		}
	}
	observedAt := item.ObservedAt.UTC()
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	reportedEntities := normalizeReportedEntities(item)
	entityValue := primaryReportedEntityValue(reportedEntities, item.EntityValue)
	nextState := applyReportedEvent(previous.State, rule, item, observedAt)
	if err := s.state.Upsert(ctx, []models.DeviceStateSnapshot{{
		DeviceID: device.ID,
		PluginID: device.PluginID,
		TS:       observedAt,
		State:    nextState,
	}}); err != nil {
		return err
	}

	deviceEvent := models.Event{
		ID:       uuidOrNew(item.EventID),
		Type:     models.EventDeviceOccurred,
		PluginID: device.PluginID,
		DeviceID: device.ID,
		TS:       observedAt,
		Payload: map[string]any{
			"source":          "capability:" + models.VisionCapabilityID,
			"capability_id":   models.VisionCapabilityID,
			"rule_id":         rule.ID,
			"rule_name":       rule.Name,
			"event_status":    item.Status,
			"dwell_seconds":   item.DwellSeconds,
			"entity_value":    entityValue,
			"entity_selector": rule.EntitySelector,
			"metadata":        cloneMap(item.Metadata),
		},
	}
	if len(reportedEntities) > 0 {
		deviceEvent.Payload["entities"] = reportedEntities
	}
	if err := s.store.AppendEvent(ctx, deviceEvent); err != nil {
		return err
	}
	s.bus.Publish(deviceEvent)

	stateEvent := models.Event{
		ID:       uuid.NewString(),
		Type:     models.EventDeviceStateChanged,
		PluginID: device.PluginID,
		DeviceID: device.ID,
		TS:       observedAt,
		Payload: map[string]any{
			"source":         "capability:" + models.VisionCapabilityID,
			"capability_id":  models.VisionCapabilityID,
			"rule_id":        rule.ID,
			"rule_name":      rule.Name,
			"previous_state": cloneMap(previous.State),
			"state":          cloneMap(nextState),
			"changed_keys":   changedStateKeys(previous.State, nextState),
		},
	}
	if err := s.store.AppendEvent(ctx, stateEvent); err != nil {
		return err
	}
	s.bus.Publish(stateEvent)
	return nil
}

func (s *Service) publishStatusEvent(status models.VisionCapabilityStatus) {
	event := models.Event{
		ID:   uuid.NewString(),
		Type: models.EventCapabilityStatusChanged,
		TS:   time.Now().UTC(),
		Payload: map[string]any{
			"capability_id":    models.VisionCapabilityID,
			"status":           status.Status,
			"message":          status.Message,
			"service_version":  status.ServiceVersion,
			"sync_error":       status.SyncError,
			"last_synced_at":   formatNullableTime(status.LastSyncedAt),
			"last_reported_at": formatNullableTime(status.LastReportedAt),
			"last_event_at":    formatNullableTime(status.LastEventAt),
		},
	}
	_ = s.store.AppendEvent(context.Background(), event)
	s.bus.Publish(event)
}
