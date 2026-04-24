import { Plus, Trash2 } from 'lucide-react';
import { Button } from '../../ui/button';
import { Textarea } from '../../ui/textarea';
import { Field, FieldGrid, SelectField, ToggleField } from '../AgentFormFields';
import type { AgentTopicNode, AgentTopicSource } from '../../../lib/agent-topic';
import { asStringArray, asTopicSources, updateNodeData } from '../../../lib/topic-workflow';

type InspectorOption = Array<{ value: string; label: string }>;

export function TopicWorkflowInspector(props: {
  node: AgentTopicNode;
  groups: AgentTopicNode[];
  providerOptions: InspectorOption;
  searchProviderOptions: InspectorOption;
  wecomOptions: InspectorOption;
  onChange: (node: AgentTopicNode) => void;
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
      {node.type === 'prompt_unit' ? <PromptNodeEditor node={node} onChange={onChange} /> : null}
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

function RSSNodeEditor({ node, onChange }: { node: AgentTopicNode; onChange: (node: AgentTopicNode) => void }) {
  const sources = asTopicSources(node.data?.sources);
  const updateSource = (index: number, patch: Partial<AgentTopicSource>) => {
    const next = sources.map((source, current) => (current === index ? { ...source, ...patch } : source));
    onChange(updateNodeData(node, { sources: next }));
  };
  return (
    <div className="stack">
      <div className="button-row">
        <Button variant="secondary" onClick={() => onChange(updateNodeData(node, { sources: [...sources, blankSource()] }))}>
          <Plus className="mr-2 h-4 w-4" />
          Add RSS Source
        </Button>
      </div>
      {sources.map((source, index) => (
        <div key={`${source.id || 'source'}-${index}`} className="topic-workflow__source">
          <ToggleField label="Enabled" checked={source.enabled !== false} onChange={(enabled) => updateSource(index, { enabled })} />
          <FieldGrid>
            <Field label="Name" value={source.name ?? ''} onChange={(name) => updateSource(index, { name })} />
            <Field label="Category" value={source.category ?? ''} onChange={(category) => updateSource(index, { category })} />
            <Field label="Feed URL" value={source.feed_url ?? ''} onChange={(feed_url) => updateSource(index, { feed_url })} />
            <Field label="Weight" value={String(source.weight ?? 1)} onChange={(weight) => updateSource(index, { weight: Number(weight) || 1 })} />
          </FieldGrid>
          <Button variant="danger" onClick={() => onChange(updateNodeData(node, { sources: sources.filter((_, current) => current !== index) }))}>
            Remove Source
          </Button>
        </div>
      ))}
      {sources.length === 0 ? <div className="detail">Add one or more RSS feeds to this node.</div> : null}
    </div>
  );
}

function PromptNodeEditor({ node, onChange }: { node: AgentTopicNode; onChange: (node: AgentTopicNode) => void }) {
  return (
    <label className="stack text-sm font-medium">
      <span>Prompt</span>
      <Textarea
        value={String(node.data?.prompt ?? '')}
        onChange={(event) => onChange(updateNodeData(node, { prompt: event.target.value }))}
        placeholder="Write the reusable summary prompt block here."
      />
    </label>
  );
}

function LLMNodeEditor(props: { node: AgentTopicNode; providerOptions: InspectorOption; onChange: (node: AgentTopicNode) => void }) {
  const { node, providerOptions, onChange } = props;
  return (
    <>
      <SelectField
        label="Provider"
        value={String(node.data?.provider_id ?? '')}
        options={providerOptions}
        onChange={(provider_id) => onChange(updateNodeData(node, { provider_id }))}
      />
      <label className="stack text-sm font-medium">
        <span>User Prompt</span>
        <Textarea
          value={String(node.data?.user_prompt ?? '')}
          onChange={(event) => onChange(updateNodeData(node, { user_prompt: event.target.value }))}
          placeholder="Optional user-level instruction appended to connected prompt/context input."
        />
      </label>
      <div className="detail">`tool` and `skill` ports are already exposed on the node, but this first cut only executes `prompt`, `context`, and `search` inputs.</div>
    </>
  );
}

function SearchNodeEditor(props: { node: AgentTopicNode; providerOptions: InspectorOption; onChange: (node: AgentTopicNode) => void }) {
  const { node, providerOptions, onChange } = props;
  return (
    <>
      <SelectField
        label="Search Provider"
        value={String(node.data?.provider_id ?? '')}
        options={providerOptions}
        onChange={(provider_id) => onChange(updateNodeData(node, { provider_id }))}
      />
      <Field label="Query" value={String(node.data?.query ?? '')} onChange={(query) => onChange(updateNodeData(node, { query }))} />
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
          onChange={(recency) => onChange(updateNodeData(node, { recency }))}
        />
        <Field
          label="Max Items"
          value={String(node.data?.max_items ?? 8)}
          onChange={(max_items) => onChange(updateNodeData(node, { max_items: Number(max_items) || 8 }))}
        />
      </FieldGrid>
      <label className="stack text-sm font-medium">
        <span>Sites</span>
        <Textarea
          value={asStringArray(node.data?.sites).join('\n')}
          onChange={(event) =>
            onChange(
              updateNodeData(node, {
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

function WeComOutputNodeEditor(props: { node: AgentTopicNode; wecomOptions: InspectorOption; onChange: (node: AgentTopicNode) => void }) {
  const { node, wecomOptions, onChange } = props;
  return (
    <SelectField
      label="Recipient"
      value={String(node.data?.to_user ?? '')}
      options={wecomOptions}
      onChange={(to_user) => onChange(updateNodeData(node, { to_user }))}
    />
  );
}

function GroupNodeEditor({ node, onChange }: { node: AgentTopicNode; onChange: (node: AgentTopicNode) => void }) {
  return (
    <FieldGrid>
      <Field label="Width" value={String(node.width ?? 360)} onChange={(width) => onChange({ ...node, width: Number(width) || 360 })} />
      <Field label="Height" value={String(node.height ?? 240)} onChange={(height) => onChange({ ...node, height: Number(height) || 240 })} />
    </FieldGrid>
  );
}

function blankSource(): AgentTopicSource {
  return {
    id: `source-${Date.now()}`,
    name: '',
    category: '',
    feed_url: '',
    weight: 1,
    enabled: true,
  };
}
