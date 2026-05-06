import type { AgentWorkflowDefinition, AgentWorkflowEdge, AgentWorkflowNode, AgentWorkflowSource } from './agent-workflow';

export type WorkflowNodeType =
  | 'group'
  | 'timer'
  | 'device_state_changed'
  | 'device_state_is'
  | 'time_window'
  | 'rss_sources'
  | 'text'
  | 'llm'
  | 'search_provider'
  | 'wecom_output'
  | 'device_command'
  | 'agent_function';

export const workflowNodeCatalog: Array<{ type: WorkflowNodeType; label: string; description: string; kind?: 'trigger' | 'accessory' | 'action' }> = [
  { type: 'group', label: 'Group', description: 'Organize related nodes into a bounded canvas section.' },
  { type: 'timer', label: 'Timer', description: 'Autonomously starts downstream nodes on a daily schedule or interval.', kind: 'trigger' },
  { type: 'device_state_changed', label: 'Device State Changed', description: 'Autonomously starts when a device state key transitions.', kind: 'trigger' },
  { type: 'device_state_is', label: 'Device State Is', description: 'Autonomously starts or gates a path when current state matches.', kind: 'trigger' },
  { type: 'time_window', label: 'Time Window', description: 'Accessory gate that constrains connected trigger paths by local clock.', kind: 'accessory' },
  { type: 'rss_sources', label: 'RSS Sources', description: 'Fetch RSS or Atom feeds and emit only newly appeared items.' },
  { type: 'text', label: 'Text', description: 'Compose reusable text blocks and concatenate upstream text in connection order.' },
  { type: 'llm', label: 'LLM', description: 'Run a selected provider with prompt, context, and optional search input.' },
  { type: 'search_provider', label: 'Search Provider', description: 'Run a configured search provider and stream results into the workflow.' },
  { type: 'wecom_output', label: 'WeCom Output', description: 'Deliver the generated text to a configured WeCom user.' },
  { type: 'device_command', label: 'Device Command', description: 'Execute a real gateway command through policy, audit, and plugin dispatch.', kind: 'action' },
  { type: 'agent_function', label: 'Agent Function', description: 'Run project input or Agent text and optionally deliver the result.', kind: 'action' },
];

export function createWorkflowDefinition(): AgentWorkflowDefinition {
  const now = new Date().toISOString();
  return {
    id: `workflow-${Date.now()}`,
    name: 'Untitled Workflow',
    description: '',
    nodes: [],
    edges: [],
    updated_at: now,
  };
}

export function createWorkflowNode(type: WorkflowNodeType, index: number): AgentWorkflowNode {
  const base = {
    id: `${type}-${Date.now()}-${index}`,
    type,
    label: workflowNodeCatalog.find((item) => item.type === type)?.label ?? 'Node',
    position: { x: 120 + index * 36, y: 120 + index * 24 },
    data: defaultNodeData(type),
  } satisfies AgentWorkflowNode;
  if (type === 'group') {
    return { ...base, width: 360, height: 240 };
  }
  return base;
}

export function defaultNodeData(type: WorkflowNodeType): Record<string, unknown> {
  switch (type) {
    case 'timer':
      return {
        schedule: 'daily',
        at: '08:00',
        interval_seconds: 600,
        timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || '',
      };
    case 'device_state_changed':
      return { device_id: '', state_key: '', from: { operator: 'not_equals' }, to: { operator: 'equals' } };
    case 'device_state_is':
      return { device_id: '', state_key: '', match: { operator: 'equals' } };
    case 'time_window':
      return { start: '08:00', end: '18:00', timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || '' };
    case 'rss_sources':
      return { sources: [] as AgentWorkflowSource[] };
    case 'text':
      return { text: '' };
    case 'llm':
      return { provider_id: '', user_prompt: '' };
    case 'search_provider':
      return { provider_id: '', query: '', recency: '', max_items: 8, sites: [] as string[] };
    case 'wecom_output':
      return { to_user: '' };
    case 'device_command':
      return { device_id: '', action: '', params: {} };
    case 'agent_function':
      return { input: '', session_id: '', touchpoints: [] };
    default:
      return {};
  }
}

export function cloneWorkflow<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}

export function normalizeWorkflowNode(node: AgentWorkflowNode): AgentWorkflowNode {
  return {
    ...node,
    type: node.type,
    label: node.label || workflowNodeCatalog.find((item) => item.type === node.type)?.label || 'Node',
    data: { ...(node.data ?? {}) },
  };
}

export function normalizeWorkflowDefinition(workflow: AgentWorkflowDefinition): AgentWorkflowDefinition {
  return {
    ...workflow,
    nodes: workflow.nodes.map((node) => normalizeWorkflowNode(node)),
  };
}

export function replaceWorkflowDefinition(workflows: AgentWorkflowDefinition[], next: AgentWorkflowDefinition) {
  return workflows.some((workflow) => workflow.id === next.id)
    ? workflows.map((workflow) => (workflow.id === next.id ? next : workflow))
    : [...workflows, next];
}

export function removeWorkflowDefinition(workflows: AgentWorkflowDefinition[], workflowId: string) {
  return workflows.filter((workflow) => workflow.id !== workflowId);
}

export function replaceWorkflowNode(nodes: AgentWorkflowNode[], next: AgentWorkflowNode) {
  return nodes.some((node) => node.id === next.id) ? nodes.map((node) => (node.id === next.id ? next : node)) : [...nodes, next];
}

export function removeWorkflowNode(nodes: AgentWorkflowNode[], nodeId: string) {
  return nodes.filter((node) => node.id !== nodeId);
}

export function removeWorkflowEdgesForNode(edges: AgentWorkflowEdge[], nodeId: string) {
  return edges.filter((edge) => edge.source !== nodeId && edge.target !== nodeId);
}

export function asWorkflowSources(value: unknown): AgentWorkflowSource[] {
  return Array.isArray(value) ? (value as AgentWorkflowSource[]) : [];
}

export function asStringArray(value: unknown): string[] {
  return Array.isArray(value) ? value.map((item) => String(item ?? '').trim()).filter(Boolean) : [];
}

export function updateWorkflowNodeData(node: AgentWorkflowNode, patch: Record<string, unknown>) {
  return {
    ...node,
    data: {
      ...(node.data ?? {}),
      ...patch,
    },
  };
}

export function workflowGroups(workflow: AgentWorkflowDefinition) {
  return workflow.nodes.filter((node) => node.type === 'group');
}
