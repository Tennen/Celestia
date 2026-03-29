import { useEffect, useMemo, useState, type KeyboardEvent } from 'react';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { Icon } from '../ui/icon';
import { Input } from '../ui/input';
import { Section } from '../ui/section';
import { asArray } from '../../lib/admin';
import {
  applyToggleOverrides,
  type ToggleControlOverrideMap,
  type ToggleControlPendingMap,
} from '../../lib/control-state';
import type { DeviceView } from '../../lib/types';
import { DeviceAdvancedCommandSection } from './DeviceAdvancedCommandSection';
import { DeviceQuickControlsPanel } from './DeviceQuickControlsPanel';

type Props = {
  deviceSearch: string;
  onDeviceSearchChange: (value: string) => void;
  onRefresh: () => void;
  devices: DeviceView[];
  selectedDeviceId: string;
  onSelectDevice: (deviceId: string) => void;
  selectedDevice: DeviceView | null;
  toggleOverrides: ToggleControlOverrideMap;
  togglePending: ToggleControlPendingMap;
  busy: string;
  selectedAction: string;
  onSelectedActionChange: (value: string) => void;
  actor: string;
  onActorChange: (value: string) => void;
  commandParams: string;
  onCommandParamsChange: (value: string) => void;
  commandSuggestions: Array<{ label: string; action: string; params: Record<string, unknown> }>;
  onApplySuggestion: (action: string, params: Record<string, unknown>) => void;
  onSendCommand: () => void;
  onToggleControl: (controlId: string, on: boolean) => void;
  onActionControl: (controlId: string) => void;
  onValueControl: (controlId: string, value: string | number) => void;
  onUpdateDevicePreference: (payload: { alias?: string }) => void;
  onUpdateControlPreference: (controlId: string, payload: { alias?: string; visible: boolean }) => void;
  commandResult: string;
  selectedDeviceDetails: string;
};

function EditIcon() {
  return (
    <Icon size="lg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M12 20h9" />
      <path d="m16.5 3.5 4 4L8 20l-5 1 1-5Z" />
    </Icon>
  );
}

function CheckIcon() {
  return (
    <Icon viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="m5 12 5 5L20 7" />
    </Icon>
  );
}

function ResetIcon() {
  return (
    <Icon viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M3 12a9 9 0 1 0 3-6.7" />
      <path d="M3 4v5h5" />
    </Icon>
  );
}

export function DeviceWorkspace({
  deviceSearch,
  onDeviceSearchChange,
  onRefresh,
  devices,
  selectedDeviceId,
  onSelectDevice,
  selectedDevice,
  toggleOverrides,
  togglePending,
  busy,
  selectedAction,
  onSelectedActionChange,
  actor,
  onActorChange,
  commandParams,
  onCommandParamsChange,
  commandSuggestions,
  onApplySuggestion,
  onSendCommand,
  onToggleControl,
  onActionControl,
  onValueControl,
  onUpdateDevicePreference,
  onUpdateControlPreference,
  commandResult,
  selectedDeviceDetails,
}: Props) {
  const [deviceAliasDraft, setDeviceAliasDraft] = useState('');
  const [editingDeviceAlias, setEditingDeviceAlias] = useState(false);
  const [aliasDrafts, setAliasDrafts] = useState<Record<string, string>>({});
  const [controlDrafts, setControlDrafts] = useState<Record<string, string>>({});
  const displayDevice = useMemo(() => applyToggleOverrides(selectedDevice, toggleOverrides), [selectedDevice, toggleOverrides]);
  const deviceView = displayDevice ?? selectedDevice;

  useEffect(() => {
    if (!selectedDevice) {
      setDeviceAliasDraft('');
      setEditingDeviceAlias(false);
      setAliasDrafts({});
      setControlDrafts({});
      return;
    }
    const nextAliasDrafts: Record<string, string> = {};
    const nextControlDrafts: Record<string, string> = {};
    setDeviceAliasDraft(selectedDevice.device.alias ?? '');
    setEditingDeviceAlias(false);
    for (const control of selectedDevice.controls ?? []) {
      nextAliasDrafts[control.id] = control.alias ?? '';
      if (control.kind === 'select' || control.kind === 'number') {
        nextControlDrafts[control.id] = control.value === null || control.value === undefined ? '' : String(control.value);
      }
    }
    setAliasDrafts(nextAliasDrafts);
    setControlDrafts(nextControlDrafts);
  }, [selectedDevice]);
  const savedDeviceAlias = deviceView?.device.alias ?? '';
  const defaultDeviceName = deviceView?.device.default_name ?? deviceView?.device.name ?? '';
  const hasSavedDeviceAlias = Boolean((deviceView?.device.alias ?? '').trim());
  const devicePrefBusy = Boolean(deviceView && busy === `device-pref-${deviceView.device.id}`);

  const saveDeviceAlias = () => {
    onUpdateDevicePreference({ alias: deviceAliasDraft.trim() });
    setEditingDeviceAlias(false);
  };

  const resetDeviceAlias = () => {
    setDeviceAliasDraft('');
    onUpdateDevicePreference({ alias: '' });
    setEditingDeviceAlias(false);
  };

  const onDeviceAliasKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'Enter') {
      event.preventDefault();
      saveDeviceAlias();
      return;
    }
    if (event.key === 'Escape') {
      event.preventDefault();
      setDeviceAliasDraft(savedDeviceAlias);
      setEditingDeviceAlias(false);
    }
  };

  return (
    <Section className="grid grid--two">
      <Card>
        <CardHeader>
          <CardTitle>Device List</CardTitle>
          <CardDescription>Stable device list with command shortcuts and search.</CardDescription>
        </CardHeader>
        <CardContent className="stack">
          <div className="toolbar">
            <Input value={deviceSearch} onChange={(event) => onDeviceSearchChange(event.target.value)} placeholder="Search devices" />
            <Button variant="secondary" onClick={onRefresh}>
              Search
            </Button>
          </div>
          <div className="table">
            {devices.map((item) => (
              <button
                key={item.device.id}
                type="button"
                className={`table__row ${selectedDeviceId === item.device.id ? 'is-selected' : ''}`}
                onClick={() => onSelectDevice(item.device.id)}
              >
                <div>
                  <strong>{item.device.name}</strong>
                  <p>{item.device.id}</p>
                </div>
                <div>
                  <Badge tone={item.device.online ? 'good' : 'bad'}>{item.device.online ? 'online' : 'offline'}</Badge>
                </div>
                <div>
                  <span>{item.device.kind}</span>
                  <p>{item.device.room || 'no room'}</p>
                </div>
              </button>
            ))}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Device Detail</CardTitle>
          <CardDescription>Selected device state, direct controls, and an advanced command panel for vendor-specific operations.</CardDescription>
        </CardHeader>
        <CardContent className="stack">
          {deviceView ? (
            <div className="detail">
              <div className="detail__header">
                <div className="control-card__title">
                  {editingDeviceAlias ? (
                    <div className="control-card__title-edit">
                      <Input
                        className="control-card__title-input"
                        value={deviceAliasDraft}
                        onChange={(event) => setDeviceAliasDraft(event.target.value)}
                        placeholder={defaultDeviceName}
                        autoFocus
                        onKeyDown={onDeviceAliasKeyDown}
                      />
                      <div className="control-card__title-actions">
                        <Button
                          type="button"
                          variant="ghost"
                          size="sm"
                          className="control-card__icon-button control-card__icon-button--confirm"
                          onClick={saveDeviceAlias}
                          disabled={devicePrefBusy}
                          aria-label={`Save label for ${deviceView.device.name}`}
                          title="Save label"
                        >
                          <CheckIcon />
                        </Button>
                        <Button
                          type="button"
                          variant="ghost"
                          size="sm"
                          className="control-card__icon-button control-card__icon-button--reset"
                          onClick={resetDeviceAlias}
                          disabled={devicePrefBusy || (!hasSavedDeviceAlias && deviceAliasDraft.trim() === '')}
                          aria-label={`Reset label for ${deviceView.device.name}`}
                          title="Reset label"
                        >
                          <ResetIcon />
                        </Button>
                      </div>
                    </div>
                  ) : (
                    <div className="control-card__title-row">
                      <h3>{deviceView.device.name}</h3>
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        className="control-card__icon-button control-card__icon-button--edit"
                        onClick={() => setEditingDeviceAlias(true)}
                        disabled={devicePrefBusy}
                        aria-label={`Edit label for ${deviceView.device.name}`}
                        title="Edit label"
                      >
                        <EditIcon />
                      </Button>
                    </div>
                  )}
                  <p>{deviceView.device.id}</p>
                  {hasSavedDeviceAlias ? <p>Default: {defaultDeviceName}</p> : null}
                </div>
                <Badge tone={deviceView.device.online ? 'good' : 'bad'}>
                  {deviceView.device.online ? 'online' : 'offline'}
                </Badge>
              </div>
              <div className="chip-list">
                {asArray(deviceView.device.capabilities).map((capability) => (
                  <Badge key={capability} tone="neutral">
                    {capability}
                  </Badge>
                ))}
              </div>
              <div className="stack">
                <DeviceQuickControlsPanel
                  device={deviceView}
                  selectedDevice={selectedDevice}
                  toggleOverrides={toggleOverrides}
                  togglePending={togglePending}
                  busy={busy}
                  aliasDrafts={aliasDrafts}
                  controlDrafts={controlDrafts}
                  onAliasChange={(controlId, value) => setAliasDrafts((current) => ({ ...current, [controlId]: value }))}
                  onSavePreference={(controlId, visible) =>
                    onUpdateControlPreference(controlId, { alias: (aliasDrafts[controlId] ?? '').trim(), visible })
                  }
                  onResetPreference={(controlId) => {
                    setAliasDrafts((current) => ({ ...current, [controlId]: '' }));
                    const control = deviceView.controls?.find((item) => item.id === controlId);
                    onUpdateControlPreference(controlId, { alias: '', visible: control?.visible !== false });
                  }}
                  onToggleVisibility={(controlId, visible) =>
                    onUpdateControlPreference(controlId, {
                      alias: (aliasDrafts[controlId] ?? '').trim(),
                      visible,
                    })
                  }
                  onToggle={(controlId, on) => onToggleControl(controlId, on)}
                  onAction={(controlId) => onActionControl(controlId)}
                  onValueChange={(controlId, value) => setControlDrafts((current) => ({ ...current, [controlId]: value }))}
                  onValueControl={(controlId, value) => onValueControl(controlId, value)}
                />
                <DeviceAdvancedCommandSection
                  deviceId={deviceView.device.id}
                  selectedAction={selectedAction}
                  onSelectedActionChange={onSelectedActionChange}
                  actor={actor}
                  onActorChange={onActorChange}
                  commandParams={commandParams}
                  onCommandParamsChange={onCommandParamsChange}
                  commandSuggestions={commandSuggestions}
                  onApplySuggestion={onApplySuggestion}
                  onSendCommand={onSendCommand}
                />
              </div>
              {commandResult ? <pre className="log-box">{commandResult}</pre> : null}
              <pre className="log-box">{selectedDeviceDetails}</pre>
            </div>
          ) : (
            <p className="muted">No device selected.</p>
          )}
        </CardContent>
      </Card>
    </Section>
  );
}
