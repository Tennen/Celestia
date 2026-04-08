import { useRef } from 'react';
import { Input } from '../ui/input';
import type { VisionZoneBox } from '../../lib/types';

type Props = {
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

export function ZoneBoxEditor({ value, onChange }: Props) {
  const surfaceRef = useRef<HTMLDivElement | null>(null);
  const dragStartRef = useRef<Point | null>(null);

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

  return (
    <div className="zone-editor">
      <div
        ref={surfaceRef}
        className="zone-editor__surface"
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
        <div className="zone-editor__grid" />
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
        Drag on the surface to redraw the target zone. Values are stored as normalized percentages.
      </p>
    </div>
  );
}
