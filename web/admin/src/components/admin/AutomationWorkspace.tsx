import { useEffect, useState } from 'react';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { Input } from '../ui/input';
import { ScrollArea } from '../ui/scroll-area';
import { Section } from '../ui/section';
import { Switch } from '../ui/switch';
import { defaultAutomation, cloneAutomation, parseActionParams, prettyActionParams, type AutomationActionTemplate } from '../../lib/automation';
import { deleteAutomation, saveAutomation } from '../../lib/api';
import { formatTime } from '../../lib/utils';
import { useAdminStore } from '../../stores/adminStore';
import type { Automation } from '../../lib/types';
import { ActionsEditor } from './automation/ActionsEditor';
import { ConditionsEditor } from './automation/ConditionsEditor';
import { TimeWindowEditor } from './automation/TimeWindowEditor';
import { TriggerEditor } from './automation/TriggerEditor';
import { CardHeading } from './shared/CardHeading';
import { SelectableListItem } from './shared/SelectableListItem';

function buildActionParamDrafts(actions: Automation['actions']) {
  return Object.fromEntries(actions.map((action, index) => [index, prettyActionParams(action.params)]));
}

export function AutomationWorkspace() {
  const { automations, devices, refreshAll, reportError } = useAdminStore();
  const [selectedId, setSelectedId] = useState('');
  const [draft, setDraft] = useState<Automation | null>(null);
  const [busy, setBusy] = useState('');
  const [actionParamDrafts, setActionParamDrafts] = useState<Record<number, string>>({});

  const loadDraft = (automation: Automation) => {
    const next = cloneAutomation(automation);
    setSelectedId(next.id);
    setDraft(next);
    setActionParamDrafts(buildActionParamDrafts(next.actions));
  };

  const startNewAutomation = () => {
    const next = defaultAutomation(devices);
    setSelectedId('');
    setDraft(next);
    setActionParamDrafts(buildActionParamDrafts(next.actions));
  };

  useEffect(() => {
    if (draft) return;
    if (automations.length > 0) {
      loadDraft(automations[0]);
      return;
    }
    if (devices.length > 0) {
      startNewAutomation();
    }
  }, [automations, devices, draft]);

  useEffect(() => {
    if (!selectedId) return;
    if (automations.some((automation) => automation.id === selectedId)) return;
    if (automations.length > 0) {
      loadDraft(automations[0]);
      return;
    }
    startNewAutomation();
  }, [automations, devices, selectedId]);

  const updateDraft = (updater: (current: Automation) => Automation) => {
    setDraft((current) => (current ? updater(cloneAutomation(current)) : current));
  };

  const handleSave = async () => {
    if (!draft) return;
    setBusy('save');
    try {
      const payload = cloneAutomation(draft);
      payload.actions = payload.actions.map((action, index) => ({
        ...action,
        params: parseActionParams(actionParamDrafts[index] ?? prettyActionParams(action.params)),
      }));
      const saved = await saveAutomation(payload);
      loadDraft(saved);
      await refreshAll();
    } catch (error) {
      reportError(error instanceof Error ? error.message : 'Failed to save automation');
    } finally {
      setBusy('');
    }
  };

  const handleDelete = async () => {
    if (!selectedId) {
      startNewAutomation();
      return;
    }
    setBusy('delete');
    try {
      await deleteAutomation(selectedId);
      setSelectedId('');
      setDraft(null);
      setActionParamDrafts({});
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
      setActionParamDrafts((existing) => ({
        ...existing,
        [index]: prettyActionParams(template?.params ?? actions[index].params),
      }));
      return { ...current, actions };
    });
  };

  return (
    <Section stack={false} className="plugin-workspace">
      <Card className="plugin-explorer">
        <CardHeader>
          <CardTitle>Automations</CardTitle>
          <CardDescription>State-change rules executed by Core against your existing devices.</CardDescription>
        </CardHeader>
        <CardContent className="stack">
          <div className="button-row">
            <Button onClick={startNewAutomation}>New Automation</Button>
          </div>
          <ScrollArea className="max-h-[calc(100vh-15rem)] pr-3">
            <div className="list-stack">
              {automations.map((automation) => (
                <SelectableListItem
                  key={automation.id}
                  className={`table__row ${selectedId === automation.id ? 'is-selected' : ''}`}
                  onClick={() => loadDraft(automation)}
                  selected={selectedId === automation.id}
                  title={automation.name || automation.id}
                  description={automation.trigger.device_id || 'No trigger device'}
                  badges={
                    <>
                      <Badge tone={automation.enabled ? 'good' : 'neutral'} size="sm">
                        {automation.enabled ? 'enabled' : 'disabled'}
                      </Badge>
                      <Badge
                        size="sm"
                        tone={
                          automation.last_run_status === 'failed'
                            ? 'bad'
                            : automation.last_run_status === 'succeeded'
                              ? 'good'
                              : 'neutral'
                        }
                      >
                        {automation.last_run_status ?? 'idle'}
                      </Badge>
                    </>
                  }
                />
              ))}
              {automations.length === 0 ? <div className="detail">No automations configured yet.</div> : null}
            </div>
          </ScrollArea>
        </CardContent>
      </Card>

      <div className="detail-stack">
        <Card>
          <CardHeader>
            <CardHeading
              title={selectedId ? 'Automation Editor' : 'New Automation'}
              description="Trigger on one device state transition, optionally gate it with other conditions and a time window, then execute actions on existing devices."
              aside={
                draft ? (
                  <div className="plugin-card__badges">
                    <Badge tone={draft.enabled ? 'good' : 'neutral'}>
                      {draft.enabled ? 'enabled' : 'disabled'}
                    </Badge>
                    <Badge
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
                  </div>
                ) : null
              }
            />
          </CardHeader>
          <CardContent className="stack">
            {draft ? (
              <>
                <div className="grid grid--two">
                  <div className="stack">
                    <label>Name</label>
                    <Input value={draft.name} onChange={(e) => updateDraft((current) => ({ ...current, name: e.target.value }))} placeholder="Haier washer done" />
                  </div>
                  <div className="config-field-list__item">
                    <div className="section-title section-title--inline">
                      <div>
                        <strong>Enabled</strong>
                        <p className="muted">Control whether Core can evaluate and execute this rule.</p>
                      </div>
                      <Switch
                        checked={draft.enabled}
                        onCheckedChange={(checked) =>
                          updateDraft((current) => ({ ...current, enabled: checked }))
                        }
                      />
                    </div>
                  </div>
                </div>

                <TriggerEditor draft={draft} devices={devices} onChange={updateDraft} />
                <ConditionsEditor draft={draft} devices={devices} onChange={updateDraft} />
                <TimeWindowEditor draft={draft} onChange={updateDraft} />
                <ActionsEditor
                  draft={draft}
                  devices={devices}
                  actionParamDrafts={actionParamDrafts}
                  onChange={updateDraft}
                  onParamDraftChange={(index, value) => setActionParamDrafts((current) => ({ ...current, [index]: value }))}
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
                    {selectedId ? 'Delete Automation' : 'Discard Draft'}
                  </Button>
                </div>

                <div className="runtime-metadata">
                  {draft.last_triggered_at ? <p>Last triggered {formatTime(draft.last_triggered_at)}</p> : <p>No executions yet.</p>}
                  {draft.last_error ? <p className="muted">Last error: {draft.last_error}</p> : null}
                </div>
              </>
            ) : (
              <p className="muted">Loading automation editor…</p>
            )}
          </CardContent>
        </Card>
      </div>
    </Section>
  );
}
