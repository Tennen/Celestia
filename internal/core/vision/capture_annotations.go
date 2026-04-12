package vision

import "strings"

func normalizeCaptureMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return map[string]any{}
	}
	normalized := cloneMap(metadata)
	annotations, ok := normalizeCaptureAnnotations(normalized["annotations"])
	if !ok {
		delete(normalized, "annotations")
		return normalized
	}
	normalized["annotations"] = annotations
	return normalized
}

func normalizeCaptureAnnotations(value any) (map[string]any, bool) {
	raw, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	imageKind := strings.TrimSpace(stringValue(raw["image_kind"]))
	if imageKind != "annotated" {
		imageKind = "raw"
	}
	detections := normalizeCaptureDetections(raw["detections"])
	if len(detections) == 0 {
		return nil, false
	}
	out := map[string]any{
		"image_kind":       imageKind,
		"coordinate_space": "normalized_xywh",
		"detections":       detections,
	}
	if source := strings.TrimSpace(stringValue(raw["source"])); source != "" {
		out["source"] = source
	}
	return out, true
}

func normalizeCaptureDetections(value any) []map[string]any {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		detection, ok := normalizeCaptureDetection(item)
		if !ok {
			continue
		}
		out = append(out, detection)
	}
	return out
}

func normalizeCaptureDetection(value any) (map[string]any, bool) {
	raw, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	box, ok := normalizeCaptureBox(raw["box"])
	if !ok {
		return nil, false
	}
	entityValue := strings.TrimSpace(stringValue(raw["value"]))
	label := strings.TrimSpace(stringValue(raw["display_name"]))
	if label == "" {
		label = entityValue
	}
	if label == "" {
		return nil, false
	}
	out := map[string]any{
		"box":          box,
		"display_name": label,
	}
	if kind := strings.TrimSpace(stringValue(raw["kind"])); kind != "" {
		out["kind"] = kind
	}
	if entityValue != "" {
		out["value"] = entityValue
	}
	if confidence, ok := normalizedUnitFloat(raw["confidence"]); ok {
		out["confidence"] = confidence
	}
	if trackID := strings.TrimSpace(stringValue(raw["track_id"])); trackID != "" {
		out["track_id"] = trackID
	}
	return out, true
}

func normalizeCaptureBox(value any) (map[string]any, bool) {
	raw, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	x, ok := normalizedUnitFloat(raw["x"])
	if !ok {
		return nil, false
	}
	y, ok := normalizedUnitFloat(raw["y"])
	if !ok {
		return nil, false
	}
	width, ok := normalizedUnitFloat(raw["width"])
	if !ok || width <= 0 {
		return nil, false
	}
	height, ok := normalizedUnitFloat(raw["height"])
	if !ok || height <= 0 {
		return nil, false
	}
	if x+width > 1 {
		width = 1 - x
	}
	if y+height > 1 {
		height = 1 - y
	}
	if width <= 0 || height <= 0 {
		return nil, false
	}
	return map[string]any{
		"x":      x,
		"y":      y,
		"width":  width,
		"height": height,
	}, true
}

func normalizedUnitFloat(value any) (float64, bool) {
	number, ok := floatValue(value)
	if !ok {
		return 0, false
	}
	return clampUnit(number), true
}

func floatValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	default:
		return 0, false
	}
}

func stringValue(value any) string {
	if typed, ok := value.(string); ok {
		return typed
	}
	return ""
}
