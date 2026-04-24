import { request } from './api';

export type AgentTopicSource = {
  id: string;
  name: string;
  category: string;
  feed_url: string;
  weight: number;
  enabled: boolean;
};

export type AgentTopicNode = {
  id: string;
  type: string;
  label?: string;
  parent_id?: string;
  position: { x: number; y: number };
  width?: number;
  height?: number;
  data?: Record<string, unknown>;
};

export type AgentTopicEdge = {
  id: string;
  source: string;
  source_handle?: string;
  target: string;
  target_handle?: string;
  label?: string;
};

export type AgentTopicWorkflow = {
  id: string;
  name: string;
  description?: string;
  nodes: AgentTopicNode[];
  edges: AgentTopicEdge[];
  updated_at: string;
};

export type AgentTopicNodeResult = {
  node_id: string;
  node_type: string;
  status: string;
  summary?: string;
  metadata?: Record<string, unknown>;
};

export type AgentTopicRun = {
  id: string;
  workflow_id?: string;
  workflow_name?: string;
  profile_id?: string;
  created_at: string;
  started_at?: string;
  finished_at?: string;
  status?: string;
  summary: string;
  output_text?: string;
  items: Array<Record<string, unknown>>;
  node_results?: AgentTopicNodeResult[];
  fetch_errors?: Array<{ target: string; error: string }>;
  delivery_errors?: Array<{ target: string; error: string }>;
};

export type AgentTopicSnapshot = {
  active_workflow_id: string;
  active_profile_id?: string;
  workflows: AgentTopicWorkflow[];
  runs: AgentTopicRun[];
  sent_log?: Array<Record<string, unknown>>;
  updated_at: string;
};

export function saveAgentTopic(payload: AgentTopicSnapshot) {
  return request('/agent/topic', { method: 'PUT', body: JSON.stringify(payload) });
}

export function runAgentTopic(workflowId?: string) {
  return request<AgentTopicRun>('/agent/topic/run', {
    method: 'POST',
    body: JSON.stringify({ workflow_id: workflowId ?? '' }),
  });
}

export function normalizeAgentTopicSnapshot(input: AgentTopicSnapshot | null | undefined): AgentTopicSnapshot {
  const topic = input ?? ({} as AgentTopicSnapshot);
  return {
    ...topic,
    workflows: arrayOrEmpty(topic.workflows),
    runs: arrayOrEmpty(topic.runs),
    sent_log: arrayOrEmpty(topic.sent_log),
  };
}

function arrayOrEmpty<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : [];
}
