import { useEffect, useRef, useState } from 'react';
import type { Dispatch, SetStateAction } from 'react';
import { fetchXiaomiOAuthSession, getApiBase, startXiaomiOAuth } from '../lib/api';
import { getPluginDraftText, getXiaomiDraftSeed, mergeXiaomiAccountConfig } from '../lib/admin';
import type { StatusBanner } from '../lib/admin';
import type { CatalogPlugin, OAuthSession, PluginRuntimeView } from '../lib/types';

type Params = {
  plugins: PluginRuntimeView[];
  installDrafts: Record<string, string>;
  configDrafts: Record<string, string>;
  setInstallDrafts: Dispatch<SetStateAction<Record<string, string>>>;
  setConfigDrafts: Dispatch<SetStateAction<Record<string, string>>>;
};

export function useXiaomiOAuth({
  plugins,
  installDrafts,
  configDrafts,
  setInstallDrafts,
  setConfigDrafts,
}: Params) {
  const [oauthBanner, setOauthBanner] = useState<StatusBanner | null>(null);

  const pluginsRef = useRef<PluginRuntimeView[]>(plugins);
  const installDraftsRef = useRef(installDrafts);
  const configDraftsRef = useRef(configDrafts);
  const sessionRef = useRef('');
  const popupRef = useRef<Window | null>(null);
  const pollRef = useRef<number | null>(null);

  useEffect(() => {
    pluginsRef.current = plugins;
  }, [plugins]);

  useEffect(() => {
    installDraftsRef.current = installDrafts;
  }, [installDrafts]);

  useEffect(() => {
    configDraftsRef.current = configDrafts;
  }, [configDrafts]);

  const clearFlow = () => {
    if (pollRef.current !== null) {
      window.clearInterval(pollRef.current);
      pollRef.current = null;
    }
    if (popupRef.current && !popupRef.current.closed) {
      popupRef.current.close();
    }
    popupRef.current = null;
    sessionRef.current = '';
  };

  useEffect(
    () => () => {
      clearFlow();
    },
    [],
  );

  const applySession = (session: OAuthSession) => {
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

  const syncSession = async (sessionId: string) => {
    const session = await fetchXiaomiOAuthSession(sessionId);
    if (session.status === 'pending') {
      return;
    }

    clearFlow();
    if (session.status === 'completed') {
      try {
        applySession(session);
      } catch (error) {
        setOauthBanner({
          tone: 'bad',
          text: error instanceof Error ? error.message : 'Failed to apply Xiaomi OAuth session.',
        });
      }
      return;
    }

    setOauthBanner({
      tone: session.status === 'expired' ? 'warn' : 'bad',
      text: session.error || `Xiaomi OAuth ${session.status}.`,
    });
  };

  const ensurePolling = (sessionId: string) => {
    if (pollRef.current !== null) {
      window.clearInterval(pollRef.current);
    }
    sessionRef.current = sessionId;
    pollRef.current = window.setInterval(() => {
      void syncSession(sessionId).catch((error) => {
        clearFlow();
        setOauthBanner({
          tone: 'bad',
          text: error instanceof Error ? error.message : 'Failed to refresh Xiaomi OAuth session.',
        });
      });
    }, 1500);
  };

  useEffect(() => {
    const handleMessage = (event: MessageEvent) => {
      const data = event.data as Partial<{ type: string; session_id: string }> | null;
      if (!data || data.type !== 'celestia:xiaomi-oauth') {
        return;
      }
      if (event.origin !== window.location.origin) {
        return;
      }
      if (!data.session_id || data.session_id !== sessionRef.current) {
        return;
      }
      void syncSession(data.session_id).catch((error) => {
        setOauthBanner({
          tone: 'bad',
          text: error instanceof Error ? error.message : 'Failed to refresh Xiaomi OAuth session.',
        });
      });
    };

    window.addEventListener('message', handleMessage);
    return () => window.removeEventListener('message', handleMessage);
  }, []);

  const startFlow = async (plugin: CatalogPlugin) => {
    const runtime = pluginsRef.current.find((item) => item.record.plugin_id === plugin.id) ?? null;
    const draftText = getPluginDraftText(plugin.id, Boolean(runtime), installDraftsRef.current, configDraftsRef.current);
    const seed = getXiaomiDraftSeed(draftText);
    const popup = window.open('', 'celestia-xiaomi-oauth', 'width=540,height=760');
    if (!popup) {
      throw new Error('Popup blocked. Allow popups to connect Xiaomi.');
    }

    popup.document.write('<!doctype html><title>Starting Xiaomi OAuth</title><p>Opening Xiaomi authorization...</p>');
    popup.document.close();
    popupRef.current = popup;
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
        clearFlow();
        throw new Error('Xiaomi OAuth start did not return an authorization URL');
      }

      popupRef.current = popup;
      popup.location.href = session.auth_url;
      ensurePolling(session.id);
      void syncSession(session.id).catch((error) => {
        setOauthBanner({
          tone: 'bad',
          text: error instanceof Error ? error.message : 'Failed to refresh Xiaomi OAuth session.',
        });
      });
    } catch (error) {
      clearFlow();
      setOauthBanner({
        tone: 'bad',
        text: error instanceof Error ? error.message : 'Failed to start Xiaomi OAuth.',
      });
      throw error;
    }
  };

  return {
    oauthBanner,
    oauthActive: Boolean(sessionRef.current),
    startFlow,
  };
}
