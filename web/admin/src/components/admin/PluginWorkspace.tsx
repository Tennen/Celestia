import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { Section } from '../ui/section';
import { Textarea } from '../ui/textarea';
import { asArray } from '../../lib/admin';
import { formatTime, prettyJson } from '../../lib/utils';
import type { CatalogPlugin, PluginRuntimeView } from '../../lib/types';

type Props = {
  catalog: CatalogPlugin[];
  plugins: PluginRuntimeView[];
  selectedPluginId: string;
  selectedCatalogPlugin: CatalogPlugin | null;
  selectedPlugin: PluginRuntimeView | null;
  pluginDraft: string;
  pluginLogs: string[];
  busy: string;
  xiaomiOAuthActive: boolean;
  xiaomiOAuthAvailable: boolean;
  onSelectPlugin: (pluginId: string) => void;
  onDraftChange: (value: string) => void;
  onConnectXiaomiOAuth: () => void;
  onInstall: () => void;
  onEnable: () => void;
  onDisable: () => void;
  onDiscover: () => void;
  onDelete: () => void;
  onSaveConfig: () => void;
  onReloadLogs: () => void;
};

export function PluginWorkspace({
  catalog,
  plugins,
  selectedPluginId,
  selectedCatalogPlugin,
  selectedPlugin,
  pluginDraft,
  pluginLogs,
  busy,
  xiaomiOAuthActive,
  xiaomiOAuthAvailable,
  onSelectPlugin,
  onDraftChange,
  onConnectXiaomiOAuth,
  onInstall,
  onEnable,
  onDisable,
  onDiscover,
  onDelete,
  onSaveConfig,
  onReloadLogs,
}: Props) {
  return (
    <Section className="plugin-workspace">
      <Card className="plugin-explorer">
        <CardHeader>
          <CardTitle>Plugin List</CardTitle>
          <CardDescription>Stable ordering by plugin id. Select an item to open its full management panel.</CardDescription>
        </CardHeader>
        <CardContent className="stack">
          <div className="plugin-list">
            {catalog.map((plugin) => {
              const runtime = plugins.find((item) => item.record.plugin_id === plugin.id);
              return (
                <button
                  key={plugin.id}
                  type="button"
                  className={`plugin-list__item ${selectedPluginId === plugin.id ? 'is-selected' : ''}`}
                  onClick={() => onSelectPlugin(plugin.id)}
                >
                  <div className="plugin-list__meta">
                    <strong>{plugin.name}</strong>
                    <p>{plugin.id}</p>
                  </div>
                  <div className="plugin-list__badges">
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

      <div className="detail-stack">
        {selectedCatalogPlugin ? (
          <>
            <Card>
              <CardHeader>
                <div className="section-title">
                  <div>
                    <CardTitle>{selectedCatalogPlugin.name}</CardTitle>
                    <CardDescription>{selectedCatalogPlugin.description}</CardDescription>
                  </div>
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
                </div>
              </CardHeader>
              <CardContent className="stack">
                <div className="plugin-card__grid">
                  <div>
                    <label>Config JSON</label>
                    <Textarea rows={14} value={pluginDraft} onChange={(event) => onDraftChange(event.target.value)} />
                  </div>
                  <div className="plugin-card__side">
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
                    {selectedCatalogPlugin.id === 'xiaomi' ? (
                      <div className="plugin-auth-guide">
                        <div className="plugin-auth-guide__header">
                          <Badge tone="accent">Xiaomi Login</Badge>
                          <strong>Install or save config to authenticate.</strong>
                        </div>
                        <p className="muted">
                          Primary path: fill <code>accounts[0].region</code>, <code>username</code>,{' '}
                          <code>password</code>, and <code>device_id</code> in the JSON on the left, then click{' '}
                          <code>{selectedPlugin ? 'Save Config' : 'Install'}</code> and <code>Enable</code>. The plugin
                          authenticates when the runtime starts.
                        </p>
                        <p className="muted">
                          Optional paths: provide <code>service_token</code>, <code>ssecurity</code>, and{' '}
                          <code>user_id</code> to reuse an existing Xiaomi session, or use <code>Connect OAuth</code>{' '}
                          only when the draft already contains <code>client_id</code> and <code>redirect_url</code>.
                        </p>
                      </div>
                    ) : null}
                    <div className="button-row">
                      {selectedCatalogPlugin.id === 'xiaomi' ? (
                        <Button
                          variant="secondary"
                          onClick={onConnectXiaomiOAuth}
                          disabled={!xiaomiOAuthAvailable || busy === `xiaomi-oauth-${selectedCatalogPlugin.id}` || xiaomiOAuthActive}
                        >
                          Connect OAuth
                        </Button>
                      ) : null}
                      <Button onClick={onInstall} disabled={busy === `install-${selectedCatalogPlugin.id}`}>
                        Install
                      </Button>
                      <Button variant="secondary" onClick={onEnable}>
                        Enable
                      </Button>
                      <Button variant="secondary" onClick={onDisable}>
                        Disable
                      </Button>
                    </div>
                    <div className="button-row">
                      <Button variant="secondary" onClick={onDiscover}>
                        Discover
                      </Button>
                      <Button variant="danger" onClick={onDelete}>
                        Uninstall
                      </Button>
                      {selectedPlugin ? (
                        <Button variant="secondary" onClick={onSaveConfig}>
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
                </div>
              </CardContent>
            </Card>

            <Section className="grid grid--two">
              <Card>
                <CardHeader>
                  <CardTitle>Runtime Details</CardTitle>
                  <CardDescription>Current runtime view for the selected plugin.</CardDescription>
                </CardHeader>
                <CardContent className="stack">
                  {selectedPlugin ? (
                    <pre className="log-box">{prettyJson(selectedPlugin)}</pre>
                  ) : (
                    <p className="muted">No runtime data yet. Install and enable the plugin first.</p>
                  )}
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <div className="section-title">
                    <div>
                      <CardTitle>Plugin Logs</CardTitle>
                      <CardDescription>Live buffer for the currently selected plugin.</CardDescription>
                    </div>
                    <Badge tone="neutral">{selectedCatalogPlugin.id}</Badge>
                  </div>
                </CardHeader>
                <CardContent className="stack">
                  <div className="button-row">
                    <Button variant="secondary" onClick={onReloadLogs}>
                      Reload
                    </Button>
                  </div>
                  <pre className="log-box">
                    {selectedPlugin ? pluginLogs.join('\n') || 'No logs captured yet.' : 'Plugin is not installed.'}
                  </pre>
                </CardContent>
              </Card>
            </Section>
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
