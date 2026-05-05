import { Plus, Trash2 } from 'lucide-react';
import { Button } from '../../ui/button';
import { Textarea } from '../../ui/textarea';
import { Field, FieldGrid, SelectField, ToggleField } from '../AgentFormFields';
import type { AgentWorkflowNode, AgentWorkflowSource } from '../../../lib/agent-workflow';
import { asStringArray, asWorkflowSources, updateWorkflowNodeData } from '../../../lib/workflow-canvas';

type InspectorOption = Array<{ value: string; label: string }>;

export function WorkflowCanvasInspector(props: {
  node: AgentWorkflowNode;
  groups: AgentWorkflowNode[];
  providerOptions: InspectorOption;
  searchProviderOptions: InspectorOption;
  wecomOptions: InspectorOption;
  deviceOptions: InspectorOption;
  onChange: (node: AgentWorkflowNode) => void;
  onDelete: () => void;
}) {
  const { node, groups, providerOptions, searchProviderOptions, wecomOptions, deviceOptions, onChange, onDelete } = props;
  const parentOptions = [
    { value: '', label: 'No Group' },
    ...groups.filter((item) => item.id !== node.id).map((item) => ({ value: item.id, label: item.label || item.id })),
  ];

  return (
    <>
      <Field label="Label" value={node.label ?? ''} onChange={(label) => onChange({ ...node, label })} />
      {node.type !== 'group' ? (
        <SelectField label="Parent Group" value={node.parent_id ?? ''} options={parentOptions} onChange={(parent_id) => onChange({ ...node, parent_id })} />
      ) : null}

      {node.type === 'rss_sources' ? <RSSNodeEditor node={node} onChange={onChange} /> : null}
      {node.type === 'timer' ? <TimerNodeEditor node={node} onChange={onChange} /> : null}
      {node.type === 'device_state_changed' ? <DeviceStateChangedNodeEditor node={node} deviceOptions={deviceOptions} onChange={onChange} /> : null}
      {node.type === 'device_state_is' ? <DeviceStateIsNodeEditor node={node} deviceOptions={deviceOptions} onChange={onChange} /> : null}
      {node.type === 'time_window' ? <TimeWindowNodeEditor node={node} onChange={onChange} /> : null}
      {node.type === 'text' ? <TextNodeEditor node={node} onChange={onChange} /> : null}
      {node.type === 'llm' ? <LLMNodeEditor node={node} providerOptions={providerOptions} onChange={onChange} /> : null}
      {node.type === 'search_provider' ? <SearchNodeEditor node={node} providerOptions={searchProviderOptions} onChange={onChange} /> : null}
      {node.type === 'wecom_output' ? <WeComOutputNodeEditor node={node} wecomOptions={wecomOptions} onChange={onChange} /> : null}
      {node.type === 'device_command' ? <DeviceCommandNodeEditor node={node} deviceOptions={deviceOptions} onChange={onChange} /> : null}
      {node.type === 'agent_function' ? <AgentFunctionNodeEditor node={node} wecomOptions={wecomOptions} deviceOptions={deviceOptions} onChange={onChange} /> : null}
      {node.type === 'group' ? <GroupNodeEditor node={node} onChange={onChange} /> : null}

      <Button variant="danger" onClick={onDelete}>
        <Trash2 className="mr-2 h-4 w-4" />
        Delete Node
      </Button>
    </>
  );
}

function RSSNodeEditor({ node, onChange }: { node: AgentWorkflowNode; onChange: (node: AgentWorkflowNode) => void }) {
  const sources = asWorkflowSources(node.data?.sources);
  const updateSource = (index: number, patch: Partial<AgentWorkflowSource>) => {
    const next = sources.map((source, current) => (current === index ? { ...source, ...patch } : source));
    onChange(updateWorkflowNodeData(node, { sources: next }));
  };
  return (
    <div className="stack">
      <div className="button-row">
        <Button variant="secondary" onClick={() => onChange(updateWorkflowNodeData(node, { sources: [...sources, blankSource()] }))}>
          <Plus className="mr-2 h-4 w-4" />
          Add RSS Source
        </Button>
      </div>
      {sources.map((source, index) => (
        <div key={`${source.id || 'source'}-${index}`} className="workflow-canvas__source">
          <ToggleField label="Enabled" checked={source.enabled !== false} onChange={(enabled) => updateSource(index, { enabled })} />
          <FieldGrid>
            <Field label="Name" value={source.name ?? ''} onChange={(name) => updateSource(index, { name })} />
            <Field label="Category" value={source.category ?? ''} onChange={(category) => updateSource(index, { category })} />
            <Field label="Feed URL" value={source.feed_url ?? ''} onChange={(feed_url) => updateSource(index, { feed_url })} />
            <Field label="Weight" value={String(source.weight ?? 1)} onChange={(weight) => updateSource(index, { weight: Number(weight) || 1 })} />
          </FieldGrid>
          <Button variant="danger" onClick={() => onChange(updateWorkflowNodeData(node, { sources: sources.filter((_, current) => current !== index) }))}>
            Remove Source
          </Button>
        </div>
      ))}
      {sources.length === 0 ? <div className="detail">Add one or more RSS feeds to this node.</div> : null}
    </div>
  );
}

function TimerNodeEditor({ node, onChange }: { node: AgentWorkflowNode; onChange: (node: AgentWorkflowNode) => void }) {
  const schedule = String(node.data?.schedule ?? 'daily');
  const updateTimer = (patch: Record<string, unknown>) => onChange(updateWorkflowNodeData(node, patch));
  return (
    <FieldGrid>
      <SelectField
        label="Schedule"
        value={schedule}
        options={[
          { value: 'daily', label: 'Daily' },
          { value: 'interval', label: 'Interval' },
        ]}
        onChange={(nextSchedule) =>
          onChange(
            updateWorkflowNodeData(node, {
              schedule: nextSchedule,
              at: String(node.data?.at ?? '08:00'),
              interval_seconds: Number(node.data?.interval_seconds ?? 600) || 600,
            }),
          )
        }
      />
      {schedule === 'interval' ? (
        <Field
          label="Interval Seconds"
          value={String(node.data?.interval_seconds ?? 600)}
          onChange={(interval_seconds) => updateTimer({ interval_seconds: Number(interval_seconds) || 600 })}
        />
      ) : (
        <Field label="At" value={String(node.data?.at ?? '08:00')} onChange={(at) => updateTimer({ at })} />
      )}
      <Field
        label="Timezone"
        value={String(node.data?.timezone ?? '')}
        onChange={(timezone) => updateTimer({ timezone })}
      />
    </FieldGrid>
  );
}

function DeviceStateChangedNodeEditor(props: { node: AgentWorkflowNode; deviceOptions: InspectorOption; onChange: (node: AgentWorkflowNode) => void }) {
  const { node, deviceOptions, onChange } = props;
  return (
    <div className="stack">
      <FieldGrid>
        <SelectField label="Device" value={String(node.data?.device_id ?? '')} options={deviceOptions} onChange={(device_id) => onChange(updateWorkflowNodeData(node, { device_id }))} />
        <Field label="State Key" value={String(node.data?.state_key ?? '')} onChange={(state_key) => onChange(updateWorkflowNodeData(node, { state_key }))} />
      </FieldGrid>
      <StateMatchEditor title="From" match={asMatch(node.data?.from, { operator: 'any' })} allowAny onChange={(from) => onChange(updateWorkflowNodeData(node, { from }))} />
      <StateMatchEditor title="To" match={asMatch(node.data?.to, { operator: 'exists' })} onChange={(to) => onChange(updateWorkflowNodeData(node, { to }))} />
    </div>
  );
}

function DeviceStateIsNodeEditor(props: { node: AgentWorkflowNode; deviceOptions: InspectorOption; onChange: (node: AgentWorkflowNode) => void }) {
  const { node, deviceOptions, onChange } = props;
  return (
    <div className="stack">
      <FieldGrid>
        <SelectField label="Device" value={String(node.data?.device_id ?? '')} options={deviceOptions} onChange={(device_id) => onChange(updateWorkflowNodeData(node, { device_id }))} />
        <Field label="State Key" value={String(node.data?.state_key ?? '')} onChange={(state_key) => onChange(updateWorkflowNodeData(node, { state_key }))} />
      </FieldGrid>
      <StateMatchEditor title="Match" match={asMatch(node.data?.match, { operator: 'exists' })} onChange={(match) => onChange(updateWorkflowNodeData(node, { match }))} />
    </div>
  );
}

function TimeWindowNodeEditor({ node, onChange }: { node: AgentWorkflowNode; onChange: (node: AgentWorkflowNode) => void }) {
  const updateWindow = (patch: Record<string, unknown>) => onChange(updateWorkflowNodeData(node, patch));
  return (
    <FieldGrid>
      <Field label="Start" value={String(node.data?.start ?? '08:00')} onChange={(start) => updateWindow({ start })} />
      <Field label="End" value={String(node.data?.end ?? '18:00')} onChange={(end) => updateWindow({ end })} />
      <Field label="Timezone" value={String(node.data?.timezone ?? '')} onChange={(timezone) => updateWindow({ timezone })} />
    </FieldGrid>
  );
}

function TextNodeEditor({ node, onChange }: { node: AgentWorkflowNode; onChange: (node: AgentWorkflowNode) => void }) {
  return (
    <label className="stack text-sm font-medium">
      <span>Text</span>
      <Textarea
        value={String(node.data?.text ?? node.data?.prompt ?? '')}
        onChange={(event) => onChange(updateWorkflowNodeData(node, { text: event.target.value }))}
        placeholder="Write the text block that should be concatenated downstream."
      />
    </label>
  );
}

function LLMNodeEditor(props: { node: AgentWorkflowNode; providerOptions: InspectorOption; onChange: (node: AgentWorkflowNode) => void }) {
  const { node, providerOptions, onChange } = props;
  return (
    <>
      <SelectField
        label="Provider"
        value={String(node.data?.provider_id ?? '')}
        options={providerOptions}
        onChange={(provider_id) => onChange(updateWorkflowNodeData(node, { provider_id }))}
      />
      <label className="stack text-sm font-medium">
        <span>User Prompt</span>
        <Textarea
          value={String(node.data?.user_prompt ?? '')}
          onChange={(event) => onChange(updateWorkflowNodeData(node, { user_prompt: event.target.value }))}
          placeholder="Optional user-level instruction appended to connected prompt and context input."
        />
      </label>
      <div className="detail">`tool` and `skill` ports are visible on the node, but this delivery only executes `prompt`, `context`, and `search` inputs.</div>
    </>
  );
}

function SearchNodeEditor(props: { node: AgentWorkflowNode; providerOptions: InspectorOption; onChange: (node: AgentWorkflowNode) => void }) {
  const { node, providerOptions, onChange } = props;
  return (
    <>
      <SelectField
        label="Search Provider"
        value={String(node.data?.provider_id ?? '')}
        options={providerOptions}
        onChange={(provider_id) => onChange(updateWorkflowNodeData(node, { provider_id }))}
      />
      <Field label="Query" value={String(node.data?.query ?? '')} onChange={(query) => onChange(updateWorkflowNodeData(node, { query }))} />
      <FieldGrid>
        <SelectField
          label="Recency"
          value={String(node.data?.recency ?? '')}
          options={[
            { value: '', label: 'No Filter' },
            { value: 'day', label: 'Day' },
            { value: 'week', label: 'Week' },
            { value: 'month', label: 'Month' },
            { value: 'year', label: 'Year' },
          ]}
          onChange={(recency) => onChange(updateWorkflowNodeData(node, { recency }))}
        />
        <Field
          label="Max Items"
          value={String(node.data?.max_items ?? 8)}
          onChange={(max_items) => onChange(updateWorkflowNodeData(node, { max_items: Number(max_items) || 8 }))}
        />
      </FieldGrid>
      <label className="stack text-sm font-medium">
        <span>Sites</span>
        <Textarea
          value={asStringArray(node.data?.sites).join('\n')}
          onChange={(event) =>
            onChange(
              updateWorkflowNodeData(node, {
                sites: event.target.value
                  .split(/\n|,/)
                  .map((item) => item.trim())
                  .filter(Boolean),
              }),
            )
          }
          placeholder="Optional site filters, one per line."
        />
      </label>
    </>
  );
}

function WeComOutputNodeEditor(props: { node: AgentWorkflowNode; wecomOptions: InspectorOption; onChange: (node: AgentWorkflowNode) => void }) {
  const { node, wecomOptions, onChange } = props;
  return (
    <SelectField
      label="Recipient"
      value={String(node.data?.to_user ?? '')}
      options={wecomOptions}
      onChange={(to_user) => onChange(updateWorkflowNodeData(node, { to_user }))}
    />
  );
}

function DeviceCommandNodeEditor(props: { node: AgentWorkflowNode; deviceOptions: InspectorOption; onChange: (node: AgentWorkflowNode) => void }) {
  const { node, deviceOptions, onChange } = props;
  return (
    <div className="stack">
      <FieldGrid>
        <SelectField label="Device" value={String(node.data?.device_id ?? '')} options={deviceOptions} onChange={(device_id) => onChange(updateWorkflowNodeData(node, { device_id }))} />
        <Field label="Action" value={String(node.data?.action ?? '')} onChange={(action) => onChange(updateWorkflowNodeData(node, { action }))} />
      </FieldGrid>
      <JSONField label="Params JSON" value={asRecord(node.data?.params)} onChange={(params) => onChange(updateWorkflowNodeData(node, { params }))} />
    </div>
  );
}

function AgentFunctionNodeEditor(props: {
  node: AgentWorkflowNode;
  wecomOptions: InspectorOption;
  deviceOptions: InspectorOption;
  onChange: (node: AgentWorkflowNode) => void;
}) {
  const { node, wecomOptions, deviceOptions, onChange } = props;
  const touchpoints = asTouchpoints(node.data?.touchpoints);
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
        <Button variant="secondary" onClick={() => updateTouchpoints([...touchpoints, { type: 'device', device_id: '', action: 'push_voice_message', params: {} }])}>
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
                options={deviceOptions}
                onChange={(device_id) => updateTouchpoints(touchpoints.map((item, current) => (current === index ? { ...item, device_id } : item)))}
              />
              <Field
                label="Action"
                value={String(touchpoint.action ?? 'push_voice_message')}
                onChange={(action) => updateTouchpoints(touchpoints.map((item, current) => (current === index ? { ...item, action } : item)))}
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
  match: { operator: string; value?: unknown };
  allowAny?: boolean;
  onChange: (match: { operator: string; value?: unknown }) => void;
}) {
  const options = [
    ...(props.allowAny ? [{ value: 'any', label: 'Any Change' }] : []),
    { value: 'equals', label: 'Equals' },
    { value: 'not_equals', label: 'Not Equals' },
    { value: 'in', label: 'In List' },
    { value: 'not_in', label: 'Not In List' },
    { value: 'exists', label: 'Exists' },
    { value: 'missing', label: 'Missing' },
  ];
  const needsValue = ['equals', 'not_equals', 'in', 'not_in'].includes(props.match.operator);
  return (
    <div className="workflow-canvas__source">
      <SelectField label={props.title} value={props.match.operator} options={options} onChange={(operator) => props.onChange({ ...props.match, operator })} />
      {needsValue ? <Field label="Value JSON" value={JSON.stringify(props.match.value ?? '')} onChange={(raw) => props.onChange({ ...props.match, value: parseJSONValue(raw) })} /> : null}
    </div>
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

function GroupNodeEditor({ node, onChange }: { node: AgentWorkflowNode; onChange: (node: AgentWorkflowNode) => void }) {
  return (
    <FieldGrid>
      <Field label="Width" value={String(node.width ?? 360)} onChange={(width) => onChange({ ...node, width: Number(width) || 360 })} />
      <Field label="Height" value={String(node.height ?? 240)} onChange={(height) => onChange({ ...node, height: Number(height) || 240 })} />
    </FieldGrid>
  );
}

function asMatch(value: unknown, fallback: { operator: string; value?: unknown }) {
  return typeof value === 'object' && value !== null && !Array.isArray(value) ? (value as { operator: string; value?: unknown }) : fallback;
}

function asRecord(value: unknown): Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value) ? (value as Record<string, unknown>) : {};
}

function asTouchpoints(value: unknown): Array<Record<string, unknown>> {
  return Array.isArray(value) ? value.map((item) => asRecord(item)) : [];
}

function parseJSONValue(raw: string): unknown {
  try {
    return JSON.parse(raw);
  } catch {
    return raw;
  }
}

function parseJSONObject(raw: string, fallback: Record<string, unknown>) {
  try {
    const parsed = JSON.parse(raw) as unknown;
    return asRecord(parsed);
  } catch {
    return fallback;
  }
}

function blankSource(): AgentWorkflowSource {
  return {
    id: `source-${Date.now()}`,
    name: '',
    category: '',
    feed_url: '',
    weight: 1,
    enabled: true,
  };
}
