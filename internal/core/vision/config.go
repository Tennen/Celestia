package vision

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/storage"
)

func (s *Service) SaveConfig(ctx context.Context, config models.VisionCapabilityConfig) (models.VisionCapabilityDetail, error) {
	normalized, err := s.normalizeConfig(ctx, config)
	if err != nil {
		return models.VisionCapabilityDetail{}, err
	}
	if err := s.validateConfigAgainstCatalog(ctx, normalized); err != nil {
		return models.VisionCapabilityDetail{}, err
	}
	if err := s.store.UpsertVisionConfig(ctx, normalized); err != nil {
		return models.VisionCapabilityDetail{}, err
	}
	if err := s.deleteExpiredCaptures(ctx, normalized); err != nil {
		return models.VisionCapabilityDetail{}, err
	}
	if err := s.reconcileDeviceStates(ctx, normalized); err != nil {
		return models.VisionCapabilityDetail{}, err
	}

	status, err := s.syncConfig(ctx, normalized)
	if err != nil {
		return models.VisionCapabilityDetail{}, err
	}
	if err := s.store.UpsertVisionStatus(ctx, status); err != nil {
		return models.VisionCapabilityDetail{}, err
	}
	s.publishStatusEvent(status)

	events, err := s.RecentEvents(ctx, 12)
	if err != nil {
		return models.VisionCapabilityDetail{}, err
	}
	return models.VisionCapabilityDetail{
		Config:       normalized,
		Runtime:      status,
		RecentEvents: events,
	}, nil
}

func (s *Service) normalizeConfig(ctx context.Context, config models.VisionCapabilityConfig) (models.VisionCapabilityConfig, error) {
	now := time.Now().UTC()
	config.ServiceWSURL = normalizeServiceWSURL(config.ServiceWSURL)
	config.ModelName = normalizeModelName(config.ModelName)
	config.EventCaptureRetentionHours = normalizeCaptureRetentionHours(config.EventCaptureRetentionHours)
	config.UpdatedAt = now
	if config.Rules == nil {
		config.Rules = []models.VisionRule{}
	}
	normalizedRules := make([]models.VisionRule, 0, len(config.Rules))
	seen := map[string]struct{}{}
	for idx, rule := range config.Rules {
		item, err := s.normalizeRule(ctx, rule, idx)
		if err != nil {
			return models.VisionCapabilityConfig{}, err
		}
		if _, exists := seen[item.ID]; exists {
			return models.VisionCapabilityConfig{}, fmt.Errorf("vision rule id %q is duplicated", item.ID)
		}
		seen[item.ID] = struct{}{}
		normalizedRules = append(normalizedRules, item)
	}
	slices.SortFunc(normalizedRules, func(left, right models.VisionRule) int {
		return strings.Compare(left.ID, right.ID)
	})
	config.Rules = normalizedRules
	return config, nil
}

func normalizeCaptureRetentionHours(value int) int {
	if value <= 0 {
		return models.DefaultVisionEventCaptureRetentionHours
	}
	return value
}

func (s *Service) normalizeRule(ctx context.Context, rule models.VisionRule, idx int) (models.VisionRule, error) {
	rule.Name = strings.TrimSpace(rule.Name)
	if rule.Name == "" {
		rule.Name = fmt.Sprintf("Vision Rule %d", idx+1)
	}
	rule.ID = sanitizeRuleID(rule.ID)
	if rule.ID == "" {
		rule.ID = sanitizeRuleID(rule.Name)
	}
	if rule.ID == "" {
		return models.VisionRule{}, fmt.Errorf("vision rule %d id is required", idx)
	}
	rule.CameraDeviceID = strings.TrimSpace(rule.CameraDeviceID)
	if rule.CameraDeviceID == "" {
		return models.VisionRule{}, fmt.Errorf("vision rule %q camera_device_id is required", rule.ID)
	}
	device, ok, err := s.registry.Get(ctx, rule.CameraDeviceID)
	if err != nil {
		return models.VisionRule{}, err
	}
	if !ok {
		return models.VisionRule{}, fmt.Errorf("vision camera %q not found", rule.CameraDeviceID)
	}
	if device.Kind != models.DeviceKindCameraLike {
		return models.VisionRule{}, fmt.Errorf("vision rule %q camera %q is not camera_like", rule.ID, device.ID)
	}
	rule.RTSPSource.URL = strings.TrimSpace(rule.RTSPSource.URL)
	if rule.RTSPSource.URL == "" {
		resolvedRTSPURL, err := s.resolveCameraRTSPSource(ctx, device)
		if err != nil {
			return models.VisionRule{}, err
		}
		rule.RTSPSource.URL = resolvedRTSPURL
	}
	if rule.RTSPSource.URL == "" && rule.Enabled && rule.RecognitionEnabled {
		return models.VisionRule{}, fmt.Errorf("vision rule %q rtsp_source.url is required", rule.ID)
	}
	rule.EntitySelector.Kind = strings.TrimSpace(rule.EntitySelector.Kind)
	if rule.EntitySelector.Kind == "" {
		rule.EntitySelector.Kind = "label"
	}
	rule.EntitySelector.Value = strings.TrimSpace(rule.EntitySelector.Value)
	rule.Behavior = strings.TrimSpace(rule.Behavior)
	keyEntities, err := normalizeRuleKeyEntities(rule.ID, rule.KeyEntities)
	if err != nil {
		return models.VisionRule{}, err
	}
	rule.KeyEntities = keyEntities
	if rule.StayThresholdSeconds <= 0 {
		rule.StayThresholdSeconds = defaultVisionThresholdSeconds
	}
	rule.Zone = normalizeZone(rule.Zone)
	if rule.Zone.Width <= 0 || rule.Zone.Height <= 0 {
		return models.VisionRule{}, fmt.Errorf("vision rule %q zone must have positive width and height", rule.ID)
	}
	return rule, nil
}

func (s *Service) resolveCameraRTSPSource(ctx context.Context, device models.Device) (string, error) {
	snapshot, ok, err := s.state.Get(ctx, device.ID)
	if err != nil {
		return "", err
	}
	if ok {
		if rtspURL := mapString(snapshot.State, "rtsp_url"); rtspURL != "" {
			return rtspURL, nil
		}
	}
	if rtspURL := mapString(device.Metadata, "rtsp_url"); rtspURL != "" {
		return rtspURL, nil
	}
	return "", nil
}

func mapString(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	raw, ok := values[key].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(raw)
}

func normalizeZone(zone models.VisionZoneBox) models.VisionZoneBox {
	zone.X = clampUnit(zone.X)
	zone.Y = clampUnit(zone.Y)
	zone.Width = clampUnit(zone.Width)
	zone.Height = clampUnit(zone.Height)
	if zone.X+zone.Width > 1 {
		zone.Width = 1 - zone.X
	}
	if zone.Y+zone.Height > 1 {
		zone.Height = 1 - zone.Y
	}
	return zone
}

func clampUnit(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func (s *Service) reconcileDeviceStates(ctx context.Context, config models.VisionCapabilityConfig) error {
	devices, err := s.registry.List(ctx, storage.DeviceFilter{Kind: string(models.DeviceKindCameraLike)})
	if err != nil {
		return err
	}
	rulesByDevice := map[string][]models.VisionRule{}
	for _, rule := range config.Rules {
		rulesByDevice[rule.CameraDeviceID] = append(rulesByDevice[rule.CameraDeviceID], rule)
	}
	for _, device := range devices {
		snapshot, ok, err := s.state.Get(ctx, device.ID)
		if err != nil {
			return err
		}
		if !ok {
			snapshot = models.DeviceStateSnapshot{
				DeviceID: device.ID,
				PluginID: device.PluginID,
				State:    map[string]any{},
			}
		}
		nextState := cloneMap(snapshot.State)
		for key := range nextState {
			if strings.HasPrefix(key, visionStatePrefix) {
				delete(nextState, key)
			}
		}
		for _, rule := range rulesByDevice[device.ID] {
			for key, value := range initialRuleState(rule) {
				nextState[key] = value
			}
		}
		snapshot.State = nextState
		snapshot.TS = time.Now().UTC()
		if snapshot.PluginID == "" {
			snapshot.PluginID = device.PluginID
		}
		if err := s.state.Upsert(ctx, []models.DeviceStateSnapshot{snapshot}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) syncConfig(ctx context.Context, config models.VisionCapabilityConfig) (models.VisionCapabilityStatus, error) {
	status, err := s.GetStatus(ctx)
	if err != nil {
		return models.VisionCapabilityStatus{}, err
	}
	now := time.Now().UTC()
	status.UpdatedAt = now
	if !config.RecognitionEnabled {
		status.Status = models.HealthStateStopped
		status.Message = "vision recognition disabled"
		status.SyncError = ""
		return status, nil
	}
	if config.ServiceWSURL == "" {
		status.Status = models.HealthStateDegraded
		status.Message = "vision service websocket URL not configured"
		status.SyncError = "service_ws_url is required to sync enabled vision rules"
		return status, nil
	}
	if err := validateServiceWSURL(config.ServiceWSURL); err != nil {
		status.Status = models.HealthStateDegraded
		status.Message = err.Error()
		status.SyncError = err.Error()
		return status, nil
	}
	status, err = s.session.SyncConfig(ctx, config)
	if err != nil {
		return models.VisionCapabilityStatus{}, err
	}
	return status, nil
}

func (s *Service) buildSyncPayload(ctx context.Context, config models.VisionCapabilityConfig) (models.VisionServiceSyncPayload, error) {
	rules := make([]models.VisionServiceRule, 0, len(config.Rules))
	for _, rule := range config.Rules {
		device, ok, err := s.registry.Get(ctx, rule.CameraDeviceID)
		if err != nil {
			return models.VisionServiceSyncPayload{}, err
		}
		if !ok {
			return models.VisionServiceSyncPayload{}, fmt.Errorf("vision camera %q not found", rule.CameraDeviceID)
		}
		camera := models.VisionServiceCamera{
			DeviceID:       device.ID,
			PluginID:       device.PluginID,
			VendorDeviceID: device.VendorDeviceID,
			Name:           device.Name,
		}
		if entryID, _ := device.Metadata["entry_id"].(string); entryID != "" {
			camera.EntryID = entryID
		}
		rules = append(rules, models.VisionServiceRule{
			ID:                   rule.ID,
			Name:                 rule.Name,
			Enabled:              rule.Enabled && rule.RecognitionEnabled,
			Camera:               camera,
			RTSPSource:           rule.RTSPSource,
			EntitySelector:       rule.EntitySelector,
			Behavior:             rule.Behavior,
			KeyEntities:          buildVisionServiceKeyEntities(rule.KeyEntities),
			Zone:                 rule.Zone,
			StayThresholdSeconds: rule.StayThresholdSeconds,
		})
	}
	return models.VisionServiceSyncPayload{
		SchemaVersion:      visionControlSchemaVersion,
		SentAt:             time.Now().UTC(),
		RecognitionEnabled: config.RecognitionEnabled,
		Rules:              rules,
	}, nil
}
