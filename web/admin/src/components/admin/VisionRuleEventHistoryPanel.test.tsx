import { act, render, screen } from '@testing-library/react';
import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import type { EventRecord, VisionRule } from '../../lib/types';
import { VisionRuleEventHistoryPanel } from './VisionRuleEventHistoryPanel';

const fetchVisionRuleEvents = vi.fn();
const deleteVisionRuleEvent = vi.fn();

vi.mock('../../lib/api', () => ({
  fetchVisionRuleEvents: (...args: unknown[]) => fetchVisionRuleEvents(...args),
  deleteVisionRuleEvent: (...args: unknown[]) => deleteVisionRuleEvent(...args),
  visionCaptureURL: (captureId: string) => `/captures/${captureId}`,
}));

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function buildRule(overrides: Partial<VisionRule> = {}): VisionRule {
  return {
    id: 'feeder-zone',
    name: 'Feeder Zone',
    enabled: true,
    recognition_enabled: true,
    camera_device_id: 'hikvision:camera:entry-1',
    rtsp_source: { url: 'rtsp://camera/live' },
    entity_selector: { kind: 'label', value: 'cat' },
    behavior: '',
    zone: { x: 0.1, y: 0.2, width: 0.3, height: 0.4 },
    stay_threshold_seconds: 5,
    ...overrides,
  };
}

function buildEvent(id: string, label: string): EventRecord {
  return {
    id,
    type: 'device.event.occurred',
    plugin_id: 'hikvision',
    device_id: 'hikvision:camera:entry-1',
    ts: '2026-04-12T08:00:00Z',
    payload: {
      dwell_seconds: 5,
      entities: [{ kind: 'label', value: label.toLowerCase(), display_name: label }],
    },
  };
}

beforeAll(() => {
  class ResizeObserverMock {
    observe() {}
    unobserve() {}
    disconnect() {}
  }

  global.ResizeObserver = ResizeObserverMock as typeof ResizeObserver;
});

describe('VisionRuleEventHistoryPanel', () => {
  beforeEach(() => {
    fetchVisionRuleEvents.mockReset();
    deleteVisionRuleEvent.mockReset();
  });

  it('clears prior rule events immediately when switching rules', async () => {
    const feederRequest = deferred<EventRecord[]>();
    const waterRequest = deferred<EventRecord[]>();
    fetchVisionRuleEvents.mockImplementation((ruleId: string) => {
      if (ruleId === 'feeder-zone') {
        return feederRequest.promise;
      }
      if (ruleId === 'water-zone') {
        return waterRequest.promise;
      }
      return Promise.resolve([]);
    });

    const { rerender } = render(
      <VisionRuleEventHistoryPanel
        onBack={vi.fn()}
        onError={vi.fn()}
        rule={buildRule()}
        updatedAtKey="2026-04-12T08:00:00Z"
      />,
    );

    await act(async () => {
      feederRequest.resolve([buildEvent('evt-feeder', 'Cat')]);
      await feederRequest.promise;
    });

    expect(screen.getByTitle('Cat')).toBeTruthy();

    rerender(
      <VisionRuleEventHistoryPanel
        onBack={vi.fn()}
        onError={vi.fn()}
        rule={buildRule({ id: 'water-zone', name: 'Water Zone', entity_selector: { kind: 'label', value: 'dog' } })}
        updatedAtKey="2026-04-12T08:00:00Z"
      />,
    );

    expect(screen.queryByTitle('Cat')).toBeNull();
    expect(screen.getByText(/loading persisted rule events/i)).toBeTruthy();

    await act(async () => {
      waterRequest.resolve([buildEvent('evt-water', 'Dog')]);
      await waterRequest.promise;
    });

    expect(screen.getByTitle('Dog')).toBeTruthy();
  });
});
