import { useEffect, useMemo, useState } from 'react';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { Icon } from '../ui/icon';
import { Input } from '../ui/input';
import { Section } from '../ui/section';
import { Textarea } from '../ui/textarea';
import { asArray } from '../../lib/admin';
import {
  applyToggleOverrides,
  isToggleControlPending,
  isToggleControlRequestPending,
  type ToggleControlOverrideMap,
  type ToggleControlPendingMap,
} from '../../lib/control-state';
import type { DeviceView } from '../../lib/types';
import { cn } from '../../lib/utils';
import { DeviceControlCard } from './DeviceControlCard';

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
  const [aliasDrafts, setAliasDrafts] = useState<Record<string, string>>({});
  const [controlDrafts, setControlDrafts] = useState<Record<string, string>>({});
  const [hiddenControlsCollapsed, setHiddenControlsCollapsed] = useState(true);
  const [advancedCommandCollapsed, setAdvancedCommandCollapsed] = useState(true);
  const displayDevice = useMemo(() => applyToggleOverrides(selectedDevice, toggleOverrides), [selectedDevice, toggleOverrides]);
  const deviceView = displayDevice ?? selectedDevice;

  useEffect(() => {
    if (!selectedDevice) {
      setDeviceAliasDraft('');
      setAliasDrafts({});
      setControlDrafts({});
      setHiddenControlsCollapsed(true);
      setAdvancedCommandCollapsed(true);
      return;
    }
    const nextAliasDrafts: Record<string, string> = {};
    const nextControlDrafts: Record<string, string> = {};
    setDeviceAliasDraft(selectedDevice.device.alias ?? '');
    for (const control of selectedDevice.controls ?? []) {
      nextAliasDrafts[control.id] = control.alias ?? '';
      if (control.kind === 'select' || control.kind === 'number') {
        nextControlDrafts[control.id] = control.value === null || control.value === undefined ? '' : String(control.value);
      }
    }
    setAliasDrafts(nextAliasDrafts);
    setControlDrafts(nextControlDrafts);
    setHiddenControlsCollapsed(true);
    setAdvancedCommandCollapsed(true);
  }, [selectedDevice]);

  const visibleControls = useMemo(
    () => (deviceView?.controls ?? []).filter((control) => control.visible !== false),
    [deviceView],
  );
  const hiddenControls = useMemo(
    () => (deviceView?.controls ?? []).filter((control) => control.visible === false),
    [deviceView],
  );
  const defaultDeviceName = deviceView?.device.default_name ?? deviceView?.device.name ?? '';
  const hasSavedDeviceAlias = Boolean((deviceView?.device.alias ?? '').trim());

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
                <div>
                  <h3>{deviceView.device.name}</h3>
                  <p>{deviceView.device.id}</p>
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
                <div>
                  <label>Device Alias</label>
                  <p className="muted">Set a custom display name for this device without changing the upstream vendor record.</p>
                </div>
                <div className="grid grid--detail">
                  <div className="grid__full">
                    <Input
                      value={deviceAliasDraft}
                      onChange={(event) => setDeviceAliasDraft(event.target.value)}
                      placeholder={defaultDeviceName}
                    />
                  </div>
                </div>
                <div className="button-row">
                  <Button variant="secondary" onClick={() => onUpdateDevicePreference({ alias: deviceAliasDraft.trim() })}>
                    Save Alias
                  </Button>
                  <Button
                    variant="ghost"
                    onClick={() => {
                      setDeviceAliasDraft('');
                      onUpdateDevicePreference({ alias: '' });
                    }}
                    disabled={!hasSavedDeviceAlias && deviceAliasDraft.trim() === ''}
                  >
                    Reset Alias
                  </Button>
                </div>
                {hasSavedDeviceAlias ? <p className="muted">Default: {defaultDeviceName}</p> : null}
              </div>
              <div className="stack">
                <div>
                  <label>Quick Controls</label>
                  {visibleControls.length > 0 ? (
                    <div className="control-grid">
                      {visibleControls.map((control) => (
                        <DeviceControlCard
                          key={control.id}
                          deviceId={deviceView.device.id}
                          control={control}
                          busy={busy}
                          aliasValue={aliasDrafts[control.id] ?? ''}
                          valueDraft={controlDrafts[control.id] ?? ''}
                          togglePending={isToggleControlPending(selectedDevice, control.id, toggleOverrides)}
                          toggleDisabled={isToggleControlRequestPending(selectedDevice, control.id, togglePending)}
                          onAliasChange={(value) => setAliasDrafts((current) => ({ ...current, [control.id]: value }))}
                          onSavePreference={() =>
                            onUpdateControlPreference(control.id, { alias: (aliasDrafts[control.id] ?? '').trim(), visible: control.visible !== false })
                          }
                          onResetPreference={() => {
                            setAliasDrafts((current) => ({ ...current, [control.id]: '' }));
                            onUpdateControlPreference(control.id, { alias: '', visible: control.visible !== false });
                          }}
                          onToggleVisibility={() =>
                            onUpdateControlPreference(control.id, {
                              alias: (aliasDrafts[control.id] ?? '').trim(),
                              visible: control.visible === false,
                            })
                          }
                          onToggle={(on) => onToggleControl(control.id, on)}
                          onAction={() => onActionControl(control.id)}
                          onValueChange={(value) => setControlDrafts((current) => ({ ...current, [control.id]: value }))}
                          onValueControl={(value) => onValueControl(control.id, value)}
                        />
                      ))}
                    </div>
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
                    <div id="hidden-controls-panel" className="control-grid">
                      {hiddenControls.map((control) => (
                        <DeviceControlCard
                          key={control.id}
                          deviceId={deviceView.device.id}
                          control={control}
                          busy={busy}
                          aliasValue={aliasDrafts[control.id] ?? ''}
                          valueDraft={controlDrafts[control.id] ?? ''}
                          hidden
                          showControlBody={false}
                          togglePending={isToggleControlPending(selectedDevice, control.id, toggleOverrides)}
                          toggleDisabled={isToggleControlRequestPending(selectedDevice, control.id, togglePending)}
                          onAliasChange={(value) => setAliasDrafts((current) => ({ ...current, [control.id]: value }))}
                          onSavePreference={() =>
                            onUpdateControlPreference(control.id, { alias: (aliasDrafts[control.id] ?? '').trim(), visible: control.visible !== false })
                          }
                          onResetPreference={() => {
                            setAliasDrafts((current) => ({ ...current, [control.id]: '' }));
                            onUpdateControlPreference(control.id, { alias: '', visible: control.visible !== false });
                          }}
                          onToggleVisibility={() =>
                            onUpdateControlPreference(control.id, {
                              alias: (aliasDrafts[control.id] ?? '').trim(),
                              visible: control.visible === false,
                            })
                          }
                          onToggle={(on) => onToggleControl(control.id, on)}
                          onAction={() => onActionControl(control.id)}
                          onValueChange={(value) => setControlDrafts((current) => ({ ...current, [control.id]: value }))}
                          onValueControl={(value) => onValueControl(control.id, value)}
                        />
                      ))}
                    </div>
                  ) : null}
                  {hiddenControls.length > 0 && hiddenControlsCollapsed ? (
                    <p className="muted">Hidden quick controls are collapsed.</p>
                  ) : null}
                  {hiddenControls.length === 0 ? (
                    <p className="muted">No hidden quick controls.</p>
                  ) : null}
                </div>

                <div className="advanced-command">
                  <div className="advanced-command__header">
                    <div>
                      <label>Advanced Command</label>
                    </div>
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      className="collapse-toggle"
                      onClick={() => setAdvancedCommandCollapsed((current) => !current)}
                      aria-expanded={!advancedCommandCollapsed}
                      aria-controls="advanced-command-panel"
                    >
                      <span>{advancedCommandCollapsed ? 'Show' : 'Hide'}</span>
                      <ChevronIcon expanded={!advancedCommandCollapsed} />
                    </Button>
                  </div>
                  {!advancedCommandCollapsed ? (
                    <div id="advanced-command-panel" className="stack">
                      <p className="muted">
                        Use this only for vendor-specific operations or parameter tuning. Most day-to-day controls are wrapped
                        above. Click a preset below to prefill a known command shape before editing.
                      </p>
                      <div className="button-row">
                        {commandSuggestions.map((suggestion) => (
                          <Button
                            key={suggestion.label}
                            variant="secondary"
                            onClick={() => onApplySuggestion(suggestion.action, suggestion.params)}
                          >
                            Prefill {suggestion.label}
                          </Button>
                        ))}
                      </div>
                      <div className="grid grid--detail">
                        <div>
                          <label>Action</label>
                          <Input value={selectedAction} onChange={(event) => onSelectedActionChange(event.target.value)} />
                        </div>
                        <div>
                          <label>Actor</label>
                          <Input value={actor} onChange={(event) => onActorChange(event.target.value)} />
                        </div>
                        <div className="grid__full">
                          <label>Params JSON</label>
                          <Textarea rows={6} value={commandParams} onChange={(event) => onCommandParamsChange(event.target.value)} />
                        </div>
                      </div>
                      <div className="button-row">
                        <Button onClick={onSendCommand}>Send Advanced Command</Button>
                      </div>
                    </div>
                  ) : (
                    <p className="muted">Advanced command panel is collapsed.</p>
                  )}
                </div>
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
