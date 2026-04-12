import { useEffect, useState, type CSSProperties } from 'react';
import { ArrowLeft, RefreshCcw, Trash2, X } from 'lucide-react';
import { deleteVisionRuleEvent, fetchVisionRuleEvents, visionCaptureURL } from '../../lib/api';
import type { EventRecord, VisionRule } from '../../lib/types';
import { cn, formatTime, prettyJson } from '../../lib/utils';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardHeader } from '../ui/card';
import { ScrollArea } from '../ui/scroll-area';
import { visionEventCapturesFromPayload, VisionEventCaptureGallery } from './VisionEventCaptureGallery';
import { AggregatedInfoCard } from './shared/AggregatedInfoCard';
import { CardHeading } from './shared/CardHeading';

type Props = {
  onBack: () => void;
  onError: (message: string) => void;
  rule: VisionRule;
  updatedAtKey: string;
};

type EventModalProps = {
  busy: boolean;
  event: EventRecord | null;
  onClose: () => void;
  onDelete: (event: EventRecord) => void;
};

type EventCardProps = {
  active: boolean;
  busy: boolean;
  event: EventRecord;
  onDelete: (event: EventRecord) => void;
  onOpen: (eventId: string) => void;
};

function readString(value: unknown, fallback = '') {
  return typeof value === 'string' ? value : fallback;
}

function readNumber(value: unknown, fallback = 0) {
  return typeof value === 'number' ? value : fallback;
}

function VisionHistoryEventModal({ busy, event, onClose, onDelete }: EventModalProps) {
  useEffect(() => {
    if (!event) {
      return;
    }
    const onKeyDown = (keyEvent: KeyboardEvent) => {
      if (keyEvent.key === 'Escape') {
        onClose();
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [event, onClose]);

  if (!event) {
    return null;
  }

  return (
    <div className="admin-modal" onMouseDown={onClose}>
      <Card className="admin-modal__card vision-history-modal" onMouseDown={(mouseEvent) => mouseEvent.stopPropagation()}>
        <CardHeader className="vision-history-modal__header">
          <CardHeading
            title={readString(event.payload?.event_status, event.type)}
            description={`${formatTime(event.ts)} · ${readString(event.payload?.entity_value, 'entity')}`}
            aside={
              <div className="vision-history-modal__aside">
                <Button
                  type="button"
                  variant="danger"
                  size="icon"
                  aria-label="Delete Event"
                  title="Delete Event"
                  onClick={() => onDelete(event)}
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
                  value: event.id,
                  title: event.id,
                },
                {
                  label: 'Dwell Seconds',
                  value: `${readNumber(event.payload?.dwell_seconds)}s`,
                },
                {
                  label: 'Event Status',
                  value: readString(event.payload?.event_status, event.type),
                  title: readString(event.payload?.event_status, event.type),
                },
              ]}
            />

            <VisionEventCaptureGallery captures={visionEventCapturesFromPayload(event.payload)} />

            <div className="vision-history-detail__payload">
              <span className="muted">Event Metadata</span>
              <pre className="vision-history-detail__json">{prettyJson(event.payload?.metadata ?? event.payload ?? {})}</pre>
            </div>
          </CardContent>
        </div>
      </Card>
    </div>
  );
}

function VisionHistoryEventCard({ active, busy, event, onDelete, onOpen }: EventCardProps) {
  const captures = visionEventCapturesFromPayload(event.payload);
  const leadCapture = captures[0] ?? null;
  const backgroundStyle: CSSProperties | undefined = leadCapture
    ? { backgroundImage: `url("${visionCaptureURL(leadCapture.capture_id)}")` }
    : undefined;

  return (
    <article className={cn('vision-history-card', active && 'is-active')}>
      <button type="button" className="vision-history-card__surface" onClick={() => onOpen(event.id)}>
        <div className={cn('vision-history-card__media', !leadCapture && 'is-placeholder')} style={backgroundStyle} />
        <div className="vision-history-card__veil" />
        <div className="vision-history-card__top">
          <Badge className="vision-history-card__tag" tone="accent" size="xs">
            {readNumber(event.payload?.dwell_seconds)}s
          </Badge>
        </div>
        <div className="vision-history-card__bottom">
          <span className="vision-history-card__time">{formatTime(event.ts)}</span>
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
          onDelete(event);
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

  useEffect(() => {
    setOpenEventId((current) => (events.some((event) => event.id === current) ? current : ''));
  }, [events]);

  const openEvent = events.find((event) => event.id === openEventId) ?? null;

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
          <ScrollArea className="vision-history-grid-scroll">
            {events.length > 0 ? (
              <div className="vision-history-grid">
                {events.map((event) => (
                  <VisionHistoryEventCard
                    key={event.id}
                    active={event.id === openEventId}
                    busy={busy !== '' || deletingEventId !== ''}
                    event={event}
                    onDelete={(item) => void deleteEvent(item)}
                    onOpen={setOpenEventId}
                  />
                ))}
              </div>
            ) : (
              <div className="detail">
                {busy === 'load' ? 'Loading persisted rule events…' : 'No persisted events for this rule in the current retention window.'}
              </div>
            )}
          </ScrollArea>
        </CardContent>
      </Card>

      <VisionHistoryEventModal
        busy={deletingEventId !== ''}
        event={openEvent}
        onClose={() => setOpenEventId('')}
        onDelete={(event) => void deleteEvent(event)}
      />
    </>
  );
}
