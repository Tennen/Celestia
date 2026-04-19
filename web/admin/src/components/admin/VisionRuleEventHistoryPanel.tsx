import { useEffect, useMemo, useState } from 'react';
import { ArrowLeft, ChevronLeft, ChevronRight, RefreshCcw, SlidersHorizontal } from 'lucide-react';
import { deleteVisionRuleEvent, fetchVisionRuleEvents } from '../../lib/api';
import type { EventRecord, VisionRule } from '../../lib/types';
import { Button } from '../ui/button';
import { Card, CardContent, CardHeader } from '../ui/card';
import { ConfirmDialog } from '../ui/confirm-dialog';
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

type KeyEntityOption = {
  label: string;
  value: string;
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
  const [selectedKeyEntityID, setSelectedKeyEntityID] = useState('');
  const [busy, setBusy] = useState<'load' | 'refresh' | ''>('load');
  const [deletingEventId, setDeletingEventId] = useState('');
  const [pendingDeleteEvent, setPendingDeleteEvent] = useState<EventRecord | null>(null);
  const [draftDate, setDraftDate] = useState('');
  const [appliedDate, setAppliedDate] = useState('');
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [pageCursors, setPageCursors] = useState<EventCursor[]>([{}]);
  const [pageIndex, setPageIndex] = useState(0);
  const [hasOlderPage, setHasOlderPage] = useState(false);

  const currentCursor = pageCursors[pageIndex] ?? {};
  const hasAppliedDateFilter = appliedDate !== '';
  const hasDraftChanges = draftDate !== appliedDate;
  const hasActiveFilters = hasAppliedDateFilter || selectedEntityKey !== '' || selectedKeyEntityID !== '';
  const canApplyDateFilter = hasDraftChanges || pageIndex > 0;

  useEffect(() => {
    setEvents([]);
    setOpenEventId('');
    setSelectedEntityKey('');
    setSelectedKeyEntityID('');
    setDeletingEventId('');
    setPendingDeleteEvent(null);
    setDraftDate('');
    setAppliedDate('');
    setFiltersOpen(false);
    setPageCursors([{}]);
    setPageIndex(0);
    setHasOlderPage(false);
  }, [rule.id]);

  useEffect(() => {
    let cancelled = false;
    setBusy('load');
    void fetchVisionRuleEvents(rule.id, {
      limit: HISTORY_PAGE_SIZE + 1,
      fromTs: toStartOfDayISO(appliedDate),
      toTs: toExclusiveEndOfDayISO(appliedDate),
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
  }, [appliedDate, currentCursor.beforeId, currentCursor.beforeTs, onError, rule.id, updatedAtKey]);

  const eventViews = useMemo(() => events.map((event) => buildHistoryEventView(event, rule)), [events, rule]);

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

  const keyEntityOptions = useMemo(() => {
    const options = new Map<string, KeyEntityOption>();
    for (const item of rule.key_entities) {
      const label = item.name.trim() || item.description?.trim() || `Key Entity #${item.id}`;
      options.set(String(item.id), { label, value: String(item.id) });
    }
    for (const eventView of eventViews) {
      if (eventView.keyEntity && !options.has(String(eventView.keyEntity.id))) {
        options.set(String(eventView.keyEntity.id), {
          label: eventView.keyEntity.label,
          value: String(eventView.keyEntity.id),
        });
      }
    }
    return Array.from(options.values()).sort((left, right) => left.label.localeCompare(right.label));
  }, [eventViews, rule.key_entities]);

  const filteredEventViews = useMemo(
    () =>
      eventViews.filter((eventView) => {
        if (selectedEntityKey && !eventView.entities.some((entity) => entity.key === selectedEntityKey)) {
          return false;
        }
        if (selectedKeyEntityID && String(eventView.keyEntity?.id ?? '') !== selectedKeyEntityID) {
          return false;
        }
        return true;
      }),
    [eventViews, selectedEntityKey, selectedKeyEntityID],
  );

  useEffect(() => {
    setSelectedEntityKey((current) => (current && !entityOptions.some((entity) => entity.key === current) ? '' : current));
  }, [entityOptions]);

  useEffect(() => {
    setSelectedKeyEntityID((current) =>
      current && !keyEntityOptions.some((entity) => entity.value === current) ? '' : current,
    );
  }, [keyEntityOptions]);

  useEffect(() => {
    setOpenEventId((current) => (filteredEventViews.some((eventView) => eventView.event.id === current) ? current : ''));
  }, [filteredEventViews]);

  const openEvent = filteredEventViews.find((eventView) => eventView.event.id === openEventId) ?? null;
  const filteredCountLabel =
    selectedEntityKey || selectedKeyEntityID ? `${filteredEventViews.length}/${eventViews.length}` : `${eventViews.length}`;

  const applyDateFilter = () => {
    const fromTs = toStartOfDayISO(draftDate);
    if (draftDate && !fromTs) {
      onError('Enter a valid date before applying the history filter.');
      return;
    }
    setAppliedDate(draftDate);
    setPageCursors([{}]);
    setPageIndex(0);
  };

  const clearDateFilter = () => {
    setDraftDate('');
    setAppliedDate('');
    setPageCursors([{}]);
    setPageIndex(0);
  };

  const refreshEvents = async () => {
    setBusy('refresh');
    try {
      const items = await fetchVisionRuleEvents(rule.id, {
        limit: HISTORY_PAGE_SIZE + 1,
        fromTs: toStartOfDayISO(appliedDate),
        toTs: toExclusiveEndOfDayISO(appliedDate),
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

  const requestDeleteEvent = (event: EventRecord) => {
    setPendingDeleteEvent(event);
  };

  const confirmDeleteEvent = async () => {
    const event = pendingDeleteEvent;
    if (!event) {
      return;
    }
    setDeletingEventId(event.id);
    try {
      await deleteVisionRuleEvent(rule.id, event.id);
      setEvents((current) => current.filter((item) => item.id !== event.id));
      setOpenEventId((current) => (current === event.id ? '' : current));
      setPendingDeleteEvent(null);
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
          <div className="vision-history-toolbar">
            <div className="vision-history-toolbar__summary">
              <span className="muted">{filteredCountLabel}</span>
              <span className="muted">p{pageIndex + 1}</span>
            </div>
            <div className="vision-history-toolbar__actions">
              <Button
                type="button"
                variant={filtersOpen || hasActiveFilters ? 'secondary' : 'ghost'}
                size="icon"
                className="vision-history-toolbar__icon"
                aria-label={filtersOpen ? 'Hide filters' : 'Show filters'}
                title={filtersOpen ? 'Hide Filters' : 'Show Filters'}
                onClick={() => setFiltersOpen((current) => !current)}
              >
                <SlidersHorizontal className="h-4 w-4" />
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="vision-history-toolbar__icon"
                aria-label="Newer page"
                title="Newer"
                onClick={loadNewerPage}
                disabled={busy !== '' || pageIndex === 0}
              >
                <ChevronLeft className="h-4 w-4" />
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="vision-history-toolbar__icon"
                aria-label="Older page"
                title="Older"
                onClick={loadOlderPage}
                disabled={busy !== '' || !hasOlderPage}
              >
                <ChevronRight className="h-4 w-4" />
              </Button>
            </div>
          </div>

          {filtersOpen ? (
            <div className="vision-history-toolbar__panel">
              <div className="vision-history-toolbar__filter">
                <span className="muted">Entity</span>
                <select
                  className="select vision-history-toolbar__select"
                  aria-label="Filter rule history by entity"
                  value={selectedEntityKey}
                  onChange={(event) => setSelectedEntityKey(event.target.value)}
                >
                  <option value="">All</option>
                  {entityOptions.map((entity) => (
                    <option key={entity.key} value={entity.key}>
                      {entity.label}
                    </option>
                  ))}
                </select>
              </div>
              <div className="vision-history-toolbar__filter">
                <span className="muted">Key Entity</span>
                <select
                  className="select vision-history-toolbar__select"
                  aria-label="Filter rule history by key entity"
                  value={selectedKeyEntityID}
                  onChange={(event) => setSelectedKeyEntityID(event.target.value)}
                >
                  <option value="">All</option>
                  {keyEntityOptions.map((entity) => (
                    <option key={entity.value} value={entity.value}>
                      {entity.label}
                    </option>
                  ))}
                </select>
              </div>
              <div className="vision-history-toolbar__date-filter">
                <Input
                  type="date"
                  className="vision-history-toolbar__range"
                  value={draftDate}
                  onChange={(event) => setDraftDate(event.target.value)}
                  aria-label="Filter rule history by date"
                />
                <Button type="button" variant="secondary" size="sm" onClick={applyDateFilter} disabled={busy !== '' || !canApplyDateFilter}>
                  Apply
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={clearDateFilter}
                  disabled={busy !== '' || (!hasAppliedDateFilter && draftDate === '' && pageIndex === 0)}
                >
                  Clear
                </Button>
              </div>
            </div>
          ) : null}

          <ScrollArea className="vision-history-grid-scroll">
            {filteredEventViews.length > 0 ? (
              <div className="vision-history-grid">
                {filteredEventViews.map((eventView) => (
                  <VisionHistoryEventCard
                    key={eventView.event.id}
                    active={eventView.event.id === openEventId}
                    busy={busy !== '' || deletingEventId !== ''}
                    eventView={eventView}
                    onDelete={requestDeleteEvent}
                    onOpen={setOpenEventId}
                  />
                ))}
              </div>
            ) : (
              <div className="detail">
                {busy === 'load'
                  ? 'Loading persisted rule events…'
                  : eventViews.length > 0
                    ? 'No persisted events match the selected filters.'
                    : hasAppliedDateFilter
                      ? 'No persisted events matched the selected day.'
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
        onDelete={requestDeleteEvent}
      />
      <ConfirmDialog
        open={pendingDeleteEvent !== null}
        title="Delete Recognition Event"
        description="This removes the persisted recognition event and any stored capture evidence. This action cannot be undone."
        tone="danger"
        cancelLabel="Keep Event"
        confirmLabel="Delete Event"
        loading={deletingEventId !== ''}
        onOpenChange={(open) => {
          if (!open && deletingEventId === '') {
            setPendingDeleteEvent(null);
          }
        }}
        onConfirm={() => void confirmDeleteEvent()}
      />
    </>
  );
}
