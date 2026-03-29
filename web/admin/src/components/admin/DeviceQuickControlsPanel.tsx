import { useEffect, useMemo, useState } from 'react';
import { Button } from '../ui/button';
import { Icon } from '../ui/icon';
import type {
  ToggleControlOverrideMap,
  ToggleControlPendingMap,
} from '../../lib/control-state';
import {
  isToggleControlPending,
  isToggleControlRequestPending,
} from '../../lib/control-state';
import type { DeviceView } from '../../lib/types';
import { cn } from '../../lib/utils';
import { DeviceControlCard } from './DeviceControlCard';

type Props = {
  device: DeviceView;
  selectedDevice: DeviceView | null;
  toggleOverrides: ToggleControlOverrideMap;
  togglePending: ToggleControlPendingMap;
  busy: string;
  aliasDrafts: Record<string, string>;
  controlDrafts: Record<string, string>;
  onAliasChange: (controlId: string, value: string) => void;
  onSavePreference: (controlId: string, visible: boolean) => void;
  onResetPreference: (controlId: string) => void;
  onToggleVisibility: (controlId: string, visible: boolean) => void;
  onToggle: (controlId: string, on: boolean) => void;
  onAction: (controlId: string) => void;
  onValueChange: (controlId: string, value: string) => void;
  onValueControl: (controlId: string, value: string | number) => void;
};

function ChevronIcon({ expanded }: { expanded: boolean }) {
  return (
    <Icon
      size="md"
      className={cn('collapse-toggle__icon', expanded && 'is-expanded')}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="m6 9 6 6 6-6" />
    </Icon>
  );
}

function ControlGrid({
  device,
  controls,
  selectedDevice,
  toggleOverrides,
  togglePending,
  busy,
  aliasDrafts,
  controlDrafts,
  hidden = false,
  showControlBody = true,
  onAliasChange,
  onSavePreference,
  onResetPreference,
  onToggleVisibility,
  onToggle,
  onAction,
  onValueChange,
  onValueControl,
}: {
  device: DeviceView;
  controls: NonNullable<DeviceView['controls']>;
  selectedDevice: DeviceView | null;
  toggleOverrides: ToggleControlOverrideMap;
  togglePending: ToggleControlPendingMap;
  busy: string;
  aliasDrafts: Record<string, string>;
  controlDrafts: Record<string, string>;
  hidden?: boolean;
  showControlBody?: boolean;
  onAliasChange: (controlId: string, value: string) => void;
  onSavePreference: (controlId: string, visible: boolean) => void;
  onResetPreference: (controlId: string) => void;
  onToggleVisibility: (controlId: string, visible: boolean) => void;
  onToggle: (controlId: string, on: boolean) => void;
  onAction: (controlId: string) => void;
  onValueChange: (controlId: string, value: string) => void;
  onValueControl: (controlId: string, value: string | number) => void;
}) {
  return (
    <div className="control-grid">
      {controls.map((control) => (
        <DeviceControlCard
          key={control.id}
          deviceId={device.device.id}
          control={control}
          busy={busy}
          aliasValue={aliasDrafts[control.id] ?? ''}
          valueDraft={controlDrafts[control.id] ?? ''}
          hidden={hidden}
          showControlBody={showControlBody}
          togglePending={isToggleControlPending(selectedDevice, control.id, toggleOverrides)}
          toggleDisabled={isToggleControlRequestPending(selectedDevice, control.id, togglePending)}
          onAliasChange={(value) => onAliasChange(control.id, value)}
          onSavePreference={() => onSavePreference(control.id, control.visible !== false)}
          onResetPreference={() => onResetPreference(control.id)}
          onToggleVisibility={() => onToggleVisibility(control.id, control.visible === false)}
          onToggle={(on) => onToggle(control.id, on)}
          onAction={() => onAction(control.id)}
          onValueChange={(value) => onValueChange(control.id, value)}
          onValueControl={(value) => onValueControl(control.id, value)}
        />
      ))}
    </div>
  );
}

export function DeviceQuickControlsPanel({
  device,
  selectedDevice,
  toggleOverrides,
  togglePending,
  busy,
  aliasDrafts,
  controlDrafts,
  onAliasChange,
  onSavePreference,
  onResetPreference,
  onToggleVisibility,
  onToggle,
  onAction,
  onValueChange,
  onValueControl,
}: Props) {
  const [hiddenControlsCollapsed, setHiddenControlsCollapsed] = useState(true);

  useEffect(() => {
    setHiddenControlsCollapsed(true);
  }, [device.device.id]);

  const visibleControls = useMemo(
    () => (device.controls ?? []).filter((control) => control.visible !== false),
    [device.controls],
  );
  const hiddenControls = useMemo(
    () => (device.controls ?? []).filter((control) => control.visible === false),
    [device.controls],
  );

  return (
    <>
      <div>
        <label>Quick Controls</label>
        {visibleControls.length > 0 ? (
          <ControlGrid
            device={device}
            controls={visibleControls}
            selectedDevice={selectedDevice}
            toggleOverrides={toggleOverrides}
            togglePending={togglePending}
            busy={busy}
            aliasDrafts={aliasDrafts}
            controlDrafts={controlDrafts}
            onAliasChange={onAliasChange}
            onSavePreference={onSavePreference}
            onResetPreference={onResetPreference}
            onToggleVisibility={onToggleVisibility}
            onToggle={onToggle}
            onAction={onAction}
            onValueChange={onValueChange}
            onValueControl={onValueControl}
          />
        ) : (
          <p className="muted">No visible quick controls are configured for this device.</p>
        )}
      </div>

      <div>
        <div className="section-title section-title--inline">
          <label>Hidden Controls</label>
          {hiddenControls.length > 0 ? (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="collapse-toggle"
              onClick={() => setHiddenControlsCollapsed((current) => !current)}
              aria-expanded={!hiddenControlsCollapsed}
              aria-controls="hidden-controls-panel"
            >
              <span>{hiddenControlsCollapsed ? `Show ${hiddenControls.length}` : `Hide ${hiddenControls.length}`}</span>
              <ChevronIcon expanded={!hiddenControlsCollapsed} />
            </Button>
          ) : null}
        </div>
        {hiddenControls.length > 0 && !hiddenControlsCollapsed ? (
          <div id="hidden-controls-panel">
            <ControlGrid
              device={device}
              controls={hiddenControls}
              selectedDevice={selectedDevice}
              toggleOverrides={toggleOverrides}
              togglePending={togglePending}
              busy={busy}
              aliasDrafts={aliasDrafts}
              controlDrafts={controlDrafts}
              hidden
              showControlBody={false}
              onAliasChange={onAliasChange}
              onSavePreference={onSavePreference}
              onResetPreference={onResetPreference}
              onToggleVisibility={onToggleVisibility}
              onToggle={onToggle}
              onAction={onAction}
              onValueChange={onValueChange}
              onValueControl={onValueControl}
            />
          </div>
        ) : null}
        {hiddenControls.length === 0 ? <p className="muted">No hidden quick controls.</p> : null}
      </div>
    </>
  );
}
