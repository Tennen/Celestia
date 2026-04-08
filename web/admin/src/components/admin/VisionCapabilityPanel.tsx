import { useEffect, useMemo, useState } from 'react';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardHeader } from '../ui/card';
import { Input } from '../ui/input';
import { ScrollArea } from '../ui/scroll-area';
import { Switch } from '../ui/switch';
import { fetchCapability, saveVisionCapabilityConfig } from '../../lib/api';
import { cameraLabel, cloneVisionConfig, createVisionRule, defaultVisionConfig } from '../../lib/capability';
import { formatTime, prettyJson } from '../../lib/utils';
import { useAdminStore } from '../../stores/adminStore';
import type { CapabilityDetail, CapabilitySummary, DeviceView, HealthState, VisionCapabilityConfig, VisionRule } from '../../lib/types';
import { CardHeading } from './shared/CardHeading';
import { SelectableListItem } from './shared/SelectableListItem';
import { ZoneBoxEditor } from './ZoneBoxEditor';

type Props = {
  summary: CapabilitySummary | null;
  devices: DeviceView[];
  onError: (message: string) => void;
};

function toneFromHealth(status: HealthState) {
  if (status === 'healthy') return 'good' as const;
  if (status === 'degraded' || status === 'unknown') return 'warn' as const;
  if (status === 'stopped') return 'neutral' as const;
  return 'bad' as const;
}

function ensureSelection(currentId: string, rules: VisionRule[]) {
  if (!rules.length) return '';
  if (rules.some((rule) => rule.id === currentId)) return currentId;
  return rules[0].id;
}

export function VisionCapabilityPanel({ summary, devices, onError }: Props) {
  const { refreshAll } = useAdminStore();
  const cameraDevices = useMemo(
    () => devices.filter((device) => device.device.kind === 'camera_like'),
    [devices],
  );
  const [detail, setDetail] = useState<CapabilityDetail | null>(null);
  const [draft, setDraft] = useState<VisionCapabilityConfig>(() => defaultVisionConfig());
  const [selectedRuleId, setSelectedRuleId] = useState('');
  const [busy, setBusy] = useState<'load' | 'save' | ''>('load');

  useEffect(() => {
    if (summary?.id !== 'vision_entity_stay_zone') return;
    let cancelled = false;
    setBusy('load');
    void fetchCapability(summary.id)
      .then((nextDetail) => {
        if (cancelled) return;
        const nextDraft = cloneVisionConfig(nextDetail.vision?.config ?? defaultVisionConfig());
        setDetail(nextDetail);
        setDraft(nextDraft);
        setSelectedRuleId((current) => ensureSelection(current, nextDraft.rules));
      })
      .catch((error) => {
        if (!cancelled) {
          onError(error instanceof Error ? error.message : 'Failed to load vision capability');
        }
      })
      .finally(() => {
        if (!cancelled) {
          setBusy('');
        }
      });
    return () => {
      cancelled = true;
    };
  }, [summary?.id, summary?.updated_at, onError]);

  const runtime = detail?.vision?.runtime;
  const recentEvents = detail?.vision?.recent_events ?? [];
  const selectedRule = draft.rules.find((rule) => rule.id === selectedRuleId) ?? null;

  const updateDraft = (updater: (current: VisionCapabilityConfig) => VisionCapabilityConfig) => {
    setDraft((current) => updater(cloneVisionConfig(current)));
  };

  const updateRule = (ruleId: string, updater: (current: VisionRule) => VisionRule) => {
    updateDraft((current) => ({
      ...current,
      rules: current.rules.map((rule) => (rule.id === ruleId ? updater({ ...rule }) : rule)),
    }));
  };

  const addRule = () => {
    const rule = createVisionRule(cameraDevices, draft.rules.length);
    updateDraft((current) => ({
      ...current,
      rules: [...current.rules, rule],
    }));
    setSelectedRuleId(rule.id);
  };

  const removeRule = (ruleId: string) => {
    const nextRules = draft.rules.filter((rule) => rule.id !== ruleId);
    updateDraft((current) => ({
      ...current,
      rules: nextRules,
    }));
    setSelectedRuleId(ensureSelection(selectedRuleId === ruleId ? '' : selectedRuleId, nextRules));
  };

  const handleSave = async () => {
    setBusy('save');
    try {
      const saved = await saveVisionCapabilityConfig(draft);
      const nextDraft = cloneVisionConfig(saved.vision?.config ?? draft);
      setDetail(saved);
      setDraft(nextDraft);
      setSelectedRuleId((current) => ensureSelection(current, nextDraft.rules));
      await refreshAll();
    } catch (error) {
      onError(error instanceof Error ? error.message : 'Failed to save vision capability');
    } finally {
      setBusy('');
    }
  };

  return (
    <div className="capability-detail">
      <Card>
        <CardHeader>
          <CardHeading
            title="Vision Stay Zone Recognition"
            description="Gateway manages vision control-plane config only: service address, camera and RTSP bindings, entity selectors, zones, thresholds, and runtime status."
            aside={
              <div className="automation-editor__meta">
                <Badge tone={toneFromHealth(summary?.status ?? 'unknown')}>{summary?.status ?? 'unknown'}</Badge>
                <Switch
                  checked={draft.recognition_enabled}
                  onCheckedChange={(checked) =>
                    updateDraft((current) => ({ ...current, recognition_enabled: checked }))
                  }
                  aria-label="Toggle vision recognition"
                />
              </div>
            }
          />
        </CardHeader>
        <CardContent className="stack">
          <div className="automation-field">
            <label>Vision Service Address</label>
            <Input
              value={draft.service_url}
              onChange={(event) => updateDraft((current) => ({ ...current, service_url: event.target.value }))}
              placeholder="http://127.0.0.1:8090"
            />
          </div>

          <div className="vision-runtime-grid">
            <div className="kv">
              <span>Runtime Status</span>
              <strong>{runtime?.status ?? 'unknown'}</strong>
            </div>
            <div className="kv">
              <span>Service Version</span>
              <strong>{runtime?.service_version || 'unknown'}</strong>
            </div>
            <div className="kv">
              <span>Last Sync</span>
              <strong>{runtime?.last_synced_at ? formatTime(runtime.last_synced_at) : 'never'}</strong>
            </div>
            <div className="kv">
              <span>Last Event</span>
              <strong>{runtime?.last_event_at ? formatTime(runtime.last_event_at) : 'none yet'}</strong>
            </div>
          </div>

          {runtime?.message ? <p className="muted">Status: {runtime.message}</p> : null}
          {runtime?.sync_error ? <p className="muted">Sync error: {runtime.sync_error}</p> : null}

          {runtime?.runtime && Object.keys(runtime.runtime).length > 0 ? (
            <div className="automation-field">
              <label>Reported Runtime Payload</label>
              <pre className="plugin-config__preview">{prettyJson(runtime.runtime)}</pre>
            </div>
          ) : null}

          <div className="button-row">
            <Button onClick={() => void handleSave()} disabled={busy === 'save'}>
              Save Vision Capability
            </Button>
            <Button
              variant="secondary"
              onClick={() => {
                const nextDraft = cloneVisionConfig(detail?.vision?.config ?? defaultVisionConfig());
                setDraft(nextDraft);
                setSelectedRuleId(ensureSelection(selectedRuleId, nextDraft.rules));
              }}
            >
              Reset Draft
            </Button>
          </div>
        </CardContent>
      </Card>

      <div className="vision-rule-layout">
        <Card className="explorer-card">
          <CardContent className="explorer-card__content pt-6">
            <div className="button-row">
              <Button onClick={addRule}>New Rule</Button>
            </div>
            <ScrollArea className="explorer-scroll">
              <div className="list-stack">
                {draft.rules.map((rule) => (
                  <SelectableListItem
                    key={rule.id}
                    selected={rule.id === selectedRuleId}
                    onClick={() => setSelectedRuleId(rule.id)}
                    title={rule.name || rule.id}
                    description={rule.camera_device_id || 'No camera selected'}
                    badges={
                      <>
                        <Badge size="xs" tone={rule.enabled ? 'good' : 'neutral'}>
                          {rule.enabled ? 'enabled' : 'disabled'}
                        </Badge>
                        <Badge size="xs" tone={rule.recognition_enabled ? 'accent' : 'neutral'}>
                          {rule.recognition_enabled ? 'recognition on' : 'recognition off'}
                        </Badge>
                      </>
                    }
                  />
                ))}
                {draft.rules.length === 0 ? <div className="detail">No vision rules configured yet.</div> : null}
              </div>
            </ScrollArea>
          </CardContent>
        </Card>

        <ScrollArea className="detail-scroll">
          <div className="detail-stack">
            <Card>
              <CardHeader>
                <CardHeading
                  title={selectedRule ? selectedRule.name || 'Rule Editor' : 'Rule Editor'}
                  description="Bind a camera and RTSP source to a generic entity stay-zone rule. Gateway persists this config and pushes a normalized copy to the external Vision Service."
                  aside={
                    selectedRule ? (
                      <div className="automation-editor__meta">
                        <Badge size="xs" tone={selectedRule.enabled ? 'good' : 'neutral'}>
                          {selectedRule.enabled ? 'enabled' : 'disabled'}
                        </Badge>
                        <Button variant="danger" size="sm" onClick={() => removeRule(selectedRule.id)}>
                          Delete Rule
                        </Button>
                      </div>
                    ) : null
                  }
                />
              </CardHeader>
              <CardContent className="stack">
                {selectedRule ? (
                  <>
                    <div className="automation-field-grid">
                      <div className="automation-field">
                        <label>Name</label>
                        <Input
                          value={selectedRule.name}
                          onChange={(event) =>
                            updateRule(selectedRule.id, (current) => ({ ...current, name: event.target.value }))
                          }
                          placeholder="Feeder Zone Stay"
                        />
                      </div>
                      <div className="automation-field">
                        <label>Rule ID</label>
                        <Input
                          value={selectedRule.id}
                          onChange={(event) => {
                            const nextId = event.target.value;
                            updateRule(selectedRule.id, (current) => ({ ...current, id: nextId }));
                            setSelectedRuleId(nextId);
                          }}
                          placeholder="feeder-zone-stay"
                        />
                      </div>
                    </div>

                    <div className="automation-field-grid">
                      <div className="automation-field">
                        <label>Camera Device</label>
                        <select
                          className="select"
                          value={selectedRule.camera_device_id}
                          onChange={(event) =>
                            updateRule(selectedRule.id, (current) => ({
                              ...current,
                              camera_device_id: event.target.value,
                            }))
                          }
                        >
                          {cameraDevices.map((device) => (
                            <option key={device.device.id} value={device.device.id}>
                              {cameraLabel(device)}
                            </option>
                          ))}
                        </select>
                      </div>
                      <div className="automation-field">
                        <label>Stay Threshold Seconds</label>
                        <Input
                          type="number"
                          min={1}
                          step={1}
                          value={selectedRule.stay_threshold_seconds}
                          onChange={(event) =>
                            updateRule(selectedRule.id, (current) => ({
                              ...current,
                              stay_threshold_seconds: Math.max(1, Number(event.target.value) || 1),
                            }))
                          }
                        />
                      </div>
                    </div>

                    <div className="automation-field-grid">
                      <div className="automation-field">
                        <label>Entity Selector Kind</label>
                        <Input
                          value={selectedRule.entity_selector.kind}
                          onChange={(event) =>
                            updateRule(selectedRule.id, (current) => ({
                              ...current,
                              entity_selector: { ...current.entity_selector, kind: event.target.value },
                            }))
                          }
                          placeholder="label"
                        />
                      </div>
                      <div className="automation-field">
                        <label>Entity Selector Value</label>
                        <Input
                          value={selectedRule.entity_selector.value}
                          onChange={(event) =>
                            updateRule(selectedRule.id, (current) => ({
                              ...current,
                              entity_selector: { ...current.entity_selector, value: event.target.value },
                            }))
                          }
                          placeholder="cat"
                        />
                      </div>
                    </div>

                    <div className="automation-field">
                      <label>RTSP Source URL</label>
                      <Input
                        value={selectedRule.rtsp_source.url}
                        onChange={(event) =>
                          updateRule(selectedRule.id, (current) => ({
                            ...current,
                            rtsp_source: { ...current.rtsp_source, url: event.target.value },
                          }))
                        }
                        placeholder="rtsp://user:password@camera-host:554/stream"
                      />
                    </div>

                    <div className="automation-field-grid automation-field-grid--compact">
                      <div className="automation-field">
                        <label>Rule Enabled</label>
                        <div className="vision-switch-row">
                          <Switch
                            checked={selectedRule.enabled}
                            onCheckedChange={(checked) =>
                              updateRule(selectedRule.id, (current) => ({ ...current, enabled: checked }))
                            }
                          />
                          <span>{selectedRule.enabled ? 'Enabled' : 'Disabled'}</span>
                        </div>
                      </div>
                      <div className="automation-field">
                        <label>Recognition Toggle</label>
                        <div className="vision-switch-row">
                          <Switch
                            checked={selectedRule.recognition_enabled}
                            onCheckedChange={(checked) =>
                              updateRule(selectedRule.id, (current) => ({
                                ...current,
                                recognition_enabled: checked,
                              }))
                            }
                          />
                          <span>{selectedRule.recognition_enabled ? 'Vision active' : 'Vision paused'}</span>
                        </div>
                      </div>
                    </div>

                    <div className="automation-field">
                      <label>Zone Selection</label>
                      <ZoneBoxEditor
                        value={selectedRule.zone}
                        onChange={(zone) => updateRule(selectedRule.id, (current) => ({ ...current, zone }))}
                      />
                    </div>
                  </>
                ) : (
                  <p className="muted">
                    {busy === 'load' ? 'Loading vision capability…' : 'Select or create a rule to edit it.'}
                  </p>
                )}
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardHeading
                  title="Recent Vision Events"
                  description="Structured events reported by the external Vision Service and already attached to the Core event bus."
                />
              </CardHeader>
              <CardContent className="stack">
                <div className="vision-event-feed">
                  {recentEvents.length > 0 ? (
                    recentEvents.map((event) => (
                      <article key={event.id} className="vision-event-card">
                        <div className="feed__meta">
                          <Badge tone="accent" size="sm">
                            {String(event.payload?.event_status ?? event.type)}
                          </Badge>
                          <span>{formatTime(event.ts)}</span>
                        </div>
                        <strong>{String(event.payload?.rule_name ?? event.device_id ?? 'vision event')}</strong>
                        <p className="muted">
                          {String(event.payload?.entity_value ?? 'entity')} · dwell {String(event.payload?.dwell_seconds ?? 0)}s
                        </p>
                      </article>
                    ))
                  ) : (
                    <p className="muted">No vision events received yet.</p>
                  )}
                </div>
              </CardContent>
            </Card>
          </div>
        </ScrollArea>
      </div>
    </div>
  );
}
