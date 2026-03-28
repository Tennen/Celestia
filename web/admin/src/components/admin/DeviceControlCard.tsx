import { useEffect, useState, type KeyboardEvent } from 'react';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Input } from '../ui/input';
import { ToggleSwitch } from '../ui/toggle-switch';
import type { DeviceControl } from '../../lib/types';
import { cn } from '../../lib/utils';

type ControlValue = string | number;

type Props = {
  deviceId: string;
  control: DeviceControl;
  busy: string;
  aliasValue: string;
  valueDraft: string;
  hidden?: boolean;
  showControlBody?: boolean;
  togglePending?: boolean;
  onAliasChange: (value: string) => void;
  onSavePreference: () => void;
  onResetPreference: () => void;
  onToggleVisibility: () => void;
  onToggle: (on: boolean) => void;
  onAction: () => void;
  onValueChange: (value: string) => void;
  onValueControl: (value: ControlValue) => void;
};

function toggleTone(control: DeviceControl, pending = false) {
  if (pending) return 'accent';
  if (control.state === true) return 'good';
  if (control.state === false) return 'neutral';
  return 'warn';
}

function formatValue(value: DeviceControl['value']) {
  if (value === null || value === undefined || value === '') return 'unset';
  if (typeof value === 'string') {
    return value.replace(/[-_]+/g, ' ');
  }
  return String(value);
}

function controlText(control: DeviceControl) {
  switch (control.kind) {
    case 'action':
      return null;
    case 'select': {
      const current = control.options?.find((option) => String(option.value) === String(control.value));
      return current?.label ?? formatValue(control.value);
    }
    default:
      return control.unit && typeof control.value === 'number' ? `${formatValue(control.value)} ${control.unit}` : formatValue(control.value);
  }
}

function numberHint(control: DeviceControl) {
  const parts: string[] = [];
  if (typeof control.min === 'number' || typeof control.max === 'number') {
    parts.push(`range ${control.min ?? 'auto'}-${control.max ?? 'auto'}`);
  }
  if (typeof control.step === 'number') {
    parts.push(`step ${control.step}`);
  }
  if (control.unit) {
    parts.push(`unit ${control.unit}`);
  }
  return parts.join(' · ');
}

function VisibilityIcon({ visible }: { visible: boolean }) {
  if (visible) {
    return (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
        <path d="M2 12s3.6-6 10-6 10 6 10 6-3.6 6-10 6-10-6-10-6Z" />
        <circle cx="12" cy="12" r="3" />
      </svg>
    );
  }

  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M10.7 5.1A11.4 11.4 0 0 1 12 5c6.4 0 10 7 10 7a17.2 17.2 0 0 1-2.4 3.2" />
      <path d="M6.6 6.7A16.8 16.8 0 0 0 2 12s3.6 7 10 7a10.8 10.8 0 0 0 4.1-.8" />
      <path d="m3 3 18 18" />
      <path d="M9.9 9.9a3 3 0 0 0 4.2 4.2" />
    </svg>
  );
}

function EditIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M12 20h9" />
      <path d="m16.5 3.5 4 4L8 20l-5 1 1-5Z" />
    </svg>
  );
}

function CheckIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="m5 12 5 5L20 7" />
    </svg>
  );
}

function ResetIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M3 12a9 9 0 1 0 3-6.7" />
      <path d="M3 4v5h5" />
    </svg>
  );
}

function PlayIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
      <path d="m8 6 10 6-10 6Z" />
    </svg>
  );
}

export function DeviceControlCard({
  deviceId,
  control,
  busy,
  aliasValue,
  valueDraft,
  hidden = false,
  showControlBody = true,
  togglePending = false,
  onAliasChange,
  onSavePreference,
  onResetPreference,
  onToggleVisibility,
  onToggle,
  onAction,
  onValueChange,
  onValueControl,
}: Props) {
  const prefBusy = busy === `control-pref-${deviceId}.${control.id}`;
  const toggleBusy = busy === `toggle-${deviceId}.${control.id}-on` || busy === `toggle-${deviceId}.${control.id}-off`;
  const actionBusy = busy === `action-${deviceId}.${control.id}`;
  const valueBusy = busy === `value-${deviceId}.${control.id}`;
  const defaultLabel = control.default_label ?? control.label;
  const isVisible = control.visible !== false;
  const parsedNumber = Number(valueDraft);
  const canApplyNumber = valueDraft.trim() !== '' && !Number.isNaN(parsedNumber);
  const toggleChecked = control.state === true;
  const toggleUnknown = control.state !== true && control.state !== false;
  const savedAlias = control.alias ?? '';
  const statusText = controlText(control);
  const [editingLabel, setEditingLabel] = useState(false);

  useEffect(() => {
    setEditingLabel(false);
  }, [deviceId, control.id]);

  const saveLabel = () => {
    onSavePreference();
    setEditingLabel(false);
  };

  const resetLabel = () => {
    onResetPreference();
    setEditingLabel(false);
  };

  const onAliasKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'Enter') {
      event.preventDefault();
      saveLabel();
      return;
    }
    if (event.key === 'Escape') {
      event.preventDefault();
      onAliasChange(savedAlias);
      setEditingLabel(false);
    }
  };

  const renderControlBody = () => {
    switch (control.kind) {
      case 'toggle':
      case 'action':
        return null;
      case 'select':
        return (
          <div className="stack">
            <select className="input" value={valueDraft} onChange={(event) => onValueChange(event.target.value)} disabled={valueBusy}>
              <option value="" disabled>
                Select a mode
              </option>
              {(control.options ?? []).map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
            <div className="button-row">
              <Button variant="secondary" onClick={() => onValueControl(valueDraft)} disabled={valueBusy || valueDraft.trim() === ''}>
                Apply
              </Button>
            </div>
          </div>
        );
      case 'number':
        return (
          <div className="stack">
            <div className="toolbar">
              <Input
                type="number"
                value={valueDraft}
                min={control.min}
                max={control.max}
                step={control.step}
                onChange={(event) => onValueChange(event.target.value)}
              />
              <Button variant="secondary" onClick={() => onValueControl(parsedNumber)} disabled={valueBusy || !canApplyNumber}>
                Apply
              </Button>
            </div>
            {numberHint(control) ? <p className="muted">{numberHint(control)}</p> : null}
          </div>
        );
      default:
        return null;
    }
  };

  return (
    <div className={cn('control-card', hidden && 'control-card--hidden')}>
      <div className="control-card__header">
        <div className="control-card__title">
          {editingLabel ? (
            <div className="control-card__title-edit">
              <Input
                className="control-card__title-input"
                value={aliasValue}
                onChange={(event) => onAliasChange(event.target.value)}
                placeholder={defaultLabel}
                autoFocus
                onKeyDown={onAliasKeyDown}
              />
              <div className="control-card__title-actions">
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className={cn('control-card__icon-button', 'control-card__icon-button--confirm')}
                  onClick={saveLabel}
                  disabled={prefBusy}
                  aria-label={`Save label for ${control.label}`}
                  title="Save label"
                >
                  <CheckIcon />
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className={cn('control-card__icon-button', 'control-card__icon-button--reset')}
                  onClick={resetLabel}
                  disabled={prefBusy}
                  aria-label={`Reset label for ${control.label}`}
                  title="Reset label"
                >
                  <ResetIcon />
                </Button>
              </div>
            </div>
          ) : (
            <div className="control-card__title-row">
              <strong>{control.label}</strong>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="control-card__icon-button"
                onClick={() => setEditingLabel(true)}
                disabled={prefBusy}
                aria-label={`Edit label for ${control.label}`}
                title="Edit label"
              >
                <EditIcon />
              </Button>
            </div>
          )}
          {control.alias && control.default_label ? <p>Default: {control.default_label}</p> : null}
          {control.description ? <p>{control.description}</p> : null}
        </div>
        <div className="control-card__meta">
          <div className="control-card__status">
            {showControlBody && control.kind === 'toggle' ? (
              <ToggleSwitch
                checked={toggleChecked}
                pending={togglePending}
                unknown={toggleUnknown && !togglePending}
                disabled={toggleBusy}
                label={control.label}
                onChange={onToggle}
              />
            ) : null}
            {showControlBody && control.kind === 'action' ? (
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="control-card__icon-button control-card__icon-button--play"
                onClick={onAction}
                disabled={actionBusy}
                aria-label={`Run ${control.label}`}
                title="Run action"
              >
                <PlayIcon />
              </Button>
            ) : null}
            {showControlBody && control.kind !== 'toggle' && control.kind !== 'action' && statusText ? (
              <Badge tone="accent">{statusText}</Badge>
            ) : null}
          </div>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="control-card__icon-button control-card__visibility"
            onClick={onToggleVisibility}
            disabled={prefBusy}
            aria-label={isVisible ? `Hide ${control.label}` : `Show ${control.label}`}
            title={isVisible ? 'Hide control' : 'Show control'}
          >
            <VisibilityIcon visible={isVisible} />
          </Button>
        </div>
      </div>

      {showControlBody ? renderControlBody() : null}

    </div>
  );
}
