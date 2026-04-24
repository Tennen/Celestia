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
  onChange: (node: AgentWorkflowNode) => void;
  onDelete: () => void;
}) {
  const { node, groups, providerOptions, searchProviderOptions, wecomOptions, onChange, onDelete } = props;
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
      {node.type === 'text' ? <TextNodeEditor node={node} onChange={onChange} /> : null}
      {node.type === 'llm' ? <LLMNodeEditor node={node} providerOptions={providerOptions} onChange={onChange} /> : null}
      {node.type === 'search_provider' ? <SearchNodeEditor node={node} providerOptions={searchProviderOptions} onChange={onChange} /> : null}
      {node.type === 'wecom_output' ? <WeComOutputNodeEditor node={node} wecomOptions={wecomOptions} onChange={onChange} /> : null}
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

function GroupNodeEditor({ node, onChange }: { node: AgentWorkflowNode; onChange: (node: AgentWorkflowNode) => void }) {
  return (
    <FieldGrid>
      <Field label="Width" value={String(node.width ?? 360)} onChange={(width) => onChange({ ...node, width: Number(width) || 360 })} />
      <Field label="Height" value={String(node.height ?? 240)} onChange={(height) => onChange({ ...node, height: Number(height) || 240 })} />
    </FieldGrid>
  );
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
