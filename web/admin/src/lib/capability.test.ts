import { describe, expect, it } from 'vitest';
import { cameraRTSPSourceURL, normalizeCapabilityDetail, normalizeVisionEntityCatalog } from './capability';
import type { CapabilityDetail, DeviceView, VisionCapabilityConfig, VisionEntityCatalog } from './types';

describe('capability normalization', () => {
  it('fills missing vision arrays in capability detail payloads', () => {
    const detail = normalizeCapabilityDetail({
      id: 'vision_entity_stay_zone',
      kind: 'vision_entity_stay_zone',
      name: 'Vision Stay Zone Recognition',
      description: 'Vision capability',
      enabled: true,
      status: 'healthy',
      updated_at: '2026-04-09T00:00:00Z',
      vision: {
        config: {
          service_url: 'http://127.0.0.1:8090',
          recognition_enabled: true,
          event_capture_retention_hours: 168,
          updated_at: '2026-04-09T00:00:00Z',
        } as VisionCapabilityConfig,
        runtime: {
          status: 'healthy',
          updated_at: '2026-04-09T00:00:00Z',
        },
        catalog: {
          service_url: 'http://127.0.0.1:8090',
          schema_version: 'v1',
          fetched_at: '2026-04-09T00:00:00Z',
        } as VisionEntityCatalog,
      },
    } as CapabilityDetail);

    expect(detail.vision?.config.rules).toEqual([]);
    expect(detail.vision?.catalog?.entities).toEqual([]);
    expect(detail.vision?.recent_events).toEqual([]);
  });

  it('drops malformed vision entities instead of surfacing undefined values to the UI', () => {
    const catalog = normalizeVisionEntityCatalog({
      service_url: 'http://127.0.0.1:8090',
      schema_version: 'v1',
      fetched_at: '2026-04-09T00:00:00Z',
      entities: [
        { kind: 'label', value: 'cat', display_name: 'Cat' },
        { kind: 'label' } as never,
        null as never,
      ],
    });

    expect(catalog?.entities).toEqual([{ kind: 'label', value: 'cat', display_name: 'Cat' }]);
  });

  it('derives the camera RTSP source from device state before falling back to metadata', () => {
    const device = {
      device: {
        id: 'camera-1',
        plugin_id: 'hikvision',
        vendor_device_id: 'vendor-camera-1',
        kind: 'camera_like',
        name: 'Entry Camera',
        online: true,
        capabilities: ['stream'],
        metadata: {
          rtsp_url: 'rtsp://metadata-user:metadata-pass@camera/metadata',
        },
      },
      state: {
        device_id: 'camera-1',
        plugin_id: 'hikvision',
        ts: '2026-04-10T00:00:00Z',
        state: {
          rtsp_url: 'rtsp://state-user:state-pass@camera/live',
        },
      },
    } as DeviceView;

    expect(cameraRTSPSourceURL(device)).toBe('rtsp://state-user:state-pass@camera/live');
  });
});
