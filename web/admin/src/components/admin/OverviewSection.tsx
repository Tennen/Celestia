import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { ScrollArea } from '../ui/scroll-area';
import { Section } from '../ui/section';
import { SelectableListItem } from './shared/SelectableListItem';
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
      <Section stack={false} className="grid grid--stats">
        {[
          ['Plugins', dashboard?.plugins ?? 0],
          ['Enabled', dashboard?.enabled_plugins ?? 0],
          ['Devices', dashboard?.devices ?? 0],
          ['Online', dashboard?.online_devices ?? 0],
          ['Events', dashboard?.events ?? 0],
          ['Audits', dashboard?.audits ?? 0],
        ].map(([label, value]) => (
          <Card key={label as string} className="stat">
            <div className="stat__content">
              <CardDescription className="stat__label">{label}</CardDescription>
              <CardTitle className="stat__value">{value as number}</CardTitle>
            </div>
          </Card>
        ))}
      </Section>

      <Section stack={false} className="grid grid--two">
        <Card>
          <CardHeader>
            <CardTitle>Runtime Snapshot</CardTitle>
            <CardDescription>Installed plugins in stable order. Open any item to inspect or reconfigure it.</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <ScrollArea className="max-h-[360px] pr-4">
              <div className="preview-list">
                {catalog.map((plugin) => {
                  const runtime = plugins.find((item) => item.record.plugin_id === plugin.id);
                  return (
                    <SelectableListItem
                      key={plugin.id}
                      className="preview-list__item"
                      onClick={() => {
                        onSelectPlugin(plugin.id);
                        onOpenSection('plugins');
                      }}
                      title={plugin.name}
                      description={plugin.id}
                      badges={
                        <>
                          <Badge
                            tone={runtime?.record.status === 'enabled' ? 'good' : 'neutral'}
                            size="sm"
                          >
                            {runtime?.record.status ?? 'uninstalled'}
                          </Badge>
                          <Badge
                            size="sm"
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
                        </>
                      }
                    />
                  );
                })}
              </div>
            </ScrollArea>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Recent Events</CardTitle>
            <CardDescription>Latest event bus activity with one-click jump into the activity module.</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <ScrollArea className="max-h-[320px] pr-4">
              <div className="feed">
                {overviewEvents.length > 0 ? (
                  overviewEvents.map((event) => (
                    <article key={event.id} className="feed__item">
                      <div className="feed__meta">
                        <Badge tone="accent" size="sm">
                          {event.type}
                        </Badge>
                        <span>{formatTime(event.ts)}</span>
                      </div>
                      <strong>{event.device_id || event.plugin_id || 'system'}</strong>
                    </article>
                  ))
                ) : (
                  <p className="muted">No recent events.</p>
                )}
              </div>
            </ScrollArea>
            <Button variant="secondary" onClick={() => onOpenSection('activity')}>
              Open Activity
            </Button>
          </CardContent>
        </Card>
      </Section>

      <Section stack={false} className="grid grid--two">
        <Card>
          <CardHeader>
            <CardTitle>Recent Audits</CardTitle>
            <CardDescription>Latest policy decisions and command history.</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <ScrollArea className="max-h-[320px] pr-4">
              <div className="feed">
                {overviewAudits.length > 0 ? (
                  overviewAudits.map((audit) => (
                    <article key={audit.id} className="feed__item">
                      <div className="feed__meta">
                        <Badge tone={audit.allowed ? 'good' : 'bad'} size="sm">
                          {audit.allowed ? 'allowed' : 'denied'}
                        </Badge>
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
            </ScrollArea>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Quick Entry</CardTitle>
            <CardDescription>Jump straight into the currently selected device or plugin without scrolling through the whole console.</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            {selectedCatalogPlugin ? (
              <SelectableListItem
                className="preview-list__item"
                onClick={() => onOpenSection('plugins')}
                title={selectedCatalogPlugin.name}
                description={selectedCatalogPlugin.id}
                badges={
                  <Badge tone="accent" size="sm">
                    Plugin
                  </Badge>
                }
              />
            ) : null}
            {selectedDevice ? (
              <SelectableListItem
                className="preview-list__item"
                onClick={() => onOpenSection('devices')}
                title={selectedDevice.device.name}
                description={selectedDevice.device.id}
                badges={
                  <Badge tone={selectedDevice.device.online ? 'good' : 'bad'} size="sm">
                    {selectedDevice.device.online ? 'online' : 'offline'}
                  </Badge>
                }
              />
            ) : null}
          </CardContent>
        </Card>
      </Section>
    </>
  );
}
