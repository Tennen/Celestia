import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { Section } from '../ui/section';
import type { AppSection } from '../../lib/admin';
import { formatTime } from '../../lib/utils';
import type { AuditRecord, CatalogPlugin, DashboardSummary, DeviceView, EventRecord, PluginRuntimeView } from '../../lib/types';

type Props = {
  dashboard: DashboardSummary | null;
  catalog: CatalogPlugin[];
  plugins: PluginRuntimeView[];
  events: EventRecord[];
  audits: AuditRecord[];
  selectedCatalogPlugin: CatalogPlugin | null;
  selectedDevice: DeviceView | null;
  onOpenSection: (section: AppSection) => void;
  onSelectPlugin: (pluginId: string) => void;
};

export function OverviewSection({
  dashboard,
  catalog,
  plugins,
  events,
  audits,
  selectedCatalogPlugin,
  selectedDevice,
  onOpenSection,
  onSelectPlugin,
}: Props) {
  const overviewEvents = events.slice(0, 5);
  const overviewAudits = audits.slice(0, 5);

  return (
    <>
      <Section className="grid grid--stats">
        {[
          ['Plugins', dashboard?.plugins ?? 0],
          ['Enabled', dashboard?.enabled_plugins ?? 0],
          ['Devices', dashboard?.devices ?? 0],
          ['Online', dashboard?.online_devices ?? 0],
          ['Events', dashboard?.events ?? 0],
          ['Audits', dashboard?.audits ?? 0],
        ].map(([label, value]) => (
          <Card key={label as string} className="stat">
            <CardDescription>{label}</CardDescription>
            <CardTitle>{value as number}</CardTitle>
          </Card>
        ))}
      </Section>

      <Section className="grid grid--two">
        <Card>
          <CardHeader>
            <CardTitle>Runtime Snapshot</CardTitle>
            <CardDescription>Installed plugins in stable order. Open any item to inspect or reconfigure it.</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <div className="preview-list">
              {catalog.map((plugin) => {
                const runtime = plugins.find((item) => item.record.plugin_id === plugin.id);
                return (
                  <button
                    key={plugin.id}
                    type="button"
                    className="preview-list__item"
                    onClick={() => {
                      onSelectPlugin(plugin.id);
                      onOpenSection('plugins');
                    }}
                  >
                    <div>
                      <strong>{plugin.name}</strong>
                      <p>{plugin.id}</p>
                    </div>
                    <div className="chip-list">
                      <Badge tone={runtime?.record.status === 'enabled' ? 'good' : 'neutral'}>
                        {runtime?.record.status ?? 'uninstalled'}
                      </Badge>
                      <Badge
                        tone={
                          runtime?.health.status === 'healthy'
                            ? 'good'
                            : runtime?.health.status === 'unhealthy'
                              ? 'bad'
                              : 'warn'
                        }
                      >
                        {runtime?.health.status ?? 'unknown'}
                      </Badge>
                    </div>
                  </button>
                );
              })}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Recent Events</CardTitle>
            <CardDescription>Latest event bus activity with one-click jump into the activity module.</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <div className="feed">
              {overviewEvents.length > 0 ? (
                overviewEvents.map((event) => (
                  <article key={event.id} className="feed__item">
                    <div className="feed__meta">
                      <Badge tone="accent">{event.type}</Badge>
                      <span>{formatTime(event.ts)}</span>
                    </div>
                    <strong>{event.device_id || event.plugin_id || 'system'}</strong>
                  </article>
                ))
              ) : (
                <p className="muted">No recent events.</p>
              )}
            </div>
            <Button variant="secondary" onClick={() => onOpenSection('activity')}>
              Open Activity
            </Button>
          </CardContent>
        </Card>
      </Section>

      <Section className="grid grid--two">
        <Card>
          <CardHeader>
            <CardTitle>Recent Audits</CardTitle>
            <CardDescription>Latest policy decisions and command history.</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <div className="feed">
              {overviewAudits.length > 0 ? (
                overviewAudits.map((audit) => (
                  <article key={audit.id} className="feed__item">
                    <div className="feed__meta">
                      <Badge tone={audit.allowed ? 'good' : 'bad'}>{audit.allowed ? 'allowed' : 'denied'}</Badge>
                      <span>{formatTime(audit.created_at)}</span>
                    </div>
                    <strong>
                      {audit.actor} · {audit.action}
                    </strong>
                    <p>{audit.device_id}</p>
                  </article>
                ))
              ) : (
                <p className="muted">No recent audits.</p>
              )}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Quick Entry</CardTitle>
            <CardDescription>Jump straight into the currently selected device or plugin without scrolling through the whole console.</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            {selectedCatalogPlugin ? (
              <button type="button" className="preview-list__item" onClick={() => onOpenSection('plugins')}>
                <div>
                  <strong>{selectedCatalogPlugin.name}</strong>
                  <p>{selectedCatalogPlugin.id}</p>
                </div>
                <Badge tone="accent">Plugin</Badge>
              </button>
            ) : null}
            {selectedDevice ? (
              <button type="button" className="preview-list__item" onClick={() => onOpenSection('devices')}>
                <div>
                  <strong>{selectedDevice.device.name}</strong>
                  <p>{selectedDevice.device.id}</p>
                </div>
                <Badge tone={selectedDevice.device.online ? 'good' : 'bad'}>
                  {selectedDevice.device.online ? 'online' : 'offline'}
                </Badge>
              </button>
            ) : null}
          </CardContent>
        </Card>
      </Section>
    </>
  );
}
