import { useEffect, useMemo, useState, type KeyboardEvent } from 'react';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { Input } from '../ui/input';
import { Section } from '../ui/section';
import { CheckIcon, EditIcon, ResetIcon } from '../ui/icons';
import { asArray } from '../../lib/admin';
import { applyToggleOverrides } from '../../lib/control-state';
import { buildCommandSuggestions } from '../../lib/command-suggestions';
import { useAdminStore } from '../../stores/adminStore';
import { useDeviceStore } from '../../stores/deviceStore';
import { DeviceAdvancedCommandSection } from './DeviceAdvancedCommandSection';
import { DeviceQuickControlsPanel } from './DeviceQuickControlsPanel';
import { StreamViewerPanel } from './StreamViewerPanel';

export function DeviceWorkspace() {
  const { devices, refreshAll } = useAdminStore();
  const {
    selectedDeviceId,
    deviceSearch,
    selectedAction,
    commandParams,
    actor,
    commandResult,
    busy,
    toggleOverrides,
    togglePending,
    setSelectedDeviceId,
    setDeviceSearch,
    setSelectedAction,
    setCommandParams,
    setActor,
    applySuggestion,
    sendCommand,
    onToggleControl,
    onActionControl,
    onValueControl,
    updateDevicePreference,
    updateControlPreference,
  } = useDeviceStore();

  const selectedDevice = devices.find((d) => d.device.id === selectedDeviceId) ?? null;

  const [deviceAliasDraft, setDeviceAliasDraft] = useState('');
  const [editingDeviceAlias, setEditingDeviceAlias] = useState(false);
  const [aliasDrafts, setAliasDrafts] = useState<Record<string, string>>({});
  const [controlDrafts, setControlDrafts] = useState<Record<string, string>>({});

  const displayDevice = useMemo(
    () => applyToggleOverrides(selectedDevice, toggleOverrides),
    [selectedDevice, toggleOverrides],
  );
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
        nextControlDrafts[control.id] =
          control.value === null || control.value === undefined ? '' : String(control.value);
      }
    }
    setAliasDrafts(nextAliasDrafts);
    setControlDrafts(nextControlDrafts);
  }, [selectedDevice]);

  const commandSuggestions = useMemo(
    () => (selectedDevice ? buildCommandSuggestions(selectedDevice) : []),
    [selectedDevice],
  );

  const savedDeviceAlias = deviceView?.device.alias ?? '';
  const defaultDeviceName = deviceView?.device.default_name ?? deviceView?.device.name ?? '';
  const hasSavedDeviceAlias = Boolean((deviceView?.device.alias ?? '').trim());
  const devicePrefBusy = Boolean(deviceView && busy === `device-pref-${deviceView.device.id}`);
  const selectedDeviceDetails = selectedDevice ? JSON.stringify(selectedDevice, null, 2) : '';

  const saveDeviceAlias = () => {
    if (!selectedDevice) return;
    void updateDevicePreference(selectedDevice, { alias: deviceAliasDraft.trim() });
    setEditingDeviceAlias(false);
  };

  const resetDeviceAlias = () => {
    if (!selectedDevice) return;
    setDeviceAliasDraft('');
    void updateDevicePreference(selectedDevice, { alias: '' });
    setEditingDeviceAlias(false);
  };

  const onDeviceAliasKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'Enter') { event.preventDefault(); saveDeviceAlias(); return; }
    if (event.key === 'Escape') { event.preventDefault(); setDeviceAliasDraft(savedDeviceAlias); setEditingDeviceAlias(false); }
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
            <Input
              value={deviceSearch}
              onChange={(e) => setDeviceSearch(e.target.value)}
              placeholder="Search devices"
            />
            <Button variant="secondary" onClick={() => void refreshAll()}>
              Search
            </Button>
          </div>
          <div className="table">
            {devices.map((item) => (
              <button
                key={item.device.id}
                type="button"
                className={`table__row ${selectedDeviceId === item.device.id ? 'is-selected' : ''}`}
                onClick={() => setSelectedDeviceId(item.device.id)}
              >
                <div>
                  <strong>{item.device.name}</strong>
                  <p>{item.device.id}</p>
                </div>
                <div>
                  <Badge tone={item.device.online ? 'good' : 'bad'}>
                    {item.device.online ? 'online' : 'offline'}
                  </Badge>
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
                        onChange={(e) => setDeviceAliasDraft(e.target.value)}
                        placeholder={defaultDeviceName}
                        autoFocus
                        onKeyDown={onDeviceAliasKeyDown}
                      />
                      <div className="control-card__title-actions">
                        <Button type="button" variant="ghost" size="sm"
                          className="control-card__icon-button control-card__icon-button--confirm"
                          onClick={saveDeviceAlias} disabled={devicePrefBusy}
                          aria-label={`Save label for ${deviceView.device.name}`} title="Save label">
                          <CheckIcon />
                        </Button>
                        <Button type="button" variant="ghost" size="sm"
                          className="control-card__icon-button control-card__icon-button--reset"
                          onClick={resetDeviceAlias}
                          disabled={devicePrefBusy || (!hasSavedDeviceAlias && deviceAliasDraft.trim() === '')}
                          aria-label={`Reset label for ${deviceView.device.name}`} title="Reset label">
                          <ResetIcon />
                        </Button>
                      </div>
                    </div>
                  ) : (
                    <div className="control-card__title-row">
                      <h3>{deviceView.device.name}</h3>
                      <Button type="button" variant="ghost" size="sm"
                        className="control-card__icon-button control-card__icon-button--edit"
                        onClick={() => setEditingDeviceAlias(true)} disabled={devicePrefBusy}
                        aria-label={`Edit label for ${deviceView.device.name}`} title="Edit label">
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
                  <Badge key={capability} tone="neutral">{capability}</Badge>
                ))}
              </div>
              {deviceView.device.capabilities.includes('stream') && (
                <StreamViewerPanel deviceId={deviceView.device.id} />
              )}
              <div className="stack">
                <DeviceQuickControlsPanel
                  device={deviceView}
                  selectedDevice={selectedDevice}
                  toggleOverrides={toggleOverrides}
                  togglePending={togglePending}
                  busy={busy}
                  aliasDrafts={aliasDrafts}
                  controlDrafts={controlDrafts}
                  onAliasChange={(controlId, value) =>
                    setAliasDrafts((c) => ({ ...c, [controlId]: value }))
                  }
                  onSavePreference={(controlId, visible) => {
                    if (!selectedDevice) return;
                    void updateControlPreference(selectedDevice, controlId, {
                      alias: (aliasDrafts[controlId] ?? '').trim(),
                      visible,
                    });
                  }}
                  onResetPreference={(controlId) => {
                    if (!selectedDevice) return;
                    setAliasDrafts((c) => ({ ...c, [controlId]: '' }));
                    const control = deviceView.controls?.find((item) => item.id === controlId);
                    void updateControlPreference(selectedDevice, controlId, {
                      alias: '',
                      visible: control?.visible !== false,
                    });
                  }}
                  onToggleVisibility={(controlId, visible) => {
                    if (!selectedDevice) return;
                    void updateControlPreference(selectedDevice, controlId, {
                      alias: (aliasDrafts[controlId] ?? '').trim(),
                      visible,
                    });
                  }}
                  onToggle={(controlId, on) => {
                    if (!selectedDevice) return;
                    void onToggleControl(selectedDevice, controlId, on);
                  }}
                  onAction={(controlId) => {
                    if (!selectedDevice) return;
                    void onActionControl(selectedDevice, controlId);
                  }}
                  onValueChange={(controlId, value) =>
                    setControlDrafts((c) => ({ ...c, [controlId]: value }))
                  }
                  onValueControl={(controlId, value) => {
                    if (!selectedDevice) return;
                    void onValueControl(selectedDevice, controlId, value);
                  }}
                />
                <DeviceAdvancedCommandSection
                  deviceId={deviceView.device.id}
                  selectedAction={selectedAction}
                  onSelectedActionChange={setSelectedAction}
                  actor={actor}
                  onActorChange={setActor}
                  commandParams={commandParams}
                  onCommandParamsChange={setCommandParams}
                  commandSuggestions={commandSuggestions}
                  onApplySuggestion={applySuggestion}
                  onSendCommand={() => {
                    if (!selectedDevice) return;
                    void sendCommand(selectedDevice);
                  }}
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
