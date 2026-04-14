import { useEffect, useMemo, useState } from 'react';
import { ArrowLeft, RefreshCcw } from 'lucide-react';
import { deleteVisionRuleEvent, fetchVisionRuleEvents } from '../../lib/api';
import type { EventRecord, VisionRule } from '../../lib/types';
import { Button } from '../ui/button';
import { Card, CardContent, CardHeader } from '../ui/card';
import { Input } from '../ui/input';
import { ScrollArea } from '../ui/scroll-area';
import { buildHistoryEventView, type HistoryEntity, VisionHistoryEventCard, VisionHistoryEventModal } from './VisionRuleEventHistoryCards';
import { CardHeading } from './shared/CardHeading';

type Props = {
  onBack: () => void;
  onError: (message: string) => void;
  rule: VisionRule;
  updatedAtKey: string;
};

type EventCursor = {
  beforeTs?: string;
  beforeId?: string;
};

const HISTORY_PAGE_SIZE = 50;

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

export function VisionRuleEventHistoryPanel({ onBack, onError, rule, updatedAtKey }: Props) {
  const [events, setEvents] = useState<EventRecord[]>([]);
  const [openEventId, setOpenEventId] = useState('');
  const [selectedEntityKey, setSelectedEntityKey] = useState('');
  const [busy, setBusy] = useState<'load' | 'refresh' | ''>('load');
  const [deletingEventId, setDeletingEventId] = useState('');
  const [draftFromDate, setDraftFromDate] = useState('');
  const [draftToDate, setDraftToDate] = useState('');
  const [appliedFromDate, setAppliedFromDate] = useState('');
  const [appliedToDate, setAppliedToDate] = useState('');
  const [pageCursors, setPageCursors] = useState<EventCursor[]>([{}]);
  const [pageIndex, setPageIndex] = useState(0);
  const [hasOlderPage, setHasOlderPage] = useState(false);

  const currentCursor = pageCursors[pageIndex] ?? {};
  const hasAppliedDateFilter = appliedFromDate !== '' || appliedToDate !== '';
  const hasDraftChanges = draftFromDate !== appliedFromDate || draftToDate !== appliedToDate;

  useEffect(() => {
    setEvents([]);
    setOpenEventId('');
    setSelectedEntityKey('');
    setDeletingEventId('');
    setDraftFromDate('');
    setDraftToDate('');
    setAppliedFromDate('');
    setAppliedToDate('');
    setPageCursors([{}]);
    setPageIndex(0);
    setHasOlderPage(false);
  }, [rule.id]);

  useEffect(() => {
    let cancelled = false;
    setBusy('load');
    void fetchVisionRuleEvents(rule.id, {
      limit: HISTORY_PAGE_SIZE + 1,
      fromTs: toStartOfDayISO(appliedFromDate),
      toTs: toExclusiveEndOfDayISO(appliedToDate),
      beforeTs: currentCursor.beforeTs,
      beforeId: currentCursor.beforeId,
    })
      .then((items) => {
        if (cancelled) {
          return;
        }
        setEvents(items.slice(0, HISTORY_PAGE_SIZE));
        setHasOlderPage(items.length > HISTORY_PAGE_SIZE);
      })
      .catch((error) => {
        if (!cancelled) {
          onError(error instanceof Error ? error.message : 'Failed to load persisted rule events');
        }
      })
      .finally(() => {
        if (!cancelled) {
          setBusy('');
        }
      });
    return () => {
      cancelled = true;
    };
  }, [appliedFromDate, appliedToDate, currentCursor.beforeId, currentCursor.beforeTs, onError, rule.id, updatedAtKey]);

  const eventViews = useMemo(() => events.map((event) => buildHistoryEventView(event)), [events]);

  const entityOptions = useMemo(() => {
    const entities = new Map<string, HistoryEntity>();
    for (const eventView of eventViews) {
      for (const entity of eventView.entities) {
        if (!entities.has(entity.key)) {
          entities.set(entity.key, entity);
        }
      }
    }
    return Array.from(entities.values()).sort((left, right) => left.label.localeCompare(right.label));
  }, [eventViews]);

  const filteredEventViews = useMemo(
    () =>
      selectedEntityKey
        ? eventViews.filter((eventView) => eventView.entities.some((entity) => entity.key === selectedEntityKey))
        : eventViews,
    [eventViews, selectedEntityKey],
  );

  useEffect(() => {
    setSelectedEntityKey((current) => (current && !entityOptions.some((entity) => entity.key === current) ? '' : current));
  }, [entityOptions]);

  useEffect(() => {
    setOpenEventId((current) => (filteredEventViews.some((eventView) => eventView.event.id === current) ? current : ''));
  }, [filteredEventViews]);

  const openEvent = filteredEventViews.find((eventView) => eventView.event.id === openEventId) ?? null;
  const filteredCountLabel =
    filteredEventViews.length === eventViews.length
      ? `${eventViews.length} events on this page`
      : `${filteredEventViews.length} of ${eventViews.length} events on this page`;

  const applyDateFilter = () => {
    const fromTs = toStartOfDayISO(draftFromDate);
    const toTs = toExclusiveEndOfDayISO(draftToDate);
    if ((draftFromDate && !fromTs) || (draftToDate && !toTs)) {
      onError('Enter valid dates before applying the history filter.');
      return;
    }
    if (fromTs && toTs && fromTs >= toTs) {
      onError('History end date must be on or after the start date.');
      return;
    }
    setAppliedFromDate(draftFromDate);
    setAppliedToDate(draftToDate);
    setPageCursors([{}]);
    setPageIndex(0);
  };

  const clearDateFilter = () => {
    setDraftFromDate('');
    setDraftToDate('');
    setAppliedFromDate('');
    setAppliedToDate('');
    setPageCursors([{}]);
    setPageIndex(0);
  };

  const refreshEvents = async () => {
    setBusy('refresh');
    try {
      const items = await fetchVisionRuleEvents(rule.id, {
        limit: HISTORY_PAGE_SIZE + 1,
        fromTs: toStartOfDayISO(appliedFromDate),
        toTs: toExclusiveEndOfDayISO(appliedToDate),
        beforeTs: currentCursor.beforeTs,
        beforeId: currentCursor.beforeId,
      });
      setEvents(items.slice(0, HISTORY_PAGE_SIZE));
      setHasOlderPage(items.length > HISTORY_PAGE_SIZE);
    } catch (error) {
      onError(error instanceof Error ? error.message : 'Failed to refresh persisted rule events');
    } finally {
      setBusy('');
    }
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

  const deleteEvent = async (event: EventRecord) => {
    if (!window.confirm('Delete this persisted event and any stored captures?')) {
      return;
    }
    setDeletingEventId(event.id);
    try {
      await deleteVisionRuleEvent(rule.id, event.id);
      setEvents((current) => current.filter((item) => item.id !== event.id));
      setOpenEventId((current) => (current === event.id ? '' : current));
    } catch (error) {
      onError(error instanceof Error ? error.message : 'Failed to delete persisted rule event');
    } finally {
      setDeletingEventId('');
    }
  };

  return (
    <>
      <Card className="vision-history-panel">
        <CardHeader>
          <CardHeading
            title={rule.name || rule.id}
            description="Persisted recognition history for this rule within the configured retention window."
            aside={
              <div className="automation-editor__meta">
                <Button type="button" variant="secondary" size="sm" onClick={onBack}>
                  <ArrowLeft className="h-4 w-4" />
                  <span>Back To Rule</span>
                </Button>
                <Button
                  type="button"
                  variant="secondary"
                  size="sm"
                  onClick={() => void refreshEvents()}
                  disabled={busy !== '' || deletingEventId !== ''}
                >
                  <RefreshCcw className="h-4 w-4" />
                  <span>{busy === 'refresh' ? 'Refreshing…' : 'Refresh'}</span>
                </Button>
              </div>
            }
          />
        </CardHeader>
        <CardContent className="vision-history-panel__content">
          <div className="vision-toolbar vision-history-toolbar vision-history-toolbar--compact">
            <div className="vision-history-toolbar__filter">
              <span className="muted">Entity</span>
              <select
                className="select vision-history-toolbar__select"
                value={selectedEntityKey}
                onChange={(event) => setSelectedEntityKey(event.target.value)}
              >
                <option value="">All Entities</option>
                {entityOptions.map((entity) => (
                  <option key={entity.key} value={entity.key}>
                    {entity.label}
                  </option>
                ))}
              </select>
            </div>
            <div className="vision-history-toolbar__dates">
              <Input
                type="date"
                className="vision-history-toolbar__range"
                value={draftFromDate}
                onChange={(event) => setDraftFromDate(event.target.value)}
                aria-label="Filter rule history from date"
              />
              <Input
                type="date"
                className="vision-history-toolbar__range"
                value={draftToDate}
                onChange={(event) => setDraftToDate(event.target.value)}
                aria-label="Filter rule history to date"
              />
              <Button type="button" variant="secondary" size="sm" onClick={applyDateFilter} disabled={busy !== '' || !hasDraftChanges}>
                Apply
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={clearDateFilter}
                disabled={busy !== '' || (!hasAppliedDateFilter && draftFromDate === '' && draftToDate === '' && pageIndex === 0)}
              >
                Clear
              </Button>
            </div>
            <div className="vision-history-toolbar__page">
              <span className="muted">{filteredCountLabel}</span>
              <span className="muted">Page {pageIndex + 1}</span>
              <Button type="button" variant="secondary" size="sm" onClick={loadNewerPage} disabled={busy !== '' || pageIndex === 0}>
                Newer
              </Button>
              <Button type="button" variant="secondary" size="sm" onClick={loadOlderPage} disabled={busy !== '' || !hasOlderPage}>
                Older
              </Button>
            </div>
          </div>

          <ScrollArea className="vision-history-grid-scroll">
            {filteredEventViews.length > 0 ? (
              <div className="vision-history-grid">
                {filteredEventViews.map((eventView) => (
                  <VisionHistoryEventCard
                    key={eventView.event.id}
                    active={eventView.event.id === openEventId}
                    busy={busy !== '' || deletingEventId !== ''}
                    eventView={eventView}
                    onDelete={(event) => void deleteEvent(event)}
                    onOpen={setOpenEventId}
                  />
                ))}
              </div>
            ) : (
              <div className="detail">
                {busy === 'load'
                  ? 'Loading persisted rule events…'
                  : eventViews.length > 0
                    ? 'No persisted events match the selected entity filter.'
                    : hasAppliedDateFilter
                      ? 'No persisted events matched the selected date range.'
                      : 'No persisted events for this rule in the current retention window.'}
              </div>
            )}
          </ScrollArea>
        </CardContent>
      </Card>

      <VisionHistoryEventModal
        busy={deletingEventId !== ''}
        eventView={openEvent}
        onClose={() => setOpenEventId('')}
        onDelete={(event) => void deleteEvent(event)}
      />
    </>
  );
}
