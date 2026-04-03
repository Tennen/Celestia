import { useEffect, useState } from 'react';
import {
  Activity,
  Home,
  PlugZap,
  RefreshCw,
  Smartphone,
  Workflow,
} from 'lucide-react';
import { AutomationWorkspace } from './components/admin/AutomationWorkspace';
import { ActivitySection } from './components/admin/ActivitySection';
import { DeviceWorkspace } from './components/admin/DeviceWorkspace';
import { OverviewSection } from './components/admin/OverviewSection';
import { PluginWorkspace } from './components/admin/PluginWorkspace';
import { Badge } from './components/ui/badge';
import { Button } from './components/ui/button';
import { Card, CardContent } from './components/ui/card';
import { getApiBase } from './lib/api';
import type { AppSection } from './lib/admin';
import { useAdminStore, setAutoSelectHandlers, setDeviceSearchProvider } from './stores/adminStore';
import { usePluginStore } from './stores/pluginStore';
import { useDeviceStore } from './stores/deviceStore';
import { useXiaomiOAuth } from './hooks/useXiaomiOAuth';

function App() {
  const [activeSection, setActiveSection] = useState<AppSection>('overview');

  const {
    loading,
    refreshing,
    hasLoaded,
    error,
    catalog,
    plugins,
    automations,
    devices,
    events,
    audits,
    dashboard,
    refreshAll,
  } =
    useAdminStore();
  const { selectedPluginId } = usePluginStore();
  const { selectedDeviceId } = useDeviceStore();
  const { oauthBanner, oauthActive, startFlow } = useXiaomiOAuth();

  // Wire auto-select handlers and device search provider into adminStore
  useEffect(() => {
    setDeviceSearchProvider(() => useDeviceStore.getState().deviceSearch);
    setAutoSelectHandlers(
      (cat) => {
        const { selectedPluginId, setSelectedPluginId } = usePluginStore.getState();
        if (!selectedPluginId && cat.length > 0) {
          setSelectedPluginId(cat[0].id);
        } else if (selectedPluginId && !cat.some((p) => p.id === selectedPluginId)) {
          setSelectedPluginId(cat[0]?.id ?? '');
        }
      },
      (devs) => {
        const { selectedDeviceId, setSelectedDeviceId } = useDeviceStore.getState();
        if (!selectedDeviceId && devs.length > 0) {
          setSelectedDeviceId(devs[0].device.id);
        } else if (selectedDeviceId && !devs.some((d) => d.device.id === selectedDeviceId)) {
          setSelectedDeviceId(devs[0]?.device.id ?? '');
        }
      },
    );
  }, []);

  // Init polling + SSE on mount
  useEffect(() => {
    const cleanup = useAdminStore.getState().initPolling();
    return cleanup;
  }, []);

  // Sync drafts whenever catalog/plugins change
  useEffect(() => {
    usePluginStore.getState().initDraftsFromCatalog(catalog, plugins);
  }, [catalog, plugins]);

  // Prune stale toggle overrides when devices list changes
  useEffect(() => {
    useDeviceStore.getState().pruneOverrides(devices);
  }, [devices]);

  const sectionLabel: Record<AppSection, string> = {
    overview: 'Overview',
    plugins: 'Plugins',
    automations: 'Automations',
    devices: 'Devices',
    activity: 'Activity',
  };

  const sectionItems: Array<{
    id: AppSection;
    label: string;
    count: number;
    icon: typeof Home;
  }> = [
    { id: 'overview', label: 'Overview', count: dashboard?.plugins ?? 0, icon: Home },
    { id: 'plugins', label: 'Plugins', count: catalog.length, icon: PlugZap },
    { id: 'automations', label: 'Automations', count: automations.length, icon: Workflow },
    { id: 'devices', label: 'Devices', count: devices.length, icon: Smartphone },
    { id: 'activity', label: 'Activity', count: events.length + audits.length, icon: Activity },
  ];

  const selectedCatalogPlugin = catalog.find((p) => p.id === selectedPluginId) ?? null;
  const selectedDevice = devices.find((d) => d.device.id === selectedDeviceId) ?? null;
  const runtimeBadgeTone = !hasLoaded && loading ? 'accent' : error ? 'bad' : 'good';
  const runtimeBadgeText = !hasLoaded && loading ? 'Connecting' : error ? 'Attention' : 'Stable';
  const runtimeHeaderText = !hasLoaded && loading ? 'Connecting' : error ? 'Needs Attention' : 'Runtime Stable';
  const isRefreshing = loading || refreshing;
  const splitSection = activeSection !== 'overview';

  return (
    <div className="shell shell--app">
      <div className="ambient ambient--one" />
      <div className="ambient ambient--two" />
      <div className="ambient ambient--three" />

      <div className="app-frame">
        <aside className="sidemenu">
          <div className="sidemenu__brand">
            <p className="eyebrow">Celestia Core Runtime</p>
            <h1>Admin Console</h1>
            <p className="topbar__sub">
              Real plugin orchestration, device control, and runtime inspection against{' '}
              <code>{getApiBase()}</code>
            </p>
            <div className="sidemenu__meta">
              <Badge tone="accent" size="xs">
                Gateway API
              </Badge>
              <Badge tone={runtimeBadgeTone} size="xs">
                {runtimeBadgeText}
              </Badge>
            </div>
          </div>
          <nav className="sidemenu__nav">
            {sectionItems.map((section) => {
              const Icon = section.icon;
              return (
                <button
                  key={section.id}
                  type="button"
                  className={`sidemenu__button ${activeSection === section.id ? 'is-active' : ''}`}
                  onClick={() => setActiveSection(section.id)}
                >
                  <span className="flex items-center gap-3">
                    <Icon className="h-4 w-4" />
                    {section.label}
                  </span>
                  <Badge
                    tone={activeSection === section.id ? 'accent' : 'neutral'}
                    size="xs"
                    className="min-w-6 tabular-nums"
                  >
                    {section.count}
                  </Badge>
                </button>
              );
            })}
          </nav>
          <div className="sidemenu__footer">
            <Badge tone={error ? 'bad' : 'good'} size="xs">
              {error ? 'Degraded' : 'Connected'}
            </Badge>
            <Button
              variant="secondary"
              onClick={() => void refreshAll()}
              aria-busy={isRefreshing}
              className="min-w-[7.25rem] border-slate-700 bg-white/10 text-white hover:bg-white/15 hover:text-white"
            >
              <RefreshCw className={`mr-2 h-4 w-4 ${isRefreshing ? 'animate-spin' : ''}`} />
              Refresh
            </Button>
          </div>
        </aside>

        <main className="workspace">
          <header className="module-header">
            <p className="module-header__title">{sectionLabel[activeSection]}</p>
            <div className="module-header__meta">
              <Badge tone={runtimeBadgeTone}>{runtimeHeaderText}</Badge>
              {refreshing ? (
                <Badge tone="neutral" size="sm">
                  Syncing
                </Badge>
              ) : null}
              <span>
                Endpoint <code>{getApiBase()}</code>
              </span>
              {selectedCatalogPlugin ? (
                <span>
                  Plugin <strong>{selectedCatalogPlugin.name}</strong>
                </span>
              ) : null}
              {selectedDevice ? (
                <span>
                  Device <strong>{selectedDevice.device.name}</strong>
                </span>
              ) : null}
            </div>
          </header>

          <div className="workspace__body">
            {error ? (
              <Card className="panel panel--error">
                <CardContent>{error}</CardContent>
              </Card>
            ) : null}

            {oauthBanner ? (
              <Card className="panel panel--notice">
                <CardContent className="panel__notice">
                  <Badge tone={oauthBanner.tone}>
                    {oauthBanner.tone === 'good'
                      ? 'Connected'
                      : oauthBanner.tone === 'warn'
                        ? 'Pending'
                        : 'Error'}
                  </Badge>
                  <span>{oauthBanner.text}</span>
                </CardContent>
              </Card>
            ) : null}

            <div className={`workspace__panel ${splitSection ? 'workspace__panel--fixed' : 'workspace__panel--scroll'}`}>
              {activeSection === 'overview' ? (
                <OverviewSection
                  dashboard={dashboard}
                  catalog={catalog}
                  plugins={plugins}
                  events={events}
                  onOpenSection={setActiveSection}
                  onSelectPlugin={(id) => usePluginStore.getState().setSelectedPluginId(id)}
                />
              ) : null}

              {activeSection === 'plugins' ? (
                <PluginWorkspace
                  oauthActive={oauthActive}
                  onConnectXiaomiOAuth={() => {
                    const { selectedPluginId } = usePluginStore.getState();
                    const plugin = catalog.find((p) => p.id === selectedPluginId);
                    if (!plugin) return;
                    void usePluginStore.setState({ busy: `xiaomi-oauth-${plugin.id}` });
                    void startFlow(plugin)
                      .catch(() => {})
                      .finally(() => usePluginStore.setState({ busy: '' }));
                  }}
                />
              ) : null}

              {activeSection === 'automations' ? <AutomationWorkspace /> : null}

              {activeSection === 'devices' ? <DeviceWorkspace /> : null}

              {activeSection === 'activity' ? (
                <ActivitySection events={events} audits={audits} />
              ) : null}
            </div>
          </div>
        </main>
      </div>
    </div>
  );
}

export default App;
