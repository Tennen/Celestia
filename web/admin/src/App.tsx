import { useEffect, useMemo, useState } from 'react';
import { ActivitySection } from './components/admin/ActivitySection';
import { DeviceWorkspace } from './components/admin/DeviceWorkspace';
import { OverviewSection } from './components/admin/OverviewSection';
import { PluginWorkspace } from './components/admin/PluginWorkspace';
import { Badge } from './components/ui/badge';
import { Button } from './components/ui/button';
import { Card, CardContent } from './components/ui/card';
import {
  deletePlugin,
  discoverPlugin,
  enablePlugin,
  disablePlugin,
  getApiBase,
  installPlugin,
  sendCommand,
  updatePluginConfig,
} from './lib/api';
import { asArray, canStartXiaomiOAuth, DEFAULT_INSTALL_CONFIGS, getPluginDraftText } from './lib/admin';
import { prettyJson } from './lib/utils';
import type { CatalogPlugin } from './lib/types';
import type { AppSection } from './lib/admin';
import { useAdminConsole } from './hooks/useAdminConsole';
import { useXiaomiOAuth } from './hooks/useXiaomiOAuth';

function App() {
  const [activeSection, setActiveSection] = useState<AppSection>('overview');
  const [selectedAction, setSelectedAction] = useState('feed_once');
  const [commandParams, setCommandParams] = useState('{\n  "portions": 1\n}');
  const [actor, setActor] = useState('admin');
  const [installDrafts, setInstallDrafts] = useState<Record<string, string>>(DEFAULT_INSTALL_CONFIGS);
  const [configDrafts, setConfigDrafts] = useState<Record<string, string>>({});
  const [commandResult, setCommandResult] = useState<string>('');
  const [busy, setBusy] = useState<string>('');
  const {
    state,
    refreshAll,
    selectedPluginId,
    setSelectedPluginId,
    selectedDeviceId,
    setSelectedDeviceId,
    deviceSearch,
    setDeviceSearch,
    pluginLogs,
    reloadPluginLogs,
    selectedCatalogPlugin,
    selectedPlugin,
    selectedDevice,
    reportError,
  } = useAdminConsole();

  const { oauthBanner, oauthActive, startFlow: startXiaomiOAuthFlow } = useXiaomiOAuth({
    plugins: state.plugins,
    installDrafts,
    configDrafts,
    setInstallDrafts,
    setConfigDrafts,
  });

  useEffect(() => {
    const drafts: Record<string, string> = {};
    for (const plugin of state.plugins) {
      drafts[plugin.record.plugin_id] = JSON.stringify(plugin.record.config ?? {}, null, 2);
    }
    setConfigDrafts((current) => ({ ...drafts, ...current }));
  }, [state.plugins]);

  const commandSuggestions = useMemo(() => {
    if (!selectedDevice) return [];
    const capabilities = asArray(selectedDevice.device.capabilities);
    const suggestions: Array<{ label: string; action: string; params: Record<string, unknown> }> = [];
    if (capabilities.includes('feed_once')) suggestions.push({ label: 'Feed once', action: 'feed_once', params: { portions: 1 } });
    if (capabilities.includes('clean_now')) suggestions.push({ label: 'Clean now', action: 'clean_now', params: {} });
    if (capabilities.includes('start')) suggestions.push({ label: 'Start cycle', action: 'start', params: {} });
    if (capabilities.includes('pause')) suggestions.push({ label: 'Pause', action: 'pause', params: {} });
    if (capabilities.includes('resume')) suggestions.push({ label: 'Resume', action: 'resume', params: {} });
    if (capabilities.includes('power')) suggestions.push({ label: 'Toggle power', action: 'set_power', params: { on: true } });
    if (capabilities.includes('pump_power')) suggestions.push({ label: 'Pump on', action: 'set_pump_power', params: { on: true } });
    if (capabilities.includes('light_power')) suggestions.push({ label: 'Aquarium light', action: 'set_light_power', params: { on: true } });
    if (capabilities.includes('light_brightness'))
      suggestions.push({ label: 'Aquarium brightness', action: 'set_light_brightness', params: { value: 80 } });
    if (capabilities.includes('voice_push'))
      suggestions.push({ label: 'Voice push', action: 'push_voice_message', params: { message: '检测到异常，请查看鱼缸状态', volume: 55 } });
    if (capabilities.includes('volume')) suggestions.push({ label: 'Set volume', action: 'set_volume', params: { value: 55 } });
    if (capabilities.includes('mute')) suggestions.push({ label: 'Mute speaker', action: 'set_mute', params: { on: true } });
    return suggestions;
  }, [selectedDevice]);

  const selectedDeviceDetails = selectedDevice ? JSON.stringify(selectedDevice, null, 2) : '';

  const runAction = async (label: string, action: () => Promise<unknown>) => {
    setBusy(label);
    try {
      await action();
      await refreshAll();
    } catch (error) {
      reportError(error instanceof Error ? error.message : 'Operation failed');
    } finally {
      setBusy('');
    }
  };

  const onInstall = async (plugin: CatalogPlugin) => {
    const config = JSON.parse(installDrafts[plugin.id] ?? '{}') as Record<string, unknown>;
    await installPlugin({ plugin_id: plugin.id, config });
  };

  const onUpdateConfig = async (pluginId: string) => {
    const config = JSON.parse(configDrafts[pluginId] ?? '{}') as Record<string, unknown>;
    await updatePluginConfig(pluginId, config);
  };

  const onSendCommand = async () => {
    if (!selectedDevice) return;
    const parsed = JSON.parse(commandParams || '{}') as Record<string, unknown>;
    const response = await sendCommand(selectedDevice.device.id, selectedAction, parsed, actor);
    setCommandResult(prettyJson(response));
  };

  const sectionMeta: Record<AppSection, { label: string; description: string }> = {
    overview: {
      label: 'Overview',
      description: 'Dashboard summary and recent runtime activity.',
    },
    plugins: {
      label: 'Plugins',
      description: 'Browse a stable plugin list and open each plugin in its own detail pane.',
    },
    devices: {
      label: 'Devices',
      description: 'Browse unified devices and issue commands against the selected item.',
    },
    activity: {
      label: 'Activity',
      description: 'Inspect recent events and audit decisions.',
    },
  };

  const sectionItems: Array<{ id: AppSection; label: string; count: number }> = [
    { id: 'overview', label: 'Overview', count: state.dashboard?.plugins ?? 0 },
    { id: 'plugins', label: 'Plugins', count: state.catalog.length },
    { id: 'devices', label: 'Devices', count: state.devices.length },
    { id: 'activity', label: 'Activity', count: state.events.length + state.audits.length },
  ];

  const pluginDraft = selectedCatalogPlugin
    ? getPluginDraftText(selectedCatalogPlugin.id, Boolean(selectedPlugin), installDrafts, configDrafts)
    : '{}';
  const xiaomiOAuthAvailable =
    selectedCatalogPlugin?.id === 'xiaomi' && canStartXiaomiOAuth(pluginDraft);

  return (
    <div className="shell shell--app">
      <div className="ambient ambient--one" />
      <div className="ambient ambient--two" />

      <div className="app-frame">
        <aside className="sidemenu">
          <div className="sidemenu__brand">
            <p className="eyebrow">Celestia Core Runtime</p>
            <h1>Admin Console</h1>
            <p className="topbar__sub">
              Plugin orchestration, device control, and event inspection against <code>{getApiBase()}</code>
            </p>
          </div>
          <nav className="sidemenu__nav">
            {sectionItems.map((section) => (
              <button
                key={section.id}
                type="button"
                className={`sidemenu__button ${activeSection === section.id ? 'is-active' : ''}`}
                onClick={() => setActiveSection(section.id)}
              >
                <span>{section.label}</span>
                <Badge tone={activeSection === section.id ? 'accent' : 'neutral'}>{section.count}</Badge>
              </button>
            ))}
          </nav>
          <div className="sidemenu__footer">
            <Badge tone={state.error ? 'bad' : 'good'}>{state.error ? 'Degraded' : 'Connected'}</Badge>
            <Button variant="secondary" onClick={() => void refreshAll()} disabled={state.loading}>
              Refresh
            </Button>
          </div>
        </aside>

        <main className="workspace">
          <header className="module-header">
            <div>
              <p className="eyebrow">{sectionMeta[activeSection].label}</p>
              <h2>{sectionMeta[activeSection].label}</h2>
              <p className="topbar__sub">{sectionMeta[activeSection].description}</p>
            </div>
          </header>

          {state.error ? (
            <Card className="panel panel--error">
              <CardContent>{state.error}</CardContent>
            </Card>
          ) : null}

          {oauthBanner ? (
            <Card className="panel panel--notice">
              <CardContent className="panel__notice">
                <Badge tone={oauthBanner.tone}>
                  {oauthBanner.tone === 'good' ? 'Connected' : oauthBanner.tone === 'warn' ? 'Pending' : 'Error'}
                </Badge>
                <span>{oauthBanner.text}</span>
              </CardContent>
            </Card>
          ) : null}

          {activeSection === 'overview' ? (
            <OverviewSection
              dashboard={state.dashboard}
              catalog={state.catalog}
              plugins={state.plugins}
              events={state.events}
              audits={state.audits}
              selectedCatalogPlugin={selectedCatalogPlugin}
              selectedDevice={selectedDevice}
              onOpenSection={setActiveSection}
              onSelectPlugin={setSelectedPluginId}
            />
          ) : null}

          {activeSection === 'plugins' ? (
            <PluginWorkspace
              catalog={state.catalog}
              plugins={state.plugins}
              selectedPluginId={selectedPluginId}
              selectedCatalogPlugin={selectedCatalogPlugin}
              selectedPlugin={selectedPlugin}
              pluginDraft={pluginDraft}
              pluginLogs={pluginLogs}
              busy={busy}
              xiaomiOAuthActive={oauthActive}
              xiaomiOAuthAvailable={xiaomiOAuthAvailable}
              onSelectPlugin={setSelectedPluginId}
              onDraftChange={(value) => {
                if (!selectedCatalogPlugin) {
                  return;
                }
                if (selectedPlugin) {
                  setConfigDrafts((current) => ({ ...current, [selectedCatalogPlugin.id]: value }));
                  return;
                }
                setInstallDrafts((current) => ({ ...current, [selectedCatalogPlugin.id]: value }));
              }}
              onConnectXiaomiOAuth={() => {
                if (!selectedCatalogPlugin) {
                  return;
                }
                void runAction(`xiaomi-oauth-${selectedCatalogPlugin.id}`, () => startXiaomiOAuthFlow(selectedCatalogPlugin));
              }}
              onInstall={() => {
                if (!selectedCatalogPlugin) {
                  return;
                }
                void runAction(`install-${selectedCatalogPlugin.id}`, () => onInstall(selectedCatalogPlugin));
              }}
              onEnable={() => {
                if (!selectedCatalogPlugin) {
                  return;
                }
                void runAction(`enable-${selectedCatalogPlugin.id}`, () => enablePlugin(selectedCatalogPlugin.id));
              }}
              onDisable={() => {
                if (!selectedCatalogPlugin) {
                  return;
                }
                void runAction(`disable-${selectedCatalogPlugin.id}`, () => disablePlugin(selectedCatalogPlugin.id));
              }}
              onDiscover={() => {
                if (!selectedCatalogPlugin) {
                  return;
                }
                void runAction(`discover-${selectedCatalogPlugin.id}`, () => discoverPlugin(selectedCatalogPlugin.id));
              }}
              onDelete={() => {
                if (!selectedCatalogPlugin) {
                  return;
                }
                void runAction(`delete-${selectedCatalogPlugin.id}`, () => deletePlugin(selectedCatalogPlugin.id));
              }}
              onSaveConfig={() => {
                if (!selectedCatalogPlugin) {
                  return;
                }
                void runAction(`refresh-config-${selectedCatalogPlugin.id}`, () => onUpdateConfig(selectedCatalogPlugin.id));
              }}
              onReloadLogs={() => {
                if (!selectedCatalogPlugin) {
                  return;
                }
                void reloadPluginLogs(selectedCatalogPlugin.id);
              }}
            />
          ) : null}

          {activeSection === 'devices' ? (
            <DeviceWorkspace
              deviceSearch={deviceSearch}
              onDeviceSearchChange={setDeviceSearch}
              onRefresh={() => void refreshAll()}
              devices={state.devices}
              selectedDeviceId={selectedDeviceId}
              onSelectDevice={setSelectedDeviceId}
              selectedDevice={selectedDevice}
              selectedAction={selectedAction}
              onSelectedActionChange={setSelectedAction}
              actor={actor}
              onActorChange={setActor}
              commandParams={commandParams}
              onCommandParamsChange={setCommandParams}
              commandSuggestions={commandSuggestions}
              onApplySuggestion={(action, params) => {
                setSelectedAction(action);
                setCommandParams(JSON.stringify(params, null, 2));
              }}
              onSendCommand={() => {
                void runAction('send-command', onSendCommand);
              }}
              commandResult={commandResult}
              selectedDeviceDetails={selectedDeviceDetails}
            />
          ) : null}

          {activeSection === 'activity' ? <ActivitySection events={state.events} audits={state.audits} /> : null}
        </main>
      </div>
    </div>
  );
}

export default App;
