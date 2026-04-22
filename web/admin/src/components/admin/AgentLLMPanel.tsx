import { useEffect, useState } from 'react';
import { Plus, Save, Trash2 } from 'lucide-react';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import {
  saveAgentSettings,
  type AgentLLMProvider,
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

const providerTypes = [
  { value: 'ollama', label: 'ollama' },
  { value: 'openai', label: 'openai-like' },
  { value: 'gemini', label: 'gemini-like' },
  { value: 'llama-server', label: 'llama-server' },
  { value: 'codex', label: 'codex-cli' },
  { value: 'gpt-plugin', label: 'gpt-plugin' },
];

const sttProviders = [{ value: 'fast-whisper', label: 'fast-whisper' }];
const md2imgModes = [
  { value: 'long-image', label: 'long-image' },
  { value: 'multi-page', label: 'multi-page' },
];

const emptyProvider = (): AgentLLMProvider => ({
  id: '',
  name: '',
  type: 'openai',
  base_url: '',
  api_key: '',
  model: '',
  chat_path: '',
});

export function AgentLLMPanel({ snapshot, busy, onRun }: Props) {
  const [provider, setProvider] = useState<AgentLLMProvider>(snapshot.settings.llm_providers[0] ?? emptyProvider());
  const [terminalEnabled, setTerminalEnabled] = useState(snapshot.settings.terminal.enabled);
  const [terminalCwd, setTerminalCwd] = useState(snapshot.settings.terminal.cwd ?? '');
  const [terminalTimeout, setTerminalTimeout] = useState(numberValue(snapshot.settings.terminal.timeout_ms));
  const [sttEnabled, setSttEnabled] = useState(snapshot.settings.stt?.enabled === true);
  const [sttProvider, setSttProvider] = useState(textOf(snapshot.settings.stt?.provider) || 'fast-whisper');
  const [sttCommand, setSttCommand] = useState(textOf(snapshot.settings.stt?.command));
  const [sttTimeout, setSttTimeout] = useState(numberValue(snapshot.settings.stt?.timeout_ms));
  const [memoryEnabled, setMemoryEnabled] = useState(snapshot.settings.memory?.enabled === true);
  const [memoryRounds, setMemoryRounds] = useState(numberValue(snapshot.settings.memory?.compact_every_rounds));
  const [md2imgEnabled, setMd2imgEnabled] = useState(snapshot.settings.md2img?.enabled === true);
  const [md2imgMode, setMd2imgMode] = useState(textOf(snapshot.settings.md2img?.default_mode) || 'long-image');
  const [md2imgOutputDir, setMd2imgOutputDir] = useState(textOf(snapshot.settings.md2img?.output_dir));
  const [md2imgTimeout, setMd2imgTimeout] = useState(numberValue(snapshot.settings.md2img?.timeout_ms));

  useEffect(() => {
    setProvider(snapshot.settings.llm_providers[0] ?? emptyProvider());
    setTerminalEnabled(snapshot.settings.terminal.enabled);
    setTerminalCwd(snapshot.settings.terminal.cwd ?? '');
    setTerminalTimeout(numberValue(snapshot.settings.terminal.timeout_ms));
    setSttEnabled(snapshot.settings.stt?.enabled === true);
    setSttProvider(textOf(snapshot.settings.stt?.provider) || 'fast-whisper');
    setSttCommand(textOf(snapshot.settings.stt?.command));
    setSttTimeout(numberValue(snapshot.settings.stt?.timeout_ms));
    setMemoryEnabled(snapshot.settings.memory?.enabled === true);
    setMemoryRounds(numberValue(snapshot.settings.memory?.compact_every_rounds));
    setMd2imgEnabled(snapshot.settings.md2img?.enabled === true);
    setMd2imgMode(textOf(snapshot.settings.md2img?.default_mode) || 'long-image');
    setMd2imgOutputDir(textOf(snapshot.settings.md2img?.output_dir));
    setMd2imgTimeout(numberValue(snapshot.settings.md2img?.timeout_ms));
  }, [snapshot]);

  const saveProvider = () => {
    const id = provider.id || slugId(provider.name || provider.model || provider.type, 'llm');
    const nextProvider = { ...provider, id, type: normalizeProviderType(provider.type), timeout_ms: parseOptionalNumber(String(provider.timeout_ms ?? '')) };
    const providers = replaceById(snapshot.settings.llm_providers, nextProvider);
    onRun(
      'settings-save',
      () => saveAgentSettings({ ...snapshot.settings, llm_providers: providers, default_llm_provider_id: snapshot.settings.default_llm_provider_id || id }),
      false,
    );
  };

  const saveRuntime = () => {
    const settings: AgentSettings = {
      ...snapshot.settings,
      terminal: {
        enabled: terminalEnabled,
        cwd: terminalCwd.trim() || undefined,
        timeout_ms: parseOptionalNumber(terminalTimeout),
      },
      stt: {
        ...(snapshot.settings.stt ?? {}),
        enabled: sttEnabled,
        provider: sttProvider,
        command: sttCommand.trim(),
        timeout_ms: parseOptionalNumber(sttTimeout),
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
          <CardTitle>LLM Providers</CardTitle>
          <CardDescription>Provider profile for routing, planning, and business runtimes</CardDescription>
        </CardHeader>
        <CardContent className="stack">
          <div className="button-row">
            {snapshot.settings.llm_providers.map((item) => (
              <Button
                key={item.id}
                variant={item.id === provider.id ? 'default' : 'secondary'}
                onClick={() => {
                  setProvider(item);
                }}
              >
                {item.name || item.model || item.type}
              </Button>
            ))}
            <Button
              variant="secondary"
              onClick={() => {
                setProvider(emptyProvider());
              }}
            >
              <Plus className="mr-2 h-4 w-4" />
              New
            </Button>
          </div>
          <div className="button-row">
            <Badge tone={provider.id === snapshot.settings.default_llm_provider_id ? 'accent' : 'neutral'}>
              {provider.id === snapshot.settings.default_llm_provider_id ? 'default' : 'profile'}
            </Badge>
          </div>
          <FieldGrid>
            <Field label="Name" value={provider.name} onChange={(name) => setProvider({ ...provider, name })} />
            <SelectField label="Type" value={normalizeProviderType(provider.type)} options={providerTypes} onChange={(type) => setProvider({ ...provider, type })} />
            <Field label="Model" value={provider.model ?? ''} onChange={(model) => setProvider({ ...provider, model })} />
            <Field label="Base URL" value={provider.base_url ?? ''} onChange={(base_url) => setProvider({ ...provider, base_url })} />
            {['openai', 'gemini', 'llama-server', 'gpt-plugin'].includes(normalizeProviderType(provider.type)) ? (
              <Field label="API Key" value={provider.api_key ?? ''} onChange={(api_key) => setProvider({ ...provider, api_key })} />
            ) : null}
            {normalizeProviderType(provider.type) === 'openai' ? (
              <Field label="Chat completions path" value={provider.chat_path ?? ''} onChange={(chat_path) => setProvider({ ...provider, chat_path })} />
            ) : null}
            {normalizeProviderType(provider.type) === 'codex' ? (
              <SelectField
                label="Reasoning effort"
                value={provider.chat_path ?? ''}
                options={[
                  { value: '', label: 'follow default' },
                  { value: 'minimal', label: 'minimal' },
                  { value: 'low', label: 'low' },
                  { value: 'medium', label: 'medium' },
                  { value: 'high', label: 'high' },
                  { value: 'xhigh', label: 'xhigh' },
                ]}
                onChange={(chat_path) => setProvider({ ...provider, chat_path })}
              />
            ) : null}
            <Field label="Timeout ms" value={numberValue(provider.timeout_ms)} onChange={(value) => setProvider({ ...provider, timeout_ms: parseOptionalNumber(value) })} />
          </FieldGrid>
          <div className="button-row">
            <Button onClick={saveProvider} disabled={busy === 'settings-save' || !provider.name.trim()}>
              <Save className="mr-2 h-4 w-4" />
              Save Provider
            </Button>
            <Button
              variant="secondary"
              onClick={() => onRun('settings-save', () => saveAgentSettings({ ...snapshot.settings, default_llm_provider_id: provider.id }), false)}
              disabled={!provider.id || provider.id === snapshot.settings.default_llm_provider_id}
            >
              Set Default
            </Button>
            <Button
              variant="danger"
              disabled={!provider.id || snapshot.settings.llm_providers.length <= 1}
              onClick={() =>
                onRun(
                  'settings-save',
                  () => saveAgentSettings({ ...snapshot.settings, llm_providers: snapshot.settings.llm_providers.filter((item) => item.id !== provider.id) }),
                  false,
                )
              }
            >
              <Trash2 className="mr-2 h-4 w-4" />
              Delete
            </Button>
          </div>
        </CardContent>
      </Card>

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
          <ToggleField label="Voice input transcription enabled" checked={sttEnabled} onChange={setSttEnabled} />
          <FieldGrid>
            <SelectField label="STT provider" value={sttProvider} options={sttProviders} onChange={setSttProvider} />
            <Field label="STT command" value={sttCommand} onChange={setSttCommand} />
            <Field label="STT timeout ms" value={sttTimeout} onChange={setSttTimeout} />
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

function normalizeProviderType(type: string) {
  if (type === 'openai-like') return 'openai';
  if (type === 'gemini-like') return 'gemini';
  if (['ollama', 'openai', 'gemini', 'llama-server', 'codex', 'gpt-plugin'].includes(type)) return type;
  return 'openai';
}

function replaceById<T extends { id: string }>(items: T[], next: T): T[] {
  return items.some((item) => item.id === next.id) ? items.map((item) => (item.id === next.id ? next : item)) : [...items, next];
}

function slugId(raw: string, prefix: string) {
  const slug = raw
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9._-]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 48);
  return slug || `${prefix}-${Date.now()}`;
}

function textOf(value: unknown) {
  return typeof value === 'string' ? value : '';
}
