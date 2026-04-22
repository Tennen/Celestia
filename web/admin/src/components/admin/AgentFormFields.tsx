import type { ReactNode } from 'react';
import { Input } from '../ui/input';
import { ToggleSwitch } from '../ui/toggle-switch';

export function Field(props: {
  label: string;
  value: string;
  placeholder?: string;
  type?: string;
  onChange: (value: string) => void;
}) {
  return (
    <label className="stack text-sm font-medium">
      <span>{props.label}</span>
      <Input
        type={props.type}
        value={props.value}
        placeholder={props.placeholder}
        onChange={(event) => props.onChange(event.target.value)}
      />
    </label>
  );
}

export function ToggleField(props: {
  label: string;
  checked: boolean;
  disabled?: boolean;
  onChange: (checked: boolean) => void;
}) {
  return (
    <div className="flex items-center justify-between gap-3 rounded-md border border-border-light p-3 text-sm">
      <span className="font-medium">{props.label}</span>
      <ToggleSwitch label={props.label} checked={props.checked} disabled={props.disabled} onChange={props.onChange} />
    </div>
  );
}

export function FieldGrid(props: { children: ReactNode }) {
  return <div className="grid gap-3 md:grid-cols-2">{props.children}</div>;
}

export function numberValue(value: unknown): string {
  return typeof value === 'number' && Number.isFinite(value) ? String(value) : '';
}

export function parseOptionalNumber(value: string): number | undefined {
  const trimmed = value.trim();
  if (!trimmed) return undefined;
  const parsed = Number(trimmed);
  return Number.isFinite(parsed) ? parsed : undefined;
}

export function requiredNumber(value: string, fallback = 0): number {
  return parseOptionalNumber(value) ?? fallback;
}
