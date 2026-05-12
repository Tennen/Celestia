import { useEffect, useMemo, useState } from 'react';
import { Play, Save, ShieldCheck, Wrench } from 'lucide-react';
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
import { createAgentApproval, runEvolutionOperation } from '../../lib/agent-ops';
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
  const [agentProviderId, setAgentProviderId] = useState(snapshot.settings.evolution.agent_provider_id ?? '');
  const [maxFixAttempts, setMaxFixAttempts] = useState(numberValue(snapshot.settings.evolution.max_fix_attempts));
  const [pushRemote, setPushRemote] = useState(snapshot.settings.evolution.push_remote ?? '');
  const [pushBranch, setPushBranch] = useState(snapshot.settings.evolution.push_branch ?? '');
  const [rebuildCommand, setRebuildCommand] = useState(snapshot.settings.evolution.rebuild_command ?? './deploy.sh');
  const [restartCommand, setRestartCommand] = useState(snapshot.settings.evolution.restart_command ?? './tool/celestia-service.sh restart');
  const [codexApprovalPolicy, setCodexApprovalPolicy] = useState(snapshot.settings.evolution.codex_approval_policy ?? 'never');
  const [autoCommit, setAutoCommit] = useState(snapshot.settings.evolution.auto_commit === true);
  const [autoPush, setAutoPush] = useState(snapshot.settings.evolution.auto_push === true);
  const [autoRebuild, setAutoRebuild] = useState(snapshot.settings.evolution.auto_rebuild === true);
  const [autoRestart, setAutoRestart] = useState(snapshot.settings.evolution.auto_restart === true);
  const [structureReview, setStructureReview] = useState(snapshot.settings.evolution.structure_review === true);

  const runnableGoal = useMemo(
    () => snapshot.evolution.goals.find((item) => item.status !== 'succeeded') ?? snapshot.evolution.goals[0],
    [snapshot.evolution.goals],
  );
  const agentProviders = useMemo(
    () => snapshot.settings.agent_providers,
    [snapshot.settings.agent_providers],
  );
  const providerOptions = useMemo(
    () => [
      { value: '', label: 'select agent provider' },
      ...agentProviders.map((provider) => ({
        value: provider.id,
        label: `${provider.name || provider.id}${provider.model ? ` · ${provider.model}` : ''}`,
      })),
    ],
    [agentProviders],
  );

  useEffect(() => {
    setCommand(snapshot.settings.evolution.command ?? '');
    setCwd(snapshot.settings.evolution.cwd ?? '');
    setTimeout(numberValue(snapshot.settings.evolution.timeout_ms));
    setAgentProviderId(snapshot.settings.evolution.agent_provider_id ?? '');
    setMaxFixAttempts(numberValue(snapshot.settings.evolution.max_fix_attempts));
    setPushRemote(snapshot.settings.evolution.push_remote ?? '');
    setPushBranch(snapshot.settings.evolution.push_branch ?? '');
    setRebuildCommand(snapshot.settings.evolution.rebuild_command ?? './deploy.sh');
    setRestartCommand(snapshot.settings.evolution.restart_command ?? './tool/celestia-service.sh restart');
    setCodexApprovalPolicy(snapshot.settings.evolution.codex_approval_policy ?? 'never');
    setAutoCommit(snapshot.settings.evolution.auto_commit === true);
    setAutoPush(snapshot.settings.evolution.auto_push === true);
    setAutoRebuild(snapshot.settings.evolution.auto_rebuild === true);
    setAutoRestart(snapshot.settings.evolution.auto_restart === true);
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
            agent_provider_id: agentProviderId.trim() || undefined,
            max_fix_attempts: parseOptionalNumber(maxFixAttempts),
            push_remote: pushRemote.trim() || undefined,
            push_branch: pushBranch.trim() || undefined,
            rebuild_command: rebuildCommand.trim() || undefined,
            restart_command: restartCommand.trim() || undefined,
            codex_approval_policy: codexApprovalPolicy,
            auto_commit: autoCommit,
            auto_push: autoPush,
            auto_rebuild: autoRebuild,
            auto_restart: autoRestart,
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
            <SelectField label="Agent provider" value={agentProviderId} options={providerOptions} onChange={setAgentProviderId} />
            <SelectField
              label="Codex approval"
              value={codexApprovalPolicy}
              options={[
                { value: 'never', label: 'never' },
                { value: 'on-request', label: 'on-request' },
                { value: 'untrusted', label: 'untrusted' },
              ]}
              onChange={setCodexApprovalPolicy}
            />
            <Field label="Max fix attempts" value={maxFixAttempts} onChange={setMaxFixAttempts} />
            <Field label="Push remote" value={pushRemote} onChange={setPushRemote} />
            <Field label="Push branch" value={pushBranch} onChange={setPushBranch} />
            <Field label="Rebuild command" value={rebuildCommand} onChange={setRebuildCommand} />
            <Field label="Restart command" value={restartCommand} onChange={setRestartCommand} />
          </FieldGrid>
          <ToggleField label="Auto commit" checked={autoCommit} onChange={setAutoCommit} />
          <ToggleField label="Auto push" checked={autoPush} onChange={setAutoPush} />
          <ToggleField label="Auto rebuild" checked={autoRebuild} onChange={setAutoRebuild} />
          <ToggleField label="Auto restart" checked={autoRestart} onChange={setAutoRestart} />
          <ToggleField label="Structure review" checked={structureReview} onChange={setStructureReview} />
          <div className="button-row">
            {['commit', 'pull', 'push', 'rebuild', 'restart'].map((action) => (
              <Button key={action} variant="secondary" onClick={() => onRun(`evolution-${action}`, () => runEvolutionOperation({ action, goal_id: runnableGoal?.id, commit_message: commitMessage || undefined }))}>
                <Wrench className="mr-2 h-4 w-4" />
                {action}
              </Button>
            ))}
          </div>
          <div className="button-row">
            {['commit', 'pull', 'push', 'rebuild', 'restart'].map((action) => (
              <Button key={action} variant="secondary" onClick={() => onRun(`approval-${action}`, () => createAgentApproval({ kind: 'evolution_operation', action, goal_id: runnableGoal?.id, title: `Approve evolution ${action}` }))}>
                <ShieldCheck className="mr-2 h-4 w-4" />
                Request {action}
              </Button>
            ))}
          </div>
          <Button onClick={saveSettings}>
            <Save className="mr-2 h-4 w-4" />
            Save Settings
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
