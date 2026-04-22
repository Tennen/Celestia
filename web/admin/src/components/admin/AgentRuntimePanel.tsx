import { useEffect, useMemo, useState } from 'react';
import { MessageSquare, Play, Plus, Save, Trash2 } from 'lucide-react';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../ui/tabs';
import { Textarea } from '../ui/textarea';
import {
  runAgentConversation,
  saveAgentDirectInput,
  saveAgentPush,
  saveAgentSettings,
  type AgentDirectInputRule,
  type AgentLLMProvider,
  type AgentSettings,
  type AgentSnapshot,
} from '../../lib/agent';
import { Field, FieldGrid, ToggleField, numberValue, parseOptionalNumber } from './AgentFormFields';
import type { AgentRunner } from './AgentWorkspace';

type Props = {
  snapshot: AgentSnapshot;
  busy: string;
  onRun: AgentRunner;
};

type PushUser = Record<string, unknown>;
type PushTask = Record<string, unknown>;

const newId = (prefix: string) => `${prefix}-${Date.now()}`;
const textOf = (value: unknown) => (typeof value === 'string' ? value : '');
const valueText = (value: unknown) => (typeof value === 'number' && Number.isFinite(value) ? String(value) : textOf(value));
const boolOf = (value: unknown) => value === true;

const emptyProvider: AgentLLMProvider = {
  id: '',
  name: '',
  type: 'openai-compatible',
  base_url: '',
  api_key: '',
  model: '',
  chat_path: '',
};

const emptyRule: AgentDirectInputRule = {
  id: '',
  name: '',
  pattern: '',
  target_text: '',
  match_mode: 'exact',
  enabled: true,
};

export function AgentRuntimePanel({ snapshot, busy, onRun }: Props) {
  const [provider, setProvider] = useState<AgentLLMProvider>(snapshot.settings.llm_providers[0] ?? emptyProvider);
  const [defaultProviderId, setDefaultProviderId] = useState(snapshot.settings.default_llm_provider_id ?? '');
  const [terminalEnabled, setTerminalEnabled] = useState(snapshot.settings.terminal.enabled);
  const [terminalCwd, setTerminalCwd] = useState(snapshot.settings.terminal.cwd ?? '');
  const [terminalTimeout, setTerminalTimeout] = useState(numberValue(snapshot.settings.terminal.timeout_ms));
  const [sttEnabled, setSttEnabled] = useState(boolOf(snapshot.settings.stt?.enabled));
  const [sttProvider, setSttProvider] = useState(textOf(snapshot.settings.stt?.provider));
  const [sttCommand, setSttCommand] = useState(textOf(snapshot.settings.stt?.command));
  const [sttTimeout, setSttTimeout] = useState(numberValue(snapshot.settings.stt?.timeout_ms));
  const [memoryEnabled, setMemoryEnabled] = useState(boolOf(snapshot.settings.memory?.enabled));
  const [memoryRounds, setMemoryRounds] = useState(numberValue(snapshot.settings.memory?.compact_every_rounds));
  const [md2imgEnabled, setMd2imgEnabled] = useState(boolOf(snapshot.settings.md2img?.enabled));
  const [md2imgMode, setMd2imgMode] = useState(textOf(snapshot.settings.md2img?.default_mode) || 'long-image');
  const [md2imgOutputDir, setMd2imgOutputDir] = useState(textOf(snapshot.settings.md2img?.output_dir));
  const [md2imgTimeout, setMd2imgTimeout] = useState(numberValue(snapshot.settings.md2img?.timeout_ms));
  const [rule, setRule] = useState<AgentDirectInputRule>(snapshot.direct_input.rules[0] ?? emptyRule);
  const [pushUser, setPushUser] = useState<PushUser>(snapshot.push.users[0] ?? { id: '', name: '', wecom_user: '', enabled: true });
  const [pushTask, setPushTask] = useState<PushTask>(
    snapshot.push.tasks[0] ?? { id: '', name: '', user_id: '', text: '', interval_minutes: 60, enabled: true },
  );
  const [conversationInput, setConversationInput] = useState('');

  useEffect(() => {
    setProvider(snapshot.settings.llm_providers[0] ?? emptyProvider);
    setDefaultProviderId(snapshot.settings.default_llm_provider_id ?? '');
    setTerminalEnabled(snapshot.settings.terminal.enabled);
    setTerminalCwd(snapshot.settings.terminal.cwd ?? '');
    setTerminalTimeout(numberValue(snapshot.settings.terminal.timeout_ms));
    setSttEnabled(boolOf(snapshot.settings.stt?.enabled));
    setSttProvider(textOf(snapshot.settings.stt?.provider));
    setSttCommand(textOf(snapshot.settings.stt?.command));
    setSttTimeout(numberValue(snapshot.settings.stt?.timeout_ms));
    setMemoryEnabled(boolOf(snapshot.settings.memory?.enabled));
    setMemoryRounds(numberValue(snapshot.settings.memory?.compact_every_rounds));
    setMd2imgEnabled(boolOf(snapshot.settings.md2img?.enabled));
    setMd2imgMode(textOf(snapshot.settings.md2img?.default_mode) || 'long-image');
    setMd2imgOutputDir(textOf(snapshot.settings.md2img?.output_dir));
    setMd2imgTimeout(numberValue(snapshot.settings.md2img?.timeout_ms));
    setRule(snapshot.direct_input.rules[0] ?? emptyRule);
    setPushUser(snapshot.push.users[0] ?? { id: '', name: '', wecom_user: '', enabled: true });
    setPushTask(snapshot.push.tasks[0] ?? { id: '', name: '', user_id: '', text: '', interval_minutes: 60, enabled: true });
  }, [snapshot]);

  const currentProviderId = provider.id || defaultProviderId;
  const providerCount = snapshot.settings.llm_providers.length;
  const ruleCount = snapshot.direct_input.rules.length;

  const saveProvider = () => {
    const id = provider.id.trim() || newId('llm');
    const nextProvider = { ...provider, id, timeout_ms: parseOptionalNumber(String(provider.timeout_ms ?? '')) };
    const providers = replaceById(snapshot.settings.llm_providers, nextProvider);
    onRun(
      'settings-save',
      () => saveAgentSettings({ ...snapshot.settings, llm_providers: providers, default_llm_provider_id: defaultProviderId || id }),
      false,
    );
  };

  const removeProvider = () => {
    const providers = snapshot.settings.llm_providers.filter((item) => item.id !== provider.id);
    onRun('settings-save', () => saveAgentSettings({ ...snapshot.settings, llm_providers: providers }), false);
  };

  const saveExecutionSettings = () => {
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
        provider: sttProvider.trim(),
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
        default_mode: md2imgMode.trim() || 'long-image',
        output_dir: md2imgOutputDir.trim() || undefined,
        timeout_ms: parseOptionalNumber(md2imgTimeout),
      },
    };
    onRun('settings-save', () => saveAgentSettings(settings), false);
  };

  const saveRule = () => {
    const id = rule.id.trim() || newId('direct');
    onRun('direct-save', () => saveAgentDirectInput({ ...snapshot.direct_input, rules: replaceById(snapshot.direct_input.rules, { ...rule, id }) }), false);
  };

  const deleteRule = () => {
    onRun('direct-save', () => saveAgentDirectInput({ ...snapshot.direct_input, rules: snapshot.direct_input.rules.filter((item) => item.id !== rule.id) }), false);
  };

  const savePushUser = () => {
    const id = textOf(pushUser.id).trim() || newId('user');
    onRun('push-save', () => saveAgentPush({ ...snapshot.push, users: replaceRecordById(snapshot.push.users, { ...pushUser, id }) }), false);
  };

  const deletePushUser = () => {
    onRun('push-save', () => saveAgentPush({ ...snapshot.push, users: snapshot.push.users.filter((item) => textOf(item.id) !== textOf(pushUser.id)) }), false);
  };

  const savePushTask = () => {
    const id = textOf(pushTask.id).trim() || newId('task');
    const interval = parseOptionalNumber(valueText(pushTask.interval_minutes));
    onRun(
      'push-save',
      () => saveAgentPush({ ...snapshot.push, tasks: replaceRecordById(snapshot.push.tasks, { ...pushTask, id, interval_minutes: interval }) }),
      false,
    );
  };

  const deletePushTask = () => {
    onRun('push-save', () => saveAgentPush({ ...snapshot.push, tasks: snapshot.push.tasks.filter((item) => textOf(item.id) !== textOf(pushTask.id)) }), false);
  };

  const activeConversations = useMemo(() => snapshot.conversations.slice(0, 3), [snapshot.conversations]);

  return (
    <Tabs defaultValue="llm">
      <TabsList className="flex-wrap justify-start">
        <TabsTrigger value="llm">LLM</TabsTrigger>
        <TabsTrigger value="execution">Execution</TabsTrigger>
        <TabsTrigger value="direct">Direct Input</TabsTrigger>
        <TabsTrigger value="push">Push</TabsTrigger>
        <TabsTrigger value="conversation">Conversation</TabsTrigger>
      </TabsList>

      <TabsContent value="llm" className="grid grid--two">
        <Card className="panel">
          <CardHeader>
            <CardTitle>LLM Providers</CardTitle>
            <CardDescription>{providerCount} configured providers</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <div className="button-row">
              {snapshot.settings.llm_providers.map((item) => (
                <Button key={item.id} variant={item.id === currentProviderId ? 'default' : 'secondary'} onClick={() => setProvider(item)}>
                  {item.name || item.id}
                </Button>
              ))}
              <Button variant="secondary" onClick={() => setProvider({ ...emptyProvider, id: newId('llm') })}>
                <Plus className="mr-2 h-4 w-4" />
                New
              </Button>
            </div>
            <FieldGrid>
              <Field label="ID" value={provider.id} onChange={(value) => setProvider({ ...provider, id: value })} />
              <Field label="Name" value={provider.name} onChange={(value) => setProvider({ ...provider, name: value })} />
              <Field label="Type" value={provider.type} onChange={(value) => setProvider({ ...provider, type: value })} />
              <Field label="Model" value={provider.model ?? ''} onChange={(value) => setProvider({ ...provider, model: value })} />
              <Field label="Base URL" value={provider.base_url ?? ''} onChange={(value) => setProvider({ ...provider, base_url: value })} />
              <Field label="Chat Path" value={provider.chat_path ?? ''} onChange={(value) => setProvider({ ...provider, chat_path: value })} />
              <Field label="API Key" value={provider.api_key ?? ''} onChange={(value) => setProvider({ ...provider, api_key: value })} />
              <Field label="Timeout ms" value={numberValue(provider.timeout_ms)} onChange={(value) => setProvider({ ...provider, timeout_ms: parseOptionalNumber(value) })} />
            </FieldGrid>
            <div className="button-row">
              <Button onClick={saveProvider} disabled={busy === 'settings-save'}>
                <Save className="mr-2 h-4 w-4" />
                Save Provider
              </Button>
              <Button
                variant="secondary"
                onClick={() => {
                  setDefaultProviderId(provider.id);
                  onRun('settings-save', () => saveAgentSettings({ ...snapshot.settings, default_llm_provider_id: provider.id }), false);
                }}
                disabled={!provider.id}
              >
                Set Default
              </Button>
              <Button variant="danger" onClick={removeProvider} disabled={!provider.id}>
                <Trash2 className="mr-2 h-4 w-4" />
                Delete
              </Button>
              <Badge tone="accent">Default {defaultProviderId || 'unset'}</Badge>
            </div>
          </CardContent>
        </Card>
      </TabsContent>

      <TabsContent value="execution" className="grid grid--two">
        <Card className="panel">
          <CardHeader>
            <CardTitle>Execution Settings</CardTitle>
            <CardDescription>Terminal, STT, memory, and markdown rendering runtime controls</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <ToggleField label="Terminal enabled" checked={terminalEnabled} onChange={setTerminalEnabled} />
            <FieldGrid>
              <Field label="Terminal cwd" value={terminalCwd} onChange={setTerminalCwd} />
              <Field label="Terminal timeout ms" value={terminalTimeout} onChange={setTerminalTimeout} />
            </FieldGrid>
            <ToggleField label="Speech transcription enabled" checked={sttEnabled} onChange={setSttEnabled} />
            <FieldGrid>
              <Field label="STT provider" value={sttProvider} onChange={setSttProvider} />
              <Field label="STT command" value={sttCommand} onChange={setSttCommand} />
              <Field label="STT timeout ms" value={sttTimeout} onChange={setSttTimeout} />
            </FieldGrid>
            <ToggleField label="Memory enabled" checked={memoryEnabled} onChange={setMemoryEnabled} />
            <Field label="Compact every rounds" value={memoryRounds} onChange={setMemoryRounds} />
            <ToggleField label="Markdown image renderer enabled" checked={md2imgEnabled} onChange={setMd2imgEnabled} />
            <FieldGrid>
              <Field label="md2img mode" value={md2imgMode} onChange={setMd2imgMode} />
              <Field label="Output directory" value={md2imgOutputDir} onChange={setMd2imgOutputDir} />
              <Field label="md2img timeout ms" value={md2imgTimeout} onChange={setMd2imgTimeout} />
            </FieldGrid>
            <Button onClick={saveExecutionSettings} disabled={busy === 'settings-save'}>
              <Save className="mr-2 h-4 w-4" />
              Save Runtime
            </Button>
          </CardContent>
        </Card>
      </TabsContent>

      <TabsContent value="direct" className="grid grid--two">
        <Card className="panel">
          <CardHeader>
            <CardTitle>Direct Input Rules</CardTitle>
            <CardDescription>{ruleCount} fixed mappings before LLM planning</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <div className="button-row">
              {snapshot.direct_input.rules.map((item) => (
                <Button key={item.id} variant={item.id === rule.id ? 'default' : 'secondary'} onClick={() => setRule(item)}>
                  {item.name || item.id}
                </Button>
              ))}
              <Button variant="secondary" onClick={() => setRule({ ...emptyRule, id: newId('direct') })}>
                <Plus className="mr-2 h-4 w-4" />
                New
              </Button>
            </div>
            <ToggleField label="Rule enabled" checked={rule.enabled} onChange={(enabled) => setRule({ ...rule, enabled })} />
            <FieldGrid>
              <Field label="ID" value={rule.id} onChange={(value) => setRule({ ...rule, id: value })} />
              <Field label="Name" value={rule.name} onChange={(value) => setRule({ ...rule, name: value })} />
              <Field label="Pattern" value={rule.pattern} onChange={(value) => setRule({ ...rule, pattern: value })} />
              <Field label="Match mode" value={rule.match_mode} onChange={(value) => setRule({ ...rule, match_mode: value === 'fuzzy' ? 'fuzzy' : 'exact' })} />
            </FieldGrid>
            <Textarea value={rule.target_text} onChange={(event) => setRule({ ...rule, target_text: event.target.value })} placeholder="Resolved input text" />
            <div className="button-row">
              <Button onClick={saveRule} disabled={busy === 'direct-save'}>
                <Save className="mr-2 h-4 w-4" />
                Save Rule
              </Button>
              <Button variant="danger" onClick={deleteRule} disabled={!rule.id}>
                <Trash2 className="mr-2 h-4 w-4" />
                Delete
              </Button>
            </div>
          </CardContent>
        </Card>
      </TabsContent>

      <TabsContent value="push" className="grid grid--two">
        <PushEditor
          kind="user"
          title="Push Users"
          items={snapshot.push.users}
          draft={pushUser}
          setDraft={setPushUser}
          onNew={() => setPushUser({ id: newId('user'), name: '', wecom_user: '', enabled: true })}
          onSave={savePushUser}
          onDelete={deletePushUser}
        />
        <PushEditor
          kind="task"
          title="Push Tasks"
          items={snapshot.push.tasks}
          draft={pushTask}
          setDraft={setPushTask}
          onNew={() => setPushTask({ id: newId('task'), name: '', user_id: '', text: '', interval_minutes: 60, enabled: true })}
          onSave={savePushTask}
          onDelete={deletePushTask}
        />
      </TabsContent>

      <TabsContent value="conversation" className="grid grid--two">
        <Card className="panel">
          <CardHeader>
            <CardTitle>Conversation</CardTitle>
            <CardDescription>{snapshot.conversations.length} retained turns</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <Textarea value={conversationInput} onChange={(event) => setConversationInput(event.target.value)} placeholder="Input for the agent runtime" />
            <Button onClick={() => onRun('conversation', () => runAgentConversation({ input: conversationInput }))} disabled={!conversationInput.trim() || busy === 'conversation'}>
              <Play className="mr-2 h-4 w-4" />
              Run
            </Button>
          </CardContent>
        </Card>
        <Card className="panel">
          <CardHeader>
            <CardTitle>Recent Turns</CardTitle>
            <CardDescription>Last three conversation records</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            {activeConversations.map((item) => (
              <div key={item.id} className="rounded-md border border-border-light p-3 text-sm">
                <div className="button-row">
                  <Badge tone={item.status === 'ok' ? 'good' : 'neutral'}>{item.status}</Badge>
                  <span className="text-muted-foreground">{item.created_at}</span>
                </div>
                <p className="font-medium">{item.input}</p>
                <p className="text-muted-foreground">{item.response}</p>
              </div>
            ))}
          </CardContent>
        </Card>
      </TabsContent>
    </Tabs>
  );
}

function PushEditor(props: {
  kind: 'user' | 'task';
  title: string;
  items: Array<Record<string, unknown>>;
  draft: Record<string, unknown>;
  setDraft: (next: Record<string, unknown>) => void;
  onNew: () => void;
  onSave: () => void;
  onDelete: () => void;
}) {
  return (
    <Card className="panel">
      <CardHeader>
        <CardTitle>{props.title}</CardTitle>
        <CardDescription>{props.items.length} configured entries</CardDescription>
      </CardHeader>
      <CardContent className="stack">
        <div className="button-row">
          {props.items.map((item) => (
            <Button key={textOf(item.id)} variant={textOf(item.id) === textOf(props.draft.id) ? 'default' : 'secondary'} onClick={() => props.setDraft(item)}>
              {textOf(item.name) || textOf(item.id)}
            </Button>
          ))}
          <Button variant="secondary" onClick={props.onNew}>
            <Plus className="mr-2 h-4 w-4" />
            New
          </Button>
        </div>
        <ToggleField label="Enabled" checked={boolOf(props.draft.enabled)} onChange={(enabled) => props.setDraft({ ...props.draft, enabled })} />
        <FieldGrid>
          <Field label="ID" value={textOf(props.draft.id)} onChange={(value) => props.setDraft({ ...props.draft, id: value })} />
          <Field label="Name" value={textOf(props.draft.name)} onChange={(value) => props.setDraft({ ...props.draft, name: value })} />
          {props.kind === 'user' ? (
            <Field label="WeCom User" value={textOf(props.draft.wecom_user)} onChange={(value) => props.setDraft({ ...props.draft, wecom_user: value })} />
          ) : null}
          {props.kind === 'task' ? (
            <>
              <Field label="User ID" value={textOf(props.draft.user_id)} onChange={(value) => props.setDraft({ ...props.draft, user_id: value })} />
              <Field label="Interval minutes" value={valueText(props.draft.interval_minutes)} onChange={(value) => props.setDraft({ ...props.draft, interval_minutes: value })} />
            </>
          ) : null}
        </FieldGrid>
        {props.kind === 'task' ? (
          <Textarea value={textOf(props.draft.text)} onChange={(event) => props.setDraft({ ...props.draft, text: event.target.value })} placeholder="Push text" />
        ) : null}
        <Button onClick={props.onSave}>
          <Save className="mr-2 h-4 w-4" />
          Save
        </Button>
        <Button variant="danger" onClick={props.onDelete} disabled={!props.draft.id}>
          <Trash2 className="mr-2 h-4 w-4" />
          Delete
        </Button>
      </CardContent>
    </Card>
  );
}

function replaceById<T extends { id: string }>(items: T[], next: T): T[] {
  return items.some((item) => item.id === next.id) ? items.map((item) => (item.id === next.id ? next : item)) : [...items, next];
}

function replaceRecordById(items: Array<Record<string, unknown>>, next: Record<string, unknown>) {
  const id = textOf(next.id);
  return items.some((item) => textOf(item.id) === id) ? items.map((item) => (textOf(item.id) === id ? next : item)) : [...items, next];
}
