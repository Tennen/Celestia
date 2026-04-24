import { useEffect, useState } from 'react';
import { Badge } from './components/ui/badge';
import { Card, CardContent } from './components/ui/card';
import { ActivitySection } from './components/admin/ActivitySection';
import { AppSidemenu } from './components/admin/AppSidemenu';
import { AgentWorkspace } from './components/admin/AgentWorkspace';
import { CapabilityWorkspace } from './components/admin/CapabilityWorkspace';
import { DeviceWorkspace } from './components/admin/DeviceWorkspace';
import { OverviewSection } from './components/admin/OverviewSection';
import { PluginWorkspace } from './components/admin/PluginWorkspace';
import { TouchpointWorkspace } from './components/admin/TouchpointWorkspace';
import { WorkflowWorkspace } from './components/admin/WorkflowWorkspace';
import { agentPanelLabel, type AgentPanelId } from './lib/agent-admin';
import { getApiBase } from './lib/api';
import { capabilityDisplayName } from './lib/capability';
import type { AppSection } from './lib/admin';
import { cn } from './lib/utils';
import { workflowPageLabel, type WorkflowPageId } from './lib/workflow-admin';
import { useAdminStore, setAutoSelectHandlers } from './stores/adminStore';
import { usePluginStore } from './stores/pluginStore';
import { useDeviceStore } from './stores/deviceStore';
import { useXiaomiOAuth } from './hooks/useXiaomiOAuth';

function App() {
  const [activeSection, setActiveSection] = useState<AppSection>('overview');
  const [activeAgentPanel, setActiveAgentPanel] = useState<AgentPanelId>('llm');
  const [activeWorkflowPage, setActiveWorkflowPage] = useState<WorkflowPageId>('list');
  const [agentExpanded, setAgentExpanded] = useState(true);
  const [workflowExpanded, setWorkflowExpanded] = useState(true);
  const [activeCapabilityId, setActiveCapabilityId] = useState('');
  const [capabilitiesExpanded, setCapabilitiesExpanded] = useState(true);
  const [sidemenuCondensed, setSidemenuCondensed] = useState(false);

  const {
    loading,
    refreshing,
    hasLoaded,
    error,
    catalog,
    plugins,
    capabilities,
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

  // Wire auto-select handlers into adminStore
  useEffect(() => {
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

  // Init admin stream on mount
  useEffect(() => {
    const cleanup = useAdminStore.getState().initStream();
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

  useEffect(() => {
    if (!capabilities.length) {
      setActiveCapabilityId('');
      return;
    }
    setActiveCapabilityId((current) =>
      capabilities.some((capability) => capability.id === current) ? current : capabilities[0].id,
    );
  }, [capabilities]);

  const sectionLabel: Record<AppSection, string> = {
    overview: 'Overview',
    plugins: 'Plugins',
    workflow: 'Workflow',
    agent: 'Agent',
    touchpoints: 'Touchpoints',
    capabilities: 'Capabilities',
    devices: 'Devices',
    activity: 'Activity',
  };

  const selectedCatalogPlugin = catalog.find((p) => p.id === selectedPluginId) ?? null;
  const selectedDevice = devices.find((d) => d.device.id === selectedDeviceId) ?? null;
  const selectedCapability = capabilities.find((capability) => capability.id === activeCapabilityId) ?? capabilities[0] ?? null;
  const runtimeBadgeTone = !hasLoaded && loading ? 'accent' : error ? 'bad' : 'good';
  const runtimeBadgeText = !hasLoaded && loading ? 'Connecting' : error ? 'Attention' : 'Stable';
  const runtimeHeaderText = !hasLoaded && loading ? 'Connecting' : error ? 'Needs Attention' : 'Runtime Stable';
  const isRefreshing = loading || refreshing;
  const splitSection = activeSection !== 'overview';

  const openSection = (sectionId: AppSection) => {
    setActiveSection(sectionId);
    if (sectionId === 'workflow') {
      setWorkflowExpanded(true);
    }
    if (sectionId === 'agent') {
      setAgentExpanded(true);
    }
    if (sectionId === 'capabilities') {
      setCapabilitiesExpanded(true);
      if (!selectedCapability && capabilities[0]) {
        setActiveCapabilityId(capabilities[0].id);
      }
    }
  };

  const openWorkflowPage = (pageId: WorkflowPageId) => {
    setWorkflowExpanded(true);
    setActiveWorkflowPage(pageId);
    setActiveSection('workflow');
  };

  const openAgentPanel = (panelId: AgentPanelId) => {
    setAgentExpanded(true);
    setActiveAgentPanel(panelId);
    setActiveSection('agent');
  };

  const openCapability = (capabilityId: string) => {
    setCapabilitiesExpanded(true);
    setActiveCapabilityId(capabilityId);
    setActiveSection('capabilities');
  };

  return (
    <div className="shell shell--app">
      <div className="ambient ambient--one" />
      <div className="ambient ambient--two" />
      <div className="ambient ambient--three" />

      <div className={cn('app-frame', sidemenuCondensed && 'app-frame--condensed')}>
        <AppSidemenu
          activeSection={activeSection}
          activeAgentPanel={activeAgentPanel}
          activeWorkflowPage={activeWorkflowPage}
          selectedCapabilityId={selectedCapability?.id ?? ''}
          agentExpanded={agentExpanded}
          workflowExpanded={workflowExpanded}
          capabilitiesExpanded={capabilitiesExpanded}
          sidemenuCondensed={sidemenuCondensed}
          catalogCount={catalog.length}
          pluginCount={dashboard?.plugins ?? 0}
          deviceCount={devices.length}
          activityCount={events.length + audits.length}
          capabilities={capabilities}
          error={error}
          runtimeBadgeText={runtimeBadgeText}
          runtimeBadgeTone={runtimeBadgeTone}
          onOpenSection={openSection}
          onOpenAgentPanel={openAgentPanel}
          onOpenWorkflowPage={openWorkflowPage}
          onOpenCapability={openCapability}
          onToggleAgentExpanded={() => setAgentExpanded((current) => !current)}
          onToggleWorkflowExpanded={() => setWorkflowExpanded((current) => !current)}
          onToggleCapabilitiesExpanded={() => setCapabilitiesExpanded((current) => !current)}
          onToggleSidemenuCondensed={() => setSidemenuCondensed((current) => !current)}
          onRefresh={() => void refreshAll()}
          refreshing={isRefreshing}
        />

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
              {activeSection === 'capabilities' && selectedCapability ? (
                <span>
                  Capability <strong>{capabilityDisplayName(selectedCapability)}</strong>
                </span>
              ) : null}
              {activeSection === 'workflow' ? (
                <span>
                  Workflow <strong>{workflowPageLabel(activeWorkflowPage)}</strong>
                </span>
              ) : null}
              {activeSection === 'agent' ? (
                <span>
                  Agent <strong>{agentPanelLabel(activeAgentPanel)}</strong>
                </span>
              ) : null}
              {activeSection === 'touchpoints' ? (
                <span>
                  Project <strong>Touchpoints</strong>
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

              {activeSection === 'workflow' ? (
                <WorkflowWorkspace activePage={activeWorkflowPage} onSelectPage={openWorkflowPage} />
              ) : null}

              {activeSection === 'agent' ? <AgentWorkspace activePanel={activeAgentPanel} /> : null}

              {activeSection === 'touchpoints' ? <TouchpointWorkspace /> : null}

              {activeSection === 'capabilities' ? (
                <CapabilityWorkspace selectedCapabilityId={selectedCapability?.id ?? ''} />
              ) : null}

              {activeSection === 'devices' ? <DeviceWorkspace /> : null}

              {activeSection === 'activity' ? (
                <ActivitySection audits={audits} />
              ) : null}
            </div>
          </div>
        </main>
      </div>
    </div>
  );
}

export default App;
