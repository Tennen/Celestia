import { useEffect, useState } from 'react';
import { Save } from 'lucide-react';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import {
  saveAgentSettings,
  type AgentSettings,
  type AgentSnapshot,
} from '../../lib/agent';
import { Field, FieldGrid, SelectField, ToggleField, numberValue, parseOptionalNumber } from './AgentFormFields';
import type { AgentRunner } from './AgentWorkspace';

type Props = {
  snapshot: AgentSnapshot;
  busy: string;
  onRun: AgentRunner;
};

const md2imgModes = [
  { value: 'long-image', label: 'long-image' },
  { value: 'multi-page', label: 'multi-page' },
];

export function AgentLLMPanel({ snapshot, busy, onRun }: Props) {
  const [terminalEnabled, setTerminalEnabled] = useState(snapshot.settings.terminal.enabled);
  const [terminalCwd, setTerminalCwd] = useState(snapshot.settings.terminal.cwd ?? '');
  const [terminalTimeout, setTerminalTimeout] = useState(numberValue(snapshot.settings.terminal.timeout_ms));
  const [memoryEnabled, setMemoryEnabled] = useState(snapshot.settings.memory?.enabled === true);
  const [memoryRounds, setMemoryRounds] = useState(numberValue(snapshot.settings.memory?.compact_every_rounds));
  const [md2imgEnabled, setMd2imgEnabled] = useState(snapshot.settings.md2img?.enabled === true);
  const [md2imgMode, setMd2imgMode] = useState(textOf(snapshot.settings.md2img?.default_mode) || 'long-image');
  const [md2imgOutputDir, setMd2imgOutputDir] = useState(textOf(snapshot.settings.md2img?.output_dir));
  const [md2imgTimeout, setMd2imgTimeout] = useState(numberValue(snapshot.settings.md2img?.timeout_ms));

  useEffect(() => {
    setTerminalEnabled(snapshot.settings.terminal.enabled);
    setTerminalCwd(snapshot.settings.terminal.cwd ?? '');
    setTerminalTimeout(numberValue(snapshot.settings.terminal.timeout_ms));
    setMemoryEnabled(snapshot.settings.memory?.enabled === true);
    setMemoryRounds(numberValue(snapshot.settings.memory?.compact_every_rounds));
    setMd2imgEnabled(snapshot.settings.md2img?.enabled === true);
    setMd2imgMode(textOf(snapshot.settings.md2img?.default_mode) || 'long-image');
    setMd2imgOutputDir(textOf(snapshot.settings.md2img?.output_dir));
    setMd2imgTimeout(numberValue(snapshot.settings.md2img?.timeout_ms));
  }, [snapshot]);

  const saveRuntime = () => {
    const settings: AgentSettings = {
      ...snapshot.settings,
      terminal: {
        enabled: terminalEnabled,
        cwd: terminalCwd.trim() || undefined,
        timeout_ms: parseOptionalNumber(terminalTimeout),
      },
      memory: {
        ...(snapshot.settings.memory ?? {}),
        enabled: memoryEnabled,
        compact_every_rounds: parseOptionalNumber(memoryRounds),
      },
      md2img: {
        ...(snapshot.settings.md2img ?? {}),
        enabled: md2imgEnabled,
        default_mode: md2imgMode,
        output_dir: md2imgOutputDir.trim() || undefined,
        timeout_ms: parseOptionalNumber(md2imgTimeout),
      },
    };
    onRun('settings-save', () => saveAgentSettings(settings), false);
  };

  return (
    <div className="grid grid--two">
      <Card className="panel">
        <CardHeader>
          <CardTitle>Runtime Settings</CardTitle>
          <CardDescription>Internal runtime knobs used by the Agent, not manual execution pages</CardDescription>
        </CardHeader>
        <CardContent className="stack">
          <ToggleField label="Terminal tool enabled" checked={terminalEnabled} onChange={setTerminalEnabled} />
          <FieldGrid>
            <Field label="Terminal cwd" value={terminalCwd} onChange={setTerminalCwd} />
            <Field label="Terminal timeout ms" value={terminalTimeout} onChange={setTerminalTimeout} />
          </FieldGrid>
          <ToggleField label="Memory context enabled" checked={memoryEnabled} onChange={setMemoryEnabled} />
          <Field label="Compact every rounds" value={memoryRounds} onChange={setMemoryRounds} />
          <ToggleField label="Markdown image pipeline enabled" checked={md2imgEnabled} onChange={setMd2imgEnabled} />
          <FieldGrid>
            <SelectField label="md2img mode" value={md2imgMode} options={md2imgModes} onChange={setMd2imgMode} />
            <Field label="Output directory" value={md2imgOutputDir} onChange={setMd2imgOutputDir} />
            <Field label="md2img timeout ms" value={md2imgTimeout} onChange={setMd2imgTimeout} />
          </FieldGrid>
          <Button onClick={saveRuntime} disabled={busy === 'settings-save'}>
            <Save className="mr-2 h-4 w-4" />
            Save Runtime
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}

function textOf(value: unknown) {
  return typeof value === 'string' ? value : '';
}
