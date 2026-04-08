import { useEffect, useState, type KeyboardEvent } from 'react';
import { Check, Eye, EyeOff, PencilLine, Play, RotateCcw } from 'lucide-react';
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
  toggleDisabled?: boolean;
  onAliasChange: (value: string) => void;
  onSavePreference: () => void;
  onResetPreference: () => void;
  onToggleVisibility: () => void;
  onToggle: (on: boolean) => void;
  onAction: () => void;
  onValueChange: (value: string) => void;
  onValueControl: (value: ControlValue) => void;
};

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



export function DeviceControlCard({
  deviceId,
  control,
  busy,
  aliasValue,
  valueDraft,
  hidden = false,
  showControlBody = true,
  togglePending = false,
  toggleDisabled = false,
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
  const isDisabled = control.disabled === true;
  const disabledReason = control.disabled_reason?.trim() ?? '';
  const toggleBusy = toggleDisabled || isDisabled;
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
            <div className="toolbar control-card__value-row">
              <select
                className="select"
                value={valueDraft}
                onChange={(event) => onValueChange(event.target.value)}
                disabled={valueBusy || isDisabled}
              >
                <option value="" disabled>
                  Select a mode
                </option>
                {(control.options ?? []).map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
              <Button variant="secondary" onClick={() => onValueControl(valueDraft)} disabled={valueBusy || isDisabled || valueDraft.trim() === ''}>
                Apply
              </Button>
            </div>
            {disabledReason ? <p className="muted">{disabledReason}</p> : null}
          </div>
        );
      case 'number':
        return (
          <div className="stack">
            <div className="toolbar control-card__value-row">
              <Input
                type="number"
                value={valueDraft}
                min={control.min}
                max={control.max}
                step={control.step}
                onChange={(event) => onValueChange(event.target.value)}
                disabled={valueBusy || isDisabled}
              />
              <Button variant="secondary" onClick={() => onValueControl(parsedNumber)} disabled={valueBusy || isDisabled || !canApplyNumber}>
                Apply
              </Button>
            </div>
            {numberHint(control) ? <p className="muted">{numberHint(control)}</p> : null}
            {disabledReason ? <p className="muted">{disabledReason}</p> : null}
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
                  <Check />
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
                  <RotateCcw />
                </Button>
              </div>
            </div>
          ) : (
            <div className="control-card__title-row">
              <strong className="control-card__title-label" title={control.label}>
                {control.label}
              </strong>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="control-card__icon-button control-card__icon-button--edit"
                onClick={() => setEditingLabel(true)}
                disabled={prefBusy}
                aria-label={`Edit label for ${control.label}`}
                title="Edit label"
              >
                <PencilLine />
              </Button>
            </div>
          )}
          {control.alias && control.default_label ? <p>Default: {control.default_label}</p> : null}
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
                disabled={actionBusy || isDisabled}
                aria-label={`Run ${control.label}`}
                title={disabledReason || 'Run action'}
              >
                <Play />
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
            {isVisible ? <Eye /> : <EyeOff />}
          </Button>
        </div>
      </div>

      {showControlBody ? renderControlBody() : null}
      {showControlBody && (control.kind === 'toggle' || control.kind === 'action') && disabledReason ? (
        <p className="muted">{disabledReason}</p>
      ) : null}

    </div>
  );
}
