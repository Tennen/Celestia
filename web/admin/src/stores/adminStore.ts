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
import {
  applyAdminStreamFrame,
  emptyLoadState,
  mergeLoadStateData,
  sortAutomations,
  sortCapabilities,
  sortDevices,
  sortPlugins,
} from '../lib/admin';
import type { LoadState, RemoteLoadState } from '../lib/admin';
import type { AdminStreamFrame } from '../lib/types';

type AdminActions = {
  refreshAll: () => Promise<void>;
  reportError: (message: string) => void;
  initStream: () => () => void;
};

type AdminStore = LoadState & AdminActions;

function applyRemoteState(
  set: (
    updater: (current: AdminStore) => Partial<AdminStore>,
  ) => void,
  next: RemoteLoadState,
) {
  let mergedState: RemoteLoadState = next;
  set((current) => {
    mergedState = mergeLoadStateData(current, next);
    return {
      ...mergedState,
      loading: false,
      refreshing: false,
      hasLoaded: true,
      error: null,
    };
  });
  autoSelectPlugin(mergedState.catalog);
  autoSelectDevice(mergedState.devices);
}

function loadRemoteState(frame: {
  dashboard: LoadState['dashboard'];
  catalog: LoadState['catalog'];
  plugins: LoadState['plugins'];
  capabilities: LoadState['capabilities'];
  automations: LoadState['automations'];
  devices: LoadState['devices'];
  events: LoadState['events'];
  audits: LoadState['audits'];
}): RemoteLoadState {
  return {
    dashboard: frame.dashboard,
    catalog: frame.catalog,
    plugins: sortPlugins(frame.plugins),
    capabilities: sortCapabilities(frame.capabilities),
    automations: sortAutomations(frame.automations),
    devices: sortDevices(frame.devices),
    events: frame.events,
    audits: frame.audits,
  };
}

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
      const [dashboard, catalog, plugins, capabilities, automations, devices, events, audits] = await Promise.all([
        fetchDashboard(),
        fetchCatalogPlugins(),
        fetchPlugins(),
        fetchCapabilities(),
        fetchAutomations(),
        fetchDevices(),
        fetchEvents({ limit: 80 }),
        fetchAudits(80),
      ]);

      applyRemoteState(
        set,
        loadRemoteState({
          dashboard,
          catalog,
          plugins,
          capabilities,
          automations,
          devices,
          events,
          audits,
        }),
      );
    } catch (error) {
      set({
        loading: false,
        refreshing: false,
        error: error instanceof Error ? error.message : 'Failed to load admin data',
      });
    }
  },

  initStream: () => {
    void get().refreshAll();

    const source = new EventSource(`${getApiBase()}/admin/stream`);

    const applyFrame = (event: MessageEvent<string>) => {
      try {
        const frame = JSON.parse(event.data) as AdminStreamFrame;
        const next = applyAdminStreamFrame(get(), frame);
        applyRemoteState(set, next);
      } catch (error) {
        set({
          error: error instanceof Error ? error.message : 'Failed to apply admin stream update',
        });
      }
    };

    source.addEventListener('sync', applyFrame as EventListener);
    source.addEventListener('update', applyFrame as EventListener);
    source.onerror = () => {
      // Keep the EventSource alive so the browser can reconnect automatically.
    };

    return () => {
      source.close();
    };
  },
}));

let autoSelectPlugin: (catalog: import('../lib/types').CatalogPlugin[]) => void = () => {};
let autoSelectDevice: (devices: import('../lib/types').DeviceView[]) => void = () => {};

export function setAutoSelectHandlers(
  pluginFn: (catalog: import('../lib/types').CatalogPlugin[]) => void,
  deviceFn: (devices: import('../lib/types').DeviceView[]) => void,
) {
  autoSelectPlugin = pluginFn;
  autoSelectDevice = deviceFn;
}
