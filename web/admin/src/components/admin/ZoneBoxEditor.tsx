import { useEffect, useRef, useState } from 'react';
import { Button } from '../ui/button';
import { Input } from '../ui/input';
import { useStreamSession } from '../../hooks/useStreamSession';
import type { DeviceView, VisionZoneBox } from '../../lib/types';

type Props = {
  cameraDevice?: DeviceView | null;
  value: VisionZoneBox;
  onChange: (value: VisionZoneBox) => void;
};

type Point = {
  x: number;
  y: number;
};

function clampUnit(value: number) {
  if (value < 0) return 0;
  if (value > 1) return 1;
  return value;
}

function normalizeBox(start: Point, end: Point): VisionZoneBox {
  const left = Math.min(start.x, end.x);
  const top = Math.min(start.y, end.y);
  const right = Math.max(start.x, end.x);
  const bottom = Math.max(start.y, end.y);
  return {
    x: clampUnit(left),
    y: clampUnit(top),
    width: clampUnit(right - left),
    height: clampUnit(bottom - top),
  };
}

type VideoFrame = {
  width: number;
  height: number;
};

function supportsLivePreview(cameraDevice?: DeviceView | null) {
  return Boolean(cameraDevice?.device.capabilities.includes('stream'));
}

function previewStatusText(
  cameraDevice: DeviceView | null | undefined,
  previewSupported: boolean,
  state: 'idle' | 'connecting' | 'active' | 'error',
  errorMessage: string | null,
) {
  if (!cameraDevice) {
    return 'Select a camera to anchor the zone against a live frame.';
  }
  if (!previewSupported) {
    return 'Selected camera does not expose the stream capability, so live RTSP preview is unavailable.';
  }
  if (state === 'error') {
    return errorMessage ?? 'Live preview failed to start.';
  }
  if (state === 'connecting' || state === 'idle') {
    return 'Connecting live preview…';
  }
  return `${cameraDevice.device.name} live preview`;
}

export function ZoneBoxEditor({ cameraDevice, value, onChange }: Props) {
  const surfaceRef = useRef<HTMLDivElement | null>(null);
  const dragStartRef = useRef<Point | null>(null);
  const [frame, setFrame] = useState<VideoFrame | null>(null);
  const previewDeviceId = cameraDevice?.device.id ?? '';
  const previewSupported = supportsLivePreview(cameraDevice);
  const { state, errorMessage, videoRef, startStream, stopStream } = useStreamSession(previewDeviceId);

  const readPoint = (clientX: number, clientY: number): Point | null => {
    const rect = surfaceRef.current?.getBoundingClientRect();
    if (!rect || rect.width <= 0 || rect.height <= 0) return null;
    return {
      x: clampUnit((clientX - rect.left) / rect.width),
      y: clampUnit((clientY - rect.top) / rect.height),
    };
  };

  const updateNumericField = (field: keyof VisionZoneBox, raw: string) => {
    const parsed = Number(raw);
    if (Number.isNaN(parsed)) return;
    const next = { ...value, [field]: clampUnit(parsed / 100) };
    if (next.x + next.width > 1) {
      next.width = 1 - next.x;
    }
    if (next.y + next.height > 1) {
      next.height = 1 - next.y;
    }
    onChange(next);
  };

  useEffect(() => {
    dragStartRef.current = null;
    setFrame(null);
  }, [previewDeviceId]);

  useEffect(() => {
    if (!previewDeviceId || !previewSupported) {
      return;
    }
    void startStream();
    return () => {
      void stopStream();
    };
  }, [previewDeviceId, previewSupported, startStream, stopStream]);

  useEffect(() => {
    const video = videoRef.current;
    if (!video) {
      return;
    }

    const syncFrame = () => {
      if (video.videoWidth > 0 && video.videoHeight > 0) {
        setFrame({
          width: video.videoWidth,
          height: video.videoHeight,
        });
      }
    };

    syncFrame();
    video.addEventListener('loadedmetadata', syncFrame);
    video.addEventListener('resize', syncFrame);
    return () => {
      video.removeEventListener('loadedmetadata', syncFrame);
      video.removeEventListener('resize', syncFrame);
    };
  }, [previewDeviceId, state, videoRef]);

  const previewText = previewStatusText(cameraDevice, previewSupported, state, errorMessage);

  return (
    <div className="zone-editor">
      <div
        ref={surfaceRef}
        className="zone-editor__surface"
        style={frame ? { aspectRatio: `${frame.width} / ${frame.height}` } : { minHeight: 'clamp(15rem, 38vw, 28rem)' }}
        onPointerDown={(event) => {
          const point = readPoint(event.clientX, event.clientY);
          if (!point) return;
          dragStartRef.current = point;
          event.currentTarget.setPointerCapture(event.pointerId);
          onChange(normalizeBox(point, point));
        }}
        onPointerMove={(event) => {
          if (!dragStartRef.current) return;
          const point = readPoint(event.clientX, event.clientY);
          if (!point) return;
          onChange(normalizeBox(dragStartRef.current, point));
        }}
        onPointerUp={(event) => {
          dragStartRef.current = null;
          event.currentTarget.releasePointerCapture(event.pointerId);
        }}
        onPointerCancel={(event) => {
          dragStartRef.current = null;
          event.currentTarget.releasePointerCapture(event.pointerId);
        }}
      >
        {previewSupported ? (
          <video
            ref={videoRef}
            autoPlay
            playsInline
            muted
            className={`zone-editor__video ${state === 'active' ? 'is-active' : ''}`}
          />
        ) : null}
        <div className="zone-editor__backdrop" />
        <div className="zone-editor__grid" />
        {state !== 'active' ? (
          <div className="zone-editor__placeholder">
            <strong>{cameraDevice?.device.name ?? 'Live preview unavailable'}</strong>
            <span>{previewText}</span>
          </div>
        ) : null}
        <div
          className="zone-editor__box"
          style={{
            left: `${value.x * 100}%`,
            top: `${value.y * 100}%`,
            width: `${value.width * 100}%`,
            height: `${value.height * 100}%`,
          }}
        />
      </div>

      <div className="zone-editor__preview-meta">
        <p className="muted">
          {state === 'active'
            ? `${previewText}. Zone coordinates stay normalized to the displayed camera frame.`
            : previewText}
        </p>
        {previewSupported && state === 'error' ? (
          <Button variant="secondary" size="sm" onClick={() => void startStream()}>
            Retry Preview
          </Button>
        ) : null}
      </div>

      <div className="zone-editor__coords">
        {[
          ['x', value.x],
          ['y', value.y],
          ['width', value.width],
          ['height', value.height],
        ].map(([field, fieldValue]) => (
          <div key={field} className="automation-field">
            <label>{field}</label>
            <Input
              value={Math.round((fieldValue as number) * 100)}
              onChange={(event) => updateNumericField(field as keyof VisionZoneBox, event.target.value)}
              type="number"
              min={0}
              max={100}
              step={1}
            />
          </div>
        ))}
      </div>
      <p className="muted">
        Drag on the frame to redraw the target zone. Values are stored as normalized percentages.
      </p>
    </div>
  );
}
