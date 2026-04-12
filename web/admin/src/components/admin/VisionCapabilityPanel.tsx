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
import { useAdminStore } from '../../stores/adminStore';
import type {
  CapabilityDetail,
  CapabilitySummary,
  DeviceView,
  VisionCapabilityConfig,
  VisionEntityCatalog,
  VisionRule,
} from '../../lib/types';
import { useRetainedDraft } from '../../hooks/useRetainedDraft';
import { RecognitionSettingsModal } from './RecognitionSettingsModal';
import { SelectableListItem } from './shared/SelectableListItem';
import { VisionRuleEventHistoryPanel } from './VisionRuleEventHistoryPanel';
import { VisionRuleEditorCard } from './VisionRuleEditorCard';

type VisionRulesDraft = {
  rules: VisionRule[];
};

type RecognitionSettingsDraft = {
  event_capture_retention_hours: number;
  recognition_enabled: boolean;
  service_ws_url: string;
  model_name: string;
};

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

function normalizeVisionServiceWSURL(value: string) {
  return value.trim().replace(/\/+$/, '');
}

function normalizeVisionModelName(value: string) {
  return value.trim();
}

function catalogMatchesSelection(
  catalog: VisionEntityCatalog | null,
  serviceWSURL: string,
  modelName: string,
) {
  if (!catalog) return false;
  if (normalizeVisionServiceWSURL(catalog.service_ws_url) !== normalizeVisionServiceWSURL(serviceWSURL)) {
    return false;
  }
  const normalizedModelName = normalizeVisionModelName(modelName);
  if (!normalizedModelName) {
    return true;
  }
  return normalizeVisionModelName(catalog.model_name ?? '') === normalizedModelName;
}

function cloneDraft<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}

function settingsDraftFromConfig(config: VisionCapabilityConfig): RecognitionSettingsDraft {
  return {
    service_ws_url: config.service_ws_url,
    model_name: config.model_name ?? '',
    recognition_enabled: config.recognition_enabled,
    event_capture_retention_hours: config.event_capture_retention_hours,
  };
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
  const [detailView, setDetailView] = useState<'editor' | 'history'>('editor');
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [busy, setBusy] = useState<'load' | 'save_rule' | 'save_settings' | 'refresh_entities' | ''>('load');
  const ruleDraftState = useRetainedDraft<VisionRulesDraft>({
    source: {
      rules: detail?.vision?.config.rules ?? defaultVisionConfig().rules,
    },
    sourceKey: summary?.id ?? 'vision_entity_stay_zone',
    clone: cloneDraft,
  });
  const settingsDraftState = useRetainedDraft<RecognitionSettingsDraft>({
    source: settingsDraftFromConfig(detail?.vision?.config ?? defaultVisionConfig()),
    sourceKey: summary?.id ?? 'vision_entity_stay_zone',
    clone: cloneDraft,
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
    setSelectedRuleId((current) => ensureSelection(current, ruleDraftState.draft?.rules ?? []));
  }, [ruleDraftState.draft, ruleDraftState.revision]);

  const runtime = detail?.vision?.runtime;
  const catalog = detail?.vision?.catalog ?? null;
  const persistedConfig = detail?.vision?.config ?? defaultVisionConfig();
  const settingsDraft = settingsDraftState.draft ?? settingsDraftFromConfig(persistedConfig);
  const ruleDraft = ruleDraftState.draft ?? { rules: persistedConfig.rules };
  const normalizedSavedServiceWSURL = normalizeVisionServiceWSURL(persistedConfig.service_ws_url);
  const normalizedDraftServiceWSURL = normalizeVisionServiceWSURL(settingsDraft.service_ws_url);
  const catalogMatchesDraft = catalogMatchesSelection(catalog, settingsDraft.service_ws_url, settingsDraft.model_name);
  const catalogMatchesSaved = catalogMatchesSelection(catalog, persistedConfig.service_ws_url, persistedConfig.model_name ?? '');
  const selectedRule = ruleDraft.rules.find((rule: VisionRule) => rule.id === selectedRuleId) ?? null;
  const recognitionConfigured = Boolean(normalizedSavedServiceWSURL);

  useEffect(() => {
    if (!selectedRule) {
      setDetailView('editor');
    }
  }, [selectedRule]);

  const updateRuleDraft = (updater: (current: VisionRulesDraft) => VisionRulesDraft) => {
    ruleDraftState.setDraft(updater);
  };

  const updateSettingsDraft = (updater: (current: RecognitionSettingsDraft) => RecognitionSettingsDraft) => {
    settingsDraftState.setDraft(updater);
  };

  const addRule = () => {
    const rule = createVisionRule(cameraDevices, ruleDraft.rules.length);
    updateRuleDraft((current) => ({
      rules: [...current.rules, rule],
    }));
    setSelectedRuleId(rule.id);
    setDetailView('editor');
  };

  const removeRule = (ruleId: string) => {
    const nextRules = ruleDraft.rules.filter((rule) => rule.id !== ruleId);
    updateRuleDraft((current) => ({
      rules: nextRules,
    }));
    setSelectedRuleId(ensureSelection(selectedRuleId === ruleId ? '' : selectedRuleId, nextRules));
    setDetailView('editor');
  };

  const handleRefreshEntities = async () => {
    setBusy('refresh_entities');
    try {
      const nextCatalog = await refreshVisionEntityCatalog({
        service_ws_url: settingsDraft.service_ws_url || persistedConfig.service_ws_url || undefined,
        model_name: settingsDraft.model_name || persistedConfig.model_name || undefined,
      });
      setDetail((current: CapabilityDetail | null) =>
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

  const handleSaveRule = async () => {
    setBusy('save_rule');
    try {
      const payload: VisionCapabilityConfig = {
        ...persistedConfig,
        rules: cloneVisionConfig(ruleDraft.rules),
      };
      const saved = await saveVisionCapabilityConfig(payload);
      const nextDraft = { rules: cloneVisionConfig(saved.vision?.config.rules ?? ruleDraft.rules) };
      setDetail(saved);
      ruleDraftState.replaceDraft(nextDraft, summary?.id ?? 'vision_entity_stay_zone');
      await refreshAll();
    } catch (error) {
      onError(error instanceof Error ? error.message : 'Failed to save rule');
    } finally {
      setBusy('');
    }
  };

  const handleSaveSettings = async () => {
    setBusy('save_settings');
    try {
      const payload: VisionCapabilityConfig = {
        ...persistedConfig,
        ...settingsDraft,
        rules: cloneVisionConfig(persistedConfig.rules),
      };
      const saved = await saveVisionCapabilityConfig(payload);
      setDetail(saved);
      settingsDraftState.replaceDraft(
        settingsDraftFromConfig(saved.vision?.config ?? defaultVisionConfig()),
        summary?.id ?? 'vision_entity_stay_zone',
      );
      await refreshAll();
    } catch (error) {
      onError(error instanceof Error ? error.message : 'Failed to save recognition settings');
    } finally {
      setBusy('');
    }
  };

  const handleResetSettings = () => {
    settingsDraftState.resetDraft();
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
                {capabilityDisplayName(summary)} · {normalizedSavedServiceWSURL || 'Recognition service not configured'}
              </p>
              <ScrollArea className="explorer-scroll">
                <div className="list-stack">
                  {ruleDraft.rules.map((rule: VisionRule) => {
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
                  {ruleDraft.rules.length === 0 ? <div className="detail">No recognition rules configured yet.</div> : null}
                </div>
              </ScrollArea>
            </CardContent>
          </Card>

          {detailView === 'editor' ? (
            <ScrollArea className="detail-scroll vision-rule-layout__detail">
              <div className="detail-stack">
                <VisionRuleEditorCard
                  catalog={catalog}
                  catalogMatchesDraft={catalogMatchesDraft}
                  catalogMatchesSaved={catalogMatchesSaved}
                  cameraDevices={cameraDevices}
                  loading={busy === 'load'}
                  onSaveRule={() => void handleSaveRule()}
                  onRemoveRule={removeRule}
                  onSelectRuleId={setSelectedRuleId}
                  onUpdateRule={(ruleId, updater) =>
                    updateRuleDraft((current) => ({
                      rules: current.rules.map((rule: VisionRule) => (rule.id === ruleId ? updater({ ...rule }) : rule)),
                    }))
                  }
                  onViewHistory={() => setDetailView('history')}
                  saving={busy === 'save_rule'}
                  selectedRule={selectedRule}
                />
              </div>
            </ScrollArea>
          ) : (
            <div className="vision-rule-layout__detail vision-rule-layout__detail--fixed">
              <div className="detail-stack">
                {selectedRule ? (
                  <VisionRuleEventHistoryPanel
                    onBack={() => setDetailView('editor')}
                    onError={onError}
                    rule={selectedRule}
                    updatedAtKey={summary?.updated_at ?? ''}
                  />
                ) : (
                  <Card>
                    <CardContent className="pt-6">
                      <p className="muted">Select or create a rule to inspect its persisted event history.</p>
                    </CardContent>
                  </Card>
                )}
              </div>
            </div>
          )}
        </div>
      </div>

      {settingsDraft ? (
        <RecognitionSettingsModal
          busy={busy}
          catalog={catalog}
          catalogMatchesDraft={catalogMatchesDraft}
          draft={settingsDraft}
          normalizedDraftServiceWSURL={normalizedDraftServiceWSURL}
          normalizedDraftModelName={normalizeVisionModelName(settingsDraft.model_name)}
          onOpenChange={setSettingsOpen}
          onRefreshEntities={() => void handleRefreshEntities()}
          onResetDraft={handleResetSettings}
          onSave={() => void handleSaveSettings()}
          onUpdateDraft={updateSettingsDraft}
          open={settingsOpen}
          runtime={runtime}
          status={summary?.status ?? 'unknown'}
        />
      ) : null}
    </>
  );
}
