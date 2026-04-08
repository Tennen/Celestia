import { create } from 'zustand';
import {
  fetchAutomations,
  fetchAudits,
  fetchCapabilities,
  fetchCatalogPlugins,
  fetchDashboard,
  fetchDevices,
  fetchEvents,
  fetchPlugins,
  getApiBase,
} from '../lib/api';
import { asArray, compareText, emptyLoadState, POLL_MS } from '../lib/admin';
import type { LoadState } from '../lib/admin';

type AdminActions = {
  refreshAll: () => Promise<void>;
  reportError: (message: string) => void;
  initPolling: () => () => void;
};

type AdminStore = LoadState & AdminActions;

export const useAdminStore = create<AdminStore>((set, get) => ({
  ...emptyLoadState(),

  reportError: (message: string) => {
    set({ error: message });
  },

  refreshAll: async () => {
    const state = get();
    if (state.refreshing || (state.loading && state.hasLoaded)) {
      return;
    }

    set(
      state.hasLoaded
        ? { refreshing: true, error: null }
        : { loading: true, refreshing: false, error: null },
    );

    try {
      const deviceSearch = getDeviceSearchRef();
      const [dashboard, rawCatalog, rawPlugins, rawCapabilities, rawAutomations, rawDevices, rawEvents, rawAudits] = await Promise.all([
        fetchDashboard(),
        fetchCatalogPlugins(),
        fetchPlugins(),
        fetchCapabilities(),
        fetchAutomations(),
        fetchDevices(deviceSearch),
        fetchEvents(80),
        fetchAudits(80),
      ]);

      const catalog = asArray(rawCatalog).sort((a, b) => compareText(a.id, b.id));
      const plugins = asArray(rawPlugins).sort((a, b) =>
        compareText(a.record.plugin_id, b.record.plugin_id),
      );
      const devices = asArray(rawDevices).sort((a, b) =>
        compareText(a.device.name || a.device.id, b.device.name || b.device.id),
      );

      set({
        dashboard,
        catalog,
        plugins,
        capabilities: asArray(rawCapabilities).sort((a, b) => compareText(a.name || a.id, b.name || b.id)),
        automations: asArray(rawAutomations).sort((a, b) => compareText(a.name || a.id, b.name || b.id)),
        devices,
        events: asArray(rawEvents),
        audits: asArray(rawAudits),
        loading: false,
        refreshing: false,
        hasLoaded: true,
        error: null,
      });

      // Auto-select first plugin/device if nothing selected or selection gone
      autoSelectPlugin(catalog);
      autoSelectDevice(devices);
    } catch (error) {
      set({
        loading: false,
        refreshing: false,
        error: error instanceof Error ? error.message : 'Failed to load admin data',
      });
    }
  },

  initPolling: () => {
    void get().refreshAll();

    const interval = window.setInterval(() => {
      void get().refreshAll();
    }, POLL_MS);

    const source = new EventSource(`${getApiBase()}/events/stream`);
    const onEvent = () => void get().refreshAll();
    source.onmessage = onEvent;
    source.addEventListener('device.state.changed', onEvent);
    source.addEventListener('device.event.occurred', onEvent);
    source.addEventListener('plugin.lifecycle.changed', onEvent);
    source.addEventListener('capability.status.changed', onEvent);
    source.addEventListener('automation.triggered', onEvent);
    source.addEventListener('automation.failed', onEvent);
    source.onerror = () => source.close();

    return () => {
      window.clearInterval(interval);
      source.close();
    };
  },
}));

// ---------------------------------------------------------------------------
// Helpers for auto-selection — read/write pluginStore and deviceStore lazily
// to avoid circular import issues at module load time.
// ---------------------------------------------------------------------------

let getDeviceSearchRef: () => string = () => '';

export function setDeviceSearchProvider(fn: () => string) {
  getDeviceSearchRef = fn;
}

let autoSelectPlugin: (catalog: import('../lib/types').CatalogPlugin[]) => void = () => {};
let autoSelectDevice: (devices: import('../lib/types').DeviceView[]) => void = () => {};

export function setAutoSelectHandlers(
  pluginFn: (catalog: import('../lib/types').CatalogPlugin[]) => void,
  deviceFn: (devices: import('../lib/types').DeviceView[]) => void,
) {
  autoSelectPlugin = pluginFn;
  autoSelectDevice = deviceFn;
}
