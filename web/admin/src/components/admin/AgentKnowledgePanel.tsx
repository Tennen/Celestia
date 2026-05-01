import { useEffect, useState } from 'react';
import { Save } from 'lucide-react';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { saveAgentSettings, type AgentSnapshot } from '../../lib/agent';
import { Field, FieldGrid, ToggleField, numberValue, parseOptionalNumber } from './AgentFormFields';
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
  const [model, setModel] = useState(config.codex_model ?? '');
  const [reasoning, setReasoning] = useState(config.codex_reasoning ?? '');
  const [timeout, setTimeoutValue] = useState(numberValue(config.timeout_ms));
  const [maxOutput, setMaxOutput] = useState(numberValue(config.max_output_chars));

  useEffect(() => {
    const next = snapshot.settings.knowledge ?? { enabled: false };
    setEnabled(next.enabled === true);
    setBaseDir(next.base_dir ?? '');
    setModel(next.codex_model ?? '');
    setReasoning(next.codex_reasoning ?? '');
    setTimeoutValue(numberValue(next.timeout_ms));
    setMaxOutput(numberValue(next.max_output_chars));
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
            codex_model: model.trim() || undefined,
            codex_reasoning: reasoning.trim() || undefined,
            timeout_ms: parseOptionalNumber(timeout),
            max_output_chars: parseOptionalNumber(maxOutput),
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
            <Field label="Codex model" value={model} placeholder="follow evolution default" onChange={setModel} />
            <Field label="Reasoning effort" value={reasoning} placeholder="medium | high | xhigh" onChange={setReasoning} />
            <Field label="Timeout ms" value={timeout} placeholder="600000" onChange={setTimeoutValue} />
            <Field label="Max answer chars" value={maxOutput} placeholder="1800" onChange={setMaxOutput} />
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
          <CardDescription>Incoming WeCom text already enters ProjectInput, so these commands work from WeCom and HTTP.</CardDescription>
        </CardHeader>
        <CardContent className="stack text-sm">
          <code>/kb ask &lt;question&gt;</code>
          <code>/kb &lt;question&gt;</code>
          <code>/kb new [question]</code>
          <code>/kb status</code>
          <p className="text-muted-foreground">
            Use <code>/kb new</code> when the Codex session context gets too large or you want a clean knowledge-base conversation.
          </p>
        </CardContent>
      </Card>
    </div>
  );
}
