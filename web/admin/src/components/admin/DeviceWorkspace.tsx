import { useEffect, useMemo, useState } from 'react';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { Input } from '../ui/input';
import { Section } from '../ui/section';
import { Textarea } from '../ui/textarea';
import { asArray } from '../../lib/admin';
import { applyToggleOverrides, isToggleControlPending, type ToggleControlOverrideMap } from '../../lib/control-state';
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
  onUpdateControlPreference: (controlId: string, payload: { alias?: string; visible: boolean }) => void;
  commandResult: string;
  selectedDeviceDetails: string;
};

function ChevronIcon({ expanded }: { expanded: boolean }) {
  return (
    <svg
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
    </svg>
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
  onUpdateControlPreference,
  commandResult,
  selectedDeviceDetails,
}: Props) {
  const [aliasDrafts, setAliasDrafts] = useState<Record<string, string>>({});
  const [controlDrafts, setControlDrafts] = useState<Record<string, string>>({});
  const [hiddenControlsCollapsed, setHiddenControlsCollapsed] = useState(true);
  const displayDevice = useMemo(() => applyToggleOverrides(selectedDevice, toggleOverrides), [selectedDevice, toggleOverrides]);
  const deviceView = displayDevice ?? selectedDevice;

  useEffect(() => {
    if (!selectedDevice) {
      setAliasDrafts({});
      setControlDrafts({});
      setHiddenControlsCollapsed(true);
      return;
    }
    const nextAliasDrafts: Record<string, string> = {};
    const nextControlDrafts: Record<string, string> = {};
    for (const control of selectedDevice.controls ?? []) {
      nextAliasDrafts[control.id] = control.alias ?? '';
      if (control.kind === 'select' || control.kind === 'number') {
        nextControlDrafts[control.id] = control.value === null || control.value === undefined ? '' : String(control.value);
      }
    }
    setAliasDrafts(nextAliasDrafts);
    setControlDrafts(nextControlDrafts);
    setHiddenControlsCollapsed(true);
  }, [selectedDevice]);

  const visibleControls = useMemo(
    () => (deviceView?.controls ?? []).filter((control) => control.visible !== false),
    [deviceView],
  );
  const hiddenControls = useMemo(
    () => (deviceView?.controls ?? []).filter((control) => control.visible === false),
    [deviceView],
  );

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
                      <p className="muted">
                        Use this only for vendor-specific operations or parameter tuning. Most day-to-day controls are wrapped
                        above. Click a preset below to prefill a known command shape before editing.
                      </p>
                    </div>
                  </div>
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
