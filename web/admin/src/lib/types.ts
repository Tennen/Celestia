export type RiskLevel = 'low' | 'medium' | 'high';
export type PluginStatus = 'installed' | 'enabled' | 'disabled';
export type HealthState = 'unknown' | 'healthy' | 'degraded' | 'unhealthy' | 'stopped';
export type DeviceKind =
  | 'light'
  | 'switch'
  | 'sensor'
  | 'climate'
  | 'washer'
  | 'pet_feeder'
  | 'pet_fountain'
  | 'pet_litter_box'
  | 'aquarium'
  | 'speaker'
  | 'camera_like';

export type DashboardSummary = {
  plugins: number;
  enabled_plugins: number;
  devices: number;
  online_devices: number;
  events: number;
  audits: number;
};

export type PluginManifest = {
  id: string;
  name: string;
  version: string;
  vendor: string;
  capabilities: string[];
  config_schema?: Record<string, unknown>;
  device_kinds: DeviceKind[];
  metadata?: Record<string, unknown>;
};

export type CatalogPlugin = {
  id: string;
  name: string;
  description: string;
  binary_name: string;
  manifest: PluginManifest;
};

export type PluginInstallRecord = {
  plugin_id: string;
  version: string;
  status: PluginStatus;
  binary_path: string;
  config: Record<string, unknown>;
  config_ref?: string;
  installed_at: string;
  updated_at: string;
  last_heartbeat_at?: string | null;
  last_health_status: HealthState;
  metadata?: Record<string, unknown>;
};

export type PluginHealth = {
  plugin_id: string;
  status: HealthState;
  message: string;
  checked_at: string;
  manifest_version?: string;
  process_pid?: number;
};

export type PluginRuntimeView = {
  record: PluginInstallRecord;
  manifest?: PluginManifest | null;
  health: PluginHealth;
  running: boolean;
  last_error?: string;
  recent_logs?: string[];
  process_pid?: number;
  listen_addr?: string;
};

export type Device = {
  id: string;
  plugin_id: string;
  vendor_device_id: string;
  kind: DeviceKind;
  name: string;
  room?: string;
  online: boolean;
  capabilities: string[];
  metadata?: Record<string, unknown>;
};

export type DeviceStateSnapshot = {
  device_id: string;
  plugin_id: string;
  ts: string;
  state: Record<string, unknown>;
};

export type DeviceView = {
  device: Device;
  state: DeviceStateSnapshot;
};

export type EventRecord = {
  id: string;
  type: string;
  plugin_id?: string;
  device_id?: string;
  ts: string;
  payload?: Record<string, unknown>;
};

export type AuditRecord = {
  id: string;
  actor: string;
  device_id: string;
  action: string;
  params?: Record<string, unknown>;
  result: string;
  risk_level: RiskLevel;
  allowed: boolean;
  created_at: string;
};

export type CommandResult = {
  decision?: {
    allowed: boolean;
    risk_level: RiskLevel;
    reason?: string;
  };
  result?: {
    accepted: boolean;
    job_id?: string;
    message?: string;
  };
};
