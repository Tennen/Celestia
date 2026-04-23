import { useEffect, useMemo, useState } from 'react';
import { ChevronDown, MessageSquare, Plus, Save, Trash2 } from 'lucide-react';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { Textarea } from '../ui/textarea';
import {
  runAgentConversation,
  saveAgentDirectInput,
  type AgentDirectInputRule,
  type AgentProcessLogEvent,
  type AgentSnapshot,
} from '../../lib/agent';
import { Field, FieldGrid, SelectField, ToggleField } from './AgentFormFields';
import type { AgentRunner } from './AgentWorkspace';
import { SelectableListItem } from './shared/SelectableListItem';

type Props = {
  snapshot: AgentSnapshot;
  busy: string;
  onRun: AgentRunner;
};

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

export function AgentInputPanel({ snapshot, busy, onRun }: Props) {
  const [rule, setRule] = useState<AgentDirectInputRule>(snapshot.direct_input.rules[0] ?? emptyRule());
  const [conversationInput, setConversationInput] = useState('');

  useEffect(() => {
    setRule(snapshot.direct_input.rules[0] ?? emptyRule());
  }, [snapshot]);

  const saveRule = () => {
    const id = rule.id || slugId(rule.name || rule.pattern, 'direct');
    setRule({ ...rule, id });
    onRun('direct-save', () => saveAgentDirectInput({ ...snapshot.direct_input, rules: replaceById(snapshot.direct_input.rules, { ...rule, id }) }), false);
  };

  const activeConversations = useMemo(() => snapshot.conversations.slice(0, 3), [snapshot.conversations]);

  return (
    <div className="grid grid--two">
      <Card className="panel">
        <CardHeader>
          <CardTitle>Direct Input Mapping</CardTitle>
          <CardDescription>Maps friendly text to an Agent command before normal orchestration</CardDescription>
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
                onClick={() => {
                  setRule(item);
                }}
              />
            ))}
            {snapshot.direct_input.rules.length === 0 ? <div className="detail">No direct input mappings configured.</div> : null}
          </div>
          <div className="button-row">
            <Button
              variant="secondary"
              onClick={() => {
                setRule(emptyRule());
              }}
            >
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
          <Textarea value={rule.target_text} onChange={(event) => setRule({ ...rule, target_text: event.target.value })} placeholder="Target command or rewritten input" />
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

      <Card className="panel">
        <CardHeader>
          <CardTitle>Conversation Input</CardTitle>
          <CardDescription>Manual input enters the same orchestration path as HTTP or WeCom text</CardDescription>
        </CardHeader>
        <CardContent className="stack">
          <Textarea value={conversationInput} onChange={(event) => setConversationInput(event.target.value)} placeholder="Input for the Agent runtime" />
          <Button onClick={() => onRun('conversation', () => runAgentConversation({ input: conversationInput }))} disabled={!conversationInput.trim() || busy === 'conversation'}>
            <MessageSquare className="mr-2 h-4 w-4" />
            Send To Agent
          </Button>
          {activeConversations.map((item) => (
            <div key={item.id} className="rounded-md border border-border-light p-3 text-sm">
              <div className="button-row">
                <Badge tone={item.status === 'succeeded' ? 'good' : item.status === 'failed' ? 'bad' : 'neutral'}>{item.status}</Badge>
                <span className="text-muted-foreground">{item.created_at}</span>
              </div>
              <p className="font-medium">{item.input}</p>
              <p className="text-muted-foreground">{item.response}</p>
              <ConversationProcessLog events={item.metadata?.process_log} />
            </div>
          ))}
        </CardContent>
      </Card>
    </div>
  );
}

function ConversationProcessLog({ events }: { events?: AgentProcessLogEvent[] }) {
  if (!Array.isArray(events) || events.length === 0) {
    return null;
  }
  return (
    <details className="mt-3 rounded-md border bg-secondary/25 p-2 text-xs">
      <summary className="flex cursor-pointer list-none items-center justify-between gap-2 text-muted-foreground">
        <span>Process calls ({events.length})</span>
        <ChevronDown className="h-3.5 w-3.5" />
      </summary>
      <div className="mt-2 grid gap-2">
        {events.map((event) => (
          <div key={event.id} className="rounded-md bg-background/70 p-2">
            <div className="flex min-w-0 flex-wrap items-center justify-between gap-2">
              <div className="min-w-0 break-words font-medium">
                {event.kind}
                {event.name ? ` / ${event.name}` : ''}
              </div>
              <Badge tone={event.status === 'succeeded' ? 'good' : event.status === 'failed' ? 'bad' : 'warn'} size="xxs">
                {event.status}
              </Badge>
            </div>
            <div className="mt-1 text-muted-foreground">{event.duration_ms}ms</div>
            {event.input ? <pre className="mt-2 max-h-28 overflow-auto whitespace-pre-wrap break-words rounded bg-secondary/40 p-2">{event.input}</pre> : null}
            {event.error ? <div className="mt-2 break-words text-destructive">{event.error}</div> : null}
          </div>
        ))}
      </div>
    </details>
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
