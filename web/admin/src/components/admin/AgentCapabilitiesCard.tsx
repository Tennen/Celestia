import { Play, Puzzle } from 'lucide-react';
import { useMemo, useState } from 'react';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { Input } from '../ui/input';
import { Textarea } from '../ui/textarea';
import type { AgentCapabilityInfo } from '../../lib/agent';

export function AgentCapabilitiesCard(props: {
  capabilities: AgentCapabilityInfo[];
  busy: boolean;
  onRun: (name: string, input: string) => void;
}) {
  const [selected, setSelected] = useState(props.capabilities[0]?.name ?? '');
  const [input, setInput] = useState('');
  const current = useMemo(
    () => props.capabilities.find((item) => item.name === selected) ?? props.capabilities[0],
    [props.capabilities, selected],
  );

  return (
    <Card className="panel">
      <CardHeader>
        <CardTitle>Agent Capabilities</CardTitle>
        <CardDescription>{props.capabilities.length} built-in capabilities</CardDescription>
      </CardHeader>
      <CardContent className="stack">
        <div className="grid gap-2 sm:grid-cols-2">
          {props.capabilities.map((item) => (
            <Button
              key={item.name}
              type="button"
              variant={item.name === current?.name ? 'default' : 'secondary'}
              onClick={() => setSelected(item.name)}
            >
              <Puzzle className="mr-2 h-4 w-4" />
              {item.name}
            </Button>
          ))}
        </div>
        {current ? (
          <div className="stack rounded-md border border-border-light p-3">
            <div className="button-row">
              <Badge tone={current.terminal ? 'warn' : 'good'}>{current.terminal ? 'Terminal' : 'Core'}</Badge>
              {current.command ? <Badge tone="neutral">{current.command}</Badge> : null}
            </div>
            <p className="text-sm text-muted-foreground">{current.description}</p>
            <Input value={selected} onChange={(event) => setSelected(event.target.value)} />
            <Textarea value={input} onChange={(event) => setInput(event.target.value)} placeholder="Capability input" />
            <Button disabled={!selected || props.busy} onClick={() => props.onRun(selected, input)}>
              <Play className="mr-2 h-4 w-4" />
              Run
            </Button>
          </div>
        ) : null}
      </CardContent>
    </Card>
  );
}
