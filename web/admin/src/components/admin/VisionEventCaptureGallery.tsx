import { Badge } from '../ui/badge';
import { visionCaptureURL } from '../../lib/api';
import { formatTime } from '../../lib/utils';
import type { VisionEventCapture } from '../../lib/types';

type Props = {
  captures: VisionEventCapture[];
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function readString(value: unknown) {
  return typeof value === 'string' ? value : '';
}

function readNumber(value: unknown) {
  return typeof value === 'number' ? value : 0;
}

function readMetadata(value: unknown) {
  return isRecord(value) ? value : undefined;
}

function isCapturePhase(value: string): value is VisionEventCapture['phase'] {
  return value === 'start' || value === 'middle' || value === 'end';
}

export function visionEventCapturesFromPayload(payload?: Record<string, unknown>) {
  const raw = payload?.captures;
  if (!Array.isArray(raw)) {
    return [] as VisionEventCapture[];
  }
  return raw
    .map((item): VisionEventCapture | null => {
      if (!isRecord(item)) {
        return null;
      }
      const captureId = readString(item.capture_id);
      const eventId = readString(item.event_id);
      const phase = readString(item.phase);
      const capturedAt = readString(item.captured_at);
      const contentType = readString(item.content_type);
      if (!captureId || !eventId || !capturedAt || !contentType || !isCapturePhase(phase)) {
        return null;
      }
      return {
        capture_id: captureId,
        event_id: eventId,
        rule_id: readString(item.rule_id) || undefined,
        camera_device_id: readString(item.camera_device_id) || undefined,
        phase,
        captured_at: capturedAt,
        content_type: contentType,
        size_bytes: readNumber(item.size_bytes),
        metadata: readMetadata(item.metadata),
      };
    })
    .filter((item): item is VisionEventCapture => item !== null);
}

export function VisionEventCaptureGallery({ captures }: Props) {
  if (captures.length === 0) {
    return null;
  }
  return (
    <div className="vision-capture-gallery">
      {captures.map((capture) => (
        <a
          key={capture.capture_id}
          className="vision-capture-card"
          href={visionCaptureURL(capture.capture_id)}
          target="_blank"
          rel="noreferrer"
        >
          <div className="feed__meta">
            <Badge size="xs" tone="accent">
              {capture.phase}
            </Badge>
            <span>{formatTime(capture.captured_at)}</span>
          </div>
          <img
            className="vision-capture-image"
            src={visionCaptureURL(capture.capture_id)}
            alt={`Vision capture ${capture.phase}`}
            loading="lazy"
          />
        </a>
      ))}
    </div>
  );
}
