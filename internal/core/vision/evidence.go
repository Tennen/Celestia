package vision

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

const maxVisionCaptureBytes = 10 << 20

func (s *Service) ReportEvidence(ctx context.Context, batch models.VisionServiceEventCaptureBatch) error {
	config, err := s.GetConfig(ctx)
	if err != nil {
		return err
	}
	if len(batch.Captures) == 0 {
		return errors.New("captures are required")
	}
	if err := s.deleteExpiredCaptures(ctx, config); err != nil {
		return err
	}
	for _, item := range batch.Captures {
		asset, err := normalizeReportedCapture(item)
		if err != nil {
			return err
		}
		asset.Capture, err = s.validateCapture(ctx, asset.Capture)
		if err != nil {
			return err
		}
		if isCaptureExpired(asset.Capture, config) {
			continue
		}
		if err := s.store.UpsertVisionEventCapture(ctx, asset); err != nil {
			return err
		}
	}
	status, err := s.GetStatus(ctx)
	if err != nil {
		return err
	}
	status.UpdatedAt = time.Now().UTC()
	if status.Status == "" {
		status.Status = models.HealthStateUnknown
	}
	return s.store.UpsertVisionStatus(ctx, status)
}

func (s *Service) GetCaptureAsset(ctx context.Context, captureID string) (models.VisionEventCaptureAsset, bool, error) {
	config, err := s.GetConfig(ctx)
	if err != nil {
		return models.VisionEventCaptureAsset{}, false, err
	}
	if err := s.deleteExpiredCaptures(ctx, config); err != nil {
		return models.VisionEventCaptureAsset{}, false, err
	}
	item, ok, err := s.store.GetVisionEventCapture(ctx, captureID)
	if err != nil || !ok {
		return item, ok, err
	}
	if isCaptureExpired(item.Capture, config) {
		_ = s.deleteExpiredCaptures(ctx, config)
		return models.VisionEventCaptureAsset{}, false, nil
	}
	return item, true, nil
}

func (s *Service) EnrichEvents(ctx context.Context, items []models.Event) ([]models.Event, error) {
	if len(items) == 0 {
		return items, nil
	}
	config, err := s.GetConfig(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.deleteExpiredCaptures(ctx, config); err != nil {
		return nil, err
	}
	eventIDs := make([]string, 0, len(items))
	for _, item := range items {
		if isVisionOccurredEvent(item) {
			eventIDs = append(eventIDs, item.ID)
		}
	}
	if len(eventIDs) == 0 {
		return items, nil
	}
	capturesByEvent, err := s.store.ListVisionEventCaptures(ctx, eventIDs)
	if err != nil {
		return nil, err
	}
	out := make([]models.Event, 0, len(items))
	for _, item := range items {
		captures := capturesByEvent[item.ID]
		if len(captures) == 0 {
			out = append(out, item)
			continue
		}
		slices.SortFunc(captures, compareVisionCapture)
		payload := cloneMap(item.Payload)
		payload["capture_count"] = len(captures)
		payload["captures"] = captures
		item.Payload = payload
		out = append(out, item)
	}
	return out, nil
}

func normalizeReportedCapture(item models.VisionServiceEventCapture) (models.VisionEventCaptureAsset, error) {
	var asset models.VisionEventCaptureAsset
	asset.Capture.EventID = strings.TrimSpace(item.EventID)
	if asset.Capture.EventID == "" {
		return models.VisionEventCaptureAsset{}, errors.New("capture event_id is required")
	}
	asset.Capture.Phase = normalizeCapturePhase(item.Phase)
	if asset.Capture.Phase == "" {
		return models.VisionEventCaptureAsset{}, fmt.Errorf("capture phase %q is invalid", item.Phase)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(item.ImageBase64))
	if err != nil {
		return models.VisionEventCaptureAsset{}, errors.New("capture image_base64 is invalid")
	}
	if len(raw) == 0 {
		return models.VisionEventCaptureAsset{}, errors.New("capture image_base64 is required")
	}
	if len(raw) > maxVisionCaptureBytes {
		return models.VisionEventCaptureAsset{}, fmt.Errorf("capture exceeds %d bytes", maxVisionCaptureBytes)
	}
	contentType := strings.TrimSpace(item.ContentType)
	if contentType == "" || !strings.HasPrefix(contentType, "image/") {
		contentType = http.DetectContentType(raw)
	}
	if !strings.HasPrefix(contentType, "image/") {
		return models.VisionEventCaptureAsset{}, fmt.Errorf("capture content_type %q is not an image", contentType)
	}
	capturedAt := item.CapturedAt.UTC()
	if capturedAt.IsZero() {
		capturedAt = time.Now().UTC()
	}
	captureID := strings.TrimSpace(item.CaptureID)
	if captureID == "" {
		captureID = asset.Capture.EventID + ":" + string(asset.Capture.Phase)
	}
	asset.Capture = models.VisionEventCapture{
		CaptureID:      captureID,
		EventID:        asset.Capture.EventID,
		RuleID:         strings.TrimSpace(item.RuleID),
		CameraDeviceID: strings.TrimSpace(item.CameraDeviceID),
		Phase:          asset.Capture.Phase,
		CapturedAt:     capturedAt,
		ContentType:    contentType,
		SizeBytes:      len(raw),
		Metadata:       cloneMap(item.Metadata),
	}
	asset.Data = raw
	return asset, nil
}

func normalizeCapturePhase(value models.VisionEventCapturePhase) models.VisionEventCapturePhase {
	switch models.VisionEventCapturePhase(strings.TrimSpace(string(value))) {
	case models.VisionEventCapturePhaseStart:
		return models.VisionEventCapturePhaseStart
	case models.VisionEventCapturePhaseMiddle:
		return models.VisionEventCapturePhaseMiddle
	case models.VisionEventCapturePhaseEnd:
		return models.VisionEventCapturePhaseEnd
	default:
		return ""
	}
}

func (s *Service) validateCapture(ctx context.Context, capture models.VisionEventCapture) (models.VisionEventCapture, error) {
	event, ok, err := s.store.GetEvent(ctx, capture.EventID)
	if err != nil {
		return models.VisionEventCapture{}, err
	}
	if !ok {
		return models.VisionEventCapture{}, fmt.Errorf("vision event %q not found", capture.EventID)
	}
	if !isVisionOccurredEvent(event) {
		return models.VisionEventCapture{}, fmt.Errorf("event %q is not a persisted vision occurrence", capture.EventID)
	}
	if capture.RuleID == "" {
		ruleID, _ := event.Payload["rule_id"].(string)
		capture.RuleID = strings.TrimSpace(ruleID)
	}
	if capture.CameraDeviceID == "" {
		capture.CameraDeviceID = event.DeviceID
	}
	if capture.RuleID != "" {
		ruleID, _ := event.Payload["rule_id"].(string)
		if capture.RuleID != strings.TrimSpace(ruleID) {
			return models.VisionEventCapture{}, fmt.Errorf("capture rule_id %q does not match event rule_id %q", capture.RuleID, ruleID)
		}
	}
	if capture.CameraDeviceID != "" && capture.CameraDeviceID != event.DeviceID {
		return models.VisionEventCapture{}, fmt.Errorf("capture camera_device_id %q does not match event device %q", capture.CameraDeviceID, event.DeviceID)
	}
	return capture, nil
}

func isVisionOccurredEvent(event models.Event) bool {
	if event.Type != models.EventDeviceOccurred {
		return false
	}
	capabilityID, _ := event.Payload["capability_id"].(string)
	return capabilityID == models.VisionCapabilityID
}

func (s *Service) deleteExpiredCaptures(ctx context.Context, config models.VisionCapabilityConfig) error {
	retention := time.Duration(normalizeCaptureRetentionHours(config.EventCaptureRetentionHours)) * time.Hour
	cutoff := time.Now().UTC().Add(-retention)
	return s.store.DeleteVisionEventCapturesBefore(ctx, cutoff)
}

func isCaptureExpired(capture models.VisionEventCapture, config models.VisionCapabilityConfig) bool {
	retention := time.Duration(normalizeCaptureRetentionHours(config.EventCaptureRetentionHours)) * time.Hour
	return capture.CapturedAt.Before(time.Now().UTC().Add(-retention))
}

func compareVisionCapture(left, right models.VisionEventCapture) int {
	if diff := capturePhaseRank(left.Phase) - capturePhaseRank(right.Phase); diff != 0 {
		return diff
	}
	if left.CapturedAt.Before(right.CapturedAt) {
		return -1
	}
	if left.CapturedAt.After(right.CapturedAt) {
		return 1
	}
	return strings.Compare(left.CaptureID, right.CaptureID)
}

func capturePhaseRank(phase models.VisionEventCapturePhase) int {
	switch phase {
	case models.VisionEventCapturePhaseStart:
		return 0
	case models.VisionEventCapturePhaseMiddle:
		return 1
	case models.VisionEventCapturePhaseEnd:
		return 2
	default:
		return 3
	}
}
