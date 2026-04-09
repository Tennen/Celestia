import * as Collapsible from '@radix-ui/react-collapsible';
import { useEffect, useMemo, useState, type KeyboardEvent } from 'react';
import { Check, ChevronDown, PencilLine, RotateCcw } from 'lucide-react';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { Input } from '../ui/input';
import { ScrollArea } from '../ui/scroll-area';
import { Section } from '../ui/section';
import { asArray } from '../../lib/admin';
import { applyToggleOverrides } from '../../lib/control-state';
import { buildCommandSuggestions } from '../../lib/command-suggestions';
import type { DeviceView } from '../../lib/types';
import { useAdminStore } from '../../stores/adminStore';
import { useDeviceStore } from '../../stores/deviceStore';
import { DeviceAdvancedCommandSection } from './DeviceAdvancedCommandSection';
import { DeviceQuickControlsPanel } from './DeviceQuickControlsPanel';
import { StreamViewerPanel } from './StreamViewerPanel';
import { CardHeading } from './shared/CardHeading';
import { SelectableListItem } from './shared/SelectableListItem';

type DeviceEditorDraft = {
  deviceAlias: string;
  controlAliases: Record<string, string>;
  controlValues: Record<string, string>;
};

function buildDeviceEditorDraft(device: DeviceView): DeviceEditorDraft {
  const controlAliases: Record<string, string> = {};
  const controlValues: Record<string, string> = {};
  for (const control of device.controls ?? []) {
    controlAliases[control.id] = control.alias ?? '';
    if (control.kind === 'select' || control.kind === 'number') {
      controlValues[control.id] = control.value === null || control.value === undefined ? '' : String(control.value);
    }
  }
  return {
    deviceAlias: device.device.alias ?? '',
    controlAliases,
    controlValues,
  };
}

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
  const [summaryCollapsed, setSummaryCollapsed] = useState(true);
  const selectedDeviceKey = selectedDevice?.device.id ?? '';

  const displayDevice = useMemo(
    () => applyToggleOverrides(selectedDevice, toggleOverrides),
    [selectedDevice, toggleOverrides],
  );
  const deviceView = displayDevice ?? selectedDevice;

  useEffect(() => {
    if (!selectedDevice) {
      setDeviceAliasDraft('');
      setAliasDrafts({});
      setControlDrafts({});
      setEditingDeviceAlias(false);
      setSummaryCollapsed(true);
      return;
    }
    const draft = buildDeviceEditorDraft(selectedDevice);
    setDeviceAliasDraft(draft.deviceAlias);
    setAliasDrafts(draft.controlAliases);
    setControlDrafts(draft.controlValues);
    setEditingDeviceAlias(false);
    setSummaryCollapsed(true);
  }, [selectedDeviceKey]);

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
    if (event.key === 'Escape') {
      event.preventDefault();
      setDeviceAliasDraft(savedDeviceAlias);
      setEditingDeviceAlias(false);
    }
  };

  return (
    <Section stack={false} className="plugin-workspace xl:grid-cols-[minmax(0,360px)_minmax(0,1fr)]">
      <Card className="explorer-card">
        <CardContent className="explorer-card__content pt-6">
          <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto]">
            <Input
              value={deviceSearch}
              onChange={(e) => setDeviceSearch(e.target.value)}
              placeholder="Search devices"
            />
            <Button variant="secondary" onClick={() => void refreshAll()} className="sm:w-fit">
              Refresh
            </Button>
          </div>
          <ScrollArea className="explorer-scroll">
            <div className="list-stack pb-3">
              {devices.map((item) => (
                <SelectableListItem
                  key={item.device.id}
                  className={`table__row ${selectedDeviceId === item.device.id ? 'is-selected' : ''}`}
                  onClick={() => setSelectedDeviceId(item.device.id)}
                  selected={selectedDeviceId === item.device.id}
                  title={item.device.name}
                  description={item.device.id}
                  badges={
                    <Badge tone={item.device.online ? 'good' : 'bad'} size="sm">
                      {item.device.online ? 'online' : 'offline'}
                    </Badge>
                  }
                  support={
                    <>
                      <span>{item.device.kind}</span>
                      <span>{item.device.room || 'no room'}</span>
                    </>
                  }
                />
              ))}
            </div>
          </ScrollArea>
        </CardContent>
      </Card>

      <ScrollArea className="detail-scroll">
        <div className="detail-stack">
          {deviceView ? (
            <>
            <Card>
              <Collapsible.Root
                open={!summaryCollapsed}
                onOpenChange={(open) => setSummaryCollapsed(!open)}
              >
                <CardHeader>
                  <CardHeading
                    title="Device Summary"
                    description="Selected device identity, presence, stream capability, and unified labels."
                    aside={
                      <div className="device-summary__aside">
                        <Badge tone={deviceView.device.online ? 'good' : 'bad'} size="xs">
                          {deviceView.device.online ? 'online' : 'offline'}
                        </Badge>
                        <Collapsible.Trigger asChild>
                          <Button
                            type="button"
                            variant="ghost"
                            size="sm"
                            className="collapse-toggle"
                            aria-controls="device-summary-panel"
                          >
                            <span>{summaryCollapsed ? 'Show' : 'Hide'}</span>
                            <ChevronDown
                              className={`collapse-toggle__icon ${!summaryCollapsed ? 'is-expanded' : ''}`}
                            />
                          </Button>
                        </Collapsible.Trigger>
                      </div>
                    }
                  />
                </CardHeader>
                <Collapsible.Content id="device-summary-panel">
                  <CardContent className="stack">
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
                                <Check />
                              </Button>
                              <Button
                                type="button"
                                variant="ghost"
                                size="sm"
                                className="control-card__icon-button control-card__icon-button--reset"
                                onClick={resetDeviceAlias}
                                disabled={
                                  devicePrefBusy || (!hasSavedDeviceAlias && deviceAliasDraft.trim() === '')
                                }
                                aria-label={`Reset label for ${deviceView.device.name}`}
                                title="Reset label"
                              >
                                <RotateCcw />
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
                              <PencilLine />
                            </Button>
                          </div>
                        )}
                        <p>{deviceView.device.id}</p>
                        {hasSavedDeviceAlias ? <p>Default: {defaultDeviceName}</p> : null}
                      </div>
                    </div>
                    <div className="chip-list">
                      {asArray(deviceView.device.capabilities).map((capability) => (
                        <Badge key={capability} tone="neutral" size="xs">
                          {capability}
                        </Badge>
                      ))}
                    </div>
                    {deviceView.device.capabilities.includes('stream') ? (
                      <StreamViewerPanel deviceId={deviceView.device.id} />
                    ) : null}
                  </CardContent>
                </Collapsible.Content>
              </Collapsible.Root>
            </Card>

            <Card>
              <CardHeader>
                <CardHeading
                  title="Controls"
                  description="Quick controls and vendor-specific advanced commands for the selected device."
                />
              </CardHeader>
              <CardContent className="stack">
                <DeviceQuickControlsPanel
                  device={deviceView}
                  selectedDevice={selectedDevice}
                  toggleOverrides={toggleOverrides}
                  togglePending={togglePending}
                  busy={busy}
                  aliasDrafts={aliasDrafts}
                  controlDrafts={controlDrafts}
                  onAliasChange={(controlId, value) =>
                    setAliasDrafts((current) => ({ ...current, [controlId]: value }))
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
                    setAliasDrafts((current) => ({ ...current, [controlId]: '' }));
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
                    setControlDrafts((current) => ({ ...current, [controlId]: value }))
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
              </CardContent>
            </Card>

            {commandResult ? (
              <Card>
                <CardHeader>
                  <CardHeading
                    title="Last Command Result"
                    description="Latest response payload from the advanced command or quick control execution."
                  />
                </CardHeader>
                <CardContent>
                  <ScrollArea className="max-h-[220px]">
                    <pre className="log-box">{commandResult}</pre>
                  </ScrollArea>
                </CardContent>
              </Card>
            ) : null}

            <Card>
              <CardHeader>
                <CardHeading
                  title="Unified Device Payload"
                  description="Current unified device model and control payload as exposed to the admin UI."
                />
              </CardHeader>
              <CardContent>
                <ScrollArea className="max-h-[360px]">
                  <pre className="log-box">{selectedDeviceDetails}</pre>
                </ScrollArea>
                </CardContent>
              </Card>
            </>
          ) : (
            <Card>
              <CardHeader>
                <CardTitle>Device Detail</CardTitle>
                <CardDescription>Select a device from the list to inspect controls and runtime state.</CardDescription>
              </CardHeader>
              <CardContent>
                <p className="muted">No device selected.</p>
              </CardContent>
            </Card>
          )}
        </div>
      </ScrollArea>
    </Section>
  );
}
