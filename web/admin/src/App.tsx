import { useEffect, useMemo, useRef, useState } from 'react';
import { Badge } from './components/ui/badge';
import { Button } from './components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from './components/ui/card';
import { Input } from './components/ui/input';
import { Section } from './components/ui/section';
import { Textarea } from './components/ui/textarea';
import {
  deletePlugin,
  discoverPlugin,
  enablePlugin,
  disablePlugin,
  fetchAudits,
  fetchCatalogPlugins,
  fetchDashboard,
  fetchDevices,
  fetchEvents,
  fetchXiaomiOAuthSession,
  fetchPluginLogs,
  fetchPlugins,
  getApiBase,
  installPlugin,
  sendCommand,
  startXiaomiOAuth,
  updatePluginConfig,
} from './lib/api';
import { formatTime, prettyJson } from './lib/utils';
import type {
  AuditRecord,
  CatalogPlugin,
  DashboardSummary,
  DeviceView,
  EventRecord,
  OAuthSession,
  PluginRuntimeView,
} from './lib/types';

const DEFAULT_INSTALL_CONFIGS: Record<string, string> = {
  xiaomi: JSON.stringify(
    {
      accounts: [
        {
          name: 'primary',
          region: 'cn',
          client_id: '<xiaomi-client-id>',
          redirect_url: '<xiaomi-redirect-url>',
          access_token: '<xiaomi-access-token>',
          refresh_token: '<xiaomi-refresh-token>',
          expires_at: '<RFC3339-expiry-optional>',
          home_ids: ['<optional-home-id>'],
        },
      ],
      poll_interval_seconds: 30,
    },
    null,
    2,
  ),
  petkit: JSON.stringify(
    {
      accounts: [
        {
          name: 'primary',
          username: '<petkit-username>',
          password: '<petkit-password>',
          region: 'US',
          timezone: 'Asia/Shanghai',
        },
      ],
      poll_interval_seconds: 30,
    },
    null,
    2,
  ),
  haier: JSON.stringify(
    {
      accounts: [
        {
          name: 'primary',
          email: '<hon-email>',
          password: '<hon-password>',
          mobile_id: 'celestia-primary',
          timezone: 'Asia/Shanghai',
        },
      ],
      poll_interval_seconds: 20,
    },
    null,
    2,
  ),
};

const POLL_MS = 10000;

type StatusBanner = {
  tone: 'good' | 'warn' | 'bad';
  text: string;
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function stringValue(value: unknown) {
  return typeof value === 'string' ? value.trim() : '';
}

function parseJsonObject(raw: string, errorMessage: string) {
  const parsed = JSON.parse(raw) as unknown;
  if (!isRecord(parsed)) {
    throw new Error(errorMessage);
  }
  return parsed;
}

function getXiaomiDraftSeed(raw: string) {
  const draft = parseJsonObject(raw, 'Xiaomi config must be a JSON object');
  const accounts = Array.isArray(draft.accounts) ? draft.accounts : [];
  if (accounts.length === 0) {
    throw new Error('Xiaomi config must include at least one account');
  }
  const firstAccount = accounts[0];
  if (!isRecord(firstAccount)) {
    throw new Error('Xiaomi first account must be a JSON object');
  }
  const region = stringValue(firstAccount.region);
  if (!region) {
    throw new Error('Xiaomi first account requires region');
  }
  const clientId = stringValue(firstAccount.client_id);
  if (!clientId) {
    throw new Error('Xiaomi first account requires client_id');
  }
  return {
    draft,
    accounts,
    accountName: stringValue(firstAccount.name) || 'primary',
    region,
    clientId,
  };
}

function mergeXiaomiAccountConfig(rawDraft: string, accountConfig: Record<string, unknown>) {
  const draft = parseJsonObject(rawDraft, 'Xiaomi config must be a JSON object');
  const nextDraft: Record<string, unknown> = { ...draft };
  const accounts = Array.isArray(draft.accounts) ? [...draft.accounts] : [];
  if (accounts.length === 0) {
    throw new Error('Xiaomi config must include at least one account');
  }

  const targetName = stringValue(accountConfig.name);
  let targetIndex = -1;
  if (targetName) {
    targetIndex = accounts.findIndex((account) => isRecord(account) && stringValue(account.name) === targetName);
  }
  if (targetIndex < 0) {
    targetIndex = 0;
  }

  const currentAccount = isRecord(accounts[targetIndex]) ? accounts[targetIndex] : {};
  accounts[targetIndex] = {
    ...currentAccount,
    ...accountConfig,
  };
  nextDraft.accounts = accounts;

  return {
    draft: nextDraft,
    json: JSON.stringify(nextDraft, null, 2),
    accountName: targetName || stringValue((accounts[targetIndex] as Record<string, unknown>).name) || stringValue(accounts[0] && isRecord(accounts[0]) ? accounts[0].name : undefined) || 'primary',
  };
}

function getPluginDraftText(
  pluginId: string,
  runtimeInstalled: boolean,
  installDrafts: Record<string, string>,
  configDrafts: Record<string, string>,
) {
  if (runtimeInstalled) {
    return configDrafts[pluginId] ?? installDrafts[pluginId] ?? '{}';
  }
  return installDrafts[pluginId] ?? configDrafts[pluginId] ?? '{}';
}

type LoadState = {
  dashboard: DashboardSummary | null;
  catalog: CatalogPlugin[];
  plugins: PluginRuntimeView[];
  devices: DeviceView[];
  events: EventRecord[];
  audits: AuditRecord[];
  loading: boolean;
  error: string | null;
};

const emptyLoadState = (): LoadState => ({
  dashboard: null,
  catalog: [],
  plugins: [],
  devices: [],
  events: [],
  audits: [],
  loading: true,
  error: null,
});

function App() {
  const [state, setState] = useState<LoadState>(emptyLoadState);
  const [selectedPluginId, setSelectedPluginId] = useState<string>('');
  const [selectedDeviceId, setSelectedDeviceId] = useState<string>('');
  const [selectedLogsPluginId, setSelectedLogsPluginId] = useState<string>('');
  const [pluginLogs, setPluginLogs] = useState<string[]>([]);
  const [selectedAction, setSelectedAction] = useState('feed_once');
  const [commandParams, setCommandParams] = useState('{\n  "portions": 1\n}');
  const [actor, setActor] = useState('admin');
  const [deviceSearch, setDeviceSearch] = useState('');
  const [installDrafts, setInstallDrafts] = useState<Record<string, string>>(DEFAULT_INSTALL_CONFIGS);
  const [configDrafts, setConfigDrafts] = useState<Record<string, string>>({});
  const [commandResult, setCommandResult] = useState<string>('');
  const [busy, setBusy] = useState<string>('');
  const [oauthBanner, setOauthBanner] = useState<StatusBanner | null>(null);
  const eventSourceRef = useRef<EventSource | null>(null);
  const selectedPluginIdRef = useRef('');
  const selectedDeviceIdRef = useRef('');
  const selectedLogsPluginIdRef = useRef('');
  const deviceSearchRef = useRef('');
  const installDraftsRef = useRef(installDrafts);
  const configDraftsRef = useRef(configDrafts);
  const pluginsRef = useRef<PluginRuntimeView[]>([]);
  const xiaomiOAuthSessionRef = useRef<string>('');
  const xiaomiOAuthPopupRef = useRef<Window | null>(null);
  const xiaomiOAuthPollRef = useRef<number | null>(null);

  useEffect(() => {
    selectedPluginIdRef.current = selectedPluginId;
  }, [selectedPluginId]);

  useEffect(() => {
    selectedDeviceIdRef.current = selectedDeviceId;
  }, [selectedDeviceId]);

  useEffect(() => {
    selectedLogsPluginIdRef.current = selectedLogsPluginId;
  }, [selectedLogsPluginId]);

  useEffect(() => {
    deviceSearchRef.current = deviceSearch;
  }, [deviceSearch]);

  useEffect(() => {
    installDraftsRef.current = installDrafts;
  }, [installDrafts]);

  useEffect(() => {
    configDraftsRef.current = configDrafts;
  }, [configDrafts]);

  useEffect(() => {
    pluginsRef.current = state.plugins;
  }, [state.plugins]);

  useEffect(
    () => () => {
      if (xiaomiOAuthPollRef.current !== null) {
        window.clearInterval(xiaomiOAuthPollRef.current);
        xiaomiOAuthPollRef.current = null;
      }
      if (xiaomiOAuthPopupRef.current && !xiaomiOAuthPopupRef.current.closed) {
        xiaomiOAuthPopupRef.current.close();
      }
      xiaomiOAuthPopupRef.current = null;
      xiaomiOAuthSessionRef.current = '';
    },
    [],
  );

  const clearXiaomiOAuthFlow = () => {
    if (xiaomiOAuthPollRef.current !== null) {
      window.clearInterval(xiaomiOAuthPollRef.current);
      xiaomiOAuthPollRef.current = null;
    }
    if (xiaomiOAuthPopupRef.current && !xiaomiOAuthPopupRef.current.closed) {
      xiaomiOAuthPopupRef.current.close();
    }
    xiaomiOAuthPopupRef.current = null;
    xiaomiOAuthSessionRef.current = '';
  };

  const applyXiaomiOAuthSession = (session: OAuthSession) => {
    const accountConfig = session.account_config;
    if (!accountConfig) {
      throw new Error('Xiaomi OAuth completed without account config');
    }
    const pluginId = session.plugin_id || 'xiaomi';
    const runtimeInstalled = pluginsRef.current.some((plugin) => plugin.record.plugin_id === pluginId);
    const currentDraft = getPluginDraftText(pluginId, runtimeInstalled, installDraftsRef.current, configDraftsRef.current);
    const merged = mergeXiaomiAccountConfig(currentDraft, accountConfig);
    if (runtimeInstalled) {
      setConfigDrafts((current) => ({
        ...current,
        [pluginId]: merged.json,
      }));
    } else {
      setInstallDrafts((current) => ({
        ...current,
        [pluginId]: merged.json,
      }));
    }
    setOauthBanner({
      tone: 'good',
      text: `Xiaomi OAuth data injected into ${runtimeInstalled ? 'config' : 'install'} draft for ${merged.accountName}.`,
    });
  };

  const syncXiaomiOAuthSession = async (sessionId: string) => {
    const session = await fetchXiaomiOAuthSession(sessionId);
    if (session.status === 'pending') {
      return;
    }
    clearXiaomiOAuthFlow();
    if (session.status === 'completed') {
      try {
        applyXiaomiOAuthSession(session);
      } catch (error) {
        setOauthBanner({
          tone: 'bad',
          text: error instanceof Error ? error.message : 'Failed to apply Xiaomi OAuth session.',
        });
      }
      return;
    }
    const message = session.error || `Xiaomi OAuth ${session.status}.`;
    setOauthBanner({
      tone: session.status === 'expired' ? 'warn' : 'bad',
      text: message,
    });
  };

  const ensureXiaomiOAuthPolling = (sessionId: string) => {
    if (xiaomiOAuthPollRef.current !== null) {
      window.clearInterval(xiaomiOAuthPollRef.current);
    }
    xiaomiOAuthSessionRef.current = sessionId;
    xiaomiOAuthPollRef.current = window.setInterval(() => {
      void syncXiaomiOAuthSession(sessionId).catch((error) => {
        clearXiaomiOAuthFlow();
        setOauthBanner({
          tone: 'bad',
          text: error instanceof Error ? error.message : 'Failed to refresh Xiaomi OAuth session.',
        });
      });
    }, 1500);
  };

  const startXiaomiOAuthFlow = async (plugin: CatalogPlugin) => {
    const runtime = state.plugins.find((item) => item.record.plugin_id === plugin.id) ?? null;
    const draftText = getPluginDraftText(plugin.id, Boolean(runtime), installDraftsRef.current, configDraftsRef.current);
    const seed = getXiaomiDraftSeed(draftText);
    const popup = window.open('', 'celestia-xiaomi-oauth', 'width=540,height=760');
    if (!popup) {
      throw new Error('Popup blocked. Allow popups to connect Xiaomi.');
    }
    popup.document.write('<!doctype html><title>Starting Xiaomi OAuth</title><p>Opening Xiaomi authorization...</p>');
    popup.document.close();
    xiaomiOAuthPopupRef.current = popup;
    setOauthBanner({
      tone: 'warn',
      text: `Starting Xiaomi OAuth for account ${seed.accountName}.`,
    });
    try {
      const response = await startXiaomiOAuth({
        plugin_id: plugin.id,
        account_name: seed.accountName,
        region: seed.region,
        client_id: seed.clientId,
        redirect_base_url: new URL(getApiBase(), window.location.origin).origin,
      });
      const session = response.session;
      if (!session.auth_url) {
        clearXiaomiOAuthFlow();
        throw new Error('Xiaomi OAuth start did not return an authorization URL');
      }
      xiaomiOAuthPopupRef.current = popup;
      popup.location.href = session.auth_url;
      ensureXiaomiOAuthPolling(session.id);
      void syncXiaomiOAuthSession(session.id).catch((error) => {
        setOauthBanner({
          tone: 'bad',
          text: error instanceof Error ? error.message : 'Failed to refresh Xiaomi OAuth session.',
        });
      });
    } catch (error) {
      clearXiaomiOAuthFlow();
      setOauthBanner({
        tone: 'bad',
        text: error instanceof Error ? error.message : 'Failed to start Xiaomi OAuth.',
      });
      throw error;
    }
  };

  const refreshAll = async () => {
    setState((current) => ({ ...current, loading: true, error: null }));
    try {
      const [dashboard, catalog, plugins, devices, events, audits] = await Promise.all([
        fetchDashboard(),
        fetchCatalogPlugins(),
        fetchPlugins(),
        fetchDevices(deviceSearchRef.current),
        fetchEvents(80),
        fetchAudits(80),
      ]);

      setState({
        dashboard,
        catalog,
        plugins,
        devices,
        events,
        audits,
        loading: false,
        error: null,
      });
      if (!selectedPluginIdRef.current && plugins.length > 0) {
        setSelectedPluginId(plugins[0].record.plugin_id);
      } else if (selectedPluginIdRef.current && !plugins.some((plugin) => plugin.record.plugin_id === selectedPluginIdRef.current)) {
        setSelectedPluginId(plugins[0]?.record.plugin_id ?? '');
      }
      if (!selectedDeviceIdRef.current && devices.length > 0) {
        setSelectedDeviceId(devices[0].device.id);
      } else if (selectedDeviceIdRef.current && !devices.some((device) => device.device.id === selectedDeviceIdRef.current)) {
        setSelectedDeviceId(devices[0]?.device.id ?? '');
      }
      if (!selectedLogsPluginIdRef.current && plugins.length > 0) {
        setSelectedLogsPluginId(plugins[0].record.plugin_id);
      } else if (
        selectedLogsPluginIdRef.current &&
        !plugins.some((plugin) => plugin.record.plugin_id === selectedLogsPluginIdRef.current)
      ) {
        setSelectedLogsPluginId(plugins[0]?.record.plugin_id ?? '');
      }
      const drafts: Record<string, string> = {};
      for (const plugin of plugins) {
        drafts[plugin.record.plugin_id] = JSON.stringify(plugin.record.config ?? {}, null, 2);
      }
      setConfigDrafts((current) => ({ ...drafts, ...current }));
    } catch (error) {
      setState((current) => ({
        ...current,
        loading: false,
        error: error instanceof Error ? error.message : 'Failed to load admin data',
      }));
    }
  };

  useEffect(() => {
    void refreshAll();
    const interval = window.setInterval(() => {
      void refreshAll();
    }, POLL_MS);
    return () => window.clearInterval(interval);
  }, [deviceSearch]);

  useEffect(() => {
    if (!selectedLogsPluginId) {
      setPluginLogs([]);
      return;
    }
    let canceled = false;
    void fetchPluginLogs(selectedLogsPluginId)
      .then((data) => {
        if (!canceled) setPluginLogs(data.logs);
      })
      .catch(() => {
        if (!canceled) setPluginLogs(['Unable to load logs.']);
      });
    return () => {
      canceled = true;
    };
  }, [selectedLogsPluginId, state.plugins.length]);

  useEffect(() => {
    const source = new EventSource(`${getApiBase()}/events/stream`);
    eventSourceRef.current = source;
    source.onmessage = () => void refreshAll();
    source.addEventListener('device.state.changed', () => void refreshAll());
    source.addEventListener('device.event.occurred', () => void refreshAll());
    source.addEventListener('plugin.lifecycle.changed', () => void refreshAll());
    source.onerror = () => {
      source.close();
    };
    return () => {
      source.close();
      eventSourceRef.current = null;
    };
  }, []);

  useEffect(() => {
    const handleMessage = (event: MessageEvent) => {
      const data = event.data as Partial<{ type: string; session_id: string; status: string }> | null;
      if (!data || data.type !== 'celestia:xiaomi-oauth') {
        return;
      }
      if (event.origin !== window.location.origin) {
        return;
      }
      if (!data.session_id || data.session_id !== xiaomiOAuthSessionRef.current) {
        return;
      }
      void syncXiaomiOAuthSession(data.session_id).catch((error) => {
        setOauthBanner({
          tone: 'bad',
          text: error instanceof Error ? error.message : 'Failed to refresh Xiaomi OAuth session.',
        });
      });
    };
    window.addEventListener('message', handleMessage);
    return () => window.removeEventListener('message', handleMessage);
  }, []);

  const selectedPlugin = useMemo(
    () => state.plugins.find((plugin) => plugin.record.plugin_id === selectedPluginId) ?? null,
    [selectedPluginId, state.plugins],
  );

  const selectedDevice = useMemo(
    () => state.devices.find((item) => item.device.id === selectedDeviceId) ?? null,
    [selectedDeviceId, state.devices],
  );

  const commandSuggestions = useMemo(() => {
    if (!selectedDevice) return [];
    const capabilities = selectedDevice.device.capabilities;
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
      setState((current) => ({
        ...current,
        error: error instanceof Error ? error.message : 'Operation failed',
      }));
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

  return (
    <div className="shell">
      <div className="ambient ambient--one" />
      <div className="ambient ambient--two" />

      <header className="topbar">
        <div>
          <p className="eyebrow">Celestia Core Runtime</p>
          <h1>Admin Console</h1>
          <p className="topbar__sub">
            Plugin orchestration, device control, and event inspection against{' '}
            <code>{getApiBase()}</code>
          </p>
        </div>
        <div className="topbar__actions">
          <Badge tone={state.error ? 'bad' : 'good'}>{state.error ? 'Degraded' : 'Connected'}</Badge>
          <Button variant="secondary" onClick={() => void refreshAll()} disabled={state.loading}>
            Refresh
          </Button>
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
            <Badge tone={oauthBanner.tone}>{oauthBanner.tone === 'good' ? 'Connected' : oauthBanner.tone === 'warn' ? 'Pending' : 'Error'}</Badge>
            <span>{oauthBanner.text}</span>
          </CardContent>
        </Card>
      ) : null}

      <Section className="grid grid--stats">
        {[
          ['Plugins', state.dashboard?.plugins ?? 0],
          ['Enabled', state.dashboard?.enabled_plugins ?? 0],
          ['Devices', state.dashboard?.devices ?? 0],
          ['Online', state.dashboard?.online_devices ?? 0],
          ['Events', state.dashboard?.events ?? 0],
          ['Audits', state.dashboard?.audits ?? 0],
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
            <div className="section-title">
              <div>
                <CardTitle>Plugin Management</CardTitle>
                <CardDescription>Install, enable, disable, rediscover, update config, and inspect logs.</CardDescription>
              </div>
              <Badge tone="accent">{state.catalog.length} catalog entries</Badge>
            </div>
          </CardHeader>
          <CardContent className="stack">
            {state.catalog.map((plugin) => {
              const runtime = state.plugins.find((item) => item.record.plugin_id === plugin.id);
              const configDraft = configDrafts[plugin.id] ?? installDrafts[plugin.id] ?? '{}';
              return (
                <div key={plugin.id} className="plugin-card">
                  <div className="plugin-card__meta">
                    <div>
                      <h3>{plugin.name}</h3>
                      <p>{plugin.description}</p>
                    </div>
                    <div className="plugin-card__badges">
                      <Badge tone={runtime?.record.status === 'enabled' ? 'good' : 'neutral'}>
                        {runtime?.record.status ?? 'uninstalled'}
                      </Badge>
                      <Badge tone={runtime?.health.status === 'healthy' ? 'good' : runtime?.health.status === 'unhealthy' ? 'bad' : 'warn'}>
                        {runtime?.health.status ?? 'unknown'}
                      </Badge>
                    </div>
                  </div>
                  <div className="plugin-card__grid">
                    <div>
                      <label>Config JSON</label>
                      <Textarea
                        rows={8}
                        value={configDraft}
                        onChange={(event) =>
                          plugin.id === runtime?.record.plugin_id
                            ? setConfigDrafts((current) => ({ ...current, [plugin.id]: event.target.value }))
                            : setInstallDrafts((current) => ({ ...current, [plugin.id]: event.target.value }))
                        }
                      />
                    </div>
                    <div className="plugin-card__side">
                      <div className="kv">
                        <span>Binary</span>
                        <strong>{plugin.binary_name}</strong>
                      </div>
                      <div className="kv">
                        <span>Capabilities</span>
                        <strong>{plugin.manifest.capabilities.join(', ')}</strong>
                      </div>
                      <div className="kv">
                        <span>Devices</span>
                        <strong>{plugin.manifest.device_kinds.join(', ')}</strong>
                      </div>
                      <div className="button-row">
                        {plugin.id === 'xiaomi' ? (
                          <Button
                            variant="secondary"
                            onClick={() =>
                              void runAction(`xiaomi-oauth-${plugin.id}`, () => startXiaomiOAuthFlow(plugin))
                            }
                            disabled={busy === `xiaomi-oauth-${plugin.id}` || Boolean(xiaomiOAuthSessionRef.current)}
                          >
                            Connect OAuth
                          </Button>
                        ) : null}
                        <Button
                          onClick={() => void runAction(`install-${plugin.id}`, () => onInstall(plugin))}
                          disabled={busy === `install-${plugin.id}`}
                        >
                          Install
                        </Button>
                        <Button variant="secondary" onClick={() => void runAction(`enable-${plugin.id}`, () => enablePlugin(plugin.id))}>
                          Enable
                        </Button>
                        <Button variant="secondary" onClick={() => void runAction(`disable-${plugin.id}`, () => disablePlugin(plugin.id))}>
                          Disable
                        </Button>
                      </div>
                      <div className="button-row">
                        <Button variant="secondary" onClick={() => void runAction(`discover-${plugin.id}`, () => discoverPlugin(plugin.id))}>
                          Discover
                        </Button>
                        <Button variant="secondary" onClick={() => setSelectedLogsPluginId(plugin.id)}>
                          Logs
                        </Button>
                        <Button variant="danger" onClick={() => void runAction(`delete-${plugin.id}`, () => deletePlugin(plugin.id))}>
                          Uninstall
                        </Button>
                      </div>
                      {runtime ? (
                        <div className="runtime-metadata">
                          <p>Last updated {formatTime(runtime.record.updated_at)}</p>
                          <p>PID {runtime.process_pid ?? 'n/a'} · {runtime.listen_addr ?? 'n/a'}</p>
                          {runtime.last_error ? <p className="muted">Error: {runtime.last_error}</p> : null}
                        </div>
                      ) : null}
                    </div>
                  </div>
                  <div className="button-row">
                    {runtime ? (
                      <Button variant="secondary" onClick={() => void runAction(`refresh-config-${plugin.id}`, () => onUpdateConfig(plugin.id))}>
                        Save Config
                      </Button>
                    ) : null}
                  </div>
                </div>
              );
            })}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <div className="section-title">
              <div>
                <CardTitle>Plugin Logs</CardTitle>
                <CardDescription>Inspect live log buffer from the selected plugin process.</CardDescription>
              </div>
              <Badge tone="neutral">{selectedLogsPluginId || 'none selected'}</Badge>
            </div>
          </CardHeader>
          <CardContent className="stack">
            <div className="toolbar">
              <select
                className="select"
                value={selectedLogsPluginId}
                onChange={(event) => setSelectedLogsPluginId(event.target.value)}
              >
                {state.plugins.map((plugin) => (
                  <option key={plugin.record.plugin_id} value={plugin.record.plugin_id}>
                    {plugin.record.plugin_id}
                  </option>
                ))}
              </select>
              <Button variant="secondary" onClick={() => void fetchPluginLogs(selectedLogsPluginId).then((data) => setPluginLogs(data.logs))}>
                Reload
              </Button>
            </div>
            <pre className="log-box">{pluginLogs.join('\n') || 'No logs captured yet.'}</pre>
          </CardContent>
        </Card>
      </Section>

      <Section className="grid grid--two">
        <Card>
          <CardHeader>
            <CardTitle>Devices</CardTitle>
            <CardDescription>Browse unified devices and issue commands against the selected item.</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <div className="toolbar">
              <Input value={deviceSearch} onChange={(event) => setDeviceSearch(event.target.value)} placeholder="Search devices" />
              <Button variant="secondary" onClick={() => void refreshAll()}>
                Search
              </Button>
            </div>
            <div className="table">
              {state.devices.map((item) => (
                <button
                  key={item.device.id}
                  type="button"
                  className={`table__row ${selectedDeviceId === item.device.id ? 'is-selected' : ''}`}
                  onClick={() => setSelectedDeviceId(item.device.id)}
                >
                  <div>
                    <strong>{item.device.name}</strong>
                    <p>{item.device.id}</p>
                  </div>
                  <div>
                    <Badge tone={item.device.online ? 'good' : 'bad'}>{item.device.online ? 'online' : 'offline'}</Badge>
                  </div>
                  <div>
                    <span>{item.device.kind}</span>
                    <p>{item.device.room || 'no room'}</p>
                  </div>
                </button>
              ))}
            </div>

            {selectedDevice ? (
              <div className="detail">
                <div className="detail__header">
                  <div>
                    <h3>{selectedDevice.device.name}</h3>
                    <p>{selectedDevice.device.id}</p>
                  </div>
                  <Badge tone={selectedDevice.device.online ? 'good' : 'bad'}>
                    {selectedDevice.device.online ? 'online' : 'offline'}
                  </Badge>
                </div>
                <div className="chip-list">
                  {selectedDevice.device.capabilities.map((capability) => (
                    <Badge key={capability} tone="neutral">
                      {capability}
                    </Badge>
                  ))}
                </div>
                <div className="grid grid--detail">
                  <div>
                    <label>Action</label>
                    <Input value={selectedAction} onChange={(event) => setSelectedAction(event.target.value)} />
                  </div>
                  <div>
                    <label>Actor</label>
                    <Input value={actor} onChange={(event) => setActor(event.target.value)} />
                  </div>
                  <div className="grid__full">
                    <label>Params JSON</label>
                    <Textarea rows={6} value={commandParams} onChange={(event) => setCommandParams(event.target.value)} />
                  </div>
                </div>
                <div className="button-row">
                  {commandSuggestions.map((suggestion) => (
                    <Button
                      key={suggestion.label}
                      variant="secondary"
                      onClick={() => {
                        setSelectedAction(suggestion.action);
                        setCommandParams(JSON.stringify(suggestion.params, null, 2));
                      }}
                    >
                      {suggestion.label}
                    </Button>
                  ))}
                  <Button onClick={() => void runAction('send-command', onSendCommand)}>Send Command</Button>
                </div>
                {commandResult ? <pre className="log-box">{commandResult}</pre> : null}
                <pre className="log-box">{selectedDeviceDetails}</pre>
              </div>
            ) : null}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Selected Plugin</CardTitle>
            <CardDescription>Use runtime actions against the currently selected installation.</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            {selectedPlugin ? (
              <>
                <div className="detail__header">
                  <div>
                    <h3>{selectedPlugin.record.plugin_id}</h3>
                    <p>{selectedPlugin.record.binary_path}</p>
                  </div>
                  <Badge tone={selectedPlugin.running ? 'good' : 'neutral'}>
                    {selectedPlugin.running ? 'running' : 'stopped'}
                  </Badge>
                </div>
                <div className="kv-grid">
                  <div className="kv">
                    <span>Version</span>
                    <strong>{selectedPlugin.record.version}</strong>
                  </div>
                  <div className="kv">
                    <span>Health</span>
                    <strong>{selectedPlugin.health.status}</strong>
                  </div>
                  <div className="kv">
                    <span>Heartbeat</span>
                    <strong>{formatTime(selectedPlugin.record.last_heartbeat_at)}</strong>
                  </div>
                  <div className="kv">
                    <span>Last error</span>
                    <strong>{selectedPlugin.last_error || 'none'}</strong>
                  </div>
                </div>
                <div className="button-row">
                  <Button variant="secondary" onClick={() => void runAction(`enable-selected-${selectedPlugin.record.plugin_id}`, () => enablePlugin(selectedPlugin.record.plugin_id))}>
                    Enable
                  </Button>
                  <Button variant="secondary" onClick={() => void runAction(`disable-selected-${selectedPlugin.record.plugin_id}`, () => disablePlugin(selectedPlugin.record.plugin_id))}>
                    Disable
                  </Button>
                  <Button variant="secondary" onClick={() => void runAction(`discover-selected-${selectedPlugin.record.plugin_id}`, () => discoverPlugin(selectedPlugin.record.plugin_id))}>
                    Discover
                  </Button>
                </div>
                <div className="runtime-json">
                  <pre className="log-box">{prettyJson(selectedPlugin)}</pre>
                </div>
              </>
            ) : (
              <p className="muted">No plugin selected.</p>
            )}
          </CardContent>
        </Card>
      </Section>

      <Section className="grid grid--two">
        <Card>
          <CardHeader>
            <CardTitle>Events</CardTitle>
            <CardDescription>Recent event bus and device activity feed.</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <div className="feed">
              {state.events.map((event) => (
                <article key={event.id} className="feed__item">
                  <div className="feed__meta">
                    <Badge tone="accent">{event.type}</Badge>
                    <span>{formatTime(event.ts)}</span>
                  </div>
                  <strong>{event.device_id || event.plugin_id || 'system'}</strong>
                  <pre>{prettyJson(event.payload ?? {})}</pre>
                </article>
              ))}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Audits</CardTitle>
            <CardDescription>Command history and policy decisions.</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <div className="feed">
              {state.audits.map((audit) => (
                <article key={audit.id} className="feed__item">
                  <div className="feed__meta">
                    <Badge tone={audit.allowed ? 'good' : 'bad'}>{audit.allowed ? 'allowed' : 'denied'}</Badge>
                    <Badge tone={audit.risk_level === 'high' ? 'bad' : audit.risk_level === 'medium' ? 'warn' : 'neutral'}>
                      {audit.risk_level}
                    </Badge>
                    <span>{formatTime(audit.created_at)}</span>
                  </div>
                  <strong>{audit.actor} · {audit.action}</strong>
                  <p>{audit.device_id}</p>
                  <pre>{prettyJson(audit.params ?? {})}</pre>
                </article>
              ))}
            </div>
          </CardContent>
        </Card>
      </Section>
    </div>
  );
}

export default App;
