import { describe, expect, it } from 'vitest';
import { emptyLoadState, mergeLoadStateData } from './admin';
import type { LoadState } from './admin';
import type { AuditRecord, CapabilitySummary, CatalogPlugin, DashboardSummary, DeviceView, EventRecord, PluginRuntimeView } from './types';

function buildCatalogPlugin(id: string): CatalogPlugin {
  return {
    id,
    name: id,
    description: `${id} plugin`,
    binary_name: `${id}-plugin`,
    manifest: {
      id,
      name: id,
      version: '1.0.0',
      vendor: 'celestia',
      capabilities: ['discover'],
      device_kinds: ['light'],
    },
  };
}

function buildPluginRuntimeView(pluginId: string): PluginRuntimeView {
  return {
    record: {
      plugin_id: pluginId,
      version: '1.0.0',
      status: 'enabled',
      binary_path: `/bin/${pluginId}`,
      config: {},
      installed_at: '2026-04-09T00:00:00Z',
      updated_at: '2026-04-09T00:00:00Z',
      last_health_status: 'healthy',
    },
    health: {
      plugin_id: pluginId,
      status: 'healthy',
      message: 'ok',
      checked_at: '2026-04-09T00:00:00Z',
    },
    running: true,
  };
}

function buildCapability(id: string): CapabilitySummary {
  return {
    id,
    kind: 'automation',
    name: id,
    description: `${id} capability`,
    enabled: true,
    status: 'healthy',
    updated_at: '2026-04-09T00:00:00Z',
  };
}

function buildDeviceView(id: string, name = id): DeviceView {
  return {
    device: {
      id,
      plugin_id: 'xiaomi',
      vendor_device_id: `vendor-${id}`,
      kind: 'light',
      name,
      online: true,
      capabilities: ['toggle'],
    },
    state: {
      device_id: id,
      plugin_id: 'xiaomi',
      ts: '2026-04-09T00:00:00Z',
      state: { power: true },
    },
    controls: [
      {
        id: 'power',
        kind: 'toggle',
        label: 'Power',
        visible: true,
        state: true,
      },
    ],
  };
}

function buildEvent(id: string): EventRecord {
  return {
    id,
    type: 'device.state.changed',
    device_id: 'device-1',
    ts: '2026-04-09T00:00:00Z',
    payload: { power: true },
  };
}

function buildAudit(id: string): AuditRecord {
  return {
    id,
    actor: 'admin',
    device_id: 'device-1',
    action: 'toggle',
    result: 'accepted',
    risk_level: 'low',
    allowed: true,
    created_at: '2026-04-09T00:00:00Z',
  };
}

function buildState(overrides: Partial<LoadState> = {}): LoadState {
  return {
    ...emptyLoadState(),
    dashboard: {
      plugins: 1,
      enabled_plugins: 1,
      devices: 1,
      online_devices: 1,
      events: 1,
      audits: 1,
    },
    catalog: [buildCatalogPlugin('xiaomi')],
    plugins: [buildPluginRuntimeView('xiaomi')],
    capabilities: [buildCapability('automation')],
    automations: [],
    devices: [buildDeviceView('device-1', 'Desk Lamp')],
    events: [buildEvent('event-1')],
    audits: [buildAudit('audit-1')],
    loading: false,
    refreshing: false,
    hasLoaded: true,
    error: null,
    ...overrides,
  };
}

describe('mergeLoadStateData', () => {
  it('reuses unchanged references on refresh', () => {
    const current = buildState();
    const nextDashboard: DashboardSummary = JSON.parse(JSON.stringify(current.dashboard)) as DashboardSummary;
    const nextDevices = JSON.parse(JSON.stringify(current.devices)) as DeviceView[];

    const merged = mergeLoadStateData(current, {
      dashboard: nextDashboard,
      catalog: JSON.parse(JSON.stringify(current.catalog)) as CatalogPlugin[],
      plugins: JSON.parse(JSON.stringify(current.plugins)) as PluginRuntimeView[],
      capabilities: JSON.parse(JSON.stringify(current.capabilities)) as CapabilitySummary[],
      automations: [],
      devices: nextDevices,
      events: JSON.parse(JSON.stringify(current.events)) as EventRecord[],
      audits: JSON.parse(JSON.stringify(current.audits)) as AuditRecord[],
    });

    expect(merged.dashboard).toBe(current.dashboard);
    expect(merged.catalog).toBe(current.catalog);
    expect(merged.plugins).toBe(current.plugins);
    expect(merged.capabilities).toBe(current.capabilities);
    expect(merged.devices).toBe(current.devices);
    expect(merged.devices[0]).toBe(current.devices[0]);
    expect(merged.events).toBe(current.events);
    expect(merged.audits).toBe(current.audits);
  });

  it('reuses unchanged siblings while replacing changed items', () => {
    const firstDevice = buildDeviceView('device-1', 'Desk Lamp');
    const secondDevice = buildDeviceView('device-2', 'Hall Lamp');
    const current = buildState({
      dashboard: {
        plugins: 1,
        enabled_plugins: 1,
        devices: 2,
        online_devices: 2,
        events: 1,
        audits: 1,
      },
      devices: [firstDevice, secondDevice],
    });

    const nextDevices = [
      JSON.parse(JSON.stringify(firstDevice)) as DeviceView,
      {
        ...JSON.parse(JSON.stringify(secondDevice)) as DeviceView,
        device: {
          ...secondDevice.device,
          online: false,
        },
      },
    ];

    const merged = mergeLoadStateData(current, {
      dashboard: current.dashboard,
      catalog: current.catalog,
      plugins: current.plugins,
      capabilities: current.capabilities,
      automations: current.automations,
      devices: nextDevices,
      events: current.events,
      audits: current.audits,
    });

    expect(merged.devices).not.toBe(current.devices);
    expect(merged.devices[0]).toBe(firstDevice);
    expect(merged.devices[1]).not.toBe(secondDevice);
    expect(merged.devices[1].device.online).toBe(false);
  });
});
