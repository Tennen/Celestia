package sqlite

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

func (s *Store) UpsertVisionEventCapture(ctx context.Context, asset models.VisionEventCaptureAsset) error {
	metadataJSON, err := marshalJSON(asset.Capture.Metadata)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		insert into vision_event_captures(
			capture_id, event_id, rule_id, camera_device_id, phase, captured_at, content_type, size_bytes, metadata_json, image_data
		) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		on conflict(event_id, phase) do update set
			capture_id=excluded.capture_id,
			rule_id=excluded.rule_id,
			camera_device_id=excluded.camera_device_id,
			captured_at=excluded.captured_at,
			content_type=excluded.content_type,
			size_bytes=excluded.size_bytes,
			metadata_json=excluded.metadata_json,
			image_data=excluded.image_data
	`,
		strings.TrimSpace(asset.Capture.CaptureID),
		strings.TrimSpace(asset.Capture.EventID),
		strings.TrimSpace(asset.Capture.RuleID),
		strings.TrimSpace(asset.Capture.CameraDeviceID),
		string(asset.Capture.Phase),
		asset.Capture.CapturedAt.UTC().Format(time.RFC3339Nano),
		strings.TrimSpace(asset.Capture.ContentType),
		asset.Capture.SizeBytes,
		metadataJSON,
		asset.Data,
	)
	return err
}

func (s *Store) GetVisionEventCapture(ctx context.Context, captureID string) (models.VisionEventCaptureAsset, bool, error) {
	rows, err := s.db.QueryContext(ctx, `
		select capture_id, event_id, rule_id, camera_device_id, phase, captured_at, content_type, size_bytes, metadata_json, image_data
		from vision_event_captures where capture_id = ?
	`, strings.TrimSpace(captureID))
	if err != nil {
		return models.VisionEventCaptureAsset{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return models.VisionEventCaptureAsset{}, false, nil
	}
	item, err := scanVisionEventCapture(rows, true)
	return item, err == nil, err
}

func (s *Store) ListVisionEventCaptures(ctx context.Context, eventIDs []string) (map[string][]models.VisionEventCapture, error) {
	trimmed := make([]string, 0, len(eventIDs))
	for _, eventID := range eventIDs {
		eventID = strings.TrimSpace(eventID)
		if eventID == "" {
			continue
		}
		trimmed = append(trimmed, eventID)
	}
	if len(trimmed) == 0 {
		return map[string][]models.VisionEventCapture{}, nil
	}
	args := make([]any, 0, len(trimmed))
	placeholders := make([]string, 0, len(trimmed))
	for _, eventID := range trimmed {
		args = append(args, eventID)
		placeholders = append(placeholders, "?")
	}
	query := fmt.Sprintf(`
		select capture_id, event_id, rule_id, camera_device_id, phase, captured_at, content_type, size_bytes, metadata_json
		from vision_event_captures
		where event_id in (%s)
		order by captured_at asc
	`, strings.Join(placeholders, ","))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string][]models.VisionEventCapture, len(trimmed))
	for rows.Next() {
		item, err := scanVisionEventCapture(rows, false)
		if err != nil {
			return nil, err
		}
		out[item.Capture.EventID] = append(out[item.Capture.EventID], item.Capture)
	}
	return out, rows.Err()
}

func (s *Store) DeleteVisionEventCapturesBefore(ctx context.Context, cutoff time.Time) error {
	if cutoff.IsZero() {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		delete from vision_event_captures where captured_at < ?
	`, cutoff.UTC().Format(time.RFC3339Nano))
	return err
}

func scanVisionEventCapture(scanner interface{ Scan(...any) error }, includeData bool) (models.VisionEventCaptureAsset, error) {
	var (
		item         models.VisionEventCaptureAsset
		capturedAt   string
		metadataJSON string
		phase        string
		data         []byte
	)
	dest := []any{
		&item.Capture.CaptureID,
		&item.Capture.EventID,
		&item.Capture.RuleID,
		&item.Capture.CameraDeviceID,
		&phase,
		&capturedAt,
		&item.Capture.ContentType,
		&item.Capture.SizeBytes,
		&metadataJSON,
	}
	if includeData {
		dest = append(dest, &data)
	}
	if err := scanner.Scan(dest...); err != nil {
		return models.VisionEventCaptureAsset{}, err
	}
	parsed, err := time.Parse(time.RFC3339Nano, capturedAt)
	if err != nil {
		return models.VisionEventCaptureAsset{}, err
	}
	item.Capture.CapturedAt = parsed.UTC()
	item.Capture.Phase = models.VisionEventCapturePhase(phase)
	if err := parseJSON(metadataJSON, &item.Capture.Metadata); err != nil {
		return models.VisionEventCaptureAsset{}, err
	}
	item.Data = data
	return item, nil
}
