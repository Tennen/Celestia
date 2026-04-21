import { request } from './api';

export type AgentLLMProvider = {
  id: string;
  name: string;
  type: string;
  base_url?: string;
  api_key?: string;
  model?: string;
  chat_path?: string;
  timeout_ms?: number;
};

export type AgentSettings = {
  runtime_mode: string;
  default_llm_provider_id: string;
  llm_providers: AgentLLMProvider[];
  wecom: {
    corp_id?: string;
    corp_secret?: string;
    agent_id?: string;
    base_url?: string;
    enabled: boolean;
  };
  terminal: {
    enabled: boolean;
    cwd?: string;
    timeout_ms?: number;
  };
  evolution: {
    command?: string;
    cwd?: string;
    timeout_ms?: number;
  };
  stt?: Record<string, unknown>;
  search_engines?: Array<Record<string, unknown>>;
  updated_at: string;
};

export type AgentDirectInputRule = {
  id: string;
  name: string;
  pattern: string;
  target_text: string;
  match_mode: 'exact' | 'fuzzy';
  enabled: boolean;
};

export type AgentDirectInputConfig = {
  version: number;
  rules: AgentDirectInputRule[];
  updated_at: string;
};

export type AgentWeComButton = {
  id: string;
  name: string;
  key: string;
  enabled: boolean;
  dispatch_text: string;
  sub_buttons?: AgentWeComButton[];
};

export type AgentWeComMenuConfig = {
  version: number;
  buttons: AgentWeComButton[];
  updated_at: string;
  last_published_at?: string | null;
};

export type AgentWeComMenuSnapshot = {
  config: AgentWeComMenuConfig;
  recent_events: Array<Record<string, unknown>>;
  publish_payload?: Record<string, unknown> | null;
  validation_errors?: string[];
};

export type AgentPushSnapshot = {
  users: Array<Record<string, unknown>>;
  tasks: Array<Record<string, unknown>>;
  updated_at: string;
};

export type AgentConversation = {
  id: string;
  session_id: string;
  input: string;
  resolved?: string;
  response: string;
  status: string;
  metadata?: Record<string, unknown>;
  created_at: string;
};

export type AgentTopicSnapshot = {
  active_profile_id: string;
  profiles: Array<Record<string, unknown>>;
  runs: Array<Record<string, unknown>>;
  updated_at: string;
};

export type AgentWritingTopic = {
  id: string;
  title: string;
  status: string;
  materials: Array<Record<string, unknown>>;
  state: Record<string, string>;
  backup: Record<string, string>;
  created_at: string;
  updated_at: string;
};

export type AgentMarketPortfolio = {
  funds: Array<{
    code: string;
    name: string;
    quantity?: number;
    avg_cost?: number;
  }>;
  cash: number;
};

export type AgentMarketSnapshot = {
  portfolio: AgentMarketPortfolio;
  config: Record<string, unknown>;
  runs: Array<Record<string, unknown>>;
  updated_at: string;
};

export type AgentEvolutionGoal = {
  id: string;
  goal: string;
  commit_message?: string;
  status: string;
  stage: string;
  events: Array<Record<string, unknown>>;
  created_at: string;
  updated_at: string;
  last_error?: string;
};

export type AgentSnapshot = {
  settings: AgentSettings;
  direct_input: AgentDirectInputConfig;
  wecom_menu: AgentWeComMenuSnapshot;
  push: AgentPushSnapshot;
  conversations: AgentConversation[];
  topic_summary: AgentTopicSnapshot;
  writing: {
    topics: AgentWritingTopic[];
    updated_at: string;
  };
  market: AgentMarketSnapshot;
  evolution: {
    goals: AgentEvolutionGoal[];
    updated_at: string;
  };
  updated_at: string;
};

export type TerminalResult = {
  command: string;
  cwd?: string;
  exit_code: number;
  output: string;
  started_at: string;
  finished_at: string;
};

export type AgentSearchResult = {
  items: Array<{
    title: string;
    source: string;
    link?: string;
    published_at?: string;
    snippet?: string;
  }>;
  source_chain: string[];
  errors: string[];
};

export function fetchAgentSnapshot() {
  return request<AgentSnapshot>('/agent');
}

export function saveAgentSettings(payload: AgentSettings) {
  return request<AgentSnapshot>('/agent/settings', { method: 'PUT', body: JSON.stringify(payload) });
}

export function saveAgentDirectInput(payload: AgentDirectInputConfig) {
  return request<AgentSnapshot>('/agent/direct-input', { method: 'PUT', body: JSON.stringify(payload) });
}

export function saveAgentPush(payload: AgentPushSnapshot) {
  return request<AgentSnapshot>('/agent/push', { method: 'PUT', body: JSON.stringify(payload) });
}

export function saveAgentWeComMenu(payload: AgentWeComMenuConfig) {
  return request<AgentSnapshot>('/agent/wecom/menu', { method: 'PUT', body: JSON.stringify(payload) });
}

export function publishAgentWeComMenu() {
  return request<AgentWeComMenuSnapshot>('/agent/wecom/menu/publish', { method: 'POST' });
}

export function sendAgentWeComMessage(payload: { to_user: string; text: string }) {
  return request<{ ok: boolean }>('/agent/wecom/send', { method: 'POST', body: JSON.stringify(payload) });
}

export function runAgentConversation(payload: { input: string; session_id?: string }) {
  return request<AgentConversation>('/agent/conversation', { method: 'POST', body: JSON.stringify(payload) });
}

export function saveAgentTopic(payload: AgentTopicSnapshot) {
  return request<AgentSnapshot>('/agent/topic', { method: 'PUT', body: JSON.stringify(payload) });
}

export function runAgentTopic(profileId?: string) {
  return request<Record<string, unknown>>('/agent/topic/run', {
    method: 'POST',
    body: JSON.stringify({ profile_id: profileId ?? '' }),
  });
}

export function createWritingTopic(payload: { id?: string; title: string }) {
  return request<AgentWritingTopic>('/agent/writing/topics', { method: 'POST', body: JSON.stringify(payload) });
}

export function addWritingMaterial(topicId: string, payload: { title?: string; content: string }) {
  return request<AgentWritingTopic>(`/agent/writing/topics/${encodeURIComponent(topicId)}/materials`, {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function summarizeWritingTopic(topicId: string) {
  return request<AgentWritingTopic>(`/agent/writing/topics/${encodeURIComponent(topicId)}/summarize`, { method: 'POST' });
}

export function saveMarketPortfolio(payload: AgentMarketPortfolio) {
  return request<AgentSnapshot>('/agent/market/portfolio', { method: 'PUT', body: JSON.stringify(payload) });
}

export function runMarketAnalysis(payload: { phase: string; notes?: string }) {
  return request<Record<string, unknown>>('/agent/market/run', { method: 'POST', body: JSON.stringify(payload) });
}

export function createEvolutionGoal(payload: { goal: string; commit_message?: string }) {
  return request<AgentEvolutionGoal>('/agent/evolution/goals', { method: 'POST', body: JSON.stringify(payload) });
}

export function runEvolutionGoal(goalId: string) {
  return request<AgentEvolutionGoal>(`/agent/evolution/goals/${encodeURIComponent(goalId)}/run`, { method: 'POST' });
}

export function runTerminalCommand(payload: { command: string; cwd?: string }) {
  return request<TerminalResult>('/agent/terminal', { method: 'POST', body: JSON.stringify(payload) });
}

export function runAgentSearch(payload: {
  engine_selector?: string;
  timeout_ms?: number;
  max_items?: number;
  plans: Array<{ label: string; query: string; sites?: string[]; recency?: string }>;
}) {
  return request<AgentSearchResult>('/agent/search/run', { method: 'POST', body: JSON.stringify(payload) });
}

export function transcribeAgentSpeech(payload: { audio_path: string }) {
  return request<Record<string, unknown>>('/agent/stt/transcribe', { method: 'POST', body: JSON.stringify(payload) });
}

export function runAgentCodex(payload: {
  task_id?: string;
  prompt: string;
  model?: string;
  reasoning_effort?: string;
  timeout_ms?: number;
  cwd?: string;
}) {
  return request<Record<string, unknown>>('/agent/codex/run', { method: 'POST', body: JSON.stringify(payload) });
}

export function stableJSON(value: unknown) {
  return JSON.stringify(value, null, 2);
}

export function parseJSONObject<T>(raw: string, label: string): T {
  const parsed = JSON.parse(raw) as unknown;
  if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
    throw new Error(`${label} must be a JSON object`);
  }
  return parsed as T;
}
