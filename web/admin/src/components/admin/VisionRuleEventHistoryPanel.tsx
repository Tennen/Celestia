import { useEffect, useState } from 'react';
import { ArrowLeft, RefreshCcw, Trash2 } from 'lucide-react';
import { deleteVisionRuleEvent, fetchVisionRuleEvents, visionCaptureURL } from '../../lib/api';
import type { EventRecord, VisionRule } from '../../lib/types';
import { formatTime, prettyJson } from '../../lib/utils';
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

function readString(value: unknown, fallback = '') {
  return typeof value === 'string' ? value : fallback;
}

function readNumber(value: unknown, fallback = 0) {
  return typeof value === 'number' ? value : fallback;
}

type VisionHistoryEventListItemProps = {
  event: EventRecord;
  onSelect: () => void;
  selected: boolean;
};

function VisionHistoryEventListItem({ event, onSelect, selected }: VisionHistoryEventListItemProps) {
  const captures = visionEventCapturesFromPayload(event.payload);
  const captureCount = Math.max(readNumber(event.payload?.capture_count), captures.length);
  const leadCapture = captures[0] ?? null;
  const eventStatus = readString(event.payload?.event_status, event.type);

  return (
    <button type="button" className={`vision-history-item ${selected ? 'is-selected' : ''}`} onClick={onSelect}>
      <div className="vision-history-item__layout">
        <div className="vision-history-item__copy">
          <strong className="vision-history-item__title">{eventStatus}</strong>
          <p className="vision-history-item__description">
            {readString(event.payload?.entity_value, 'entity')} · dwell {readNumber(event.payload?.dwell_seconds)}s
          </p>
          <div className="vision-history-item__support">{formatTime(event.ts)}</div>
        </div>

        {leadCapture ? (
          <div className="vision-history-item__thumb-wrap">
            {captureCount > 0 ? (
              <Badge className="vision-history-item__count" size="xxs" tone="accent">
                {captureCount}
              </Badge>
            ) : null}
            <img
              className="vision-history-item__thumb"
              src={visionCaptureURL(leadCapture.capture_id)}
              alt={`Vision capture preview for ${eventStatus}`}
              loading="lazy"
            />
          </div>
        ) : null}
      </div>
    </button>
  );
}

export function VisionRuleEventHistoryPanel({ onBack, onError, rule, updatedAtKey }: Props) {
  const [events, setEvents] = useState<EventRecord[]>([]);
  const [selectedEventId, setSelectedEventId] = useState('');
  const [busy, setBusy] = useState<'load' | 'refresh' | 'delete' | ''>('load');

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
    setSelectedEventId((current) => (events.some((event) => event.id === current) ? current : events[0]?.id ?? ''));
  }, [events]);

  const selectedEvent = events.find((event) => event.id === selectedEventId) ?? null;

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

  const deleteSelectedEvent = async () => {
    if (!selectedEvent) {
      return;
    }
    if (!window.confirm('Delete this persisted event and any stored captures?')) {
      return;
    }
    setBusy('delete');
    try {
      await deleteVisionRuleEvent(rule.id, selectedEvent.id);
      setEvents((current) => current.filter((event) => event.id !== selectedEvent.id));
    } catch (error) {
      onError(error instanceof Error ? error.message : 'Failed to delete persisted rule event');
    } finally {
      setBusy('');
    }
  };

  return (
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
              <Button type="button" variant="secondary" size="sm" onClick={() => void refreshEvents()} disabled={busy !== ''}>
                <RefreshCcw className="h-4 w-4" />
                <span>{busy === 'refresh' ? 'Refreshing…' : 'Refresh'}</span>
              </Button>
            </div>
          }
        />
      </CardHeader>
      <CardContent className="vision-history-panel__content">
        <div className="vision-history-layout">
          <ScrollArea className="vision-history-layout__list">
            <div className="vision-history-list">
              {events.length > 0 ? (
                events.map((event) => {
                  return (
                    <VisionHistoryEventListItem
                      key={event.id}
                      onSelect={() => setSelectedEventId(event.id)}
                      selected={event.id === selectedEventId}
                      event={event}
                    />
                  );
                })
              ) : (
                <div className="detail">
                  {busy === 'load' ? 'Loading persisted rule events…' : 'No persisted events for this rule in the current retention window.'}
                </div>
              )}
            </div>
          </ScrollArea>

          <div className="vision-history-layout__detail">
            {selectedEvent ? (
              <Card className="vision-history-detail-card">
                <CardHeader>
                  <CardHeading
                    title={readString(selectedEvent.payload?.event_status, selectedEvent.type)}
                    description={`${formatTime(selectedEvent.ts)} · ${readString(selectedEvent.payload?.entity_value, 'entity')}`}
                    aside={
                      <Button
                        type="button"
                        variant="danger"
                        size="icon"
                        aria-label="Delete Event"
                        title="Delete Event"
                        onClick={() => void deleteSelectedEvent()}
                        disabled={busy !== ''}
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    }
                  />
                </CardHeader>
                <ScrollArea className="vision-history-detail-card__scroll">
                  <CardContent className="vision-history-detail-card__content">
                    <AggregatedInfoCard
                      items={[
                        {
                          className: 'aggregated-info-card__item--full',
                          label: 'Event ID',
                          value: selectedEvent.id,
                          title: selectedEvent.id,
                        },
                        {
                          label: 'Dwell Seconds',
                          value: `${readNumber(selectedEvent.payload?.dwell_seconds)}s`,
                        },
                        {
                          label: 'Event Status',
                          value: readString(selectedEvent.payload?.event_status, selectedEvent.type),
                          title: readString(selectedEvent.payload?.event_status, selectedEvent.type),
                        },
                      ]}
                    />

                    <VisionEventCaptureGallery captures={visionEventCapturesFromPayload(selectedEvent.payload)} />

                    <div className="vision-history-detail__payload">
                      <span className="muted">Event Metadata</span>
                      <pre className="vision-history-detail__json">
                        {prettyJson(selectedEvent.payload?.metadata ?? selectedEvent.payload ?? {})}
                      </pre>
                    </div>
                  </CardContent>
                </ScrollArea>
              </Card>
            ) : (
              <Card className="vision-history-detail-card">
                <CardContent className="vision-history-detail-card__empty pt-6">
                  <p className="muted">Select a persisted event to inspect its details.</p>
                </CardContent>
              </Card>
            )}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
