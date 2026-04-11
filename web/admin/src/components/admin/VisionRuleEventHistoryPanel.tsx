import { useEffect, useState } from 'react';
import { ArrowLeft, RefreshCcw } from 'lucide-react';
import { fetchVisionRuleEvents } from '../../lib/api';
import type { EventRecord, VisionRule } from '../../lib/types';
import { formatTime, prettyJson } from '../../lib/utils';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardHeader } from '../ui/card';
import { ScrollArea } from '../ui/scroll-area';
import { visionEventCapturesFromPayload, VisionEventCaptureGallery } from './VisionEventCaptureGallery';
import { SelectableListItem } from './shared/SelectableListItem';
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

export function VisionRuleEventHistoryPanel({ onBack, onError, rule, updatedAtKey }: Props) {
  const [events, setEvents] = useState<EventRecord[]>([]);
  const [selectedEventId, setSelectedEventId] = useState('');
  const [busy, setBusy] = useState<'load' | 'refresh' | ''>('load');

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
        <div className="vision-history-summary">
          <div className="vision-history-summary__item">
            <span className="muted">Rule ID</span>
            <strong>{rule.id}</strong>
          </div>
          <div className="vision-history-summary__item">
            <span className="muted">Camera</span>
            <strong>{rule.camera_device_id || 'unbound'}</strong>
          </div>
          <div className="vision-history-summary__item">
            <span className="muted">Entity</span>
            <strong>{rule.entity_selector.value || 'unset'}</strong>
          </div>
          <div className="vision-history-summary__item">
            <span className="muted">Threshold</span>
            <strong>{rule.stay_threshold_seconds}s</strong>
          </div>
        </div>

        <div className="vision-history-layout">
          <ScrollArea className="vision-history-layout__list">
            <div className="vision-history-list">
              {events.length > 0 ? (
                events.map((event) => {
                  const captureCount = readNumber(event.payload?.capture_count);
                  return (
                    <SelectableListItem
                      key={event.id}
                      layout="stacked_badges"
                      selected={event.id === selectedEventId}
                      onClick={() => setSelectedEventId(event.id)}
                      title={readString(event.payload?.event_status, event.type)}
                      description={`${readString(event.payload?.entity_value, 'entity')} · dwell ${readNumber(event.payload?.dwell_seconds)}s`}
                      badges={
                        <Badge size="xxs" tone={captureCount > 0 ? 'accent' : 'neutral'}>
                          {captureCount > 0 ? `${captureCount} captures` : 'no capture'}
                        </Badge>
                      }
                      support={<span className="muted">{formatTime(event.ts)}</span>}
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
              <Card>
                <CardHeader>
                  <CardHeading
                    title={readString(selectedEvent.payload?.event_status, selectedEvent.type)}
                    description={`${formatTime(selectedEvent.ts)} · ${readString(selectedEvent.payload?.entity_value, 'entity')}`}
                    aside={
                      <div className="vision-history-detail__badges">
                        <Badge tone="accent" size="sm">
                          {readString(selectedEvent.payload?.rule_name, rule.name || rule.id)}
                        </Badge>
                        <Badge tone="neutral" size="sm">
                          {selectedEvent.device_id || 'no device'}
                        </Badge>
                      </div>
                    }
                  />
                </CardHeader>
                <CardContent className="stack">
                  <div className="vision-history-detail__grid">
                    <div className="vision-history-detail__item">
                      <span className="muted">Event ID</span>
                      <strong>{selectedEvent.id}</strong>
                    </div>
                    <div className="vision-history-detail__item">
                      <span className="muted">Dwell Seconds</span>
                      <strong>{readNumber(selectedEvent.payload?.dwell_seconds)}s</strong>
                    </div>
                    <div className="vision-history-detail__item">
                      <span className="muted">Rule ID</span>
                      <strong>{readString(selectedEvent.payload?.rule_id, rule.id)}</strong>
                    </div>
                    <div className="vision-history-detail__item">
                      <span className="muted">Event Status</span>
                      <strong>{readString(selectedEvent.payload?.event_status, selectedEvent.type)}</strong>
                    </div>
                  </div>

                  <VisionEventCaptureGallery captures={visionEventCapturesFromPayload(selectedEvent.payload)} />

                  <div className="vision-history-detail__payload">
                    <span className="muted">Event Metadata</span>
                    <pre className="vision-history-detail__json">
                      {prettyJson(selectedEvent.payload?.metadata ?? selectedEvent.payload ?? {})}
                    </pre>
                  </div>
                </CardContent>
              </Card>
            ) : (
              <Card>
                <CardContent className="pt-6">
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
