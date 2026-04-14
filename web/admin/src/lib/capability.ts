import type {
  CapabilityDetail,
  CapabilitySummary,
  DeviceView,
  VisionCapabilityConfig,
  VisionEntityCatalog,
  VisionEntityDescriptor,
  VisionRuleKeyEntity,
  VisionRule,
} from './types';

function readString(value: unknown) {
  return typeof value === 'string' ? value : '';
}

function readBoolean(value: unknown, fallback = false) {
  return typeof value === 'boolean' ? value : fallback;
}

function readNumber(value: unknown, fallback = 0) {
  return typeof value === 'number' && Number.isFinite(value) ? value : fallback;
}

function asArray<T>(value: T[] | null | undefined) {
  return Array.isArray(value) ? value : [];
}

function readRecord(value: unknown): Record<string, unknown> | null {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;
}

export function cloneVisionConfig<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}

export function defaultVisionConfig(): VisionCapabilityConfig {
  return {
    service_ws_url: '',
    model_name: '',
    recognition_enabled: false,
    event_capture_retention_hours: 168,
    rules: [],
    updated_at: '',
  };
}

function normalizeVisionEntityDescriptor(
  entity: Partial<VisionEntityDescriptor> | null | undefined,
): VisionEntityDescriptor | null {
  const kind = readString(entity?.kind);
  const value = readString(entity?.value);
  if (!kind || !value) {
    return null;
  }
  return {
    kind,
    value,
    display_name: readString(entity?.display_name) || undefined,
  };
}

function normalizeVisionRuleKeyEntity(
  keyEntity: Partial<VisionRuleKeyEntity> | null | undefined,
): VisionRuleKeyEntity | null {
  const id = readNumber(keyEntity?.id, 0);
  if (!Number.isInteger(id) || id <= 0) {
    return null;
  }

  const imageBase64 = readString(keyEntity?.image?.base64).trim();
  const description = readString(keyEntity?.description).trim();
  if (!imageBase64 && !description) {
    return null;
  }

  return {
    id,
    image: imageBase64
      ? {
          base64: imageBase64,
          content_type: readString(keyEntity?.image?.content_type).trim() || undefined,
        }
      : undefined,
    description: description || undefined,
  };
}

function normalizeVisionRule(rule: Partial<VisionRule> | null | undefined, index: number): VisionRule {
  return {
    id: readString(rule?.id) || `vision-rule-${index + 1}`,
    name: readString(rule?.name),
    enabled: readBoolean(rule?.enabled),
    camera_device_id: readString(rule?.camera_device_id),
    recognition_enabled: readBoolean(rule?.recognition_enabled),
    rtsp_source: {
      url: readString(rule?.rtsp_source?.url),
    },
    entity_selector: {
      kind: readString(rule?.entity_selector?.kind) || 'label',
      value: readString(rule?.entity_selector?.value),
    },
    behavior: readString(rule?.behavior),
    key_entities: asArray(rule?.key_entities)
      .map((item) => normalizeVisionRuleKeyEntity(item))
      .filter((item): item is VisionRuleKeyEntity => item !== null),
    zone: {
      x: readNumber(rule?.zone?.x, 0.2),
      y: readNumber(rule?.zone?.y, 0.2),
      width: readNumber(rule?.zone?.width, 0.4),
      height: readNumber(rule?.zone?.height, 0.4),
    },
    stay_threshold_seconds: Math.max(1, readNumber(rule?.stay_threshold_seconds, 5)),
  };
}

export function normalizeVisionConfig(
  config: Partial<VisionCapabilityConfig> | null | undefined,
): VisionCapabilityConfig {
  return {
    service_ws_url: readString(config?.service_ws_url),
    model_name: readString(config?.model_name) || undefined,
    recognition_enabled: readBoolean(config?.recognition_enabled),
    event_capture_retention_hours: Math.max(1, readNumber(config?.event_capture_retention_hours, 168)),
    rules: asArray(config?.rules).map((rule, index) => normalizeVisionRule(rule, index)),
    updated_at: readString(config?.updated_at),
  };
}

export function normalizeVisionEntityCatalog(
  catalog: Partial<VisionEntityCatalog> | null | undefined,
): VisionEntityCatalog | null {
  if (!catalog) {
    return null;
  }
  return {
    service_ws_url: readString(catalog.service_ws_url),
    schema_version: readString(catalog.schema_version),
    service_version: readString(catalog.service_version) || undefined,
    model_name: readString(catalog.model_name) || undefined,
    fetched_at: readString(catalog.fetched_at),
    entities: asArray(catalog.entities)
      .map((entity) => normalizeVisionEntityDescriptor(entity))
      .filter((entity): entity is VisionEntityDescriptor => entity !== null),
  };
}

export function normalizeCapabilityDetail(detail: CapabilityDetail): CapabilityDetail {
  if (!detail.vision) {
    return detail;
  }
  return {
    ...detail,
    vision: {
      ...detail.vision,
      config: normalizeVisionConfig(detail.vision.config),
      runtime: {
        ...detail.vision.runtime,
        runtime:
          detail.vision.runtime?.runtime && typeof detail.vision.runtime.runtime === 'object'
            ? detail.vision.runtime.runtime
            : undefined,
      },
      catalog: normalizeVisionEntityCatalog(detail.vision.catalog),
      recent_events: asArray(detail.vision.recent_events),
    },
  };
}

export function createVisionRule(cameras: DeviceView[], index: number): VisionRule {
  return {
    id: `stay-zone-${Date.now().toString(36)}-${index.toString(36)}`,
    name: `Stay Rule ${index + 1}`,
    enabled: true,
    camera_device_id: cameras[0]?.device.id ?? '',
    recognition_enabled: true,
    rtsp_source: {
      url: cameraRTSPSourceURL(cameras[0]),
    },
    entity_selector: {
      kind: 'label',
      value: '',
    },
    behavior: '',
    key_entities: [],
    zone: {
      x: 0.2,
      y: 0.2,
      width: 0.4,
      height: 0.4,
    },
    stay_threshold_seconds: 5,
  };
}

export function cameraLabel(device: DeviceView) {
  return `${device.device.name} (${device.device.id})`;
}

export function cameraRTSPSourceURL(device: DeviceView | null | undefined) {
  const state = readRecord(device?.state?.state);
  const metadata = readRecord(device?.device?.metadata);
  return readString(state?.rtsp_url) || readString(metadata?.rtsp_url);
}

export function capabilityDisplayName(
  capability: Pick<CapabilitySummary, 'id' | 'name'> | string | null | undefined,
) {
  const capabilityId = typeof capability === 'string' ? capability : capability?.id;
  if (capabilityId === 'vision_entity_stay_zone') {
    return 'Recognition';
  }
  if (typeof capability === 'string') {
    return capability;
  }
  return capability?.name ?? '';
}

export function summaryNumber(capability: CapabilitySummary | null | undefined, key: string) {
  const value = capability?.summary?.[key];
  return typeof value === 'number' ? value : 0;
}

export function summaryString(capability: CapabilitySummary | null | undefined, key: string) {
  const value = capability?.summary?.[key];
  return typeof value === 'string' ? value : '';
}
