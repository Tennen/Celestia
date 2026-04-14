import type { CSSProperties } from 'react';
import { useEffect } from 'react';
import { Trash2, X } from 'lucide-react';
import { visionCaptureURL } from '../../lib/api';
import type { EventRecord, VisionRule } from '../../lib/types';
import { cn, formatTime, prettyJson } from '../../lib/utils';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardHeader } from '../ui/card';
import { visionEventCapturesFromPayload, VisionEventCaptureGallery } from './VisionEventCaptureGallery';
import { VisionEventDecisionCard } from './VisionEventDecisionCard';
import { AggregatedInfoCard, type AggregatedInfoCardItem } from './shared/AggregatedInfoCard';
import { CardHeading } from './shared/CardHeading';

export type HistoryEntity = {
  key: string;
  kind: string;
  label: string;
  value: string;
};

export type HistoryEventView = {
  entities: HistoryEntity[];
  entitySummary: string;
  event: EventRecord;
  keyEntity: HistoryKeyEntity | null;
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

type HistoryKeyEntity = {
  id: number;
  label: string;
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

function readPositiveInteger(value: unknown) {
  if (typeof value !== 'number' || !Number.isInteger(value) || value <= 0) {
    return null;
  }
  return value;
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

function historyEventStatus(eventView: HistoryEventView) {
  return readString(eventView.event.payload?.event_status, eventView.event.type);
}

function resolveHistoryKeyEntity(event: EventRecord, rule: VisionRule | null | undefined): HistoryKeyEntity | null {
  const id = readPositiveInteger(event.payload?.key_entity_id);
  if (id === null) {
    return null;
  }
  const matched = rule?.key_entities.find((item) => item.id === id);
  const label = matched?.description?.trim() || `Key Entity #${id}`;
  return { id, label };
}

export function buildHistoryEventView(event: EventRecord, rule?: VisionRule | null): HistoryEventView {
  const entities = normalizeEventEntities(event);
  return {
    entities,
    entitySummary: entities.map((entity) => entity.label).join(', '),
    event,
    keyEntity: resolveHistoryKeyEntity(event, rule),
  };
}

export function VisionHistoryEventModal({ busy, eventView, onClose, onDelete }: EventModalProps) {
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

  const infoItems = [
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
    eventView.keyEntity
      ? {
          className: 'aggregated-info-card__item--full',
          label: 'Key Entity',
          value: eventView.keyEntity.label,
          title: String(eventView.keyEntity.id),
        }
      : null,
  ].filter((item): item is AggregatedInfoCardItem => item !== null);

  return (
    <div className="admin-modal" onMouseDown={onClose}>
      <Card className="admin-modal__card vision-history-modal" onMouseDown={(mouseEvent) => mouseEvent.stopPropagation()}>
        <CardHeader className="vision-history-modal__header">
          <CardHeading
            title={historyEventStatus(eventView)}
            description={
              eventView.entitySummary
                ? `${formatTime(eventView.event.ts)} · ${eventView.entitySummary}${eventView.keyEntity ? ` · ${eventView.keyEntity.label}` : ''}`
                : `${formatTime(eventView.event.ts)}${eventView.keyEntity ? ` · ${eventView.keyEntity.label}` : ''}`
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
            <AggregatedInfoCard items={infoItems} />

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

export function VisionHistoryEventCard({ active, busy, eventView, onDelete, onOpen }: EventCardProps) {
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
          {eventView.keyEntity ? (
            <Badge className="vision-history-card__tag" tone="neutral" size="xs" title={eventView.keyEntity.label}>
              #{eventView.keyEntity.id}
            </Badge>
          ) : null}
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
