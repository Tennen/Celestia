import { fireEvent, render } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useStreamSession } from '../../hooks/useStreamSession';
import type { DeviceView, VisionZoneBox } from '../../lib/types';
import { ZoneBoxEditor } from './ZoneBoxEditor';

vi.mock('../../hooks/useStreamSession', () => ({
  useStreamSession: vi.fn(),
}));

const mockedUseStreamSession = vi.mocked(useStreamSession);

function buildCameraDevice(capabilities: string[] = ['stream']): DeviceView {
  return {
    device: {
      id: 'camera-1',
      plugin_id: 'hikvision',
      vendor_device_id: 'vendor-camera-1',
      kind: 'camera_like',
      name: 'Entry Camera',
      online: true,
      capabilities,
    },
    state: {
      device_id: 'camera-1',
      plugin_id: 'hikvision',
      ts: '2026-04-10T00:00:00Z',
      state: {},
    },
  };
}

const zone: VisionZoneBox = {
  x: 0.1,
  y: 0.2,
  width: 0.3,
  height: 0.4,
};

describe('ZoneBoxEditor', () => {
  const startStream = vi.fn().mockResolvedValue(undefined);
  const stopStream = vi.fn().mockResolvedValue(undefined);
  const videoRef = { current: null as HTMLVideoElement | null };

  beforeEach(() => {
    startStream.mockClear();
    stopStream.mockClear();
    videoRef.current = null;
    mockedUseStreamSession.mockReturnValue({
      state: 'active',
      errorMessage: null,
      videoRef,
      startStream,
      stopStream,
    });
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('starts and stops live preview for the selected camera', () => {
    const { unmount } = render(<ZoneBoxEditor cameraDevice={buildCameraDevice()} value={zone} onChange={vi.fn()} />);

    expect(mockedUseStreamSession).toHaveBeenCalledWith('camera-1');
    expect(startStream).toHaveBeenCalledTimes(1);

    unmount();

    expect(stopStream).toHaveBeenCalledTimes(1);
  });

  it('updates the zone surface aspect ratio from the loaded video frame', () => {
    const { container } = render(<ZoneBoxEditor cameraDevice={buildCameraDevice()} value={zone} onChange={vi.fn()} />);
    const surface = container.querySelector('.zone-editor__surface');
    const video = container.querySelector('video');

    if (!(surface instanceof HTMLDivElement) || !(video instanceof HTMLVideoElement)) {
      throw new Error('zone editor surface or video not found');
    }

    Object.defineProperty(video, 'videoWidth', { configurable: true, value: 1280 });
    Object.defineProperty(video, 'videoHeight', { configurable: true, value: 720 });

    fireEvent(video, new Event('loadedmetadata'));

    expect(surface.style.aspectRatio).toBe('1280 / 720');
  });

  it('does not auto-start preview for cameras without stream capability', () => {
    render(<ZoneBoxEditor cameraDevice={buildCameraDevice(['discover'])} value={zone} onChange={vi.fn()} />);

    expect(startStream).not.toHaveBeenCalled();
  });
});
