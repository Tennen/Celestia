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

type EditorState = {
  selectedId: string;
  draft: Automation | null;
  actionParamDrafts: Record<number, string>;
};

function createDraftState(draft: Automation, selectedId = ''): EditorState {
  return {
    selectedId,
    draft,
    actionParamDrafts: buildActionParamDrafts(draft.actions),
  };
}

function createAutomationState(automation: Automation): EditorState {
  return createDraftState(cloneAutomation(automation), automation.id);
}

function createInitialEditorState(automations: Automation[], devices: import('../../lib/types').DeviceView[]): EditorState {
  if (automations.length > 0) {
    return createAutomationState(automations[0]);
  }
  if (devices.length > 0) {
    return createDraftState(defaultAutomation(devices));
  }
  return { selectedId: '', draft: null, actionParamDrafts: {} };
}

export function AutomationWorkspace() {
  const { automations, devices, refreshAll, reportError } = useAdminStore();
  const [editorState, setEditorState] = useState<EditorState>(() =>
    createInitialEditorState(automations, devices),
  );
  const [busy, setBusy] = useState('');
  const { selectedId, draft, actionParamDrafts } = editorState;

  const loadDraft = (automation: Automation) => {
    setEditorState(createAutomationState(automation));
  };

  const startNewAutomation = () => {
    setEditorState(createDraftState(defaultAutomation(devices)));
  };

  useEffect(() => {
    setEditorState((current) => {
      if (!current.draft) {
        return createInitialEditorState(automations, devices);
      }
      if (!current.selectedId) {
        return current;
      }
      if (automations.some((automation) => automation.id === current.selectedId)) {
        return current;
      }
      return createInitialEditorState(automations, devices);
    });
  }, [automations, devices]);

  const updateDraft = (updater: (current: Automation) => Automation) => {
    setEditorState((current) => ({
      ...current,
      draft: current.draft ? updater(cloneAutomation(current.draft)) : current.draft,
    }));
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
      setEditorState(createAutomationState(saved));
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
      setEditorState(
        createInitialEditorState(
          automations.filter((automation) => automation.id !== selectedId),
          devices,
        ),
      );
      await refreshAll();
    } catch (error) {
      reportError(error instanceof Error ? error.message : 'Failed to delete automation');
    } finally {
      setBusy('');
    }
  };

  const applyActionTemplate = (index: number, template: AutomationActionTemplate | null) => {
    setEditorState((current) => {
      if (!current.draft) {
        return current;
      }
      const actions = [...current.draft.actions];
      const previous = actions[index];
      actions[index] = {
        ...previous,
        label: template?.label ?? previous.label ?? '',
        action: template?.action ?? previous.action,
        params: template?.params ?? previous.params ?? {},
      };
      return {
        ...current,
        draft: { ...current.draft, actions },
        actionParamDrafts: {
          ...current.actionParamDrafts,
          [index]: prettyActionParams(template?.params ?? actions[index].params),
        },
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
                      <Badge tone={automation.enabled ? 'good' : 'neutral'} size="xs">
                        {automation.enabled ? 'enabled' : 'disabled'}
                      </Badge>
                      <Badge
                        size="xs"
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

      <ScrollArea className="detail-scroll">
        <div className="detail-stack">
          <Card>
            <CardHeader>
              <CardHeading
                title={selectedId ? 'Automation Editor' : 'New Automation'}
                description="Trigger on one device state transition, optionally gate it with other conditions and a time window, then execute actions on existing devices."
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
                      onChange={(e) => updateDraft((current) => ({ ...current, name: e.target.value }))}
                      placeholder="Haier washer done"
                    />
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
                  onParamDraftChange={(index, value) =>
                    setEditorState((current) => ({
                      ...current,
                      actionParamDrafts: { ...current.actionParamDrafts, [index]: value },
                    }))
                  }
                  onApplyTemplate={applyActionTemplate}
                  onResetParamDrafts={(actions) =>
                    setEditorState((current) => ({
                      ...current,
                      actionParamDrafts: buildActionParamDrafts(actions),
                    }))
                  }
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
