import { buildCommandSuggestions } from './command-suggestions';
import type {
  Automation,
  AutomationAction,
  AutomationCondition,
  AutomationConditionType,
  AutomationMatchOperator,
  DeviceControl,
  DeviceControlOption,
  DeviceStateDescriptor,
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

function stateDescriptorMap(device: DeviceView | null | undefined): Record<string, DeviceStateDescriptor> {
  const raw = device?.device.metadata?.state_descriptors;
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) {
    return {};
  }
  return raw as Record<string, DeviceStateDescriptor>;
}

export function buildStateKeyOptions(device: DeviceView | null | undefined) {
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

export function buildStateValueOptions(device: DeviceView | null | undefined, stateKey: string): DeviceControlOption[] {
  const options = [...(stateDescriptorMap(device)[stateKey]?.options ?? [])];
  const current = device?.state.state?.[stateKey];
  if (current === null || current === undefined) {
    return options;
  }
  const currentValue = String(current);
  if (!currentValue) {
    return options;
  }
  if (options.some((option) => option.value === currentValue)) {
    return options;
  }
  return [{ value: currentValue, label: currentValue }, ...options];
}

export function coerceStateOptionValue(
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

export function coerceStateOptionValues(
  device: DeviceView | null | undefined,
  stateKey: string,
  rawValues: string[],
): unknown[] {
  return rawValues.map((item) => coerceStateOptionValue(device, stateKey, item));
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

function conditionStateValue(device: DeviceView | null | undefined, stateKey: string) {
  return stateKey ? device?.state.state?.[stateKey] : '';
}

export function getConditionType(condition: AutomationCondition): AutomationConditionType {
  if (condition.type === 'state_changed' || condition.type === 'current_state') {
    return condition.type;
  }
  return condition.from || condition.to ? 'state_changed' : 'current_state';
}

export function getStateChangedConditionDeviceId(automation: Automation) {
  const conditions = automation.conditions ?? [];
  const eventCondition = conditions.find((condition) => getConditionType(condition) === 'state_changed');
  return eventCondition?.device_id || conditions[0]?.device_id || '';
}

export function createDefaultCondition(
  devices: DeviceView[],
  options?: {
    deviceId?: string;
    type?: AutomationConditionType;
  },
): AutomationCondition {
  const selectedDevice = findDevice(devices, options?.deviceId ?? '') ?? devices[0] ?? null;
  const stateKey = buildStateKeyOptions(selectedDevice)[0]?.value ?? '';
  const value = conditionStateValue(selectedDevice, stateKey);
  const type = options?.type ?? 'state_changed';
  if (type === 'state_changed') {
    return {
      type,
      device_id: selectedDevice?.device.id ?? '',
      state_key: stateKey,
      from: {
        operator: 'not_equals',
        value,
      },
      to: {
        operator: 'equals',
        value,
      },
    };
  }
  return {
    type,
    device_id: selectedDevice?.device.id ?? '',
    state_key: stateKey,
    match: {
      operator: 'equals',
      value,
    },
  };
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
  if (Array.isArray(value)) {
    return value.map((item) => formatStateValueInput(item)).join(', ');
  }
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
  return operator === 'equals' || operator === 'not_equals' || operator === 'in' || operator === 'not_in';
}

export function operatorAllowsMultipleValues(operator: AutomationMatchOperator) {
  return operator === 'in' || operator === 'not_in';
}

export function coerceMatchValueForOperator(operator: AutomationMatchOperator, value: unknown): unknown {
  if (operatorAllowsMultipleValues(operator)) {
    if (Array.isArray(value)) {
      return value;
    }
    if (value === null || value === undefined || value === '') {
      return [];
    }
    return [value];
  }
  if (Array.isArray(value)) {
    return value[0] ?? '';
  }
  return value;
}

export function parseStateValuesInput(raw: string): unknown[] {
  const trimmed = raw.trim();
  if (!trimmed) {
    return [];
  }
  if (trimmed.startsWith('[') && trimmed.endsWith(']')) {
    try {
      const parsed = JSON.parse(trimmed) as unknown;
      if (Array.isArray(parsed)) {
        return parsed;
      }
    } catch {
      return [trimmed];
    }
  }
  return trimmed
    .split(',')
    .map((item) => parseStateValueInput(item))
    .filter((item) => item !== '');
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
  const defaultCondition = createDefaultCondition(devices);

  let actionDevice = devices.find((device) => buildActionTemplates(device).length > 0) ?? devices[0] ?? null;
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

  return {
    id: '',
    name: '',
    enabled: true,
    condition_logic: 'all',
    conditions: [defaultCondition],
    time_window: null,
    actions: [action],
    last_triggered_at: null,
    last_run_status: 'idle',
    last_error: '',
    created_at: '',
    updated_at: '',
  };
}

export const transitionFromOperators: AutomationMatchOperator[] = ['any', 'equals', 'not_equals', 'in', 'not_in', 'exists', 'missing'];
export const stateOperators: AutomationMatchOperator[] = ['equals', 'not_equals', 'in', 'not_in', 'exists', 'missing'];

export function countStateChangedConditions(conditions: AutomationCondition[] | undefined): number {
  return (conditions ?? []).filter((condition) => getConditionType(condition) === 'state_changed').length;
}
