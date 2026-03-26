import { Badge } from '../ui/badge';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { Section } from '../ui/section';
import { formatTime, prettyJson } from '../../lib/utils';
import type { AuditRecord, EventRecord } from '../../lib/types';

type Props = {
  events: EventRecord[];
  audits: AuditRecord[];
};

export function ActivitySection({ events, audits }: Props) {
  return (
    <Section className="grid grid--two">
      <Card>
        <CardHeader>
          <CardTitle>Events</CardTitle>
          <CardDescription>Recent event bus and device activity feed.</CardDescription>
        </CardHeader>
        <CardContent className="stack">
          <div className="feed">
            {events.map((event) => (
              <article key={event.id} className="feed__item">
                <div className="feed__meta">
                  <Badge tone="accent">{event.type}</Badge>
                  <span>{formatTime(event.ts)}</span>
                </div>
                <strong>{event.device_id || event.plugin_id || 'system'}</strong>
                <pre>{prettyJson(event.payload ?? {})}</pre>
              </article>
            ))}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Audits</CardTitle>
          <CardDescription>Command history and policy decisions.</CardDescription>
        </CardHeader>
        <CardContent className="stack">
          <div className="feed">
            {audits.map((audit) => (
              <article key={audit.id} className="feed__item">
                <div className="feed__meta">
                  <Badge tone={audit.allowed ? 'good' : 'bad'}>{audit.allowed ? 'allowed' : 'denied'}</Badge>
                  <Badge tone={audit.risk_level === 'high' ? 'bad' : audit.risk_level === 'medium' ? 'warn' : 'neutral'}>
                    {audit.risk_level}
                  </Badge>
                  <span>{formatTime(audit.created_at)}</span>
                </div>
                <strong>
                  {audit.actor} · {audit.action}
                </strong>
                <p>{audit.device_id}</p>
                <pre>{prettyJson(audit.params ?? {})}</pre>
              </article>
            ))}
          </div>
        </CardContent>
      </Card>
    </Section>
  );
}
