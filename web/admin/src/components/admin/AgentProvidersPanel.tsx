import { useEffect, useState, type ReactNode } from 'react';
import { Plus, Save, Trash2 } from 'lucide-react';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../ui/tabs';
import { saveAgentSettings, type AgentLLMProvider, type AgentProvider, type AgentSnapshot } from '../../lib/agent';
import { Field, FieldGrid, SelectField, numberValue, parseOptionalNumber } from './AgentFormFields';
import { AgentSearchPanel } from './AgentSearchPanel';
import type { AgentRunner } from './AgentWorkspace';
import { SelectableListItem } from './shared/SelectableListItem';

type Props = {
  snapshot: AgentSnapshot;
  busy: string;
  onRun: AgentRunner;
};

const llmProviderTypes = [
  { value: 'ollama', label: 'ollama' },
  { value: 'openai', label: 'openai-like' },
  { value: 'gemini', label: 'gemini-like' },
  { value: 'llama-server', label: 'llama-server' },
  { value: 'gpt-plugin', label: 'gpt-plugin' },
];

const reasoningEfforts = [
  { value: '', label: 'follow Codex default' },
  { value: 'minimal', label: 'minimal' },
  { value: 'low', label: 'low' },
  { value: 'medium', label: 'medium' },
  { value: 'high', label: 'high' },
  { value: 'xhigh', label: 'xhigh' },
];

export function AgentProvidersPanel({ snapshot, busy, onRun }: Props) {
  return (
    <Tabs defaultValue="llm" className="agent-tabs">
      <TabsList className="agent-tabs__list flex-wrap">
        <TabsTrigger value="llm">LLM Providers</TabsTrigger>
        <TabsTrigger value="agent">Agent Providers</TabsTrigger>
        <TabsTrigger value="search">Search Providers</TabsTrigger>
      </TabsList>
      <TabsContent value="llm" className="agent-tab-content">
        <LLMProvidersPanel snapshot={snapshot} busy={busy} onRun={onRun} />
      </TabsContent>
      <TabsContent value="agent" className="agent-tab-content">
        <CodexProvidersPanel snapshot={snapshot} busy={busy} onRun={onRun} />
      </TabsContent>
      <TabsContent value="search" className="agent-tab-content">
        <AgentSearchPanel snapshot={snapshot} busy={busy} onRun={onRun} />
      </TabsContent>
    </Tabs>
  );
}

function LLMProvidersPanel({ snapshot, busy, onRun }: Props) {
  const providers = snapshot.settings.llm_providers;
  const [provider, setProvider] = useState<AgentLLMProvider>(providers[0] ?? emptyLLMProvider());

  useEffect(() => {
    setProvider(providers[0] ?? emptyLLMProvider());
  }, [snapshot]);

  const type = normalizeProviderType(provider.type);
  const saveProvider = () => {
    const id = provider.id || slugId(provider.name || provider.model || type, 'llm');
    const next = {
      ...provider,
      id,
      type,
      timeout_ms: parseOptionalNumber(String(provider.timeout_ms ?? '')),
    };
    const defaultId = snapshot.settings.default_llm_provider_id || id;
    setProvider(next);
    onRun(
      'settings-save',
      () => saveAgentSettings({ ...snapshot.settings, llm_providers: replaceById(snapshot.settings.llm_providers, next), default_llm_provider_id: defaultId }),
      false,
    );
  };

  const deleteProvider = () => {
    const nextProviders = snapshot.settings.llm_providers.filter((item) => item.id !== provider.id);
    const nextDefault = snapshot.settings.default_llm_provider_id === provider.id ? nextProviders[0]?.id ?? '' : snapshot.settings.default_llm_provider_id;
    onRun('settings-save', () => saveAgentSettings({ ...snapshot.settings, llm_providers: nextProviders, default_llm_provider_id: nextDefault }), false);
  };

  return (
    <div className="grid grid--two">
      <Card className="panel">
        <CardHeader>
          <CardTitle>LLM Providers</CardTitle>
          <CardDescription>Text model providers used by conversations, planning, and reusable Agent workflows</CardDescription>
        </CardHeader>
        <CardContent className="stack">
          <ProviderList
            providers={providers}
            selectedID={provider.id}
            emptyText="No LLM providers configured."
            describe={(item) => `${normalizeProviderType(item.type)}${item.model ? ` · ${item.model}` : ''}`}
            badge={(item) => (
              <Badge tone={item.id === snapshot.settings.default_llm_provider_id ? 'accent' : 'neutral'} size="xxs">
                {item.id === snapshot.settings.default_llm_provider_id ? 'default' : 'profile'}
              </Badge>
            )}
            onSelect={setProvider}
          />
          <div className="button-row">
            <Button variant="secondary" onClick={() => setProvider(emptyLLMProvider())}>
              <Plus className="mr-2 h-4 w-4" />
              New
            </Button>
          </div>
          <FieldGrid>
            <Field label="Name" value={provider.name} onChange={(name) => setProvider({ ...provider, name })} />
            <SelectField label="Type" value={type} options={llmProviderTypes} onChange={(nextType) => setProvider({ ...provider, type: nextType })} />
            <Field label="Model" value={provider.model ?? ''} onChange={(model) => setProvider({ ...provider, model })} />
            <Field label="Base URL" value={provider.base_url ?? ''} onChange={(base_url) => setProvider({ ...provider, base_url })} />
            {['openai', 'gemini', 'llama-server', 'gpt-plugin'].includes(type) ? (
              <Field label="API Key" value={provider.api_key ?? ''} onChange={(api_key) => setProvider({ ...provider, api_key })} />
            ) : null}
            {type === 'openai' ? <Field label="Chat completions path" value={provider.chat_path ?? ''} onChange={(chat_path) => setProvider({ ...provider, chat_path })} /> : null}
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
            <Button variant="danger" disabled={!provider.id} onClick={deleteProvider}>
              <Trash2 className="mr-2 h-4 w-4" />
              Delete
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card className="panel">
        <CardHeader>
          <CardTitle>Provider Boundary</CardTitle>
          <CardDescription>Codex CLI is configured separately as an Agent Provider, not as a normal LLM profile</CardDescription>
        </CardHeader>
        <CardContent className="stack text-sm text-muted-foreground">
          <p>Use this tab for OpenAI-compatible, Gemini, Ollama, llama-server, and GPT plugin text providers.</p>
          <p>Evolution and Knowledge select Codex through Agent Providers using each module's own provider setting.</p>
        </CardContent>
      </Card>
    </div>
  );
}

function CodexProvidersPanel({ snapshot, busy, onRun }: Props) {
  const providers = snapshot.settings.agent_providers;
  const [provider, setProvider] = useState<AgentProvider>(providers[0] ?? emptyAgentProvider());

  useEffect(() => {
    setProvider(providers[0] ?? emptyAgentProvider());
  }, [snapshot]);

  const saveProvider = () => {
    const id = provider.id || slugId(provider.name || provider.model || 'codex', 'agent');
    const next = {
      ...provider,
      id,
      type: 'codex',
      timeout_ms: parseOptionalNumber(String(provider.timeout_ms ?? '')),
    };
    setProvider(next);
    onRun('settings-save', () => saveAgentSettings({ ...snapshot.settings, agent_providers: replaceById(snapshot.settings.agent_providers, next) }), false);
  };

  return (
    <div className="grid grid--two">
      <Card className="panel">
        <CardHeader>
          <CardTitle>Agent Providers</CardTitle>
          <CardDescription>Codex CLI provider profiles selected by Evolution, Knowledge, and other Agent-owned workflows</CardDescription>
        </CardHeader>
        <CardContent className="stack">
          <ProviderList
            providers={providers}
            selectedID={provider.id}
            emptyText="No Agent providers configured."
            describe={(item) => `codex-cli${item.model ? ` · ${item.model}` : ''}`}
            badge={() => <Badge tone="accent" size="xxs">codex</Badge>}
            onSelect={setProvider}
          />
          <div className="button-row">
            <Button variant="secondary" onClick={() => setProvider(emptyAgentProvider())}>
              <Plus className="mr-2 h-4 w-4" />
              New
            </Button>
          </div>
          <FieldGrid>
            <Field label="Name" value={provider.name} onChange={(name) => setProvider({ ...provider, name })} />
            <Field label="Model" value={provider.model ?? ''} onChange={(model) => setProvider({ ...provider, model })} />
            <SelectField label="Reasoning effort" value={provider.reasoning_effort ?? ''} options={reasoningEfforts} onChange={(reasoning_effort) => setProvider({ ...provider, reasoning_effort })} />
            <Field label="Timeout ms" value={numberValue(provider.timeout_ms)} onChange={(value) => setProvider({ ...provider, timeout_ms: parseOptionalNumber(value) })} />
          </FieldGrid>
          <div className="button-row">
            <Button onClick={saveProvider} disabled={busy === 'settings-save' || !provider.name.trim()}>
              <Save className="mr-2 h-4 w-4" />
              Save Provider
            </Button>
            <Button
              variant="danger"
              disabled={!provider.id}
              onClick={() =>
                onRun(
                  'settings-save',
                  () => saveAgentSettings({ ...snapshot.settings, agent_providers: snapshot.settings.agent_providers.filter((item) => item.id !== provider.id) }),
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
          <CardTitle>Module Usage</CardTitle>
          <CardDescription>Agent providers are selected by the module that invokes Codex</CardDescription>
        </CardHeader>
        <CardContent className="stack text-sm text-muted-foreground">
          <p>Evolution and Knowledge each store their own Agent provider id, so switching one module does not affect the other.</p>
          <p>Reasoning effort is stored on this provider profile and passed to Codex CLI when the module runs.</p>
        </CardContent>
      </Card>
    </div>
  );
}

function ProviderList<T extends AgentLLMProvider | AgentProvider>(props: {
  providers: T[];
  selectedID: string;
  emptyText: string;
  describe: (item: T) => string;
  badge: (item: T) => ReactNode;
  onSelect: (item: T) => void;
}) {
  return (
    <div className="list-stack">
      {props.providers.map((item) => (
        <SelectableListItem
          key={item.id}
          title={item.name || item.model || item.type}
          description={props.describe(item)}
          selected={item.id === props.selectedID}
          badges={props.badge(item)}
          onClick={() => props.onSelect(item)}
        />
      ))}
      {props.providers.length === 0 ? <div className="detail">{props.emptyText}</div> : null}
    </div>
  );
}

function emptyLLMProvider(): AgentLLMProvider {
  return { id: '', name: '', type: 'openai', base_url: '', api_key: '', model: '', chat_path: '' };
}

function emptyAgentProvider(): AgentProvider {
  return { id: '', name: '', type: 'codex', model: '', reasoning_effort: '', timeout_ms: undefined };
}

function normalizeProviderType(type: string) {
  if (type === 'openai-like') return 'openai';
  if (type === 'gemini-like') return 'gemini';
  if (['ollama', 'openai', 'gemini', 'llama-server', 'gpt-plugin'].includes(type)) return type;
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
