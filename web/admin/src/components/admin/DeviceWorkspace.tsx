import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { Input } from '../ui/input';
import { Section } from '../ui/section';
import { Textarea } from '../ui/textarea';
import { asArray } from '../../lib/admin';
import type { DeviceControl, DeviceView } from '../../lib/types';

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
  commandResult,
  selectedDeviceDetails,
}: Props) {
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
                  {selectedDevice.controls.length > 0 ? (
                    <div className="control-grid">
                      {selectedDevice.controls.map((control) => (
                        <div key={control.id} className="control-card">
                          <div className="control-card__header">
                            <div>
                              <strong>{control.label}</strong>
                              {control.description ? <p>{control.description}</p> : null}
                            </div>
                            <Badge tone={control.kind === 'toggle' ? toggleTone(control) : 'accent'}>
                              {control.kind === 'toggle' ? toggleText(control) : 'action'}
                            </Badge>
                          </div>
                          {control.kind === 'toggle' ? (
                            <div className="button-row">
                              <Button
                                variant="secondary"
                                onClick={() => onToggleControl(control.id, true)}
                                disabled={busy === `toggle-${selectedDevice.device.id}.${control.id}-on`}
                              >
                                On
                              </Button>
                              <Button
                                variant="secondary"
                                onClick={() => onToggleControl(control.id, false)}
                                disabled={busy === `toggle-${selectedDevice.device.id}.${control.id}-off`}
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
                        </div>
                      ))}
                    </div>
                  ) : (
                    <p className="muted">No abstract quick controls are available for this device yet.</p>
                  )}
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
