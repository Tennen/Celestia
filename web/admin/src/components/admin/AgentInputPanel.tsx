import { useMemo, useState } from 'react';
import { ChevronDown, MessageSquare } from 'lucide-react';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { Textarea } from '../ui/textarea';
import {
  runAgentConversation,
  type AgentProcessLogEvent,
  type AgentSnapshot,
} from '../../lib/agent';
import type { AgentRunner } from './AgentWorkspace';

type Props = {
  snapshot: AgentSnapshot;
  busy: string;
  onRun: AgentRunner;
};

export function AgentInputPanel({ snapshot, busy, onRun }: Props) {
  const [conversationInput, setConversationInput] = useState('');
  const activeConversations = useMemo(() => snapshot.conversations.slice(0, 3), [snapshot.conversations]);

  return (
    <div className="grid grid--two">
      <Card className="panel">
        <CardHeader>
          <CardTitle>Conversation Input</CardTitle>
          <CardDescription>Manual HTTP input goes through project slash dispatch first, then the Eino Agent when needed</CardDescription>
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
