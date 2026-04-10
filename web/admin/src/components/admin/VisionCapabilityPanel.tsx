import { useEffect, useMemo, useState } from 'react';
import { CircleAlert, Settings } from 'lucide-react';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardHeader } from '../ui/card';
import { ScrollArea } from '../ui/scroll-area';
import { fetchCapability, refreshVisionEntityCatalog, saveVisionCapabilityConfig } from '../../lib/api';
import {
  capabilityDisplayName,
  cloneVisionConfig,
  createVisionRule,
  defaultVisionConfig,
} from '../../lib/capability';
import { formatTime } from '../../lib/utils';
import { useAdminStore } from '../../stores/adminStore';
import type { CapabilityDetail, CapabilitySummary, DeviceView, VisionCapabilityConfig, VisionRule } from '../../lib/types';
import { useRetainedDraft } from '../../hooks/useRetainedDraft';
import { RecognitionSettingsModal } from './RecognitionSettingsModal';
import { SelectableListItem } from './shared/SelectableListItem';
import { VisionEventCaptureGallery, visionEventCapturesFromPayload } from './VisionEventCaptureGallery';
import { VisionRuleEditorCard } from './VisionRuleEditorCard';
import { CardHeading } from './shared/CardHeading';

type Props = {
  summary: CapabilitySummary | null;
  devices: DeviceView[];
  onError: (message: string) => void;
};

function ensureSelection(currentId: string, rules: VisionRule[]) {
  if (!rules.length) return '';
  if (rules.some((rule) => rule.id === currentId)) return currentId;
  return rules[0].id;
}

function normalizeVisionServiceURL(value: string) {
  return value.trim().replace(/\/+$/, '');
}

function recognitionRuleBadge(rule: VisionRule) {
  if (!rule.enabled) {
    return { label: 'disabled', tone: 'neutral' as const };
  }
  if (!rule.recognition_enabled) {
    return { label: 'paused', tone: 'warn' as const };
  }
  return { label: 'active', tone: 'accent' as const };
}

export function VisionCapabilityPanel({ summary, devices, onError }: Props) {
  const { refreshAll } = useAdminStore();
  const cameraDevices = useMemo(
    () => devices.filter((device) => device.device.kind === 'camera_like'),
    [devices],
  );
  const [detail, setDetail] = useState<CapabilityDetail | null>(null);
  const [selectedRuleId, setSelectedRuleId] = useState('');
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [busy, setBusy] = useState<'load' | 'save' | 'refresh_entities' | ''>('load');
  const {
    draft,
    replaceDraft,
    resetDraft,
    revision,
    setDraft,
  } = useRetainedDraft<VisionCapabilityConfig>({
    source: detail?.vision?.config ?? defaultVisionConfig(),
    sourceKey: summary?.id ?? 'vision_entity_stay_zone',
    clone: cloneVisionConfig,
  });

  useEffect(() => {
    if (summary?.id !== 'vision_entity_stay_zone') return;
    let cancelled = false;
    setBusy('load');
    void fetchCapability(summary.id)
      .then((nextDetail) => {
        if (cancelled) return;
        setDetail(nextDetail);
      })
      .catch((error) => {
        if (!cancelled) {
          onError(error instanceof Error ? error.message : 'Failed to load recognition capability');
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

  useEffect(() => {
    setSelectedRuleId((current) => ensureSelection(current, draft?.rules ?? []));
  }, [draft, revision]);

  const runtime = detail?.vision?.runtime;
  const catalog = detail?.vision?.catalog ?? null;
  const recentEvents = detail?.vision?.recent_events ?? [];
  const normalizedDraftServiceURL = normalizeVisionServiceURL(draft?.service_url ?? '');
  const catalogMatchesDraft = Boolean(
    catalog && normalizeVisionServiceURL(catalog.service_url) === normalizedDraftServiceURL,
  );
  const activeCatalog = catalogMatchesDraft ? catalog : null;
  const selectedRule = draft?.rules.find((rule) => rule.id === selectedRuleId) ?? null;
  const recognitionConfigured = Boolean(normalizedDraftServiceURL);

  const updateDraft = (updater: (current: VisionCapabilityConfig) => VisionCapabilityConfig) => {
    setDraft(updater);
  };

  const addRule = () => {
    if (!draft) {
      return;
    }
    const rule = createVisionRule(cameraDevices, draft.rules.length);
    const preferredEntity =
      activeCatalog?.entities.find((entity) => entity.kind === 'label' && entity.value === 'cat') ?? activeCatalog?.entities[0];
    if (preferredEntity) {
      rule.entity_selector = {
        kind: preferredEntity.kind,
        value: preferredEntity.value,
      };
    }
    updateDraft((current) => ({
      ...current,
      rules: [...current.rules, rule],
    }));
    setSelectedRuleId(rule.id);
  };

  const removeRule = (ruleId: string) => {
    if (!draft) {
      return;
    }
    const nextRules = draft.rules.filter((rule) => rule.id !== ruleId);
    updateDraft((current) => ({
      ...current,
      rules: nextRules,
    }));
    setSelectedRuleId(ensureSelection(selectedRuleId === ruleId ? '' : selectedRuleId, nextRules));
  };

  const handleRefreshEntities = async () => {
    setBusy('refresh_entities');
    try {
      const nextCatalog = await refreshVisionEntityCatalog({
        service_url: draft?.service_url || detail?.vision?.config.service_url || undefined,
      });
      setDetail((current) =>
        current && current.vision
          ? {
              ...current,
              vision: {
                ...current.vision,
                catalog: nextCatalog,
              },
            }
          : current,
      );
      await refreshAll();
    } catch (error) {
      onError(error instanceof Error ? error.message : 'Failed to refresh supported entities');
    } finally {
      setBusy('');
    }
  };

  const handleSave = async () => {
    if (!draft) {
      return;
    }
    setBusy('save');
    try {
      const saved = await saveVisionCapabilityConfig(draft);
      const nextDraft = cloneVisionConfig(saved.vision?.config ?? draft);
      setDetail(saved);
      replaceDraft(nextDraft, summary?.id ?? 'vision_entity_stay_zone');
      await refreshAll();
    } catch (error) {
      onError(error instanceof Error ? error.message : 'Failed to save recognition capability');
    } finally {
      setBusy('');
    }
  };

  const handleReset = () => {
    resetDraft();
  };

  return (
    <>
      <div className="capability-detail vision-capability-panel">
        <div className="vision-rule-layout vision-capability-panel__body">
          <Card className="explorer-card vision-rule-layout__list">
            <CardContent className="explorer-card__content pt-6">
              <div className="vision-toolbar">
                <div className="button-row">
                  <Button onClick={addRule}>New Rule</Button>
                  <Button onClick={() => void handleSave()} disabled={busy === 'save' || busy === 'load'}>
                    {busy === 'save' ? 'Saving…' : 'Save Changes'}
                  </Button>
                  <Button
                    type="button"
                    variant="secondary"
                    onClick={() => setSettingsOpen(true)}
                    aria-label="Open recognition settings"
                    title="Recognition settings"
                  >
                    <Settings className="h-4 w-4" />
                    <span>Recognition Settings</span>
                  </Button>
                </div>
                <div className="vision-toolbar__actions">
                  <Badge tone={summary?.status === 'healthy' ? 'good' : summary?.status === 'stopped' ? 'neutral' : 'warn'} size="xs">
                    {summary?.status ?? 'unknown'}
                  </Badge>
                  {!recognitionConfigured ? (
                    <span
                      className="vision-toolbar__alert"
                      aria-label="Recognition service not configured"
                      title="Recognition service not configured"
                    >
                      <CircleAlert className="h-4 w-4" />
                    </span>
                  ) : null}
                </div>
              </div>
              <p className="muted vision-toolbar__support">
                {capabilityDisplayName(summary)} · {normalizedDraftServiceURL || 'Recognition service not configured'}
              </p>
              <ScrollArea className="explorer-scroll">
                <div className="list-stack">
                  {draft?.rules.map((rule) => {
                    const statusBadge = recognitionRuleBadge(rule);
                    return (
                      <SelectableListItem
                        key={rule.id}
                        layout="stacked_badges"
                        selected={rule.id === selectedRuleId}
                        onClick={() => setSelectedRuleId(rule.id)}
                        title={rule.name || rule.id}
                        description={rule.camera_device_id || 'No camera selected'}
                        badges={
                          <Badge size="xxs" tone={statusBadge.tone}>
                            {statusBadge.label}
                          </Badge>
                        }
                      />
                    );
                  })}
                  {draft?.rules.length === 0 ? <div className="detail">No recognition rules configured yet.</div> : null}
                </div>
              </ScrollArea>
            </CardContent>
          </Card>

          <ScrollArea className="detail-scroll vision-rule-layout__detail">
            <div className="detail-stack">
              <VisionRuleEditorCard
                catalog={activeCatalog}
                catalogMismatch={Boolean(catalog && !catalogMatchesDraft)}
                cameraDevices={cameraDevices}
                loading={busy === 'load'}
                onRemoveRule={removeRule}
                onSelectRuleId={setSelectedRuleId}
                onUpdateRule={(ruleId, updater) =>
                  updateDraft((current) => ({
                    ...current,
                    rules: current.rules.map((rule) => (rule.id === ruleId ? updater({ ...rule }) : rule)),
                  }))
                }
                selectedRule={selectedRule}
              />

              <Card>
                <CardHeader>
                  <CardHeading
                    title="Recent Recognition Events"
                    description="Structured events reported by the external recognition service and attached to the Core event bus."
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
                          <strong>{String(event.payload?.rule_name ?? event.device_id ?? 'recognition event')}</strong>
                          <p className="muted">
                            {String(event.payload?.entity_value ?? 'entity')} · dwell {String(event.payload?.dwell_seconds ?? 0)}s
                          </p>
                          <VisionEventCaptureGallery captures={visionEventCapturesFromPayload(event.payload)} />
                        </article>
                      ))
                    ) : (
                      <p className="muted">No recognition events received yet.</p>
                    )}
                  </div>
                </CardContent>
              </Card>
            </div>
          </ScrollArea>
        </div>
      </div>

      {draft ? (
        <RecognitionSettingsModal
          busy={busy}
          catalog={catalog}
          catalogMatchesDraft={catalogMatchesDraft}
          draft={draft}
          normalizedDraftServiceURL={normalizedDraftServiceURL}
          onOpenChange={setSettingsOpen}
          onRefreshEntities={() => void handleRefreshEntities()}
          onResetDraft={handleReset}
          onSave={() => void handleSave()}
          onUpdateDraft={updateDraft}
          open={settingsOpen}
          runtime={runtime}
          status={summary?.status ?? 'unknown'}
        />
      ) : null}
    </>
  );
}
