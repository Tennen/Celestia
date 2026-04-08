import type {
  Automation,
  AuditRecord,
  CapabilityDetail,
  CapabilitySummary,
  CatalogPlugin,
  CommandResult,
  DashboardSummary,
  DeviceView,
  EventRecord,
  OAuthSession,
  PluginInstallRecord,
  PluginRuntimeView,
  XiaomiOAuthStartResponse,
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

export async function fetchCapabilities() {
  return request<CapabilitySummary[]>('/capabilities');
}

export async function fetchAutomations() {
  return request<Automation[]>('/automations');
}

export async function fetchCapability(capabilityId: string) {
  return request<CapabilityDetail>(`/capabilities/${capabilityId}`);
}

export async function saveVisionCapabilityConfig(config: import('./types').VisionCapabilityConfig) {
  return request<CapabilityDetail>('/capabilities/vision_entity_stay_zone', {
    method: 'PUT',
    body: JSON.stringify(config),
  });
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

export async function saveAutomation(payload: Automation) {
  const isUpdate = Boolean(payload.id?.trim());
  const path = isUpdate ? `/automations/${payload.id}` : '/automations';
  const body = {
    ...payload,
    last_triggered_at: payload.last_triggered_at || undefined,
    last_run_status: undefined,
    last_error: undefined,
    created_at: payload.created_at || undefined,
    updated_at: payload.updated_at || undefined,
  };
  return request<Automation>(path, {
    method: isUpdate ? 'PUT' : 'POST',
    body: JSON.stringify(body),
  });
}

export async function deleteAutomation(automationId: string) {
  return request<{ ok: boolean }>(`/automations/${automationId}`, { method: 'DELETE' });
}

export async function fetchDevice(deviceId: string) {
  return request<DeviceView>(`/devices/${deviceId}`);
}

export async function updateDevicePreference(
  deviceId: string,
  payload: {
    alias?: string;
  },
) {
  return request<{ device_id: string; alias?: string; updated_at: string }>(`/devices/${deviceId}/preference`, {
    method: 'PUT',
    body: JSON.stringify(payload),
  });
}

export async function sendCommand(deviceId: string, action: string, params: Record<string, unknown>, actor = 'admin') {
  return request<CommandResult>(`/devices/${deviceId}/commands`, {
    method: 'POST',
    headers: { 'X-Actor': actor },
    body: JSON.stringify({ action, params }),
  });
}

export async function sendToggle(compoundId: string, on: boolean, actor = 'admin') {
  return request<CommandResult>(`/toggle/${compoundId}/${on ? 'on' : 'off'}`, {
    method: 'POST',
    headers: { 'X-Actor': actor },
  });
}

export async function runActionControl(compoundId: string, actor = 'admin') {
  return request<CommandResult>(`/action/${compoundId}`, {
    method: 'POST',
    headers: { 'X-Actor': actor },
  });
}

export async function updateDeviceControlPreference(
  deviceId: string,
  controlId: string,
  payload: {
    alias?: string;
    visible: boolean;
  },
) {
  return request<{ device_id: string; control_id: string; alias?: string; visible: boolean; updated_at: string }>(
    `/devices/${deviceId}/controls/${controlId}`,
    {
      method: 'PUT',
      body: JSON.stringify(payload),
    },
  );
}

export async function fetchEvents(limit = 100) {
  return request<EventRecord[]>(`/events?limit=${limit}`);
}

export async function fetchAudits(limit = 100) {
  return request<AuditRecord[]>(`/audits?limit=${limit}`);
}

export async function startXiaomiOAuth(payload: {
  plugin_id?: string;
  account_name?: string;
  region: string;
  client_id: string;
  redirect_base_url?: string;
}) {
  return request<XiaomiOAuthStartResponse>('/oauth/xiaomi/start', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export async function fetchXiaomiOAuthSession(sessionId: string) {
  return request<OAuthSession>(`/oauth/xiaomi/sessions/${sessionId}`);
}

export { ApiError };
