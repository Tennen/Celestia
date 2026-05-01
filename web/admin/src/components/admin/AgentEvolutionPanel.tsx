import { useEffect, useMemo, useState } from 'react';
import { Play, Save } from 'lucide-react';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { Textarea } from '../ui/textarea';
import {
  createEvolutionGoal,
  runEvolutionGoal,
  saveAgentSettings,
  type AgentSnapshot,
} from '../../lib/agent';
import { Field, FieldGrid, SelectField, ToggleField, numberValue, parseOptionalNumber } from './AgentFormFields';
import type { AgentRunner } from './AgentWorkspace';
import { SelectableListItem } from './shared/SelectableListItem';

type Props = {
  snapshot: AgentSnapshot;
  onRun: AgentRunner;
};

export function AgentEvolutionPanel({ snapshot, onRun }: Props) {
  const [goal, setGoal] = useState('');
  const [commitMessage, setCommitMessage] = useState('');
  const [command, setCommand] = useState(snapshot.settings.evolution.command ?? '');
  const [cwd, setCwd] = useState(snapshot.settings.evolution.cwd ?? '');
  const [timeout, setTimeout] = useState(numberValue(snapshot.settings.evolution.timeout_ms));
  const [codexProviderId, setCodexProviderId] = useState(snapshot.settings.evolution.codex_provider_id ?? '');
  const [maxFixAttempts, setMaxFixAttempts] = useState(numberValue(snapshot.settings.evolution.max_fix_attempts));
  const [autoCommit, setAutoCommit] = useState(snapshot.settings.evolution.auto_commit === true);
  const [autoPush, setAutoPush] = useState(snapshot.settings.evolution.auto_push === true);
  const [structureReview, setStructureReview] = useState(snapshot.settings.evolution.structure_review === true);

  const runnableGoal = useMemo(
    () => snapshot.evolution.goals.find((item) => item.status !== 'succeeded') ?? snapshot.evolution.goals[0],
    [snapshot.evolution.goals],
  );
  const codexProviders = useMemo(
    () => snapshot.settings.llm_providers.filter((provider) => provider.type === 'codex'),
    [snapshot.settings.llm_providers],
  );
  const providerOptions = useMemo(
    () => [
      { value: '', label: 'select codex provider' },
      ...codexProviders.map((provider) => ({
        value: provider.id,
        label: `${provider.name || provider.id}${provider.model ? ` · ${provider.model}` : ''}`,
      })),
    ],
    [codexProviders],
  );

  useEffect(() => {
    setCommand(snapshot.settings.evolution.command ?? '');
    setCwd(snapshot.settings.evolution.cwd ?? '');
    setTimeout(numberValue(snapshot.settings.evolution.timeout_ms));
    setCodexProviderId(snapshot.settings.evolution.codex_provider_id ?? '');
    setMaxFixAttempts(numberValue(snapshot.settings.evolution.max_fix_attempts));
    setAutoCommit(snapshot.settings.evolution.auto_commit === true);
    setAutoPush(snapshot.settings.evolution.auto_push === true);
    setStructureReview(snapshot.settings.evolution.structure_review === true);
  }, [snapshot]);

  const saveSettings = () => {
    onRun(
      'settings-save',
      () =>
        saveAgentSettings({
          ...snapshot.settings,
          evolution: {
            ...snapshot.settings.evolution,
            command: command.trim() || undefined,
            cwd: cwd.trim() || undefined,
            timeout_ms: parseOptionalNumber(timeout),
            codex_provider_id: codexProviderId.trim() || undefined,
            max_fix_attempts: parseOptionalNumber(maxFixAttempts),
            auto_commit: autoCommit,
            auto_push: autoPush,
            structure_review: structureReview,
          },
        }),
      false,
    );
  };

  return (
    <div className="grid grid--two">
      <Card className="panel">
        <CardHeader>
          <CardTitle>Evolution Queue</CardTitle>
          <CardDescription>{snapshot.evolution.goals.length} goals</CardDescription>
        </CardHeader>
        <CardContent className="stack">
          <Textarea value={goal} onChange={(event) => setGoal(event.target.value)} placeholder="Goal" />
          <Field label="Commit message" value={commitMessage} onChange={setCommitMessage} />
          <div className="button-row">
            <Button onClick={() => onRun('evolution', () => createEvolutionGoal({ goal, commit_message: commitMessage || undefined }))} disabled={!goal.trim()}>
              Queue
            </Button>
            <Button variant="secondary" disabled={!runnableGoal} onClick={() => onRun('evolution', () => runEvolutionGoal(runnableGoal!.id))}>
              <Play className="mr-2 h-4 w-4" />
              Run Next
            </Button>
          </div>
          <div className="list-stack">
            {snapshot.evolution.goals.map((item) => (
              <SelectableListItem
                key={item.id}
                title={item.goal}
                description={item.commit_message ?? ''}
                selected={item.id === runnableGoal?.id}
                badges={<Badge tone={item.status === 'succeeded' ? 'good' : item.status === 'failed' ? 'bad' : 'neutral'} size="xxs">{item.status}</Badge>}
                onClick={() => {
                  setGoal(item.goal);
                  setCommitMessage(item.commit_message ?? '');
                }}
              />
            ))}
            {snapshot.evolution.goals.length === 0 ? <div className="detail">No evolution goals queued.</div> : null}
          </div>
        </CardContent>
      </Card>

      <Card className="panel">
        <CardHeader>
          <CardTitle>Evolution Settings</CardTitle>
          <CardDescription>Codex-based evolution runtime settings and optional WeCom notification flow</CardDescription>
        </CardHeader>
        <CardContent className="stack">
          <FieldGrid>
            <Field label="Command" value={command} onChange={setCommand} />
            <Field label="Cwd" value={cwd} onChange={setCwd} />
            <Field label="Timeout ms" value={timeout} onChange={setTimeout} />
            <SelectField label="Codex provider" value={codexProviderId} options={providerOptions} onChange={setCodexProviderId} />
            <Field label="Max fix attempts" value={maxFixAttempts} onChange={setMaxFixAttempts} />
          </FieldGrid>
          <ToggleField label="Auto commit" checked={autoCommit} onChange={setAutoCommit} />
          <ToggleField label="Auto push" checked={autoPush} onChange={setAutoPush} />
          <ToggleField label="Structure review" checked={structureReview} onChange={setStructureReview} />
          <Button onClick={saveSettings}>
            <Save className="mr-2 h-4 w-4" />
            Save Settings
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
