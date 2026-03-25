import type {
  AuditRecord,
  CatalogPlugin,
  CommandResult,
  DashboardSummary,
  DeviceView,
  EventRecord,
  PluginInstallRecord,
  PluginRuntimeView,
} from './types';

const RAW_BASE = import.meta.env.VITE_API_BASE_URL ?? '/api/v1';
const API_BASE = RAW_BASE.replace(/\/+$/, '');

class ApiError extends Error {
  status: number;

  constructor(message: string, status: number) {
    super(message);
    this.status = status;
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...(init?.headers ?? {}),
    },
  });

  if (!response.ok) {
    let message = `Request failed with ${response.status}`;
    try {
      const body = (await response.json()) as { error?: string };
      if (body?.error) {
        message = body.error;
      }
    } catch {
      const text = await response.text();
      if (text) {
        message = text;
      }
    }
    throw new ApiError(message, response.status);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return response.json() as Promise<T>;
}

export function getApiBase() {
  return API_BASE;
}

export async function fetchDashboard() {
  return request<DashboardSummary>('/dashboard');
}

export async function fetchCatalogPlugins() {
  return request<CatalogPlugin[]>('/catalog/plugins');
}

export async function fetchPlugins() {
  return request<PluginRuntimeView[]>('/plugins');
}

export async function installPlugin(payload: {
  plugin_id: string;
  binary_path?: string;
  config?: Record<string, unknown>;
  metadata?: Record<string, unknown>;
}) {
  return request<PluginInstallRecord>('/plugins', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export async function updatePluginConfig(pluginId: string, config: Record<string, unknown>) {
  return request<PluginInstallRecord>(`/plugins/${pluginId}/config`, {
    method: 'PUT',
    body: JSON.stringify({ config }),
  });
}

export async function enablePlugin(pluginId: string) {
  return request<{ ok: boolean }>(`/plugins/${pluginId}/enable`, { method: 'POST' });
}

export async function disablePlugin(pluginId: string) {
  return request<{ ok: boolean }>(`/plugins/${pluginId}/disable`, { method: 'POST' });
}

export async function discoverPlugin(pluginId: string) {
  return request<{ ok: boolean }>(`/plugins/${pluginId}/discover`, { method: 'POST' });
}

export async function deletePlugin(pluginId: string) {
  return request<{ ok: boolean }>(`/plugins/${pluginId}`, { method: 'DELETE' });
}

export async function fetchPluginLogs(pluginId: string) {
  return request<{ plugin_id: string; logs: string[] }>(`/plugins/${pluginId}/logs`);
}

export async function fetchDevices(query = '') {
  const suffix = query ? `?q=${encodeURIComponent(query)}` : '';
  return request<DeviceView[]>(`/devices${suffix}`);
}

export async function fetchDevice(deviceId: string) {
  return request<{ device: DeviceView['device']; state: DeviceView['state'] }>(`/devices/${deviceId}`);
}

export async function sendCommand(deviceId: string, action: string, params: Record<string, unknown>, actor = 'admin') {
  return request<CommandResult>(`/devices/${deviceId}/commands`, {
    method: 'POST',
    headers: { 'X-Actor': actor },
    body: JSON.stringify({ action, params }),
  });
}

export async function fetchEvents(limit = 100) {
  return request<EventRecord[]>(`/events?limit=${limit}`);
}

export async function fetchAudits(limit = 100) {
  return request<AuditRecord[]>(`/audits?limit=${limit}`);
}

export { ApiError };

