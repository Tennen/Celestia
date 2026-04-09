import { act, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeAll, beforeEach, describe, expect, it } from 'vitest';
import { emptyLoadState } from '../../lib/admin';
import type { DeviceView } from '../../lib/types';
import { useAdminStore } from '../../stores/adminStore';
import { useDeviceStore } from '../../stores/deviceStore';
import { DeviceWorkspace } from './DeviceWorkspace';

const initialAdminState = useAdminStore.getState();
const initialDeviceState = useDeviceStore.getState();

function buildDeviceView(overrides: Partial<DeviceView> = {}): DeviceView {
  const { device: deviceOverrides, state: stateOverrides, controls, ...rest } = overrides;
  return {
    device: {
      id: 'device-1',
      plugin_id: 'xiaomi',
      vendor_device_id: 'vendor-1',
      kind: 'light',
      name: 'Desk Lamp',
      online: true,
      capabilities: ['toggle'],
      ...(deviceOverrides ?? {}),
    },
    state: {
      device_id: 'device-1',
      plugin_id: 'xiaomi',
      ts: '2026-04-09T00:00:00Z',
      state: { power: true },
      ...(stateOverrides ?? {}),
    },
    controls: controls ?? [],
    ...rest,
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

describe('DeviceWorkspace', () => {
  beforeEach(() => {
    act(() => {
      useAdminStore.setState(
        {
          ...initialAdminState,
          ...emptyLoadState(),
          devices: [buildDeviceView()],
          loading: false,
          refreshing: false,
          hasLoaded: true,
          error: null,
        },
        true,
      );
      useDeviceStore.setState(
        {
          ...initialDeviceState,
          selectedDeviceId: 'device-1',
          deviceSearch: '',
        },
        true,
      );
    });
  });

  afterEach(() => {
    act(() => {
      useAdminStore.setState(initialAdminState, true);
      useDeviceStore.setState(initialDeviceState, true);
    });
  });

  it('keeps summary expansion and alias draft when the selected device refreshes', async () => {
    const user = userEvent.setup();
    render(<DeviceWorkspace />);

    const summaryToggle = document.querySelector('button[aria-controls="device-summary-panel"]');
    if (!(summaryToggle instanceof HTMLButtonElement)) {
      throw new Error('summary toggle not found');
    }

    await user.click(summaryToggle);
    expect(screen.getByRole('button', { name: /hide/i })).toBeTruthy();

    await user.click(screen.getByRole('button', { name: 'Edit label for Desk Lamp' }));
    const aliasInput = screen.getByPlaceholderText('Desk Lamp');
    await user.type(aliasInput, 'Kitchen Lamp');
    expect(screen.getByDisplayValue('Kitchen Lamp')).toBeTruthy();

    act(() => {
      useAdminStore.setState({
        devices: [
          buildDeviceView({
            device: {
              id: 'device-1',
              plugin_id: 'xiaomi',
              vendor_device_id: 'vendor-1',
              kind: 'light',
              name: 'Desk Lamp',
              online: false,
              capabilities: ['toggle'],
            },
            state: {
              device_id: 'device-1',
              plugin_id: 'xiaomi',
              ts: '2026-04-09T00:00:10Z',
              state: { power: false },
            },
          }),
        ],
      });
    });

    expect(screen.getByRole('button', { name: /hide/i })).toBeTruthy();
    expect(screen.getByDisplayValue('Kitchen Lamp')).toBeTruthy();
  });
});
