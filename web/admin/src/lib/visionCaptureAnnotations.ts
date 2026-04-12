import type { VisionCaptureAnnotations, VisionCaptureDetection } from './types';

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function readString(value: unknown) {
  return typeof value === 'string' ? value : '';
}

function readNumber(value: unknown) {
  return typeof value === 'number' && Number.isFinite(value) ? value : Number.NaN;
}

function clampUnit(value: number) {
  if (value < 0) return 0;
  if (value > 1) return 1;
  return value;
}

function readBox(value: unknown) {
  const box = isRecord(value) ? value : null;
  if (!box) {
    return null;
  }
  const x = clampUnit(readNumber(box.x));
  const y = clampUnit(readNumber(box.y));
  let width = clampUnit(readNumber(box.width));
  let height = clampUnit(readNumber(box.height));
  if (!Number.isFinite(x) || !Number.isFinite(y) || !Number.isFinite(width) || !Number.isFinite(height)) {
    return null;
  }
  if (width <= 0 || height <= 0) {
    return null;
  }
  if (x + width > 1) {
    width = 1 - x;
  }
  if (y + height > 1) {
    height = 1 - y;
  }
  if (width <= 0 || height <= 0) {
    return null;
  }
  return { x, y, width, height };
}

function readDetection(value: unknown): VisionCaptureDetection | null {
  const detection = isRecord(value) ? value : null;
  if (!detection) {
    return null;
  }
  const box = readBox(detection.box);
  const displayName = readString(detection.display_name).trim() || readString(detection.value).trim();
  if (!box || !displayName) {
    return null;
  }
  const confidence = readNumber(detection.confidence);
  return {
    kind: readString(detection.kind).trim() || undefined,
    value: readString(detection.value).trim() || undefined,
    display_name: displayName,
    confidence: Number.isFinite(confidence) ? clampUnit(confidence) : undefined,
    track_id: readString(detection.track_id).trim() || undefined,
    box,
  };
}

export function visionCaptureAnnotationsFromMetadata(metadata?: Record<string, unknown>) {
  const annotations = isRecord(metadata?.annotations) ? metadata.annotations : null;
  if (!annotations) {
    return null as VisionCaptureAnnotations | null;
  }
  const detections = Array.isArray(annotations.detections)
    ? annotations.detections.map((item) => readDetection(item)).filter((item): item is VisionCaptureDetection => item !== null)
    : [];
  if (detections.length === 0) {
    return null;
  }
  const imageKind = readString(annotations.image_kind).trim() === 'annotated' ? 'annotated' : 'raw';
  return {
    image_kind: imageKind,
    coordinate_space: 'normalized_xywh',
    source: readString(annotations.source).trim() || undefined,
    detections,
  } satisfies VisionCaptureAnnotations;
}
