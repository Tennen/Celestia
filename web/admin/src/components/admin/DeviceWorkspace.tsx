import { useEffect, useMemo, useState } from 'react';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { Input } from '../ui/input';
import { Section } from '../ui/section';
import { Textarea } from '../ui/textarea';
import { asArray } from '../../lib/admin';
import type { DeviceControl, DeviceView } from '../../lib/types';
import { cn } from '../../lib/utils';

type Props = {
  deviceSearch: string;
  onDeviceSearchChange: (value: string) => void;
  onRefresh: () => void;
  devices: DeviceView[];
  selectedDeviceId: string;
  onSelectDevice: (deviceId: string) => void;
  selectedDevice: DeviceView | null;
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
  onUpdateControlPreference: (controlId: string, payload: { alias?: string; visible: boolean }) => void;
  commandResult: string;
  selectedDeviceDetails: string;
};

function toggleTone(control: DeviceControl) {
  if (control.state === true) return 'good';
  if (control.state === false) return 'neutral';
  return 'warn';
}

function toggleText(control: DeviceControl) {
  if (control.state === true) return 'on';
  if (control.state === false) return 'off';
  return 'unknown';
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
  onUpdateControlPreference,
  commandResult,
  selectedDeviceDetails,
}: Props) {
  const [aliasDrafts, setAliasDrafts] = useState<Record<string, string>>({});
  const [hiddenControlsCollapsed, setHiddenControlsCollapsed] = useState(true);

  useEffect(() => {
    if (!selectedDevice) {
      setAliasDrafts({});
      setHiddenControlsCollapsed(true);
      return;
    }
    const next: Record<string, string> = {};
    for (const control of selectedDevice.controls) {
      next[control.id] = control.alias ?? '';
    }
    setAliasDrafts(next);
    setHiddenControlsCollapsed(true);
  }, [selectedDevice]);

  const visibleControls = useMemo(
    () => selectedDevice?.controls.filter((control) => control.visible !== false) ?? [],
    [selectedDevice],
  );
  const hiddenControls = useMemo(
    () => selectedDevice?.controls.filter((control) => control.visible === false) ?? [],
    [selectedDevice],
  );

  const renderPreferenceEditor = (control: DeviceControl) => {
    if (!selectedDevice) return null;
    const prefBusy = busy === `control-pref-${selectedDevice.device.id}.${control.id}`;
    const aliasValue = aliasDrafts[control.id] ?? '';
    const defaultLabel = control.default_label ?? control.label;
    return (
      <div className="control-card__editor">
        <Input
          value={aliasValue}
          onChange={(event) => setAliasDrafts((current) => ({ ...current, [control.id]: event.target.value }))}
          placeholder={defaultLabel}
        />
        <div className="button-row">
          <Button
            variant="secondary"
            onClick={() => onUpdateControlPreference(control.id, { alias: aliasValue.trim(), visible: control.visible !== false })}
            disabled={prefBusy}
          >
            Save Label
          </Button>
          <Button
            variant="secondary"
            onClick={() => {
              setAliasDrafts((current) => ({ ...current, [control.id]: '' }));
              onUpdateControlPreference(control.id, { alias: '', visible: control.visible !== false });
            }}
            disabled={prefBusy}
          >
            Reset Label
          </Button>
        </div>
      </div>
    );
  };

  const renderVisibilityButton = (control: DeviceControl) => {
    if (!selectedDevice) return null;
    const prefBusy = busy === `control-pref-${selectedDevice.device.id}.${control.id}`;
    const isVisible = control.visible !== false;
    return (
      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="control-card__visibility"
        onClick={() => onUpdateControlPreference(control.id, { alias: (aliasDrafts[control.id] ?? '').trim(), visible: !isVisible })}
        disabled={prefBusy}
        aria-label={isVisible ? `Hide ${control.label}` : `Show ${control.label}`}
        title={isVisible ? 'Hide control' : 'Show control'}
      >
        <VisibilityIcon visible={isVisible} />
      </Button>
    );
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
          {selectedDevice ? (
            <div className="detail">
              <div className="detail__header">
                <div>
                  <h3>{selectedDevice.device.name}</h3>
                  <p>{selectedDevice.device.id}</p>
                </div>
                <Badge tone={selectedDevice.device.online ? 'good' : 'bad'}>
                  {selectedDevice.device.online ? 'online' : 'offline'}
                </Badge>
              </div>
              <div className="chip-list">
                {asArray(selectedDevice.device.capabilities).map((capability) => (
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
                        <div key={control.id} className="control-card">
                          <div className="control-card__header">
                            <div>
                              <strong>{control.label}</strong>
                              {control.alias && control.default_label ? <p>Default: {control.default_label}</p> : null}
                              {control.description ? <p>{control.description}</p> : null}
                            </div>
                            <div className="control-card__meta">
                              <Badge tone={control.kind === 'toggle' ? toggleTone(control) : 'accent'}>
                                {control.kind === 'toggle' ? toggleText(control) : 'action'}
                              </Badge>
                              {renderVisibilityButton(control)}
                            </div>
                          </div>
                          {control.kind === 'toggle' ? (
                            <div className="control-toggle" role="group" aria-label={`${control.label} toggle`}>
                              <Button
                                type="button"
                                variant="ghost"
                                className={cn('control-toggle__option', control.state === true && 'is-active', 'control-toggle__option--on')}
                                onClick={() => onToggleControl(control.id, true)}
                                disabled={
                                  busy === `toggle-${selectedDevice.device.id}.${control.id}-on` ||
                                  busy === `toggle-${selectedDevice.device.id}.${control.id}-off`
                                }
                                aria-pressed={control.state === true}
                              >
                                On
                              </Button>
                              <Button
                                type="button"
                                variant="ghost"
                                className={cn('control-toggle__option', control.state === false && 'is-active', 'control-toggle__option--off')}
                                onClick={() => onToggleControl(control.id, false)}
                                disabled={
                                  busy === `toggle-${selectedDevice.device.id}.${control.id}-on` ||
                                  busy === `toggle-${selectedDevice.device.id}.${control.id}-off`
                                }
                                aria-pressed={control.state === false}
                              >
                                Off
                              </Button>
                            </div>
                          ) : (
                            <div className="button-row">
                              <Button
                                variant="secondary"
                                onClick={() => onActionControl(control.id)}
                                disabled={busy === `action-${selectedDevice.device.id}.${control.id}`}
                              >
                                Run
                              </Button>
                            </div>
                          )}
                          {renderPreferenceEditor(control)}
                        </div>
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
                        <div key={control.id} className="control-card control-card--hidden">
                          <div className="control-card__header">
                            <div>
                              <strong>{control.label}</strong>
                              {control.alias && control.default_label ? <p>Default: {control.default_label}</p> : null}
                              {control.description ? <p>{control.description}</p> : null}
                            </div>
                            <div className="control-card__meta">
                              <Badge tone="neutral">hidden</Badge>
                              {renderVisibilityButton(control)}
                            </div>
                          </div>
                          {renderPreferenceEditor(control)}
                        </div>
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
