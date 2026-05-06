import { describe, expect, it } from 'vitest';
import type { DeviceView } from './types';
import {
  buildWorkflowActionTemplates,
  buildWorkflowStateKeyOptions,
  buildWorkflowStateValueOptions,
  coerceWorkflowStateOptionValue,
  workflowNodeDataWithDeviceDefaults,
} from './workflow-node-options';

function buildHaierWasher(): DeviceView {
  return {
    device: {
      id: 'haier:wash-1',
      plugin_id: 'haier',
      vendor_device_id: 'wash-1',
      kind: 'washer',
      name: 'Laundry Washer',
      online: true,
      capabilities: ['start'],
      metadata: {
        state_descriptors: {
          machine_state: {
            label: 'Machine Status',
            options: [
              { value: 'idle', label: 'Idle' },
              { value: 'running', label: 'Running' },
            ],
          },
          hidden_debug: {
            label: 'Debug',
            hidden: true,
          },
        },
      },
    },
    state: {
      device_id: 'haier:wash-1',
      plugin_id: 'haier',
      ts: '2026-05-06T00:00:00Z',
      state: {
        machine_state: 'running',
        hidden_debug: 'raw',
      },
    },
    controls: [
      {
        id: 'start',
        kind: 'action',
        label: 'Start cycle',
        visible: true,
        command: {
          action: 'start',
          params: { program: 'cotton' },
        },
      },
    ],
  };
}

describe('workflow node option helpers', () => {
  it('preserves state key and value display labels from descriptors', () => {
    const device = buildHaierWasher();

    expect(buildWorkflowStateKeyOptions(device)).toEqual([
      { value: 'machine_state', label: 'Machine Status (machine_state)' },
    ]);
    expect(buildWorkflowStateValueOptions(device, 'machine_state')).toEqual([
      { value: 'idle', label: 'Idle' },
      { value: 'running', label: 'Running' },
    ]);
  });

  it('stores typed raw values behind displayed state options', () => {
    const device = buildHaierWasher();

    expect(coerceWorkflowStateOptionValue(device, 'machine_state', 'running')).toBe('running');
    expect(
      workflowNodeDataWithDeviceDefaults('device_state_changed', {}, [device]),
    ).toMatchObject({
      device_id: 'haier:wash-1',
      state_key: 'machine_state',
      from: { operator: 'not_equals', value: 'running' },
      to: { operator: 'equals', value: 'running' },
    });
  });

  it('restores device command behavior templates from controls', () => {
    const device = buildHaierWasher();

    expect(buildWorkflowActionTemplates(device)).toContainEqual({
      key: 'control:start',
      label: 'Control · Start cycle',
      action: 'start',
      params: { program: 'cotton' },
    });
    expect(workflowNodeDataWithDeviceDefaults('device_command', {}, [device])).toMatchObject({
      device_id: 'haier:wash-1',
      action: 'start',
      params: { program: 'cotton' },
    });
  });
});
