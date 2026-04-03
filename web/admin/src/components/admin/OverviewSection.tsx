import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { ScrollArea } from '../ui/scroll-area';
import { Section } from '../ui/section';
import { SelectableListItem } from './shared/SelectableListItem';
import type { AppSection } from '../../lib/admin';
import { getPluginStatusBadge } from '../../lib/plugin-status';
import { formatTime } from '../../lib/utils';
import type { CatalogPlugin, DashboardSummary, EventRecord, PluginRuntimeView } from '../../lib/types';

type Props = {
  dashboard: DashboardSummary | null;
  catalog: CatalogPlugin[];
  plugins: PluginRuntimeView[];
  events: EventRecord[];
  onOpenSection: (section: AppSection) => void;
  onSelectPlugin: (pluginId: string) => void;
};

export function OverviewSection({
  dashboard,
  catalog,
  plugins,
  events,
  onOpenSection,
  onSelectPlugin,
}: Props) {
  const overviewEvents = events.slice(0, 5);

  return (
    <Section className="overview-panel">
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

      <Section stack={false} className="overview-panel__main">
        <Card className="explorer-card">
          <CardHeader>
            <CardTitle>Runtime Snapshot</CardTitle>
            <CardDescription>Installed plugins in stable order. Open any item to inspect or reconfigure it.</CardDescription>
          </CardHeader>
          <CardContent className="explorer-card__content">
            <ScrollArea className="explorer-scroll">
              <div className="preview-list">
                {catalog.map((plugin) => {
                  const runtime = plugins.find((item) => item.record.plugin_id === plugin.id);
                  const statusBadge = getPluginStatusBadge(runtime);
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
                        <Badge tone={statusBadge.tone} size="xs">
                          {statusBadge.label}
                        </Badge>
                      }
                    />
                  );
                })}
              </div>
            </ScrollArea>
          </CardContent>
        </Card>

        <Card className="explorer-card">
          <CardHeader>
            <CardTitle>Recent Events</CardTitle>
            <CardDescription>Latest event bus activity with one-click jump into the activity module.</CardDescription>
          </CardHeader>
          <CardContent className="explorer-card__content">
            <ScrollArea className="explorer-scroll">
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
    </Section>
  );
}
