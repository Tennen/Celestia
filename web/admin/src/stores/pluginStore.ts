import { create } from 'zustand';
import {
  deletePlugin as deletePluginApi,
  discoverPlugin as discoverPluginApi,
  enablePlugin as enablePluginApi,
  disablePlugin as disablePluginApi,
  fetchPluginLogs,
  installPlugin as installPluginApi,
  updatePluginConfig,
} from '../lib/api';
import {
  asArray,
  buildInstallDrafts,
  mergeCatalogDefaultConfig,
  mergeXiaomiAccountConfig,
} from '../lib/admin';
import type { CatalogPlugin, PluginRuntimeView } from '../lib/types';
import { useAdminStore } from './adminStore';

type PluginState = {
  selectedPluginId: string;
  installDrafts: Record<string, string>;
  configDrafts: Record<string, string>;
  pluginLogs: string[];
  busy: string;
  xiaomiVerifyTicket: string;
};

type PluginActions = {
  setSelectedPluginId: (id: string) => void;
  setDraft: (pluginId: string, isInstalled: boolean, value: string) => void;
  setXiaomiVerifyTicket: (ticket: string) => void;
  initDraftsFromCatalog: (catalog: CatalogPlugin[], plugins: PluginRuntimeView[]) => void;
  reloadPluginLogs: (pluginId?: string) => Promise<void>;
  installPlugin: (plugin: CatalogPlugin) => Promise<void>;
  enablePlugin: (pluginId: string) => Promise<void>;
  disablePlugin: (pluginId: string) => Promise<void>;
  discoverPlugin: (pluginId: string) => Promise<void>;
  deletePlugin: (pluginId: string) => Promise<void>;
  saveConfig: (pluginId: string) => Promise<void>;
  retryXiaomiVerification: (plugin: CatalogPlugin, verifyURL: string) => Promise<void>;
};

type PluginStore = PluginState & PluginActions;

const adminRefreshAll = () => useAdminStore.getState().refreshAll();
const adminReportError = (msg: string) => useAdminStore.getState().reportError(msg);

async function withBusy(
  set: (partial: Partial<PluginState>) => void,
  busyKey: string,
  fn: () => Promise<void>,
) {
  set({ busy: busyKey });
  try {
    await fn();
  } catch (error) {
    adminReportError(error instanceof Error ? error.message : 'Operation failed');
  } finally {
    set({ busy: '' });
  }
}

export const usePluginStore = create<PluginStore>((set, get) => ({
  selectedPluginId: '',
  installDrafts: {},
  configDrafts: {},
  pluginLogs: [],
  busy: '',
  xiaomiVerifyTicket: '',

  setSelectedPluginId: (id) => {
    set({ selectedPluginId: id });
    void get().reloadPluginLogs(id);
  },

  setDraft: (pluginId, isInstalled, value) => {
    if (isInstalled) {
      set((s) => ({ configDrafts: { ...s.configDrafts, [pluginId]: value } }));
    } else {
      set((s) => ({ installDrafts: { ...s.installDrafts, [pluginId]: value } }));
    }
  },

  setXiaomiVerifyTicket: (ticket) => set({ xiaomiVerifyTicket: ticket }),

  initDraftsFromCatalog: (catalog, plugins) => {
    const installDefaults = buildInstallDrafts(catalog);
    const configDrafts: Record<string, string> = {};
    for (const plugin of plugins) {
      const catalogPlugin = catalog.find((item) => item.id === plugin.record.plugin_id);
      const config = catalogPlugin
        ? mergeCatalogDefaultConfig(catalogPlugin, plugin.record.config ?? {})
        : (plugin.record.config ?? {});
      configDrafts[plugin.record.plugin_id] = JSON.stringify(config, null, 2);
    }
    set((s) => ({
      installDrafts: { ...installDefaults, ...s.installDrafts },
      configDrafts: { ...configDrafts, ...s.configDrafts },
    }));
  },

  reloadPluginLogs: async (pluginId) => {
    const id = pluginId ?? get().selectedPluginId;
    if (!id) {
      set({ pluginLogs: [] });
      return;
    }
    try {
      const data = await fetchPluginLogs(id);
      set({ pluginLogs: asArray(data.logs) });
    } catch {
      set({ pluginLogs: ['Unable to load logs.'] });
    }
  },

  installPlugin: async (plugin) => {
    await withBusy(set, `install-${plugin.id}`, async () => {
      const { installDrafts } = get();
      const config = JSON.parse(installDrafts[plugin.id] ?? '{}') as Record<string, unknown>;
      await installPluginApi({ plugin_id: plugin.id, config });
      await adminRefreshAll();
    });
  },

  enablePlugin: async (pluginId) => {
    await withBusy(set, `enable-${pluginId}`, async () => {
      await enablePluginApi(pluginId);
      await adminRefreshAll();
    });
  },

  disablePlugin: async (pluginId) => {
    await withBusy(set, `disable-${pluginId}`, async () => {
      await disablePluginApi(pluginId);
      await adminRefreshAll();
    });
  },

  discoverPlugin: async (pluginId) => {
    await withBusy(set, `discover-${pluginId}`, async () => {
      await discoverPluginApi(pluginId);
      await adminRefreshAll();
    });
  },

  deletePlugin: async (pluginId) => {
    await withBusy(set, `delete-${pluginId}`, async () => {
      await deletePluginApi(pluginId);
      await adminRefreshAll();
    });
  },

  saveConfig: async (pluginId) => {
    await withBusy(set, `refresh-config-${pluginId}`, async () => {
      const { configDrafts } = get();
      const config = JSON.parse(configDrafts[pluginId] ?? '{}') as Record<string, unknown>;
      await updatePluginConfig(pluginId, config);
      await adminRefreshAll();
    });
  },

  retryXiaomiVerification: async (plugin, verifyURL) => {
    const { xiaomiVerifyTicket, installDrafts, configDrafts } = get();
    const ticket = xiaomiVerifyTicket.trim();
    if (!ticket) return;

    const adminState = useAdminStore.getState();
    const isInstalled = adminState.plugins.some((p) => p.record.plugin_id === plugin.id);
    const currentDraft = isInstalled
      ? (configDrafts[plugin.id] ?? installDrafts[plugin.id] ?? '{}')
      : (installDrafts[plugin.id] ?? configDrafts[plugin.id] ?? '{}');

    const merged = mergeXiaomiAccountConfig(currentDraft, {
      verify_url: verifyURL,
      verify_ticket: ticket,
    });

    set((s) => ({ configDrafts: { ...s.configDrafts, [plugin.id]: merged.json } }));

    await withBusy(set, `xiaomi-verify-${plugin.id}`, async () => {
      await updatePluginConfig(plugin.id, merged.draft);
      set({ xiaomiVerifyTicket: '' });
      await adminRefreshAll();
    });
  },
}));
