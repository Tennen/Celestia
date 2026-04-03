import { useEffect, useState } from 'react';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { Input } from '../ui/input';
import { ScrollArea } from '../ui/scroll-area';
import { Section } from '../ui/section';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../ui/tabs';
import { asArray } from '../../lib/admin';
import { formatTime, prettyJson } from '../../lib/utils';
import { getPluginDraftText, canStartXiaomiOAuth } from '../../lib/admin';
import { useAdminStore } from '../../stores/adminStore';
import { usePluginStore } from '../../stores/pluginStore';
import { PluginConfigPanel } from './PluginConfigPanel';
import { CardHeading } from './shared/CardHeading';
import { SelectableListItem } from './shared/SelectableListItem';

type Props = {
  oauthActive: boolean;
  onConnectXiaomiOAuth: () => void;
};

function extractXiaomiVerificationHint(errorText?: string | null) {
  if (!errorText) return null;
  const match = errorText.match(/requires (secondary verification|captcha) at (\S+)/i);
  if (!match) return null;
  return { kind: match[1].toLowerCase(), url: match[2] };
}

function isObjectRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function extractHikvisionRuntimeHint(plugin: import('../../lib/types').CatalogPlugin | null) {
  if (plugin?.id !== 'hikvision' || !isObjectRecord(plugin.manifest.metadata)) return null;
  const metadata = plugin.manifest.metadata;
  const runtimeMode = typeof metadata.runtime_mode === 'string' ? metadata.runtime_mode : '';
  const runtimePlatform = typeof metadata.runtime_platform === 'string' ? metadata.runtime_platform : '';
  const nativePlatform = typeof metadata.native_platform === 'string' ? metadata.native_platform : 'linux/arm64';
  const sdkLibDir = typeof metadata.sdk_lib_dir_default === 'string' ? metadata.sdk_lib_dir_default : '';
  if (!runtimeMode || !runtimePlatform) return null;
  return { runtimeMode, runtimePlatform, nativePlatform, sdkLibDir };
}

export function PluginWorkspace({ oauthActive, onConnectXiaomiOAuth }: Props) {
  const { catalog, plugins } = useAdminStore();
  const {
    selectedPluginId,
    installDrafts,
    configDrafts,
    pluginLogs,
    busy,
    xiaomiVerifyTicket,
    setSelectedPluginId,
    setDraft,
    setXiaomiVerifyTicket,
    installPlugin,
    enablePlugin,
    disablePlugin,
    discoverPlugin,
    deletePlugin,
    saveConfig,
    reloadPluginLogs,
    retryXiaomiVerification,
  } = usePluginStore();

  const [detailMode, setDetailMode] = useState<'runtime' | 'config' | 'logs'>('runtime');

  useEffect(() => {
    setDetailMode('runtime');
  }, [selectedPluginId]);

  const selectedCatalogPlugin = catalog.find((p) => p.id === selectedPluginId) ?? null;
  const selectedPlugin = plugins.find((p) => p.record.plugin_id === selectedPluginId) ?? null;
  const isInstalled = Boolean(selectedPlugin);

  const pluginDraft = selectedCatalogPlugin
    ? getPluginDraftText(selectedCatalogPlugin.id, isInstalled, installDrafts, configDrafts)
    : '{}';
  const xiaomiOAuthAvailable = selectedCatalogPlugin?.id === 'xiaomi' && canStartXiaomiOAuth(pluginDraft);

  const xiaomiVerificationHint =
    selectedCatalogPlugin?.id === 'xiaomi'
      ? extractXiaomiVerificationHint(selectedPlugin?.last_error ?? selectedPlugin?.health.message)
      : null;
  const hikvisionRuntimeHint = extractHikvisionRuntimeHint(selectedCatalogPlugin);

  return (
    <Section className="plugin-workspace">
      <Card className="plugin-explorer">
        <CardHeader>
          <CardTitle>Plugin List</CardTitle>
          <CardDescription>Stable ordering by plugin id. Select an item to open its full management panel.</CardDescription>
        </CardHeader>
        <CardContent className="stack">
          <ScrollArea className="max-h-[calc(100vh-15rem)] pr-3">
            <div className="plugin-list">
              {catalog.map((plugin) => {
                const runtime = plugins.find((item) => item.record.plugin_id === plugin.id);
                return (
                  <SelectableListItem
                    key={plugin.id}
                    className={`plugin-list__item ${selectedPluginId === plugin.id ? 'is-selected' : ''}`}
                    onClick={() => setSelectedPluginId(plugin.id)}
                    selected={selectedPluginId === plugin.id}
                    title={plugin.name}
                    description={plugin.id}
                    badges={
                      <>
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
                      </>
                    }
                  />
                );
              })}
            </div>
          </ScrollArea>
        </CardContent>
      </Card>

      <div className="detail-stack">
        {selectedCatalogPlugin ? (
          <>
            <Card>
              <CardHeader>
                <CardHeading
                  title={selectedCatalogPlugin.name}
                  description={selectedCatalogPlugin.description}
                  aside={
                    <div className="plugin-card__badges">
                      <Badge tone={selectedPlugin?.record.status === 'enabled' ? 'good' : 'neutral'}>
                        {selectedPlugin?.record.status ?? 'uninstalled'}
                      </Badge>
                      <Badge
                        tone={
                          selectedPlugin?.health.status === 'healthy'
                            ? 'good'
                            : selectedPlugin?.health.status === 'unhealthy'
                              ? 'bad'
                              : 'warn'
                        }
                      >
                        {selectedPlugin?.health.status ?? 'unknown'}
                      </Badge>
                    </div>
                  }
                />
              </CardHeader>
              <CardContent className="stack">
                <div className="grid gap-4 md:grid-cols-3">
                  <div className="kv">
                    <span>Binary</span>
                    <strong>{selectedCatalogPlugin.binary_name}</strong>
                  </div>
                  <div className="kv">
                    <span>Capabilities</span>
                    <strong>{asArray(selectedCatalogPlugin.manifest.capabilities).join(', ')}</strong>
                  </div>
                  <div className="kv">
                    <span>Devices</span>
                    <strong>{asArray(selectedCatalogPlugin.manifest.device_kinds).join(', ')}</strong>
                  </div>
                </div>
                <div className="stack">
                  {hikvisionRuntimeHint ? (
                    <div className="plugin-auth-guide">
                      <div className="plugin-auth-guide__header">
                        <Badge tone={hikvisionRuntimeHint.runtimeMode === 'native' ? 'good' : 'accent'}>
                          {hikvisionRuntimeHint.runtimeMode === 'native' ? 'Native Runtime' : 'Docker Fallback'}
                        </Badge>
                        <strong>
                          {hikvisionRuntimeHint.runtimeMode === 'native'
                            ? 'This gateway installs Hikvision like the other plugins.'
                            : 'This gateway keeps the normal install flow but runs Hikvision through Docker.'}
                        </strong>
                      </div>
                      <p className="muted">
                        Current platform <code>{hikvisionRuntimeHint.runtimePlatform}</code>. Native HCNetSDK runtime
                        is enabled only on <code>{hikvisionRuntimeHint.nativePlatform}</code>.
                      </p>
                      {hikvisionRuntimeHint.sdkLibDir ? (
                        <p className="muted">
                          Default <code>sdk_lib_dir</code>: <code>{hikvisionRuntimeHint.sdkLibDir}</code>
                        </p>
                      ) : null}
                    </div>
                  ) : null}
                  {selectedCatalogPlugin.id === 'xiaomi' ? (
                    <div className="plugin-auth-guide">
                      <div className="plugin-auth-guide__header">
                        <Badge tone="accent">Xiaomi Login</Badge>
                        <strong>Install or save config to authenticate.</strong>
                      </div>
                      <p className="muted">
                        Primary path: fill <code>accounts[0].region</code>, <code>username</code>,{' '}
                        <code>password</code>, and <code>device_id</code> in the Core-owned config draft, then click{' '}
                        <code>{selectedPlugin ? 'Save Config' : 'Install'}</code> and <code>Enable</code>.
                      </p>
                      <p className="muted">
                        Optional paths: provide <code>service_token</code>, <code>ssecurity</code>, and{' '}
                        <code>user_id</code> to reuse an existing Xiaomi session, or use <code>Connect OAuth</code>{' '}
                        only when the draft already contains <code>client_id</code> and <code>redirect_url</code>.
                      </p>
                    </div>
                  ) : null}
                  {selectedCatalogPlugin.id === 'haier' ? (
                    <div className="plugin-auth-guide">
                      <div className="plugin-auth-guide__header">
                        <Badge tone="accent">Haier China UWS</Badge>
                        <strong>Configure the real China-region credentials that the plugin now expects.</strong>
                      </div>
                      <p className="muted">
                        Use <code>accounts[0].clientId</code> and <code>accounts[0].refreshToken</code> in the Core-owned
                        config draft. The previous email/password placeholders were wrong for the current UWS flow.
                      </p>
                      <p className="muted">
                        If the runtime still cannot connect, open <code>Logs</code>. The plugin now logs whether failure
                        happened during token refresh, device discovery, or WSS subscription.
                      </p>
                    </div>
                  ) : null}
                  {xiaomiVerificationHint ? (
                    <div className="plugin-auth-warning">
                      <div className="plugin-auth-guide__header">
                        <Badge tone="warn">
                          {xiaomiVerificationHint.kind === 'captcha' ? 'Captcha Required' : '2-Step Verification'}
                        </Badge>
                        <strong>Xiaomi login is waiting for browser verification.</strong>
                      </div>
                      <p className="muted">
                        Open the Xiaomi verification page, trigger the Xiaomi SMS or email step there, then paste the
                        received verification code here and submit it back to the gateway.
                      </p>
                      <div>
                        <label>Verification Code</label>
                        <Input
                          value={xiaomiVerifyTicket}
                          onChange={(e) => setXiaomiVerifyTicket(e.target.value)}
                          placeholder="Enter SMS or email code from Xiaomi"
                        />
                      </div>
                      <div className="button-row">
                        <Button asChild variant="secondary">
                          <a href={xiaomiVerificationHint.url} target="_blank" rel="noreferrer">
                            Open Verification Page
                          </a>
                        </Button>
                        {selectedPlugin ? (
                          <Button
                            variant="secondary"
                            onClick={() => void retryXiaomiVerification(selectedCatalogPlugin, xiaomiVerificationHint.url)}
                            disabled={!xiaomiVerifyTicket.trim()}
                          >
                            Submit Code And Retry
                          </Button>
                        ) : null}
                      </div>
                      <p className="muted">
                        <code>{xiaomiVerificationHint.url}</code>
                      </p>
                    </div>
                  ) : null}
                  <div className="button-row">
                    {selectedCatalogPlugin.id === 'xiaomi' ? (
                      <Button
                        variant="secondary"
                        onClick={onConnectXiaomiOAuth}
                        disabled={!xiaomiOAuthAvailable || busy === `xiaomi-oauth-${selectedCatalogPlugin.id}` || oauthActive}
                      >
                        Connect OAuth
                      </Button>
                    ) : null}
                    <Button
                      onClick={() => void installPlugin(selectedCatalogPlugin)}
                      disabled={busy === `install-${selectedCatalogPlugin.id}`}
                    >
                      Install
                    </Button>
                    <Button variant="secondary" onClick={() => void enablePlugin(selectedCatalogPlugin.id)}>
                      Enable
                    </Button>
                    <Button variant="secondary" onClick={() => void disablePlugin(selectedCatalogPlugin.id)}>
                      Disable
                    </Button>
                  </div>
                  <div className="button-row">
                    <Button variant="secondary" onClick={() => void discoverPlugin(selectedCatalogPlugin.id)}>
                      Discover
                    </Button>
                    <Button variant="danger" onClick={() => void deletePlugin(selectedCatalogPlugin.id)}>
                      Uninstall
                    </Button>
                    {selectedPlugin ? (
                      <Button variant="secondary" onClick={() => void saveConfig(selectedCatalogPlugin.id)}>
                        Save Config
                      </Button>
                    ) : null}
                  </div>
                  {selectedPlugin ? (
                    <div className="runtime-metadata">
                      <p>Last updated {formatTime(selectedPlugin.record.updated_at)}</p>
                      <p>
                        PID {selectedPlugin.process_pid ?? 'n/a'} · {selectedPlugin.listen_addr ?? 'n/a'}
                      </p>
                      {selectedPlugin.last_error ? <p className="muted">Error: {selectedPlugin.last_error}</p> : null}
                    </div>
                  ) : (
                    <p className="muted">Install the plugin to start the runtime and expose logs.</p>
                  )}
                </div>
              </CardContent>
            </Card>

            <Tabs
              value={detailMode}
              onValueChange={(value) => setDetailMode(value as 'runtime' | 'config' | 'logs')}
            >
              <TabsList>
                <TabsTrigger value="runtime">Runtime</TabsTrigger>
                <TabsTrigger value="config">Config</TabsTrigger>
                <TabsTrigger value="logs">Logs</TabsTrigger>
              </TabsList>

              <TabsContent value="runtime">
                <Section className="grid grid--two">
                  <Card>
                    <CardHeader>
                      <CardTitle>Runtime Details</CardTitle>
                      <CardDescription>Current runtime view for the selected plugin.</CardDescription>
                    </CardHeader>
                    <CardContent className="stack">
                      {selectedPlugin ? (
                        <ScrollArea className="max-h-[520px]">
                          <pre className="log-box">{prettyJson(selectedPlugin)}</pre>
                        </ScrollArea>
                      ) : (
                        <p className="muted">No runtime data yet. Install and enable the plugin first.</p>
                      )}
                    </CardContent>
                  </Card>
                  <Card>
                    <CardHeader>
                      <CardTitle>Current Config Snapshot</CardTitle>
                      <CardDescription>The installed config record currently persisted by Core.</CardDescription>
                    </CardHeader>
                    <CardContent className="stack">
                      {selectedPlugin ? (
                        <ScrollArea className="max-h-[520px]">
                          <pre className="log-box">{prettyJson(selectedPlugin.record.config ?? {})}</pre>
                        </ScrollArea>
                      ) : (
                        <p className="muted">Install the plugin to persist runtime config.</p>
                      )}
                    </CardContent>
                  </Card>
                </Section>
              </TabsContent>

              <TabsContent value="config">
                <PluginConfigPanel
                  plugin={selectedCatalogPlugin}
                  runtimeInstalled={isInstalled}
                  pluginDraft={pluginDraft}
                  busy={busy}
                  onDraftChange={(value) => setDraft(selectedCatalogPlugin.id, isInstalled, value)}
                  onInstall={() => void installPlugin(selectedCatalogPlugin)}
                  onSaveConfig={() => void saveConfig(selectedCatalogPlugin.id)}
                />
              </TabsContent>

              <TabsContent value="logs">
                <Card>
                  <CardHeader>
                    <CardHeading
                      title="Plugin Logs"
                      description="Live buffer for the currently selected plugin."
                      aside={<Badge tone="neutral">{selectedCatalogPlugin.id}</Badge>}
                    />
                  </CardHeader>
                  <CardContent className="stack">
                    <div className="button-row">
                      <Button variant="secondary" onClick={() => void reloadPluginLogs(selectedCatalogPlugin.id)}>
                        Reload
                      </Button>
                    </div>
                    <ScrollArea className="max-h-[560px]">
                      <pre className="log-box">
                        {selectedPlugin
                          ? pluginLogs.join('\n') || 'No logs captured yet.'
                          : 'Plugin is not installed.'}
                      </pre>
                    </ScrollArea>
                  </CardContent>
                </Card>
              </TabsContent>
            </Tabs>
          </>
        ) : (
          <Card>
            <CardContent>
              <p className="muted">No plugin selected.</p>
            </CardContent>
          </Card>
        )}
      </div>
    </Section>
  );
}
