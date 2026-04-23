import { useEffect, useState } from 'react';
import { MessageSquare, Plus, RefreshCw, Save, Trash2 } from 'lucide-react';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { ScrollArea } from '../ui/scroll-area';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../ui/tabs';
import { Textarea } from '../ui/textarea';
import {
  fetchAgentSnapshot,
  runAgentConversation,
  saveAgentDirectInput,
  stableJSON,
  type AgentDirectInputRule,
  type AgentSnapshot,
} from '../../lib/agent';
import { Field, FieldGrid, SelectField, ToggleField } from './AgentFormFields';
import { AgentWeComPanel } from './AgentWeComPanel';
import type { AgentRunner } from './AgentWorkspace';
import { SelectableListItem } from './shared/SelectableListItem';

const matchModes = [
  { value: 'exact', label: 'exact' },
  { value: 'fuzzy', label: 'fuzzy' },
];

const emptyRule = (): AgentDirectInputRule => ({
  id: '',
  name: '',
  pattern: '',
  target_text: '',
  match_mode: 'exact',
  enabled: true,
});

export function TouchpointWorkspace() {
  const [snapshot, setSnapshot] = useState<AgentSnapshot | null>(null);
  const [busy, setBusy] = useState('load');
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [result, setResult] = useState('');

  const load = async () => {
    setBusy('load');
    setError('');
    try {
      setSnapshot(await fetchAgentSnapshot());
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load touchpoint state');
    } finally {
      setBusy('');
    }
  };

  useEffect(() => {
    void load();
  }, []);

  const run: AgentRunner = async (label, action, refresh = true) => {
    setBusy(label);
    setError('');
    setNotice('');
    try {
      const output = await action();
      const isSave = label.includes('save');
      setResult(isSave ? '' : stableJSON(output));
      setNotice(isSave ? 'Saved' : 'Done');
      if (refresh) {
        setSnapshot(await fetchAgentSnapshot());
      } else if (output && typeof output === 'object' && 'settings' in output) {
        setSnapshot(output as AgentSnapshot);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Request failed');
    } finally {
      setBusy('');
    }
  };

  if (!snapshot) {
    return (
      <Card className="panel">
        <CardContent className="flex items-center gap-2 p-6">
          <RefreshCw className="h-4 w-4 animate-spin" />
          Loading project touchpoints
        </CardContent>
      </Card>
    );
  }

  return (
    <ScrollArea className="detail-scroll">
      <div className="detail-stack agent-detail-stack">
        <div className="section-title">
          <div>
            <p className="eyebrow">Project Input Layer</p>
            <h2 className="text-2xl font-semibold tracking-tight">Touchpoints</h2>
          </div>
          <div className="toolbar">
            <Badge tone={snapshot.settings.wecom.enabled ? 'good' : 'neutral'}>WeCom</Badge>
            <Badge tone={snapshot.settings.stt?.enabled ? 'good' : 'neutral'}>Voice Provider</Badge>
            <Badge tone={snapshot.direct_input.rules.length ? 'accent' : 'neutral'}>{snapshot.direct_input.rules.length} mappings</Badge>
            <Button variant="secondary" onClick={() => void load()} disabled={busy === 'load'}>
              <RefreshCw className={`mr-2 h-4 w-4 ${busy === 'load' ? 'animate-spin' : ''}`} />
              Refresh
            </Button>
          </div>
        </div>

        {error ? <div className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm">{error}</div> : null}
        {notice ? <div className="rounded-md border border-primary/20 bg-primary/5 p-3 text-sm">{notice}</div> : null}

        <Tabs defaultValue="wecom" className="agent-tabs">
          <TabsList className="agent-tabs__list flex-wrap">
            <TabsTrigger value="wecom">WeCom</TabsTrigger>
            <TabsTrigger value="commands">Slash Commands</TabsTrigger>
            <TabsTrigger value="mappings">Input Mappings</TabsTrigger>
          </TabsList>
          <TabsContent value="wecom" className="agent-tab-content">
            <AgentWeComPanel snapshot={snapshot} busy={busy} onRun={run} />
          </TabsContent>
          <TabsContent value="commands" className="agent-tab-content">
            <SlashCommandPanel busy={busy} onRun={run} />
          </TabsContent>
          <TabsContent value="mappings" className="agent-tab-content">
            <InputMappingPanel snapshot={snapshot} busy={busy} onRun={run} />
          </TabsContent>
        </Tabs>

        {result ? (
          <Card className="panel log-panel agent-result-panel">
            <CardHeader>
              <CardTitle>Last Result</CardTitle>
            </CardHeader>
            <CardContent className="agent-result-content">
              <pre className="agent-result-pre">{result}</pre>
            </CardContent>
          </Card>
        ) : null}
      </div>
    </ScrollArea>
  );
}

function SlashCommandPanel({ busy, onRun }: { busy: string; onRun: AgentRunner }) {
  const [input, setInput] = useState('/home list');
  return (
    <div className="grid grid--two">
      <Card className="panel">
        <CardHeader>
          <CardTitle>Slash Command Smoke Run</CardTitle>
          <CardDescription>Commands are intercepted before the Agent and can execute project workflows directly</CardDescription>
        </CardHeader>
        <CardContent className="stack">
          <Textarea value={input} onChange={(event) => setInput(event.target.value)} />
          <Button onClick={() => onRun('slash-run', () => runAgentConversation({ input }))} disabled={!input.trim() || busy === 'slash-run'}>
            <MessageSquare className="mr-2 h-4 w-4" />
            Run Command
          </Button>
        </CardContent>
      </Card>
      <Card className="panel">
        <CardHeader>
          <CardTitle>Supported Commands</CardTitle>
          <CardDescription>Home and Market workflows do not require LLM intent detection when called this way</CardDescription>
        </CardHeader>
        <CardContent>
          <pre className="agent-result-pre">{`/home list [query]
/home "<device>" <command> [value|key=value ...]
/home "<device-or-room.command>" [value|key=value ...]
/home <command> [value|key=value ...]
/home action "<device>" <raw_action> [key=value ...]
/market portfolio
/market run [open|midday|close] [notes]
/market import <fund codes>`}</pre>
        </CardContent>
      </Card>
    </div>
  );
}

function InputMappingPanel({ snapshot, busy, onRun }: { snapshot: AgentSnapshot; busy: string; onRun: AgentRunner }) {
  const [rule, setRule] = useState<AgentDirectInputRule>(snapshot.direct_input.rules[0] ?? emptyRule());

  useEffect(() => {
    setRule(snapshot.direct_input.rules[0] ?? emptyRule());
  }, [snapshot]);

  const saveRule = () => {
    const id = rule.id || slugId(rule.name || rule.pattern, 'direct');
    setRule({ ...rule, id });
    onRun('direct-save', () => saveAgentDirectInput({ ...snapshot.direct_input, rules: replaceById(snapshot.direct_input.rules, { ...rule, id }) }), false);
  };

  return (
    <div className="grid grid--two">
      <Card className="panel">
        <CardHeader>
          <CardTitle>Input Mappings</CardTitle>
          <CardDescription>Project-level rewrites applied after slash dispatch and before Agent conversation</CardDescription>
        </CardHeader>
        <CardContent className="stack">
          <div className="list-stack">
            {snapshot.direct_input.rules.map((item) => (
              <SelectableListItem
                key={item.id}
                title={item.name || item.pattern}
                description={`${item.match_mode} · ${item.pattern}`}
                selected={item.id === rule.id}
                badges={<Badge tone={item.enabled ? 'good' : 'neutral'} size="xxs">{item.enabled ? 'enabled' : 'disabled'}</Badge>}
                onClick={() => setRule(item)}
              />
            ))}
            {snapshot.direct_input.rules.length === 0 ? <div className="detail">No input mappings configured.</div> : null}
          </div>
        </CardContent>
      </Card>
      <Card className="panel">
        <CardHeader>
          <CardTitle>Mapping Editor</CardTitle>
          <CardDescription>Use slash commands for deterministic workflows; mappings are for plain text aliases</CardDescription>
        </CardHeader>
        <CardContent className="stack">
          <div className="button-row">
            <Button variant="secondary" onClick={() => setRule(emptyRule())}>
              <Plus className="mr-2 h-4 w-4" />
              New
            </Button>
          </div>
          <ToggleField label="Rule enabled" checked={rule.enabled} onChange={(enabled) => setRule({ ...rule, enabled })} />
          <FieldGrid>
            <Field label="Name" value={rule.name} onChange={(name) => setRule({ ...rule, name })} />
            <Field label="Input pattern" value={rule.pattern} onChange={(pattern) => setRule({ ...rule, pattern })} />
            <SelectField
              label="Match mode"
              value={rule.match_mode}
              options={matchModes}
              onChange={(match_mode) => setRule({ ...rule, match_mode: match_mode === 'fuzzy' ? 'fuzzy' : 'exact' })}
            />
          </FieldGrid>
          <Textarea value={rule.target_text} onChange={(event) => setRule({ ...rule, target_text: event.target.value })} placeholder="Target text for the Agent input" />
          <div className="button-row">
            <Button onClick={saveRule} disabled={busy === 'direct-save' || !rule.pattern.trim() || !rule.target_text.trim()}>
              <Save className="mr-2 h-4 w-4" />
              Save Rule
            </Button>
            <Button
              variant="danger"
              onClick={() => onRun('direct-save', () => saveAgentDirectInput({ ...snapshot.direct_input, rules: snapshot.direct_input.rules.filter((item) => item.id !== rule.id) }), false)}
              disabled={!rule.id}
            >
              <Trash2 className="mr-2 h-4 w-4" />
              Delete
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
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
