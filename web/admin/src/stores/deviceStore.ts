import { create } from 'zustand';
import {
  runActionControl,
  sendCommand as sendCommandApi,
  sendToggle,
  updateDeviceControlPreference as updateControlPrefApi,
  updateDevicePreference as updateDevicePrefApi,
} from '../lib/api';
import {
  getToggleControlStates,
  getToggleOverrideKey,
  pruneToggleOverrides,
  type ToggleControlOverrideMap,
  type ToggleControlPendingMap,
} from '../lib/control-state';
import { prettyJson } from '../lib/utils';
import type { DeviceView } from '../lib/types';
import { useAdminStore } from './adminStore';

type DeviceState = {
  selectedDeviceId: string;
  deviceSearch: string;
  selectedAction: string;
  commandParams: string;
  actor: string;
  commandResult: string;
  busy: string;
  toggleOverrides: ToggleControlOverrideMap;
  togglePending: ToggleControlPendingMap;
};

type DeviceActions = {
  setSelectedDeviceId: (id: string) => void;
  setDeviceSearch: (search: string) => void;
  setSelectedAction: (action: string) => void;
  setCommandParams: (params: string) => void;
  setActor: (actor: string) => void;
  applySuggestion: (action: string, params: Record<string, unknown>) => void;
  sendCommand: (device: DeviceView) => Promise<void>;
  onToggleControl: (device: DeviceView, controlId: string, on: boolean) => Promise<void>;
  onActionControl: (device: DeviceView, controlId: string) => Promise<void>;
  onValueControl: (device: DeviceView, controlId: string, value: string | number) => Promise<void>;
  updateDevicePreference: (device: DeviceView, payload: { alias?: string }) => Promise<void>;
  updateControlPreference: (
    device: DeviceView,
    controlId: string,
    payload: { alias?: string; visible: boolean },
  ) => Promise<void>;
  pruneOverrides: (devices: DeviceView[]) => void;
};

type DeviceStore = DeviceState & DeviceActions;

const adminRefreshAll = () => useAdminStore.getState().refreshAll();
const adminReportError = (msg: string) => useAdminStore.getState().reportError(msg);

async function withBusy(
  set: (partial: Partial<DeviceState>) => void,
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

export const useDeviceStore = create<DeviceStore>((set, get) => ({
  selectedDeviceId: '',
  deviceSearch: '',
  selectedAction: 'feed_once',
  commandParams: '{\n  "portions": 1\n}',
  actor: 'admin',
  commandResult: '',
  busy: '',
  toggleOverrides: {},
  togglePending: {},

  setSelectedDeviceId: (id) => set({ selectedDeviceId: id }),
  setDeviceSearch: (search) => set({ deviceSearch: search }),
  setSelectedAction: (action) => set({ selectedAction: action }),
  setCommandParams: (params) => set({ commandParams: params }),
  setActor: (actor) => set({ actor }),

  applySuggestion: (action, params) => {
    set({ selectedAction: action, commandParams: JSON.stringify(params, null, 2) });
  },

  sendCommand: async (device) => {
    await withBusy(set, 'send-command', async () => {
      const { selectedAction, commandParams, actor } = get();
      const parsed = JSON.parse(commandParams || '{}') as Record<string, unknown>;
      const response = await sendCommandApi(device.device.id, selectedAction, parsed, actor);
      set({ commandResult: prettyJson(response) });
      await adminRefreshAll();
    });
  },

  onToggleControl: async (device, controlId, on) => {
    const { toggleOverrides, togglePending, actor } = get();
    const deviceId = device.device.id;
    const compoundId = `${deviceId}.${controlId}`;
    const overrideKey = getToggleOverrideKey(deviceId, controlId);
    const toggleStates = getToggleControlStates(device, controlId, toggleOverrides);

    if (!toggleStates || togglePending[overrideKey]) return;

    // Optimistic update
    set((s) => ({
      toggleOverrides: {
        ...s.toggleOverrides,
        [overrideKey]: { state: on, requestedAt: Date.now() },
      },
      togglePending: { ...s.togglePending, [overrideKey]: true },
    }));

    try {
      await sendToggle(compoundId, on, actor);
    } catch (error) {
      // Rollback
      set((s) => {
        const next = { ...s.toggleOverrides };
        const { persistedState } = toggleStates;
        if (persistedState === toggleStates.currentState) {
          delete next[overrideKey];
        } else {
          next[overrideKey] = { state: persistedState ?? false, requestedAt: Date.now() };
        }
        const nextPending = { ...s.togglePending };
        delete nextPending[overrideKey];
        return { toggleOverrides: next, togglePending: nextPending };
      });
      adminReportError(error instanceof Error ? error.message : 'Operation failed');
      return;
    }

    try {
      await adminRefreshAll();
    } catch (error) {
      adminReportError(error instanceof Error ? error.message : 'Operation failed');
    } finally {
      set((s) => {
        const nextPending = { ...s.togglePending };
        delete nextPending[overrideKey];
        return { togglePending: nextPending };
      });
    }
  },

  onActionControl: async (device, controlId) => {
    const { actor } = get();
    const compoundId = `${device.device.id}.${controlId}`;
    await withBusy(set, `action-${compoundId}`, async () => {
      await runActionControl(compoundId, actor);
      await adminRefreshAll();
    });
  },

  onValueControl: async (device, controlId, value) => {
    const { actor } = get();
    const control = (device.controls ?? []).find((item) => item.id === controlId);
    if (!control?.command?.action) return;
    const params = {
      ...(control.command.params ?? {}),
      [(control.command.value_param ?? 'value') as string]: value,
    };
    const busyKey = `value-${device.device.id}.${controlId}`;
    await withBusy(set, busyKey, async () => {
      await sendCommandApi(device.device.id, control.command!.action, params, actor);
      await adminRefreshAll();
    });
  },

  updateDevicePreference: async (device, payload) => {
    const busyKey = `device-pref-${device.device.id}`;
    await withBusy(set, busyKey, async () => {
      await updateDevicePrefApi(device.device.id, payload);
      await adminRefreshAll();
    });
  },

  updateControlPreference: async (device, controlId, payload) => {
    const busyKey = `control-pref-${device.device.id}.${controlId}`;
    await withBusy(set, busyKey, async () => {
      await updateControlPrefApi(device.device.id, controlId, payload);
      await adminRefreshAll();
    });
  },

  pruneOverrides: (devices) => {
    set((s) => ({
      toggleOverrides: pruneToggleOverrides(s.toggleOverrides, devices),
    }));
  },
}));
