import type { AgentWorkflowDefinition, AgentWorkflowEdge, AgentWorkflowNode, AgentWorkflowSource } from './agent-workflow';

export type WorkflowNodeType = 'group' | 'rss_sources' | 'text' | 'llm' | 'search_provider' | 'wecom_output';

export const workflowNodeCatalog: Array<{ type: WorkflowNodeType; label: string; description: string }> = [
  { type: 'group', label: 'Group', description: 'Organize related nodes into a bounded canvas section.' },
  { type: 'rss_sources', label: 'RSS Sources', description: 'Fetch and deduplicate multiple RSS or Atom feeds.' },
  { type: 'text', label: 'Text', description: 'Compose reusable text blocks and concatenate upstream text in connection order.' },
  { type: 'llm', label: 'LLM', description: 'Run a selected provider with prompt, context, and optional search input.' },
  { type: 'search_provider', label: 'Search Provider', description: 'Run a configured search provider and stream results into the workflow.' },
  { type: 'wecom_output', label: 'WeCom Output', description: 'Deliver the generated text to a configured WeCom user.' },
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
    default:
      return {};
  }
}

export function cloneWorkflow<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}

export function normalizeWorkflowNode(node: AgentWorkflowNode): AgentWorkflowNode {
  const normalizedType = node.type === 'prompt_unit' ? 'text' : node.type;
  const normalizedLabel = node.type === 'prompt_unit' && (!node.label || node.label === 'Prompt Unit') ? 'Text' : node.label;
  const normalizedData = { ...(node.data ?? {}) };
  if (normalizedType === 'text' && typeof normalizedData.text !== 'string' && typeof normalizedData.prompt === 'string') {
    normalizedData.text = normalizedData.prompt;
  }
  delete normalizedData.prompt;
  return {
    ...node,
    type: normalizedType,
    label: normalizedLabel || workflowNodeCatalog.find((item) => item.type === normalizedType)?.label || 'Node',
    data: normalizedData,
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
