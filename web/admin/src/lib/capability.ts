import type { CapabilitySummary, DeviceView, VisionCapabilityConfig, VisionRule } from './types';

export function cloneVisionConfig<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}

export function defaultVisionConfig(): VisionCapabilityConfig {
  return {
    service_url: '',
    recognition_enabled: false,
    event_capture_retention_hours: 168,
    rules: [],
    updated_at: '',
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
      url: '',
    },
    entity_selector: {
      kind: 'label',
      value: '',
    },
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

export function summaryNumber(capability: CapabilitySummary | null | undefined, key: string) {
  const value = capability?.summary?.[key];
  return typeof value === 'number' ? value : 0;
}

export function summaryString(capability: CapabilitySummary | null | undefined, key: string) {
  const value = capability?.summary?.[key];
  return typeof value === 'string' ? value : '';
}
