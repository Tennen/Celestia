import { useEffect, useMemo, useState, type CSSProperties } from 'react';
import { ArrowLeft, RefreshCcw, Trash2, X } from 'lucide-react';
import { deleteVisionRuleEvent, fetchVisionRuleEvents, visionCaptureURL } from '../../lib/api';
import type { EventRecord, VisionRule } from '../../lib/types';
import { cn, formatTime, prettyJson } from '../../lib/utils';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardHeader } from '../ui/card';
import { ScrollArea } from '../ui/scroll-area';
import { visionEventCapturesFromPayload, VisionEventCaptureGallery } from './VisionEventCaptureGallery';
import { VisionEventDecisionCard } from './VisionEventDecisionCard';
import { AggregatedInfoCard } from './shared/AggregatedInfoCard';
import { CardHeading } from './shared/CardHeading';

type Props = {
  onBack: () => void;
  onError: (message: string) => void;
  rule: VisionRule;
  updatedAtKey: string;
};

type HistoryEntity = {
  key: string;
  kind: string;
  label: string;
  value: string;
};

type HistoryEventView = {
  entities: HistoryEntity[];
  entitySummary: string;
  event: EventRecord;
};

type EventModalProps = {
  busy: boolean;
  eventView: HistoryEventView | null;
  onClose: () => void;
  onDelete: (event: EventRecord) => void;
};

type EventCardProps = {
  active: boolean;
  busy: boolean;
  eventView: HistoryEventView;
  onDelete: (event: EventRecord) => void;
  onOpen: (eventId: string) => void;
};

function readString(value: unknown, fallback = '') {
  return typeof value === 'string' ? value : fallback;
}

function readNumber(value: unknown, fallback = 0) {
  return typeof value === 'number' ? value : fallback;
}

function readRecord(value: unknown): Record<string, unknown> | null {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;
}

function historyEntityKey(kind: string, value: string) {
  return `${kind}::${value}`;
}

function readHistoryEntity(value: unknown): HistoryEntity | null {
  const record = readRecord(value);
  if (!record) {
    return null;
  }
  const kind = readString(record.kind).trim() || 'label';
  const entityValue = readString(record.value).trim();
  if (!entityValue) {
    return null;
  }
  return {
    key: historyEntityKey(kind, entityValue),
    kind,
    label: readString(record.display_name).trim() || entityValue,
    value: entityValue,
  };
}

function normalizeEventEntities(event: EventRecord) {
  const entities: HistoryEntity[] = [];
  const seen = new Set<string>();
  const payloadEntities = Array.isArray(event.payload?.entities) ? event.payload.entities : [];
  for (const item of payloadEntities) {
    const entity = readHistoryEntity(item);
    if (!entity || seen.has(entity.key)) {
      continue;
    }
    seen.add(entity.key);
    entities.push(entity);
  }
  const fallbackValue = readString(event.payload?.entity_value).trim();
  const fallbackKey = historyEntityKey('label', fallbackValue);
  if (fallbackValue && !seen.has(fallbackKey)) {
    entities.push({
      key: fallbackKey,
      kind: 'label',
      label: fallbackValue,
      value: fallbackValue,
    });
  }
  return entities;
}

function buildHistoryEventView(event: EventRecord): HistoryEventView {
  const entities = normalizeEventEntities(event);
  return {
    entities,
    entitySummary: entities.map((entity) => entity.label).join(', '),
    event,
  };
}

function historyEventStatus(eventView: HistoryEventView) {
  return readString(eventView.event.payload?.event_status, eventView.event.type);
}

function VisionHistoryEventModal({ busy, eventView, onClose, onDelete }: EventModalProps) {
  useEffect(() => {
    if (!eventView) {
      return;
    }
    const onKeyDown = (keyEvent: KeyboardEvent) => {
      if (keyEvent.key === 'Escape') {
        onClose();
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [eventView, onClose]);

  if (!eventView) {
    return null;
  }

  return (
    <div className="admin-modal" onMouseDown={onClose}>
      <Card className="admin-modal__card vision-history-modal" onMouseDown={(mouseEvent) => mouseEvent.stopPropagation()}>
        <CardHeader className="vision-history-modal__header">
          <CardHeading
            title={historyEventStatus(eventView)}
            description={
              eventView.entitySummary
                ? `${formatTime(eventView.event.ts)} · ${eventView.entitySummary}`
                : formatTime(eventView.event.ts)
            }
            aside={
              <div className="vision-history-modal__aside">
                <Button
                  type="button"
                  variant="danger"
                  size="icon"
                  aria-label="Delete Event"
                  title="Delete Event"
                  onClick={() => onDelete(eventView.event)}
                  disabled={busy}
                >
                  <Trash2 className="h-4 w-4" />
                </Button>
                <Button type="button" variant="ghost" size="icon" aria-label="Close Event Detail" title="Close" onClick={onClose}>
                  <X className="h-4 w-4" />
                </Button>
              </div>
            }
          />
        </CardHeader>
        <div className="admin-modal__scroll">
          <CardContent className="vision-history-modal__content">
            <AggregatedInfoCard
              items={[
                {
                  className: 'aggregated-info-card__item--full',
                  label: 'Event ID',
                  value: eventView.event.id,
                  title: eventView.event.id,
                },
                {
                  label: 'Dwell Seconds',
                  value: `${readNumber(eventView.event.payload?.dwell_seconds)}s`,
                },
                {
                  label: 'Event Status',
                  value: historyEventStatus(eventView),
                  title: historyEventStatus(eventView),
                },
              ]}
            />

            <VisionEventCaptureGallery captures={visionEventCapturesFromPayload(eventView.event.payload)} />
            <VisionEventDecisionCard metadata={eventView.event.payload?.metadata} />

            <div className="vision-history-detail__payload">
              <span className="muted">Event Metadata</span>
              <pre className="vision-history-detail__json">
                {prettyJson(eventView.event.payload?.metadata ?? eventView.event.payload ?? {})}
              </pre>
            </div>
          </CardContent>
        </div>
      </Card>
    </div>
  );
}

function VisionHistoryEventCard({ active, busy, eventView, onDelete, onOpen }: EventCardProps) {
  const captures = visionEventCapturesFromPayload(eventView.event.payload);
  const leadCapture = captures[0] ?? null;
  const backgroundStyle: CSSProperties | undefined = leadCapture
    ? { backgroundImage: `url("${visionCaptureURL(leadCapture.capture_id)}")` }
    : undefined;

  return (
    <article className={cn('vision-history-card', active && 'is-active')}>
      <button type="button" className="vision-history-card__surface" onClick={() => onOpen(eventView.event.id)}>
        <div className={cn('vision-history-card__media', !leadCapture && 'is-placeholder')} style={backgroundStyle} />
        <div className="vision-history-card__veil" />
        <div className="vision-history-card__top">
          <Badge className="vision-history-card__tag" tone="accent" size="xs">
            {readNumber(eventView.event.payload?.dwell_seconds)}s
          </Badge>
        </div>
        <div className="vision-history-card__bottom">
          {eventView.entitySummary ? (
            <span className="vision-history-card__entities" title={eventView.entitySummary}>
              {eventView.entitySummary}
            </span>
          ) : (
            <span />
          )}
          <span className="vision-history-card__time">{formatTime(eventView.event.ts)}</span>
        </div>
      </button>

      <Button
        type="button"
        variant="danger"
        size="icon"
        className="vision-history-card__delete"
        aria-label="Delete Event"
        title="Delete Event"
        onClick={(mouseEvent) => {
          mouseEvent.stopPropagation();
          onDelete(eventView.event);
        }}
        disabled={busy}
      >
        <Trash2 className="h-4 w-4" />
      </Button>
    </article>
  );
}

export function VisionRuleEventHistoryPanel({ onBack, onError, rule, updatedAtKey }: Props) {
  const [events, setEvents] = useState<EventRecord[]>([]);
  const [openEventId, setOpenEventId] = useState('');
  const [selectedEntityKey, setSelectedEntityKey] = useState('');
  const [busy, setBusy] = useState<'load' | 'refresh' | ''>('load');
  const [deletingEventId, setDeletingEventId] = useState('');

  useEffect(() => {
    let cancelled = false;
    setBusy('load');
    void fetchVisionRuleEvents(rule.id, 50)
      .then((items) => {
        if (!cancelled) {
          setEvents(items);
        }
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
  }, [onError, rule.id, updatedAtKey]);

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

  const refreshEvents = async () => {
    setBusy('refresh');
    try {
      setEvents(await fetchVisionRuleEvents(rule.id, 50));
    } catch (error) {
      onError(error instanceof Error ? error.message : 'Failed to refresh persisted rule events');
    } finally {
      setBusy('');
    }
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
          {eventViews.length > 0 ? (
            <div className="vision-toolbar vision-history-toolbar">
              <div className="vision-history-toolbar__filter">
                <span className="muted">Entity Filter</span>
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
              <span className="muted">
                {filteredEventViews.length === eventViews.length
                  ? `${eventViews.length} events`
                  : `${filteredEventViews.length} of ${eventViews.length} events`}
              </span>
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
