import { Badge } from '../ui/badge';
import { visionCaptureURL } from '../../lib/api';
import { visionCaptureAnnotationsFromMetadata } from '../../lib/visionCaptureAnnotations';
import { formatTime } from '../../lib/utils';
import type { VisionEventCapture } from '../../lib/types';

type Props = {
  captures: VisionEventCapture[];
};

type CaptureCardProps = {
  capture: VisionEventCapture;
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
      const phase = readString(item.phase).trim();
      const capturedAt = readString(item.captured_at);
      const contentType = readString(item.content_type);
      if (!captureId || !eventId || !capturedAt || !contentType || !phase) {
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

function VisionCaptureCard({ capture }: CaptureCardProps) {
  const annotations = visionCaptureAnnotationsFromMetadata(capture.metadata);

  return (
    <a
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
      <div className="vision-capture-preview">
        <img
          className="vision-capture-image"
          src={visionCaptureURL(capture.capture_id)}
          alt={`Vision capture ${capture.phase}`}
          loading="lazy"
        />
        {annotations?.image_kind === 'raw' ? (
          <div className="vision-capture-overlay">
            {annotations.detections.map((detection, index) => (
              <div
                key={`${capture.capture_id}:${detection.display_name}:${index}`}
                className="vision-capture-box"
                style={{
                  left: `${detection.box.x * 100}%`,
                  top: `${detection.box.y * 100}%`,
                  width: `${detection.box.width * 100}%`,
                  height: `${detection.box.height * 100}%`,
                }}
              >
                <span className="vision-capture-box__label">
                  {detection.display_name}
                  {typeof detection.confidence === 'number' ? ` ${Math.round(detection.confidence * 100)}%` : ''}
                </span>
              </div>
            ))}
          </div>
        ) : null}
      </div>
    </a>
  );
}

export function VisionEventCaptureGallery({ captures }: Props) {
  if (captures.length === 0) {
    return null;
  }
  const galleryClassName =
    captures.length > 3 ? 'vision-capture-gallery vision-capture-gallery--scroll' : 'vision-capture-gallery';
  return (
    <div className={galleryClassName}>
      {captures.map((capture) => <VisionCaptureCard key={capture.capture_id} capture={capture} />)}
    </div>
  );
}
