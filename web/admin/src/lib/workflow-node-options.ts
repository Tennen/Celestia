import { buildCommandSuggestions } from './command-suggestions';
import type {
  DeviceControl,
  DeviceControlOption,
  DeviceStateDescriptor,
  DeviceView,
} from './types';

export type WorkflowMatchOperator =
  | 'any'
  | 'equals'
  | 'not_equals'
  | 'in'
  | 'not_in'
  | 'exists'
  | 'missing';

export type WorkflowStateMatch = {
  operator: WorkflowMatchOperator;
  value?: unknown;
};

export type WorkflowActionTemplate = {
  key: string;
  label: string;
  action: string;
  params: Record<string, unknown>;
};

export const workflowTransitionFromOperators: WorkflowMatchOperator[] = [
  'any',
  'equals',
  'not_equals',
  'in',
  'not_in',
  'exists',
  'missing',
];

export const workflowStateOperators: WorkflowMatchOperator[] = [
  'equals',
  'not_equals',
  'in',
  'not_in',
  'exists',
  'missing',
];

function stateDescriptorMap(device: DeviceView | null | undefined): Record<string, DeviceStateDescriptor> {
  const raw = device?.device.metadata?.state_descriptors;
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) {
    return {};
  }
  return raw as Record<string, DeviceStateDescriptor>;
}

export function findWorkflowDevice(devices: DeviceView[], deviceId: string) {
  return devices.find((device) => device.device.id === deviceId) ?? null;
}

export function buildWorkflowDeviceOptions(devices: DeviceView[]) {
  return [
    { value: '', label: 'Select Device' },
    ...devices.map((view) => ({
      value: view.device.id,
      label: view.device.name || view.device.alias || view.device.id,
    })),
  ];
}

export function buildWorkflowStateKeyOptions(device: DeviceView | null | undefined) {
  const descriptors = stateDescriptorMap(device);
  return Object.keys(device?.state?.state ?? {})
    .filter((key) => !descriptors[key]?.hidden)
    .sort((a, b) => {
      const left = descriptors[a]?.label?.trim() || a;
      const right = descriptors[b]?.label?.trim() || b;
      return left.localeCompare(right, 'zh-Hans-CN');
    })
    .map((key) => {
      const label = descriptors[key]?.label?.trim();
      return {
        value: key,
        label: label && label !== key ? `${label} (${key})` : key,
      };
    });
}

export function buildWorkflowStateValueOptions(device: DeviceView | null | undefined, stateKey: string): DeviceControlOption[] {
  const options = [...(stateDescriptorMap(device)[stateKey]?.options ?? [])];
  const current = device?.state.state?.[stateKey];
  if (current === null || current === undefined) {
    return options;
  }
  const currentValue = String(current);
  if (!currentValue || options.some((option) => option.value === currentValue)) {
    return options;
  }
  return [{ value: currentValue, label: currentValue }, ...options];
}

export function workflowStateValue(device: DeviceView | null | undefined, stateKey: string) {
  return stateKey ? device?.state.state?.[stateKey] : '';
}

export function operatorNeedsWorkflowValue(operator: WorkflowMatchOperator) {
  return operator === 'equals' || operator === 'not_equals' || operator === 'in' || operator === 'not_in';
}

export function operatorAllowsWorkflowValues(operator: WorkflowMatchOperator) {
  return operator === 'in' || operator === 'not_in';
}

export function coerceWorkflowStateOptionValue(
  device: DeviceView | null | undefined,
  stateKey: string,
  rawValue: string,
): unknown {
  const current = device?.state.state?.[stateKey];
  if (typeof current === 'number') {
    const parsed = Number(rawValue);
    return Number.isNaN(parsed) ? rawValue : parsed;
  }
  if (typeof current === 'boolean') {
    return rawValue === 'true';
  }
  return rawValue;
}

export function coerceWorkflowStateOptionValues(
  device: DeviceView | null | undefined,
  stateKey: string,
  rawValues: string[],
): unknown[] {
  return rawValues.map((item) => coerceWorkflowStateOptionValue(device, stateKey, item));
}

export function coerceWorkflowMatchValueForOperator(operator: WorkflowMatchOperator, value: unknown): unknown {
  if (operatorAllowsWorkflowValues(operator)) {
    if (Array.isArray(value)) return value;
    if (value === null || value === undefined || value === '') return [];
    return [value];
  }
  if (Array.isArray(value)) {
    return value[0] ?? '';
  }
  return value;
}

function workflowMatchWithDefault(raw: unknown, operator: WorkflowMatchOperator, value: unknown): WorkflowStateMatch {
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) {
    return { operator, value };
  }
  const match = raw as { operator?: unknown; value?: unknown };
  const nextOperator = typeof match.operator === 'string' ? (match.operator as WorkflowMatchOperator) : operator;
  return {
    operator: nextOperator,
    value: operatorNeedsWorkflowValue(nextOperator)
      ? coerceWorkflowMatchValueForOperator(nextOperator, match.value ?? value)
      : match.value,
  };
}

export function parseWorkflowStateValueInput(raw: string): unknown {
  const value = raw.trim();
  if (!value) return '';
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

export function parseWorkflowStateValuesInput(raw: string): unknown[] {
  const trimmed = raw.trim();
  if (!trimmed) return [];
  if (trimmed.startsWith('[') && trimmed.endsWith(']')) {
    try {
      const parsed = JSON.parse(trimmed) as unknown;
      if (Array.isArray(parsed)) return parsed;
    } catch {
      return [trimmed];
    }
  }
  return trimmed
    .split(',')
    .map((item) => parseWorkflowStateValueInput(item))
    .filter((item) => item !== '');
}

export function formatWorkflowStateValueInput(value: unknown): string {
  if (Array.isArray(value)) {
    return value.map((item) => formatWorkflowStateValueInput(item)).join(', ');
  }
  if (typeof value === 'string') return value;
  if (value === null || value === undefined) return '';
  return JSON.stringify(value);
}

function controlActionTemplate(control: DeviceControl): WorkflowActionTemplate | null {
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

export function buildWorkflowActionTemplates(device: DeviceView | null | undefined): WorkflowActionTemplate[] {
  if (!device) return [];
  const templates: WorkflowActionTemplate[] = [];
  const seen = new Set<string>();
  for (const control of device.controls ?? []) {
    const template = controlActionTemplate(control);
    if (!template || seen.has(template.key)) continue;
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

export function workflowVoiceDevices(devices: DeviceView[]) {
  const candidates = devices.filter((device) => device.device.kind === 'speaker' || device.device.capabilities.includes('voice_push'));
  return candidates.length > 0 ? candidates : devices;
}

export function defaultWorkflowVoiceDevice(devices: DeviceView[]) {
  return workflowVoiceDevices(devices)[0]?.device.id ?? '';
}

export function workflowNodeDataWithDeviceDefaults(
  type: string,
  data: Record<string, unknown>,
  devices: DeviceView[],
): Record<string, unknown> {
  if (type === 'device_state_changed' || type === 'device_state_is') {
    const device = findWorkflowDevice(devices, String(data.device_id ?? '')) ?? devices[0] ?? null;
    const stateKey = String(data.state_key ?? '') || buildWorkflowStateKeyOptions(device)[0]?.value || '';
    const value = workflowStateValue(device, stateKey);
    if (type === 'device_state_changed') {
      return {
        ...data,
        device_id: device?.device.id ?? '',
        state_key: stateKey,
        from: workflowMatchWithDefault(data.from, 'not_equals', value),
        to: workflowMatchWithDefault(data.to, 'equals', value),
      };
    }
    return {
      ...data,
      device_id: device?.device.id ?? '',
      state_key: stateKey,
      match: workflowMatchWithDefault(data.match, 'equals', value),
    };
  }
  if (type === 'device_command') {
    const device = findWorkflowDevice(devices, String(data.device_id ?? '')) ?? devices.find((item) => buildWorkflowActionTemplates(item).length > 0) ?? devices[0] ?? null;
    const template = buildWorkflowActionTemplates(device)[0] ?? null;
    return {
      ...data,
      device_id: device?.device.id ?? '',
      action: data.action || template?.action || '',
      params: data.params ?? template?.params ?? {},
    };
  }
  return data;
}
