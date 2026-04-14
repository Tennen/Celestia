import { useEffect } from 'react';
import { X } from 'lucide-react';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardHeader } from '../ui/card';
import { Input } from '../ui/input';
import { Switch } from '../ui/switch';
import { formatTime, prettyJson } from '../../lib/utils';
import type { HealthState, VisionCapabilityStatus, VisionEntityCatalog } from '../../lib/types';
import { CardHeading } from './shared/CardHeading';

type RecognitionSettingsDraft = {
  event_capture_retention_hours: number;
  recognition_enabled: boolean;
  service_ws_url: string;
  model_name: string;
};

type Props = {
  busy: '' | 'load' | 'refresh_entities' | 'save_rule' | 'save_settings';
  catalog: VisionEntityCatalog | null;
  catalogMatchesDraft: boolean;
  draft: RecognitionSettingsDraft;
  normalizedDraftServiceWSURL: string;
  normalizedDraftModelName: string;
  onOpenChange: (open: boolean) => void;
  onRefreshEntities: () => void;
  onResetDraft: () => void;
  onSave: () => void;
  onUpdateDraft: (updater: (current: RecognitionSettingsDraft) => RecognitionSettingsDraft) => void;
  open: boolean;
  runtime: VisionCapabilityStatus | undefined;
  status: HealthState;
};

function toneFromHealth(status: HealthState) {
  if (status === 'healthy') return 'good' as const;
  if (status === 'degraded' || status === 'unknown') return 'warn' as const;
  if (status === 'stopped') return 'neutral' as const;
  return 'bad' as const;
}

export function RecognitionSettingsModal({
  busy,
  catalog,
  catalogMatchesDraft,
  draft,
  normalizedDraftServiceWSURL,
  normalizedDraftModelName,
  onOpenChange,
  onRefreshEntities,
  onResetDraft,
  onSave,
  onUpdateDraft,
  open,
  runtime,
  status,
}: Props) {
  useEffect(() => {
    if (!open) {
      return;
    }
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        onOpenChange(false);
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [onOpenChange, open]);

  if (!open) {
    return null;
  }

  return (
    <div className="admin-modal" onMouseDown={() => onOpenChange(false)}>
      <Card className="admin-modal__card recognition-settings-modal" onMouseDown={(event) => event.stopPropagation()}>
        <CardHeader className="recognition-settings-modal__header">
          <CardHeading
            title="Recognition Settings"
            description="Configure the recognition websocket endpoint, selected model, runtime sync behavior, and supported entity catalog."
            aside={
              <div className="recognition-settings-modal__aside">
                <Badge tone={toneFromHealth(status)}>{status}</Badge>
                <Switch
                  checked={draft.recognition_enabled}
                  onCheckedChange={(checked) =>
                    onUpdateDraft((current) => ({ ...current, recognition_enabled: checked }))
                  }
                  aria-label="Toggle recognition enabled"
                />
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  onClick={() => onOpenChange(false)}
                  aria-label="Close recognition settings"
                >
                  <X className="h-4 w-4" />
                </Button>
              </div>
            }
          />
        </CardHeader>
        <div className="admin-modal__scroll">
          <CardContent className="stack recognition-settings-modal__content">
            <div className="automation-field">
              <label>Recognition Service WebSocket URL</label>
              <Input
                value={draft.service_ws_url}
                onChange={(event) => onUpdateDraft((current) => ({ ...current, service_ws_url: event.target.value }))}
                placeholder="ws://127.0.0.1:8090/ws/control"
              />
            </div>

            <div className="automation-field">
              <label>Selected Model</label>
              <Input
                value={draft.model_name}
                onChange={(event) => onUpdateDraft((current) => ({ ...current, model_name: event.target.value }))}
                placeholder="Leave blank to use the service default/current model"
              />
            </div>

            <div className="automation-field">
              <label>Event Capture Retention Hours</label>
              <Input
                type="number"
                min={1}
                step={1}
                value={draft.event_capture_retention_hours}
                onChange={(event) =>
                  onUpdateDraft((current) => ({
                    ...current,
                    event_capture_retention_hours: Math.max(1, Number(event.target.value) || 1),
                  }))
                }
              />
              <p className="muted">
                Applies to persisted vision rule history and capture evidence only. The global Activity event feed is paginated separately.
              </p>
            </div>

            <div className="automation-field">
              <div className="button-row">
                <label>Supported Entities</label>
                <Button variant="secondary" onClick={onRefreshEntities} disabled={busy === 'refresh_entities'}>
                  {busy === 'refresh_entities' ? 'Refreshing…' : 'Refresh Supported Entities'}
                </Button>
              </div>
              {catalog ? (
                <>
                  <p className="muted">
                    Catalog from {catalog.service_ws_url} · fetched{' '}
                    {catalog.fetched_at ? formatTime(catalog.fetched_at) : 'unknown'} · model{' '}
                    {catalog.model_name || 'current/default'}
                  </p>
                  {!catalogMatchesDraft ? (
                    <p className="muted">
                      Current draft points to {normalizedDraftServiceWSURL || 'no websocket URL'} · model{' '}
                      {normalizedDraftModelName || 'current/default'}. Refresh again after updating the websocket URL or model so
                      entity validation matches the target runtime.
                    </p>
                  ) : null}
                  <div className="button-row">
                    {catalog.entities.length > 0 ? (
                      catalog.entities.map((entity) => (
                        <Badge key={`${entity.kind}:${entity.value}`} size="xs" tone={entity.value === 'cat' ? 'accent' : 'neutral'}>
                          {entity.display_name || entity.value}
                        </Badge>
                      ))
                    ) : (
                      <Badge size="xs" tone="neutral">
                        no entities reported
                      </Badge>
                    )}
                  </div>
                </>
              ) : (
                <p className="muted">
                  Gateway has not fetched the current Recognition Service entity catalog yet. Refresh it before configuring an
                  entity such as `cat`.
                </p>
              )}
            </div>

            <div className="vision-runtime-grid">
              <div className="kv">
                <span>Runtime Status</span>
                <strong>{runtime?.status ?? 'unknown'}</strong>
              </div>
              <div className="kv">
                <span>Service Version</span>
                <strong>{runtime?.service_version || 'unknown'}</strong>
              </div>
              <div className="kv">
                <span>Last Sync</span>
                <strong>{runtime?.last_synced_at ? formatTime(runtime.last_synced_at) : 'never'}</strong>
              </div>
              <div className="kv">
                <span>Last Event</span>
                <strong>{runtime?.last_event_at ? formatTime(runtime.last_event_at) : 'none yet'}</strong>
              </div>
            </div>

            {runtime?.message ? <p className="muted">Status: {runtime.message}</p> : null}
            {runtime?.sync_error ? <p className="muted">Sync error: {runtime.sync_error}</p> : null}

            {runtime?.runtime && Object.keys(runtime.runtime).length > 0 ? (
              <div className="automation-field">
                <label>Reported Runtime Payload</label>
                <pre className="plugin-config__preview">{prettyJson(runtime.runtime)}</pre>
              </div>
            ) : null}

            <div className="button-row recognition-settings-modal__actions">
              <Button onClick={onSave} disabled={busy === 'save_settings'}>
                {busy === 'save_settings' ? 'Saving…' : 'Save Recognition Settings'}
              </Button>
              <Button variant="secondary" onClick={onResetDraft}>
                Reset Draft
              </Button>
            </div>
          </CardContent>
        </div>
      </Card>
    </div>
  );
}
