import { useEffect, useMemo, useState } from 'react';
import { Save } from 'lucide-react';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { saveAgentSettings, type AgentSnapshot } from '../../lib/agent';
import { Field, FieldGrid, SelectField, ToggleField, numberValue, parseOptionalNumber } from './AgentFormFields';
import type { AgentRunner } from './AgentWorkspace';

type Props = {
  snapshot: AgentSnapshot;
  busy: string;
  onRun: AgentRunner;
};

export function AgentKnowledgePanel({ snapshot, busy, onRun }: Props) {
  const config = snapshot.settings.knowledge ?? { enabled: false };
  const [enabled, setEnabled] = useState(config.enabled === true);
  const [baseDir, setBaseDir] = useState(config.base_dir ?? '');
  const [agentProviderId, setAgentProviderId] = useState(config.agent_provider_id ?? '');
  const [timeout, setTimeoutValue] = useState(numberValue(config.timeout_ms));
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
    const next = snapshot.settings.knowledge ?? { enabled: false };
    setEnabled(next.enabled === true);
    setBaseDir(next.base_dir ?? '');
    setAgentProviderId(next.agent_provider_id ?? '');
    setTimeoutValue(numberValue(next.timeout_ms));
  }, [snapshot]);

  const save = () => {
    onRun(
      'settings-save',
      () =>
        saveAgentSettings({
          ...snapshot.settings,
          knowledge: {
            enabled,
            base_dir: baseDir.trim() || undefined,
            agent_provider_id: agentProviderId.trim() || undefined,
            timeout_ms: parseOptionalNumber(timeout),
          },
        }),
      false,
    );
  };

  return (
    <div className="grid grid--two">
      <Card className="panel">
        <CardHeader>
          <CardTitle>Knowledge Base</CardTitle>
          <CardDescription>Slash commands run Codex CLI against this host folder as a read-only knowledge root</CardDescription>
        </CardHeader>
        <CardContent className="stack">
          <ToggleField label="Enable /kb slash command" checked={enabled} onChange={setEnabled} />
          <Field label="Base directory" value={baseDir} placeholder="/Users/me/Documents/Knowledge" onChange={setBaseDir} />
          <FieldGrid>
            <SelectField label="Agent provider" value={agentProviderId} options={providerOptions} onChange={setAgentProviderId} />
            <Field label="Timeout ms" value={timeout} placeholder="600000" onChange={setTimeoutValue} />
          </FieldGrid>
          <Button onClick={save} disabled={busy === 'settings-save'}>
            <Save className="mr-2 h-4 w-4" />
            Save Knowledge
          </Button>
        </CardContent>
      </Card>

      <Card className="panel">
        <CardHeader>
          <CardTitle>WeCom Slash Commands</CardTitle>
          <CardDescription>Incoming WeCom text enters ProjectInput, then knowledge answers are rendered to images.</CardDescription>
        </CardHeader>
        <CardContent className="stack text-sm">
          <code>/kb ask &lt;question&gt;</code>
          <code>/kb &lt;question&gt;</code>
          <code>/kb new [question]</code>
          <code>/kb status</code>
          <p className="text-muted-foreground">
            Answers are saved as Markdown under <code>.answers</code> in the base directory and sent to WeCom as rendered images. Render failures fail the command instead of falling back to text.
          </p>
        </CardContent>
      </Card>
    </div>
  );
}
