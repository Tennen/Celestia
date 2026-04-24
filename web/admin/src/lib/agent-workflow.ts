import { request } from './api';

export type AgentWorkflowSource = {
  id: string;
  name: string;
  category: string;
  feed_url: string;
  weight: number;
  enabled: boolean;
};

export type AgentWorkflowNode = {
  id: string;
  type: string;
  label?: string;
  parent_id?: string;
  position: { x: number; y: number };
  width?: number;
  height?: number;
  data?: Record<string, unknown>;
};

export type AgentWorkflowEdge = {
  id: string;
  source: string;
  source_handle?: string;
  target: string;
  target_handle?: string;
  label?: string;
};

export type AgentWorkflowDefinition = {
  id: string;
  name: string;
  description?: string;
  nodes: AgentWorkflowNode[];
  edges: AgentWorkflowEdge[];
  updated_at: string;
};

export type AgentWorkflowNodeResult = {
  node_id: string;
  node_type: string;
  status: string;
  summary?: string;
  metadata?: Record<string, unknown>;
};

export type AgentWorkflowRun = {
  id: string;
  workflow_id?: string;
  workflow_name?: string;
  created_at: string;
  started_at?: string;
  finished_at?: string;
  status?: string;
  summary: string;
  output_text?: string;
  items: Array<Record<string, unknown>>;
  node_results?: AgentWorkflowNodeResult[];
  fetch_errors?: Array<{ target: string; error: string }>;
  delivery_errors?: Array<{ target: string; error: string }>;
};

export type AgentWorkflowSnapshot = {
  active_workflow_id: string;
  workflows: AgentWorkflowDefinition[];
  runs: AgentWorkflowRun[];
  sent_log?: Array<Record<string, unknown>>;
  updated_at: string;
};

export function saveAgentWorkflow(payload: AgentWorkflowSnapshot) {
  return request('/agent/workflow', { method: 'PUT', body: JSON.stringify(payload) });
}

export function runAgentWorkflow(workflowId?: string) {
  return request<AgentWorkflowRun>('/agent/workflow/run', {
    method: 'POST',
    body: JSON.stringify({ workflow_id: workflowId ?? '' }),
  });
}

export function normalizeAgentWorkflowSnapshot(input: AgentWorkflowSnapshot | null | undefined): AgentWorkflowSnapshot {
  const workflow = input ?? ({} as AgentWorkflowSnapshot);
  return {
    ...workflow,
    workflows: arrayOrEmpty(workflow.workflows),
    runs: arrayOrEmpty(workflow.runs),
    sent_log: arrayOrEmpty(workflow.sent_log),
  };
}

function arrayOrEmpty<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : [];
}
