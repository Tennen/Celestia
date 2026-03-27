import { useEffect, useState } from 'react';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { Input } from '../ui/input';
import { Section } from '../ui/section';
import { asArray } from '../../lib/admin';
import { formatTime, prettyJson } from '../../lib/utils';
import type { CatalogPlugin, PluginRuntimeView } from '../../lib/types';
import { PluginConfigPanel } from './PluginConfigPanel';

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
  xiaomiVerifyTicket: string;
  onSelectPlugin: (pluginId: string) => void;
  onDraftChange: (value: string) => void;
  onXiaomiVerifyTicketChange: (value: string) => void;
  onConnectXiaomiOAuth: () => void;
  onRetryXiaomiVerification: (verifyURL: string) => void;
  onInstall: () => void;
  onEnable: () => void;
  onDisable: () => void;
  onDiscover: () => void;
  onDelete: () => void;
  onSaveConfig: () => void;
  onReloadLogs: () => void;
};

function extractXiaomiVerificationHint(errorText?: string | null) {
  if (!errorText) {
    return null;
  }
  const match = errorText.match(/requires (secondary verification|captcha) at (\S+)/i);
  if (!match) {
    return null;
  }
  return {
    kind: match[1].toLowerCase(),
    url: match[2],
  };
}

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
  xiaomiVerifyTicket,
  onSelectPlugin,
  onDraftChange,
  onXiaomiVerifyTicketChange,
  onConnectXiaomiOAuth,
  onRetryXiaomiVerification,
  onInstall,
  onEnable,
  onDisable,
  onDiscover,
  onDelete,
  onSaveConfig,
  onReloadLogs,
}: Props) {
  const [detailMode, setDetailMode] = useState<'runtime' | 'config' | 'logs'>('runtime');

  useEffect(() => {
    setDetailMode('runtime');
  }, [selectedPluginId]);

  const xiaomiVerificationHint =
    selectedCatalogPlugin?.id === 'xiaomi'
      ? extractXiaomiVerificationHint(selectedPlugin?.last_error ?? selectedPlugin?.health.message)
      : null;

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
                          onChange={(event) => onXiaomiVerifyTicketChange(event.target.value)}
                          placeholder="Enter SMS or email code from Xiaomi"
                        />
                      </div>
                      <div className="button-row">
                        <a
                          className="button button--secondary"
                          href={xiaomiVerificationHint.url}
                          target="_blank"
                          rel="noreferrer"
                        >
                          Open Verification Page
                        </a>
                        {selectedPlugin ? (
                          <Button
                            variant="secondary"
                            onClick={() => onRetryXiaomiVerification(xiaomiVerificationHint.url)}
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
              </CardContent>
            </Card>

            <div className="detail-tabs">
              <button
                type="button"
                className={`detail-tabs__button ${detailMode === 'runtime' ? 'is-active' : ''}`}
                onClick={() => setDetailMode('runtime')}
              >
                Runtime
              </button>
              <button
                type="button"
                className={`detail-tabs__button ${detailMode === 'config' ? 'is-active' : ''}`}
                onClick={() => setDetailMode('config')}
              >
                Config
              </button>
              <button
                type="button"
                className={`detail-tabs__button ${detailMode === 'logs' ? 'is-active' : ''}`}
                onClick={() => setDetailMode('logs')}
              >
                Logs
              </button>
            </div>

            {detailMode === 'runtime' ? (
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
                    <CardTitle>Current Config Snapshot</CardTitle>
                    <CardDescription>The installed config record currently persisted by Core.</CardDescription>
                  </CardHeader>
                  <CardContent className="stack">
                    {selectedPlugin ? (
                      <pre className="log-box">{prettyJson(selectedPlugin.record.config ?? {})}</pre>
                    ) : (
                      <p className="muted">Install the plugin to persist runtime config.</p>
                    )}
                  </CardContent>
                </Card>
              </Section>
            ) : null}

            {detailMode === 'config' ? (
              <PluginConfigPanel
                plugin={selectedCatalogPlugin}
                runtimeInstalled={Boolean(selectedPlugin)}
                pluginDraft={pluginDraft}
                busy={busy}
                onDraftChange={onDraftChange}
                onInstall={onInstall}
                onSaveConfig={onSaveConfig}
              />
            ) : null}

            {detailMode === 'logs' ? (
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
            ) : null}
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
