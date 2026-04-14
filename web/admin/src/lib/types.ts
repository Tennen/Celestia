export type RiskLevel = 'low' | 'medium' | 'high';
export type PluginStatus = 'installed' | 'enabled' | 'disabled';
export type HealthState = 'unknown' | 'healthy' | 'degraded' | 'unhealthy' | 'stopped';
export type CapabilityKind = 'automation' | 'vision_entity_stay_zone';
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

export type AutomationLogic = 'all' | 'any';
export type AutomationMatchOperator = 'any' | 'equals' | 'not_equals' | 'in' | 'not_in' | 'exists' | 'missing';
export type AutomationRunStatus = 'idle' | 'succeeded' | 'failed';

export type AutomationStateMatch = {
  operator: AutomationMatchOperator;
  value?: unknown;
};

export type AutomationConditionType = 'state_changed' | 'current_state';

export type AutomationCondition = {
  type?: AutomationConditionType;
  device_id: string;
  state_key: string;
  from?: AutomationStateMatch;
  to?: AutomationStateMatch;
  match?: AutomationStateMatch;
};

export type AutomationTimeWindow = {
  start: string;
  end: string;
};

export type AutomationAction = {
  device_id: string;
  label?: string;
  action: string;
  params?: Record<string, unknown>;
};

export type Automation = {
  id: string;
  name: string;
  enabled: boolean;
  condition_logic: AutomationLogic;
  conditions?: AutomationCondition[];
  time_window?: AutomationTimeWindow | null;
  actions: AutomationAction[];
  last_triggered_at?: string | null;
  last_run_status?: AutomationRunStatus;
  last_error?: string;
  created_at: string;
  updated_at: string;
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

export type CapabilitySummary = {
  id: string;
  kind: CapabilityKind;
  name: string;
  description: string;
  enabled: boolean;
  status: HealthState;
  summary?: Record<string, unknown>;
  updated_at: string;
};

export type AutomationCapabilityDetail = {
  total: number;
  enabled_count: number;
  last_triggered_at?: string | null;
};

export type VisionEntitySelector = {
  kind: string;
  value: string;
};

export type VisionEntityDescriptor = {
  kind: string;
  value: string;
  display_name?: string;
};

export type VisionRuleKeyEntityImage = {
  base64: string;
  content_type?: string;
};

export type VisionRuleKeyEntity = {
  id: number;
  image?: VisionRuleKeyEntityImage;
  description?: string;
};

export type VisionZoneBox = {
  x: number;
  y: number;
  width: number;
  height: number;
};

export type VisionRTSPSource = {
  url: string;
};

export type VisionRule = {
  id: string;
  name: string;
  enabled: boolean;
  camera_device_id: string;
  recognition_enabled: boolean;
  rtsp_source: VisionRTSPSource;
  entity_selector: VisionEntitySelector;
  behavior: string;
  key_entities: VisionRuleKeyEntity[];
  zone: VisionZoneBox;
  stay_threshold_seconds: number;
};

export type VisionCapabilityConfig = {
  service_ws_url: string;
  model_name?: string;
  recognition_enabled: boolean;
  event_capture_retention_hours: number;
  rules: VisionRule[];
  updated_at: string;
};

export type VisionEntityCatalog = {
  service_ws_url: string;
  schema_version: string;
  service_version?: string;
  model_name?: string;
  fetched_at: string;
  entities: VisionEntityDescriptor[];
};

export type VisionEntityCatalogRefreshRequest = {
  service_ws_url?: string;
  model_name?: string;
};

export type VisionCapabilityStatus = {
  status: HealthState;
  message?: string;
  service_version?: string;
  last_synced_at?: string | null;
  last_reported_at?: string | null;
  last_event_at?: string | null;
  runtime?: Record<string, unknown>;
  sync_error?: string;
  updated_at: string;
};

export type VisionCapabilityDetail = {
  config: VisionCapabilityConfig;
  runtime: VisionCapabilityStatus;
  catalog?: VisionEntityCatalog | null;
  recent_events?: EventRecord[];
};

export type CapabilityDetail = CapabilitySummary & {
  automation?: AutomationCapabilityDetail | null;
  vision?: VisionCapabilityDetail | null;
};

export type Device = {
  id: string;
  plugin_id: string;
  vendor_device_id: string;
  kind: DeviceKind;
  name: string;
  default_name?: string;
  alias?: string;
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

export type DeviceControlKind = 'toggle' | 'action' | 'select' | 'number';

export type DeviceControlOption = {
  value: string;
  label: string;
};

export type DeviceStateDescriptor = {
  label?: string;
  options?: DeviceControlOption[];
  hidden?: boolean;
};

export type DeviceControlCommand = {
  action: string;
  params?: Record<string, unknown>;
  value_param?: string;
};

export type DeviceControl = {
  id: string;
  kind: DeviceControlKind;
  label: string;
  default_label?: string;
  alias?: string;
  disabled?: boolean;
  disabled_reason?: string;
  state?: boolean | null;
  value?: string | number | boolean | null;
  min?: number;
  max?: number;
  step?: number;
  unit?: string;
  options?: DeviceControlOption[];
  command?: DeviceControlCommand | null;
  visible: boolean;
};

export type DeviceView = {
  device: Device;
  state: DeviceStateSnapshot;
  controls?: DeviceControl[];
};

export type EventRecord = {
  id: string;
  type: string;
  plugin_id?: string;
  device_id?: string;
  ts: string;
  payload?: Record<string, unknown>;
};

export type VisionEventCapture = {
  capture_id: string;
  event_id: string;
  rule_id?: string;
  camera_device_id?: string;
  phase: 'start' | 'middle' | 'end';
  captured_at: string;
  content_type: string;
  size_bytes: number;
  metadata?: Record<string, unknown>;
};

export type VisionCaptureAnnotationBox = {
  x: number;
  y: number;
  width: number;
  height: number;
};

export type VisionCaptureDetection = {
  kind?: string;
  value?: string;
  display_name: string;
  confidence?: number;
  track_id?: string;
  box: VisionCaptureAnnotationBox;
};

export type VisionCaptureAnnotations = {
  image_kind: 'raw' | 'annotated';
  coordinate_space: 'normalized_xywh';
  source?: string;
  detections: VisionCaptureDetection[];
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

export type AdminStreamFrame = {
  reason?: string;
  dashboard?: DashboardSummary | null;
  plugins?: PluginRuntimeView[];
  capabilities?: CapabilitySummary[];
  automations?: Automation[];
  devices?: DeviceView[];
  events?: EventRecord[];
  audits?: AuditRecord[];
  event?: EventRecord;
  audit?: AuditRecord;
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

export type XiaomiOAuthSessionStatus = 'pending' | 'completed' | 'failed' | 'expired';

export type OAuthSession = {
  id: string;
  provider: 'xiaomi';
  plugin_id?: string;
  account_name?: string;
  region?: string;
  client_id?: string;
  redirect_url?: string;
  device_id?: string;
  auth_url?: string;
  status: XiaomiOAuthSessionStatus;
  error?: string;
  account_config?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
  completed_at?: string | null;
  state_expires_at?: string | null;
  token_expires_at?: string | null;
};

export type XiaomiOAuthStartResponse = {
  session: OAuthSession;
};
