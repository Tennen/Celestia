import { useEffect, useState } from 'react';
import { fetchEvents } from '../../lib/api';
import type { AuditRecord, EventRecord } from '../../lib/types';
import { formatTime, prettyJson } from '../../lib/utils';
import { VisionEventCaptureGallery, visionEventCapturesFromPayload } from './VisionEventCaptureGallery';
import { VisionEventDecisionCard } from './VisionEventDecisionCard';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { Input } from '../ui/input';
import { ScrollArea } from '../ui/scroll-area';
import { Section } from '../ui/section';

type Props = {
  audits: AuditRecord[];
};

const EVENT_PAGE_SIZE = 100;

type EventCursor = {
  beforeTs?: string;
  beforeId?: string;
};

function parseLocalDate(value: string) {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!match) {
    return null;
  }
  const year = Number(match[1]);
  const month = Number(match[2]);
  const day = Number(match[3]);
  if (!Number.isFinite(year) || !Number.isFinite(month) || !Number.isFinite(day)) {
    return null;
  }
  return new Date(year, month - 1, day, 0, 0, 0, 0);
}

function toStartOfDayISO(value: string) {
  const date = parseLocalDate(value);
  return date ? date.toISOString() : undefined;
}

function toExclusiveEndOfDayISO(value: string) {
  const date = parseLocalDate(value);
  if (!date) {
    return undefined;
  }
  date.setDate(date.getDate() + 1);
  return date.toISOString();
}

function dateRangeLabel(fromDate: string, toDate: string) {
  if (fromDate && toDate) {
    return `${fromDate} to ${toDate}`;
  }
  if (fromDate) {
    return `from ${fromDate}`;
  }
  if (toDate) {
    return `through ${toDate}`;
  }
  return 'all dates';
}

export function ActivitySection({ audits }: Props) {
  const [events, setEvents] = useState<EventRecord[]>([]);
  const [loadingEvents, setLoadingEvents] = useState(true);
  const [eventsError, setEventsError] = useState<string | null>(null);
  const [draftFromDate, setDraftFromDate] = useState('');
  const [draftToDate, setDraftToDate] = useState('');
  const [appliedFromDate, setAppliedFromDate] = useState('');
  const [appliedToDate, setAppliedToDate] = useState('');
  const [pageCursors, setPageCursors] = useState<EventCursor[]>([{}]);
  const [pageIndex, setPageIndex] = useState(0);
  const [hasOlderPage, setHasOlderPage] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);

  const currentCursor = pageCursors[pageIndex] ?? {};
  const hasAppliedDateFilter = appliedFromDate !== '' || appliedToDate !== '';
  const hasDraftChanges = draftFromDate !== appliedFromDate || draftToDate !== appliedToDate;
  const canApplyFilter = hasDraftChanges || pageIndex > 0;
  const activeRange = dateRangeLabel(appliedFromDate, appliedToDate);

  useEffect(() => {
    let cancelled = false;
    setLoadingEvents(true);
    setEventsError(null);

    void fetchEvents({
      limit: EVENT_PAGE_SIZE + 1,
      fromTs: toStartOfDayISO(appliedFromDate),
      toTs: toExclusiveEndOfDayISO(appliedToDate),
      beforeTs: currentCursor.beforeTs,
      beforeId: currentCursor.beforeId,
    })
      .then((items) => {
        if (cancelled) {
          return;
        }
        setEvents(items.slice(0, EVENT_PAGE_SIZE));
        setHasOlderPage(items.length > EVENT_PAGE_SIZE);
        setLoadingEvents(false);
      })
      .catch((error: unknown) => {
        if (cancelled) {
          return;
        }
        setEvents([]);
        setHasOlderPage(false);
        setEventsError(error instanceof Error ? error.message : 'Failed to load events');
        setLoadingEvents(false);
      });

    return () => {
      cancelled = true;
    };
  }, [appliedFromDate, appliedToDate, currentCursor.beforeId, currentCursor.beforeTs, reloadKey]);

  const applyDateFilter = () => {
    const fromTs = toStartOfDayISO(draftFromDate);
    const toTs = toExclusiveEndOfDayISO(draftToDate);
    if ((draftFromDate && !fromTs) || (draftToDate && !toTs)) {
      setEventsError('Enter valid dates before applying the filter.');
      return;
    }
    if (fromTs && toTs && fromTs >= toTs) {
      setEventsError('End date must be on or after start date.');
      return;
    }
    setEventsError(null);
    setAppliedFromDate(draftFromDate);
    setAppliedToDate(draftToDate);
    setPageCursors([{}]);
    setPageIndex(0);
  };

  const clearDateFilter = () => {
    setDraftFromDate('');
    setDraftToDate('');
    setEventsError(null);
    setAppliedFromDate('');
    setAppliedToDate('');
    setPageCursors([{}]);
    setPageIndex(0);
  };

  const loadOlderPage = () => {
    if (!hasOlderPage || events.length === 0) {
      return;
    }
    const lastEvent = events[events.length - 1];
    setPageCursors((current) => [
      ...current.slice(0, pageIndex + 1),
      { beforeTs: lastEvent.ts, beforeId: lastEvent.id },
    ]);
    setPageIndex((current) => current + 1);
  };

  const loadNewerPage = () => {
    setPageIndex((current) => Math.max(0, current - 1));
  };

  return (
    <Section stack={false} className="plugin-workspace xl:grid-cols-2">
      <Card className="explorer-card">
        <CardHeader>
          <CardTitle>Events</CardTitle>
          <CardDescription>Paginated event bus and device activity feed.</CardDescription>
        </CardHeader>
        <CardContent className="explorer-card__content">
          <div className="activity-toolbar">
            <div className="activity-toolbar__filters">
              <Input
                type="date"
                className="activity-toolbar__range"
                value={draftFromDate}
                onChange={(event) => setDraftFromDate(event.target.value)}
                aria-label="Filter events from date"
              />
              <Input
                type="date"
                className="activity-toolbar__range"
                value={draftToDate}
                onChange={(event) => setDraftToDate(event.target.value)}
                aria-label="Filter events to date"
              />
              <Button variant="secondary" size="sm" onClick={applyDateFilter} disabled={!canApplyFilter || loadingEvents}>
                Apply
              </Button>
              <Button
                variant="ghost"
                size="sm"
                onClick={clearDateFilter}
                disabled={!draftFromDate && !draftToDate && !hasAppliedDateFilter && pageIndex === 0}
              >
                Clear
              </Button>
            </div>
            <div className="activity-toolbar__status">
              <Badge tone="neutral" size="sm">
                Page {pageIndex + 1}
              </Badge>
              <Badge tone={hasAppliedDateFilter ? 'accent' : 'neutral'} size="sm">
                {activeRange}
              </Badge>
              <Button variant="secondary" size="sm" onClick={() => setReloadKey((current) => current + 1)} disabled={loadingEvents}>
                Refresh
              </Button>
            </div>
          </div>
          <p className="muted activity-toolbar__support">
            Vision capability &quot;Event Capture Retention Hours&quot; only applies to rule-scoped vision history and captures, not this
            global event feed.
          </p>
          <ScrollArea className="explorer-scroll">
            <div className="feed">
              {events.length > 0 ? (
                events.map((event) => (
                  <article key={event.id} className="feed__item">
                    <div className="feed__meta">
                      <Badge tone="accent" size="sm">
                        {event.type}
                      </Badge>
                      <span>{formatTime(event.ts)}</span>
                    </div>
                    <strong>{event.device_id || event.plugin_id || 'system'}</strong>
                    <VisionEventCaptureGallery captures={visionEventCapturesFromPayload(event.payload)} />
                    <VisionEventDecisionCard metadata={event.payload?.metadata} />
                    <pre className="log-box log-box--wrap">{prettyJson(event.payload ?? {})}</pre>
                  </article>
                ))
              ) : (
                <div className="activity-empty">
                  <p className="muted">
                    {loadingEvents
                      ? 'Loading events...'
                      : eventsError
                        ? eventsError
                        : hasAppliedDateFilter
                          ? 'No events matched the selected date range.'
                          : 'No recent events.'}
                  </p>
                </div>
              )}
            </div>
          </ScrollArea>
          <div className="activity-pagination">
            <p className="muted">
              {loadingEvents ? 'Loading…' : `Showing page ${pageIndex + 1} for ${activeRange}.`}
            </p>
            <div className="button-row">
              <Button variant="secondary" size="sm" onClick={loadNewerPage} disabled={pageIndex === 0 || loadingEvents}>
                Newer
              </Button>
              <Button variant="secondary" size="sm" onClick={loadOlderPage} disabled={!hasOlderPage || loadingEvents}>
                Older
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card className="explorer-card">
        <CardHeader>
          <CardTitle>Audits</CardTitle>
          <CardDescription>Command history and policy decisions.</CardDescription>
        </CardHeader>
        <CardContent className="explorer-card__content">
          <ScrollArea className="explorer-scroll">
            <div className="feed">
              {audits.map((audit) => (
                <article key={audit.id} className="feed__item">
                  <div className="feed__meta">
                    <Badge tone={audit.allowed ? 'good' : 'bad'} size="sm">
                      {audit.allowed ? 'allowed' : 'denied'}
                    </Badge>
                    <Badge
                      size="sm"
                      tone={
                        audit.risk_level === 'high'
                          ? 'bad'
                          : audit.risk_level === 'medium'
                            ? 'warn'
                            : 'neutral'
                      }
                    >
                      {audit.risk_level}
                    </Badge>
                    <span>{formatTime(audit.created_at)}</span>
                  </div>
                  <strong>
                    {audit.actor} · {audit.action}
                  </strong>
                  <p>{audit.device_id}</p>
                  <pre className="log-box log-box--wrap">{prettyJson(audit.params ?? {})}</pre>
                </article>
              ))}
            </div>
          </ScrollArea>
        </CardContent>
      </Card>
    </Section>
  );
}
