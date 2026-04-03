import * as Collapsible from '@radix-ui/react-collapsible';
import { useEffect, useMemo, useState } from 'react';
import { ChevronDown } from 'lucide-react';
import { Button } from '../ui/button';
import type {
  ToggleControlOverrideMap,
  ToggleControlPendingMap,
} from '../../lib/control-state';
import {
  isToggleControlPending,
  isToggleControlRequestPending,
} from '../../lib/control-state';
import type { DeviceView } from '../../lib/types';
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
            <Collapsible.Root
              open={!hiddenControlsCollapsed}
              onOpenChange={(open) => setHiddenControlsCollapsed(!open)}
            >
              <Collapsible.Trigger asChild>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="collapse-toggle"
                  aria-controls="hidden-controls-panel"
                >
                  <span>
                    {hiddenControlsCollapsed ? `Show ${hiddenControls.length}` : `Hide ${hiddenControls.length}`}
                  </span>
                  <ChevronDown
                    className={`collapse-toggle__icon ${!hiddenControlsCollapsed ? 'is-expanded' : ''}`}
                  />
                </Button>
              </Collapsible.Trigger>
              <Collapsible.Content id="hidden-controls-panel" className="pt-4">
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
              </Collapsible.Content>
            </Collapsible.Root>
          ) : null}
        </div>
        {hiddenControls.length === 0 ? <p className="muted">No hidden quick controls.</p> : null}
      </div>
    </>
  );
}
