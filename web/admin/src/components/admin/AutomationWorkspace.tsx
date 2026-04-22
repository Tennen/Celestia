import { useEffect, useMemo, useState } from 'react';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardHeader } from '../ui/card';
import { Input } from '../ui/input';
import { ScrollArea } from '../ui/scroll-area';
import { Section } from '../ui/section';
import { Switch } from '../ui/switch';
import {
  defaultAutomation,
  cloneAutomation,
  describeAutomationTrigger,
  getActionKind,
  parseActionParams,
  prettyActionParams,
  type AutomationActionTemplate,
} from '../../lib/automation';
import { deleteAutomation, saveAutomation } from '../../lib/api';
import { formatTime } from '../../lib/utils';
import { useAdminStore } from '../../stores/adminStore';
import type { Automation } from '../../lib/types';
import { useRetainedDraft } from '../../hooks/useRetainedDraft';
import { ActionsEditor } from './automation/ActionsEditor';
import { ConditionsEditor } from './automation/ConditionsEditor';
import { TimeWindowEditor } from './automation/TimeWindowEditor';
import { CardHeading } from './shared/CardHeading';
import { SelectableListItem } from './shared/SelectableListItem';

function buildActionParamDrafts(actions: Automation['actions']) {
  return Object.fromEntries(actions.map((action, index) => [index, prettyActionParams(action.params)]));
}

function automationDraftKey(automationId: string) {
  return automationId ? `automation:${automationId}` : 'automation:new';
}

function automationStatusBadge(automation: Automation) {
  if (!automation.enabled) {
    return { label: 'disabled', tone: 'neutral' as const };
  }
  if (automation.last_run_status === 'failed') {
    return { label: 'failed', tone: 'bad' as const };
  }
  if (automation.last_run_status === 'succeeded') {
    return { label: 'succeeded', tone: 'good' as const };
  }
  return { label: 'idle', tone: 'neutral' as const };
}

export function AutomationWorkspace() {
  const { automations, devices, refreshAll, reportError } = useAdminStore();
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [actionParamDrafts, setActionParamDrafts] = useState<Record<number, string>>({});
  const [busy, setBusy] = useState('');

  const defaultDraft = useMemo(() => defaultAutomation(devices), [devices]);
  const selectedAutomation =
    selectedId === null
      ? automations[0] ?? null
      : selectedId
        ? automations.find((automation) => automation.id === selectedId) ?? null
        : null;
  const selectedAutomationId = selectedAutomation?.id ?? '';
  const draftSource = selectedAutomation ?? defaultDraft;
  const {
    draft,
    replaceDraft,
    revision,
    setDraft,
  } = useRetainedDraft<Automation>({
    source: draftSource,
    sourceKey: automationDraftKey(selectedAutomationId),
    clone: cloneAutomation,
  });

  useEffect(() => {
    setSelectedId((current) => {
      if (current === null) {
        if (automations.length > 0) {
          return automations[0].id;
        }
        return devices.length > 0 ? '' : null;
      }
      if (!current) {
        return current;
      }
      return automations.some((automation) => automation.id === current) ? current : '';
    });
  }, [automations, devices]);

  useEffect(() => {
    setActionParamDrafts(draft ? buildActionParamDrafts(draft.actions) : {});
  }, [draft, revision]);

  const loadDraft = (automation: Automation) => {
    setSelectedId(automation.id);
    replaceDraft(automation, automationDraftKey(automation.id));
  };

  const startNewAutomation = () => {
    const nextDraft = defaultAutomation(devices);
    setSelectedId('');
    replaceDraft(nextDraft, automationDraftKey(''));
  };

  const updateDraft = (updater: (current: Automation) => Automation) => {
    setDraft(updater);
  };

  const handleSave = async () => {
    if (!draft) return;
    setBusy('save');
    try {
      const payload = cloneAutomation(draft);
      payload.actions = payload.actions.map((action, index) => ({
        ...action,
        params: getActionKind(action) === 'agent' ? action.params ?? {} : parseActionParams(actionParamDrafts[index] ?? prettyActionParams(action.params)),
      }));
      const saved = await saveAutomation(payload);
      setSelectedId(saved.id);
      replaceDraft(saved, automationDraftKey(saved.id));
      await refreshAll();
    } catch (error) {
      reportError(error instanceof Error ? error.message : 'Failed to save automation');
    } finally {
      setBusy('');
    }
  };

  const handleDelete = async () => {
    if (!selectedAutomationId) {
      startNewAutomation();
      return;
    }
    setBusy('delete');
    try {
      await deleteAutomation(selectedAutomationId);
      const nextDraft = defaultAutomation(devices);
      setSelectedId('');
      replaceDraft(nextDraft, automationDraftKey(''));
      await refreshAll();
    } catch (error) {
      reportError(error instanceof Error ? error.message : 'Failed to delete automation');
    } finally {
      setBusy('');
    }
  };

  const applyActionTemplate = (index: number, template: AutomationActionTemplate | null) => {
    updateDraft((current) => {
      const actions = [...current.actions];
      const previous = actions[index];
      actions[index] = {
        ...previous,
        label: template?.label ?? previous.label ?? '',
        action: template?.action ?? previous.action,
        params: template?.params ?? previous.params ?? {},
      };
      setActionParamDrafts((currentDrafts) => ({
        ...currentDrafts,
        [index]: prettyActionParams(template?.params ?? actions[index].params),
      }));
      return {
        ...current,
        actions,
      };
    });
  };

  return (
    <Section stack={false} className="plugin-workspace">
      <Card className="plugin-explorer explorer-card">
        <CardContent className="explorer-card__content pt-6">
          <div className="button-row">
            <Button onClick={startNewAutomation}>New Automation</Button>
          </div>
          <ScrollArea className="explorer-scroll">
            <div className="list-stack">
              {automations.map((automation) => {
                const statusBadge = automationStatusBadge(automation);
                return (
                  <SelectableListItem
                    key={automation.id}
                    className={`table__row ${selectedAutomationId === automation.id ? 'is-selected' : ''}`}
                    layout="stacked_badges"
                    onClick={() => loadDraft(automation)}
                    selected={selectedAutomationId === automation.id}
                    title={automation.name || automation.id}
                    description={describeAutomationTrigger(automation)}
                    badges={
                      <Badge tone={statusBadge.tone} size="xxs">
                        {statusBadge.label}
                      </Badge>
                    }
                  />
                );
              })}
              {automations.length === 0 ? <div className="detail">No automations configured yet.</div> : null}
            </div>
          </ScrollArea>
        </CardContent>
      </Card>

      <ScrollArea className="detail-scroll">
        <div className="detail-stack">
          <Card>
            <CardHeader>
              <CardHeading
                title={selectedAutomationId ? 'Automation Editor' : 'New Automation'}
                description="Define a state or daily time trigger, optional current-state gates, then execute device commands or Agent output flows."
                aside={
                  draft ? (
                    <div className="automation-editor__meta">
                      <Badge
                        size="xs"
                        tone={
                          draft.last_run_status === 'failed'
                            ? 'bad'
                            : draft.last_run_status === 'succeeded'
                              ? 'good'
                              : 'neutral'
                        }
                      >
                        {draft.last_run_status ?? 'idle'}
                      </Badge>
                      <Switch
                        checked={draft.enabled}
                        onCheckedChange={(checked) =>
                          updateDraft((current) => ({ ...current, enabled: checked }))
                        }
                        aria-label="Toggle automation enabled"
                      />
                    </div>
                  ) : null
                }
              />
            </CardHeader>
            <CardContent className="stack">
              {draft ? (
                <div className="automation-editor">
                  <div className="automation-summary">
                    <div className="automation-field">
                      <label>Name</label>
                      <Input
                        value={draft.name}
                        onChange={(event) => updateDraft((current) => ({ ...current, name: event.target.value }))}
                        placeholder="Haier washer done"
                      />
                    </div>
                  </div>

                  <ConditionsEditor draft={draft} devices={devices} onChange={updateDraft} />
                  <TimeWindowEditor draft={draft} onChange={updateDraft} />
                  <ActionsEditor
                    draft={draft}
                    devices={devices}
                    actionParamDrafts={actionParamDrafts}
                    onChange={updateDraft}
                    onParamDraftChange={(index, value) =>
                      setActionParamDrafts((current) => ({ ...current, [index]: value }))
                    }
                    onApplyTemplate={applyActionTemplate}
                    onResetParamDrafts={(actions) => setActionParamDrafts(buildActionParamDrafts(actions))}
                  />

                  <div className="button-row">
                    <Button onClick={() => void handleSave()} disabled={busy === 'save'}>
                      Save Automation
                    </Button>
                    <Button variant="secondary" onClick={startNewAutomation}>
                      Reset Draft
                    </Button>
                    <Button variant="danger" onClick={() => void handleDelete()} disabled={busy === 'delete'}>
                      {selectedAutomationId ? 'Delete Automation' : 'Discard Draft'}
                    </Button>
                  </div>

                  <div className="runtime-metadata">
                    {draft.last_triggered_at ? <p>Last run {formatTime(draft.last_triggered_at)}</p> : <p>No executions yet.</p>}
                    {draft.last_error ? <p className="muted">Last error: {draft.last_error}</p> : null}
                  </div>
                </div>
              ) : (
                <p className="muted">Loading automation editor…</p>
              )}
            </CardContent>
          </Card>
        </div>
      </ScrollArea>
    </Section>
  );
}
