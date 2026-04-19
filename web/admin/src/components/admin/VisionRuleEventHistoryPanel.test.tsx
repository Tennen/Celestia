import { act, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
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
    key_entities: [],
    zone: { x: 0.1, y: 0.2, width: 0.3, height: 0.4 },
    stay_threshold_seconds: 5,
    ...overrides,
  };
}

function buildEvent(id: string, label: string, keyEntityId?: number): EventRecord {
  return {
    id,
    type: 'device.event.occurred',
    plugin_id: 'hikvision',
    device_id: 'hikvision:camera:entry-1',
    ts: '2026-04-12T08:00:00Z',
    payload: {
      dwell_seconds: 5,
      ...(keyEntityId ? { key_entity_id: keyEntityId } : {}),
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

  it('filters persisted events by key entity id', async () => {
    const user = userEvent.setup();
    fetchVisionRuleEvents.mockResolvedValue([
      buildEvent('evt-feeder-101', 'Cat', 101),
      buildEvent('evt-feeder-102', 'Dog', 102),
    ]);

    render(
      <VisionRuleEventHistoryPanel
        onBack={vi.fn()}
        onError={vi.fn()}
        rule={buildRule({
          key_entities: [
            { id: 101, name: 'Feeder Cat', description: 'orange tabby with a blue collar' },
            { id: 102, name: 'Midnight', description: 'solid black cat with a white chest' },
          ],
        })}
        updatedAtKey="2026-04-12T08:00:00Z"
      />,
    );

    expect(await screen.findByTitle('Cat')).toBeTruthy();

    await user.click(screen.getByRole('button', { name: /show filters/i }));
    expect(screen.getByRole('option', { name: 'Feeder Cat' })).toBeTruthy();
    await user.selectOptions(screen.getByLabelText(/filter rule history by key entity/i), '101');

    expect(screen.getByText(/1\/2/i)).toBeTruthy();
    expect(screen.getByText('#101')).toBeTruthy();
  });

  it('opens the app confirmation dialog before deleting a persisted event', async () => {
    const user = userEvent.setup();
    fetchVisionRuleEvents.mockResolvedValue([buildEvent('evt-feeder', 'Cat')]);
    deleteVisionRuleEvent.mockResolvedValue(undefined);

    render(
      <VisionRuleEventHistoryPanel
        onBack={vi.fn()}
        onError={vi.fn()}
        rule={buildRule()}
        updatedAtKey="2026-04-12T08:00:00Z"
      />,
    );

    expect(await screen.findByTitle('Cat')).toBeTruthy();

    await user.click(screen.getByRole('button', { name: 'Delete Event' }));
    const cancelDialog = screen.getByRole('alertdialog', { name: 'Delete Recognition Event' });
    expect(deleteVisionRuleEvent).not.toHaveBeenCalled();

    await user.click(within(cancelDialog).getByRole('button', { name: 'Keep Event' }));
    expect(screen.queryByRole('alertdialog', { name: 'Delete Recognition Event' })).toBeNull();
    expect(screen.getByTitle('Cat')).toBeTruthy();

    await user.click(screen.getByRole('button', { name: 'Delete Event' }));
    const confirmDialog = screen.getByRole('alertdialog', { name: 'Delete Recognition Event' });
    await user.click(within(confirmDialog).getByRole('button', { name: 'Delete Event' }));

    expect(deleteVisionRuleEvent).toHaveBeenCalledWith('feeder-zone', 'evt-feeder');
    await waitFor(() => expect(screen.queryByTitle('Cat')).toBeNull());
  });
});
