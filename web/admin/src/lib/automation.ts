import { buildCommandSuggestions } from './command-suggestions';
import type {
  Automation,
  AutomationAction,
  AutomationCondition,
  AutomationMatchOperator,
  DeviceControl,
  DeviceView,
} from './types';

export type AutomationActionTemplate = {
  key: string;
  label: string;
  action: string;
  params: Record<string, unknown>;
};

export function cloneAutomation<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}

export function buildStateKeyOptions(device: DeviceView | null | undefined) {
  return Object.keys(device?.state?.state ?? {})
    .sort((a, b) => a.localeCompare(b, 'en'))
    .map((key) => ({ value: key, label: key }));
}

function controlActionTemplate(control: DeviceControl): AutomationActionTemplate | null {
  if (!control.command?.action) {
    return null;
  }
  return {
    key: `control:${control.id}`,
    label: `Control · ${control.label}`,
    action: control.command.action,
    params: { ...(control.command.params ?? {}) },
  };
}

export function buildActionTemplates(device: DeviceView | null | undefined): AutomationActionTemplate[] {
  if (!device) {
    return [];
  }
  const templates: AutomationActionTemplate[] = [];
  const seen = new Set<string>();
  for (const control of device.controls ?? []) {
    const template = controlActionTemplate(control);
    if (!template) continue;
    if (seen.has(template.key)) continue;
    seen.add(template.key);
    templates.push(template);
  }
  for (const suggestion of buildCommandSuggestions(device)) {
    const key = `suggest:${suggestion.action}:${JSON.stringify(suggestion.params ?? {})}`;
    if (seen.has(key)) continue;
    seen.add(key);
    templates.push({
      key,
      label: `Suggested · ${suggestion.label}`,
      action: suggestion.action,
      params: { ...(suggestion.params ?? {}) },
    });
  }
  return templates.sort((a, b) => a.label.localeCompare(b.label, 'en'));
}

export function findDevice(devices: DeviceView[], deviceId: string) {
  return devices.find((device) => device.device.id === deviceId) ?? null;
}

export function parseStateValueInput(raw: string): unknown {
  const value = raw.trim();
  if (!value) {
    return '';
  }
  if (value === 'true') return true;
  if (value === 'false') return false;
  if (/^-?\d+(\.\d+)?$/.test(value)) return Number(value);
  if ((value.startsWith('{') && value.endsWith('}')) || (value.startsWith('[') && value.endsWith(']'))) {
    try {
      return JSON.parse(value) as unknown;
    } catch {
      return value;
    }
  }
  return value;
}

export function formatStateValueInput(value: unknown): string {
  if (typeof value === 'string') {
    return value;
  }
  if (value === null || value === undefined) {
    return '';
  }
  return JSON.stringify(value);
}

export function prettyActionParams(params: Record<string, unknown> | undefined) {
  return JSON.stringify(params ?? {}, null, 2);
}

export function operatorNeedsValue(operator: AutomationMatchOperator) {
  return operator === 'equals' || operator === 'not_equals';
}

export function parseActionParams(raw: string) {
  const trimmed = raw.trim();
  if (!trimmed) {
    return {};
  }
  const parsed = JSON.parse(trimmed) as unknown;
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error('Action params must be a JSON object');
  }
  return parsed as Record<string, unknown>;
}

export function defaultAutomation(devices: DeviceView[]): Automation {
  const triggerDevice = devices[0] ?? null;
  const triggerKeys = buildStateKeyOptions(triggerDevice);
  const triggerKey = triggerKeys[0]?.value ?? '';
  const triggerValue = triggerKey ? triggerDevice?.state.state?.[triggerKey] : '';

  let actionDevice = devices.find((device) => buildActionTemplates(device).length > 0) ?? triggerDevice;
  if (!actionDevice && devices.length > 0) {
    actionDevice = devices[0];
  }
  const actionTemplate = buildActionTemplates(actionDevice)[0] ?? null;
  const action: AutomationAction = {
    device_id: actionDevice?.device.id ?? '',
    label: actionTemplate?.label ?? '',
    action: actionTemplate?.action ?? '',
    params: actionTemplate?.params ?? {},
  };
  const defaultCondition: AutomationCondition | undefined =
    triggerDevice && triggerKey
      ? {
          device_id: triggerDevice.device.id,
          state_key: triggerKey,
          match: {
            operator: 'equals',
            value: triggerValue,
          },
        }
      : undefined;

  return {
    id: '',
    name: '',
    enabled: true,
    trigger: {
      device_id: triggerDevice?.device.id ?? '',
      state_key: triggerKey,
      from: {
        operator: 'not_equals',
        value: triggerValue,
      },
      to: {
        operator: 'equals',
        value: triggerValue,
      },
    },
    condition_logic: 'all',
    conditions: defaultCondition ? [defaultCondition] : [],
    time_window: null,
    actions: [action],
    last_triggered_at: null,
    last_run_status: 'idle',
    last_error: '',
    created_at: '',
    updated_at: '',
  };
}

export const triggerFromOperators: AutomationMatchOperator[] = ['any', 'equals', 'not_equals', 'exists', 'missing'];
export const stateOperators: AutomationMatchOperator[] = ['equals', 'not_equals', 'exists', 'missing'];
