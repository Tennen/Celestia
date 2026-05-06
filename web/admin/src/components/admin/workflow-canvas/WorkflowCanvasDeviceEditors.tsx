import { Plus } from 'lucide-react';
import { Button } from '../../ui/button';
import { Input } from '../../ui/input';
import { Textarea } from '../../ui/textarea';
import { Field, FieldGrid, SelectField } from '../AgentFormFields';
import type { AgentWorkflowNode } from '../../../lib/agent-workflow';
import { updateWorkflowNodeData } from '../../../lib/workflow-canvas';
import {
  buildWorkflowActionTemplates,
  buildWorkflowDeviceOptions,
  buildWorkflowStateKeyOptions,
  buildWorkflowStateValueOptions,
  coerceWorkflowMatchValueForOperator,
  coerceWorkflowStateOptionValue,
  coerceWorkflowStateOptionValues,
  defaultWorkflowVoiceDevice,
  findWorkflowDevice,
  formatWorkflowStateValueInput,
  operatorAllowsWorkflowValues,
  operatorNeedsWorkflowValue,
  parseWorkflowStateValueInput,
  parseWorkflowStateValuesInput,
  workflowNodeDataWithDeviceDefaults,
  workflowStateOperators,
  workflowStateValue,
  workflowTransitionFromOperators,
  workflowVoiceDevices,
  type WorkflowMatchOperator,
  type WorkflowStateMatch,
} from '../../../lib/workflow-node-options';
import type { DeviceView } from '../../../lib/types';
import { cn } from '../../../lib/utils';

type InspectorOption = Array<{ value: string; label: string }>;

export function DeviceStateChangedNodeEditor(props: { node: AgentWorkflowNode; devices: DeviceView[]; onChange: (node: AgentWorkflowNode) => void }) {
  const { node, devices, onChange } = props;
  const device = findWorkflowDevice(devices, String(node.data?.device_id ?? ''));
  const stateKey = String(node.data?.state_key ?? '');
  const stateKeyOptions = [{ value: '', label: 'Select State Key' }, ...buildWorkflowStateKeyOptions(device)];
  const updateTarget = (device_id: string, state_key?: string) => {
    const nextDevice = findWorkflowDevice(devices, device_id);
    const nextStateKey = state_key ?? buildWorkflowStateKeyOptions(nextDevice)[0]?.value ?? '';
    const fallback = workflowStateValue(nextDevice, nextStateKey);
    onChange(
      updateWorkflowNodeData(node, {
        device_id,
        state_key: nextStateKey,
        from: stateMatchWithFallback(node.data?.from, fallback, 'not_equals'),
        to: stateMatchWithFallback(node.data?.to, fallback, 'equals'),
      }),
    );
  };
  return (
    <div className="stack">
      <FieldGrid>
        <SelectField label="Device" value={String(node.data?.device_id ?? '')} options={buildWorkflowDeviceOptions(devices)} onChange={(device_id) => updateTarget(device_id)} />
        <SelectField label="State Key" value={stateKey} options={stateKeyOptions} onChange={(state_key) => updateTarget(String(node.data?.device_id ?? ''), state_key)} />
      </FieldGrid>
      <StateMatchEditor
        title="From"
        device={device}
        stateKey={stateKey}
        match={asMatch(node.data?.from, { operator: 'not_equals', value: workflowStateValue(device, stateKey) })}
        operators={workflowTransitionFromOperators}
        onChange={(from) => onChange(updateWorkflowNodeData(node, { from }))}
      />
      <StateMatchEditor
        title="To"
        device={device}
        stateKey={stateKey}
        match={asMatch(node.data?.to, { operator: 'equals', value: workflowStateValue(device, stateKey) })}
        operators={workflowStateOperators}
        onChange={(to) => onChange(updateWorkflowNodeData(node, { to }))}
      />
    </div>
  );
}

export function DeviceStateIsNodeEditor(props: { node: AgentWorkflowNode; devices: DeviceView[]; onChange: (node: AgentWorkflowNode) => void }) {
  const { node, devices, onChange } = props;
  const device = findWorkflowDevice(devices, String(node.data?.device_id ?? ''));
  const stateKey = String(node.data?.state_key ?? '');
  const stateKeyOptions = [{ value: '', label: 'Select State Key' }, ...buildWorkflowStateKeyOptions(device)];
  const updateTarget = (device_id: string, state_key?: string) => {
    const nextDevice = findWorkflowDevice(devices, device_id);
    const nextStateKey = state_key ?? buildWorkflowStateKeyOptions(nextDevice)[0]?.value ?? '';
    const fallback = workflowStateValue(nextDevice, nextStateKey);
    onChange(
      updateWorkflowNodeData(node, {
        device_id,
        state_key: nextStateKey,
        match: stateMatchWithFallback(node.data?.match, fallback, 'equals'),
      }),
    );
  };
  return (
    <div className="stack">
      <FieldGrid>
        <SelectField label="Device" value={String(node.data?.device_id ?? '')} options={buildWorkflowDeviceOptions(devices)} onChange={(device_id) => updateTarget(device_id)} />
        <SelectField label="State Key" value={stateKey} options={stateKeyOptions} onChange={(state_key) => updateTarget(String(node.data?.device_id ?? ''), state_key)} />
      </FieldGrid>
      <StateMatchEditor
        title="Match"
        device={device}
        stateKey={stateKey}
        match={asMatch(node.data?.match, { operator: 'equals', value: workflowStateValue(device, stateKey) })}
        operators={workflowStateOperators}
        onChange={(match) => onChange(updateWorkflowNodeData(node, { match }))}
      />
    </div>
  );
}

export function DeviceCommandNodeEditor(props: { node: AgentWorkflowNode; devices: DeviceView[]; onChange: (node: AgentWorkflowNode) => void }) {
  const { node, devices, onChange } = props;
  const device = findWorkflowDevice(devices, String(node.data?.device_id ?? ''));
  const templates = buildWorkflowActionTemplates(device);
  const applyTemplate = (templateKey: string) => {
    const template = templates.find((item) => item.key === templateKey) ?? null;
    if (!template) return;
    onChange(updateWorkflowNodeData(node, { action: template.action, params: template.params }));
  };
  return (
    <div className="stack">
      <FieldGrid>
        <SelectField
          label="Device"
          value={String(node.data?.device_id ?? '')}
          options={buildWorkflowDeviceOptions(devices)}
          onChange={(device_id) =>
            onChange({
              ...node,
              data: workflowNodeDataWithDeviceDefaults('device_command', { ...(node.data ?? {}), device_id }, devices),
            })
          }
        />
        <SelectField
          label="Behavior"
          value=""
          options={[
            { value: '', label: 'Manual action / keep current' },
            ...templates.map((template) => ({ value: template.key, label: template.label })),
          ]}
          onChange={applyTemplate}
        />
        <Field label="Action" value={String(node.data?.action ?? '')} onChange={(action) => onChange(updateWorkflowNodeData(node, { action }))} />
      </FieldGrid>
      <JSONField label="Params JSON" value={asRecord(node.data?.params)} onChange={(params) => onChange(updateWorkflowNodeData(node, { params }))} />
    </div>
  );
}

export function AgentFunctionNodeEditor(props: {
  node: AgentWorkflowNode;
  wecomOptions: InspectorOption;
  devices: DeviceView[];
  onChange: (node: AgentWorkflowNode) => void;
}) {
  const { node, wecomOptions, devices, onChange } = props;
  const touchpoints = asTouchpoints(node.data?.touchpoints);
  const voiceDeviceOptions = buildWorkflowDeviceOptions(workflowVoiceDevices(devices));
  const updateTouchpoints = (next: Array<Record<string, unknown>>) => onChange(updateWorkflowNodeData(node, { touchpoints: next }));
  return (
    <div className="stack">
      <label className="stack text-sm font-medium">
        <span>Input</span>
        <Textarea
          value={String(node.data?.input ?? '')}
          onChange={(event) => onChange(updateWorkflowNodeData(node, { input: event.target.value }))}
          placeholder="Project input, slash command, or Agent prompt. Upstream text is used when empty."
        />
      </label>
      <Field label="Session ID" value={String(node.data?.session_id ?? '')} onChange={(session_id) => onChange(updateWorkflowNodeData(node, { session_id }))} />
      <div className="button-row">
        <Button variant="secondary" onClick={() => updateTouchpoints([...touchpoints, { type: 'wecom', to_user: '' }])}>
          <Plus className="mr-2 h-4 w-4" />
          Add WeCom
        </Button>
        <Button variant="secondary" onClick={() => updateTouchpoints([...touchpoints, { type: 'device', device_id: defaultWorkflowVoiceDevice(devices), action: 'push_voice_message', params: {} }])}>
          <Plus className="mr-2 h-4 w-4" />
          Add Device
        </Button>
      </div>
      {touchpoints.map((touchpoint, index) => (
        <div key={`${touchpoint.type}-${index}`} className="workflow-canvas__source">
          <SelectField
            label="Touchpoint"
            value={String(touchpoint.type ?? 'none')}
            options={[
              { value: 'none', label: 'None' },
              { value: 'wecom', label: 'WeCom' },
              { value: 'device', label: 'Device' },
            ]}
            onChange={(type) => updateTouchpoints(touchpoints.map((item, current) => (current === index ? { ...item, type } : item)))}
          />
          {touchpoint.type === 'wecom' ? (
            <SelectField
              label="Recipient"
              value={String(touchpoint.to_user ?? '')}
              options={wecomOptions}
              onChange={(to_user) => updateTouchpoints(touchpoints.map((item, current) => (current === index ? { ...item, to_user } : item)))}
            />
          ) : null}
          {touchpoint.type === 'device' ? (
            <>
              <SelectField
                label="Device"
                value={String(touchpoint.device_id ?? '')}
                options={voiceDeviceOptions}
                onChange={(device_id) => updateTouchpoints(touchpoints.map((item, current) => (current === index ? { ...item, device_id } : item)))}
              />
              <Field
                label="Action"
                value={String(touchpoint.action ?? 'push_voice_message')}
                onChange={(action) => updateTouchpoints(touchpoints.map((item, current) => (current === index ? { ...item, action } : item)))}
              />
              <Field
                label="Volume"
                value={textParam(asRecord(touchpoint.params).volume)}
                placeholder="optional"
                onChange={(volume) =>
                  updateTouchpoints(
                    touchpoints.map((item, current) =>
                      current === index
                        ? {
                            ...item,
                            params: {
                              ...asRecord(item.params),
                              volume: numericValue(volume),
                            },
                          }
                        : item,
                    ),
                  )
                }
              />
              <JSONField
                label="Params JSON"
                value={asRecord(touchpoint.params)}
                onChange={(params) => updateTouchpoints(touchpoints.map((item, current) => (current === index ? { ...item, params } : item)))}
              />
            </>
          ) : null}
          <Button variant="danger" onClick={() => updateTouchpoints(touchpoints.filter((_, current) => current !== index))}>
            Remove Touchpoint
          </Button>
        </div>
      ))}
    </div>
  );
}

function StateMatchEditor(props: {
  title: string;
  device: DeviceView | null;
  stateKey: string;
  match: WorkflowStateMatch;
  operators: WorkflowMatchOperator[];
  onChange: (match: WorkflowStateMatch) => void;
}) {
  return (
    <div className="workflow-canvas__source">
      <SelectField
        label={props.title}
        value={props.match.operator}
        options={props.operators.map((operator) => ({ value: operator, label: workflowOperatorLabel(operator) }))}
        onChange={(operator) =>
          props.onChange({
            ...props.match,
            operator: operator as WorkflowMatchOperator,
            value: coerceWorkflowMatchValueForOperator(operator as WorkflowMatchOperator, props.match.value),
          })
        }
      />
      <WorkflowStateValueField
        device={props.device}
        stateKey={props.stateKey}
        operator={props.match.operator}
        value={props.match.value}
        onChange={(value) => props.onChange({ ...props.match, value })}
      />
    </div>
  );
}

function WorkflowStateValueField(props: {
  device: DeviceView | null;
  stateKey: string;
  operator: WorkflowMatchOperator;
  value: unknown;
  onChange: (value: unknown) => void;
}) {
  if (!operatorNeedsWorkflowValue(props.operator)) {
    return <Input value="" placeholder="No value required" disabled />;
  }

  const multiple = operatorAllowsWorkflowValues(props.operator);
  const options = buildWorkflowStateValueOptions(props.device, props.stateKey);
  const selectedValues = Array.isArray(props.value) ? props.value.map((item) => String(item)) : [];
  if (options.length > 0) {
    if (multiple) {
      return (
        <div className="form-choice-list">
          {options.map((option) => {
            const checked = selectedValues.includes(option.value);
            return (
              <button
                key={`${props.stateKey}-${option.value}`}
                type="button"
                className={cn('form-choice', checked && 'is-selected')}
                onClick={() => {
                  const nextValues = checked
                    ? selectedValues.filter((item) => item !== option.value)
                    : [...selectedValues, option.value];
                  props.onChange(coerceWorkflowStateOptionValues(props.device, props.stateKey, nextValues));
                }}
              >
                <span className={cn('form-choice__dot', checked && 'is-selected')} />
                <span className="form-choice__label">{option.label}</span>
              </button>
            );
          })}
        </div>
      );
    }

    return (
      <select
        className="h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        value={String(props.value ?? '')}
        onChange={(event) => props.onChange(coerceWorkflowStateOptionValue(props.device, props.stateKey, event.target.value))}
      >
        {options.map((option) => (
          <option key={`${props.stateKey}-${option.value}`} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    );
  }

  return (
    <Input
      value={formatWorkflowStateValueInput(props.value)}
      placeholder={multiple ? 'A, B, C' : 'state value'}
      onChange={(event) =>
        props.onChange(multiple ? parseWorkflowStateValuesInput(event.target.value) : parseWorkflowStateValueInput(event.target.value))
      }
    />
  );
}

function JSONField(props: { label: string; value: Record<string, unknown>; onChange: (value: Record<string, unknown>) => void }) {
  return (
    <label className="stack text-sm font-medium">
      <span>{props.label}</span>
      <Textarea value={JSON.stringify(props.value, null, 2)} onChange={(event) => props.onChange(parseJSONObject(event.target.value, props.value))} />
    </label>
  );
}

function asMatch(value: unknown, fallback: WorkflowStateMatch): WorkflowStateMatch {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    return fallback;
  }
  const raw = value as { operator?: unknown; value?: unknown };
  return {
    operator: typeof raw.operator === 'string' ? (raw.operator as WorkflowMatchOperator) : fallback.operator,
    value: raw.value ?? fallback.value,
  };
}

function stateMatchWithFallback(value: unknown, fallbackValue: unknown, fallbackOperator: WorkflowMatchOperator): WorkflowStateMatch {
  const match = asMatch(value, { operator: fallbackOperator, value: fallbackValue });
  return {
    operator: match.operator,
    value: operatorNeedsWorkflowValue(match.operator)
      ? coerceWorkflowMatchValueForOperator(match.operator, match.value ?? fallbackValue)
      : match.value,
  };
}

function workflowOperatorLabel(operator: WorkflowMatchOperator) {
  switch (operator) {
    case 'any':
      return 'Any Change';
    case 'equals':
      return 'Equals';
    case 'not_equals':
      return 'Not Equals';
    case 'in':
      return 'In List';
    case 'not_in':
      return 'Not In List';
    case 'exists':
      return 'Exists';
    case 'missing':
      return 'Missing';
    default:
      return operator;
  }
}

function asRecord(value: unknown): Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value) ? (value as Record<string, unknown>) : {};
}

function asTouchpoints(value: unknown): Array<Record<string, unknown>> {
  return Array.isArray(value) ? value.map((item) => asRecord(item)) : [];
}

function textParam(value: unknown) {
  if (typeof value === 'number' && Number.isFinite(value)) return String(value);
  return typeof value === 'string' ? value : '';
}

function numericValue(value: string) {
  if (!value.trim()) return undefined;
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : value;
}

function parseJSONObject(raw: string, fallback: Record<string, unknown>) {
  try {
    const parsed = JSON.parse(raw) as unknown;
    return asRecord(parsed);
  } catch {
    return fallback;
  }
}
