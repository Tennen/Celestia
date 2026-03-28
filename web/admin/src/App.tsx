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
  runActionControl,
  sendCommand,
  updateDeviceControlPreference,
  updatePluginConfig,
} from './lib/api';
import { asArray, buildInstallDrafts, canStartXiaomiOAuth, getPluginDraftText, mergeCatalogDefaultConfig, mergeXiaomiAccountConfig } from './lib/admin';
import { prettyJson } from './lib/utils';
import type { CatalogPlugin } from './lib/types';
import type { AppSection } from './lib/admin';
import { useAdminConsole } from './hooks/useAdminConsole';
import { useToggleControlActions } from './hooks/useToggleControlActions';
import { useXiaomiOAuth } from './hooks/useXiaomiOAuth';

function App() {
  const [activeSection, setActiveSection] = useState<AppSection>('overview');
  const [selectedAction, setSelectedAction] = useState('feed_once');
  const [commandParams, setCommandParams] = useState('{\n  "portions": 1\n}');
  const [actor, setActor] = useState('admin');
  const [xiaomiVerifyTicket, setXiaomiVerifyTicket] = useState('');
  const [installDrafts, setInstallDrafts] = useState<Record<string, string>>({});
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

  const { toggleOverrides, togglePending, onToggleControl } = useToggleControlActions({
    actor,
    devices: state.devices,
    selectedDevice,
    refreshAll,
    reportError,
  });

  const commandSuggestions = useMemo(() => {
    if (!selectedDevice) return [];
    const capabilities = asArray(selectedDevice.device.capabilities);
    const selectedSound = Number((selectedDevice.state.state as Record<string, unknown> | undefined)?.selected_sound ?? 0);
    const suggestions: Array<{ label: string; action: string; params: Record<string, unknown> }> = [];
    if (capabilities.includes('feed_once')) suggestions.push({ label: 'Feed once', action: 'feed_once', params: { portions: 1 } });
    if (capabilities.includes('manual_feed_dual'))
      suggestions.push({ label: 'Feed dual', action: 'manual_feed_dual', params: { amount1: 20, amount2: 20 } });
    if (capabilities.includes('cancel_manual_feed'))
      suggestions.push({ label: 'Cancel feed', action: 'cancel_manual_feed', params: {} });
    if (capabilities.includes('reset_desiccant'))
      suggestions.push({ label: 'Reset desiccant', action: 'reset_desiccant', params: {} });
    if (capabilities.includes('food_replenished'))
      suggestions.push({ label: 'Food replenished', action: 'food_replenished', params: {} });
    if (capabilities.includes('call_pet')) suggestions.push({ label: 'Call pet', action: 'call_pet', params: {} });
    if (capabilities.includes('play_sound') && selectedSound > 0)
      suggestions.push({ label: 'Play sound', action: 'play_sound', params: { sound_id: selectedSound } });
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

  useEffect(() => {
    const drafts: Record<string, string> = {};
    for (const plugin of state.plugins) {
      const catalogPlugin = state.catalog.find((item) => item.id === plugin.record.plugin_id);
      const config = catalogPlugin
        ? mergeCatalogDefaultConfig(catalogPlugin, plugin.record.config ?? {})
        : (plugin.record.config ?? {});
      drafts[plugin.record.plugin_id] = JSON.stringify(config, null, 2);
    }
    setConfigDrafts((current) => ({ ...drafts, ...current }));
  }, [state.catalog, state.plugins]);

  useEffect(() => {
    const defaults = buildInstallDrafts(state.catalog);
    setInstallDrafts((current) => ({ ...defaults, ...current }));
  }, [state.catalog]);

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
              Plugin orchestration, device control, and event inspection against <code>{getApiBase()}</code>
            </p>
            <div className="sidemenu__meta">
              <Badge tone="accent">Gateway API</Badge>
              <Badge tone={state.loading ? 'accent' : state.error ? 'bad' : 'good'}>
                {state.loading ? 'Refreshing' : state.error ? 'Attention' : 'Stable'}
              </Badge>
            </div>
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
            <div className="module-header__meta">
              <Badge tone={state.loading ? 'accent' : state.error ? 'bad' : 'good'}>
                {state.loading ? 'Refreshing' : state.error ? 'Needs Attention' : 'Runtime Stable'}
              </Badge>
              <span>
                Endpoint <code>{getApiBase()}</code>
              </span>
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
              xiaomiVerifyTicket={xiaomiVerifyTicket}
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
              onXiaomiVerifyTicketChange={setXiaomiVerifyTicket}
              onConnectXiaomiOAuth={() => {
                if (!selectedCatalogPlugin) {
                  return;
                }
                void runAction(`xiaomi-oauth-${selectedCatalogPlugin.id}`, () => startXiaomiOAuthFlow(selectedCatalogPlugin));
              }}
              onRetryXiaomiVerification={(verifyURL) => {
                if (!selectedCatalogPlugin || !selectedPlugin) {
                  return;
                }
                const ticket = xiaomiVerifyTicket.trim();
                if (!ticket) {
                  return;
                }
                const merged = mergeXiaomiAccountConfig(pluginDraft, {
                  verify_url: verifyURL,
                  verify_ticket: ticket,
                });
                setConfigDrafts((current) => ({ ...current, [selectedCatalogPlugin.id]: merged.json }));
                void runAction(`xiaomi-verify-${selectedCatalogPlugin.id}`, async () => {
                  await updatePluginConfig(selectedCatalogPlugin.id, merged.draft);
                  setXiaomiVerifyTicket('');
                });
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
              toggleOverrides={toggleOverrides}
              togglePending={togglePending}
              busy={busy}
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
              onToggleControl={onToggleControl}
              onActionControl={(controlId) => {
                if (!selectedDevice) {
                  return;
                }
                const compoundId = `${selectedDevice.device.id}.${controlId}`;
                void runAction(`action-${compoundId}`, () => runActionControl(compoundId, actor));
              }}
              onValueControl={(controlId, value) => {
                if (!selectedDevice) {
                  return;
                }
                const control = (selectedDevice.controls ?? []).find((item) => item.id === controlId);
                if (!control?.command?.action) {
                  return;
                }
                const params = {
                  ...(control.command.params ?? {}),
                  [(control.command.value_param ?? 'value') as string]: value,
                };
                void runAction(`value-${selectedDevice.device.id}.${controlId}`, () =>
                  sendCommand(selectedDevice.device.id, control.command!.action, params, actor),
                );
              }}
              onUpdateControlPreference={(controlId, payload) => {
                if (!selectedDevice) {
                  return;
                }
                void runAction(`control-pref-${selectedDevice.device.id}.${controlId}`, () =>
                  updateDeviceControlPreference(selectedDevice.device.id, controlId, payload),
                );
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
