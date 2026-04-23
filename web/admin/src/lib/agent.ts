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
    bridge_url?: string;
    bridge_token?: string;
    bridge_stream_enabled?: boolean;
    audio_dir?: string;
    text_max_bytes?: number;
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
    codex_model?: string;
    codex_reasoning?: string;
    max_fix_attempts?: number;
    test_commands?: Array<{ name: string; command: string; timeout_ms?: number }>;
    auto_commit?: boolean;
    auto_push?: boolean;
    push_remote?: string;
    push_branch?: string;
    structure_review?: boolean;
  };
  stt?: Record<string, unknown>;
  search_engines?: Array<Record<string, unknown>>;
  memory?: Record<string, unknown>;
  md2img?: Record<string, unknown>;
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

export type AgentWeComUser = {
  id: string;
  name: string;
  wecom_user: string;
  enabled: boolean;
  updated_at?: string;
};

export type AgentPushSnapshot = {
  users: AgentWeComUser[];
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

export type AgentCapabilityInfo = {
  name: string;
  description: string;
  terminal?: boolean;
  command?: string;
  install?: string;
  keywords?: string[];
  direct_commands?: string[];
  tool?: string;
  action?: string;
  params?: string[];
  prefer_tool_result?: boolean;
  detail?: string;
};

export type AgentCapabilityRunResult = {
  capability: string;
  tool?: string;
  action?: string;
  input?: string;
  used_command?: string;
  output?: string;
  terminal?: TerminalResult;
  metadata?: Record<string, unknown>;
};

export type AgentMarkdownRenderResult = {
  mode: string;
  images: Array<{ path: string; content_type: string; size_bytes: number; width?: number; height?: number }>;
  output_dir: string;
  source_chars: number;
  rendered_at: string;
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
  artifact_root?: string;
  raw_files?: Array<Record<string, unknown>>;
  artifacts?: Record<string, unknown>;
  last_summarized_at?: string | null;
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
  plan?: { steps: string[]; current_step: number };
  fix_attempts?: number;
  started_from_ref?: string;
  completed_commit?: string;
  test_results?: Array<Record<string, unknown>>;
  raw_tail?: Array<Record<string, unknown>>;
  last_codex_output?: string;
  events: Array<Record<string, unknown>>;
  created_at: string;
  updated_at: string;
  last_error?: string;
};

export type AgentSnapshot = {
  settings: AgentSettings;
  capabilities: AgentCapabilityInfo[];
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
  return request<AgentSnapshot>('/agent').then(normalizeAgentSnapshot);
}

export function saveAgentSettings(payload: AgentSettings) {
  return request<AgentSnapshot>('/agent/settings', { method: 'PUT', body: JSON.stringify(payload) }).then(normalizeAgentSnapshot);
}

export function saveAgentDirectInput(payload: AgentDirectInputConfig) {
  return request<AgentSnapshot>('/agent/direct-input', { method: 'PUT', body: JSON.stringify(payload) }).then(normalizeAgentSnapshot);
}

export function saveAgentPush(payload: AgentPushSnapshot) {
  return request<AgentSnapshot>('/agent/push', { method: 'PUT', body: JSON.stringify(payload) }).then(normalizeAgentSnapshot);
}

export function saveAgentWeComMenu(payload: AgentWeComMenuConfig) {
  return request<AgentSnapshot>('/agent/wecom/menu', { method: 'PUT', body: JSON.stringify(payload) }).then(normalizeAgentSnapshot);
}

export function publishAgentWeComMenu() {
  return request<AgentWeComMenuSnapshot>('/agent/wecom/menu/publish', { method: 'POST' });
}

export function sendAgentWeComMessage(payload: { to_user: string; text: string }) {
  return request<{ ok: boolean }>('/agent/wecom/send', { method: 'POST', body: JSON.stringify(payload) });
}

export function sendAgentWeComImage(payload: { to_user: string; base64: string; filename?: string; content_type?: string }) {
  return request<{ ok: boolean }>('/agent/wecom/image', { method: 'POST', body: JSON.stringify(payload) });
}

export function runAgentConversation(payload: { input: string; session_id?: string }) {
  return request<AgentConversation>('/agent/conversation', { method: 'POST', body: JSON.stringify(payload) });
}

export function fetchAgentCapabilities() {
  return request<AgentCapabilityInfo[]>('/agent/capabilities').then((items) => (Array.isArray(items) ? items : []));
}

export function fetchAgentCapability(name: string) {
  return request<AgentCapabilityInfo>(`/agent/capabilities/${encodeURIComponent(name)}`);
}

export function runAgentCapability(name: string, payload: { input?: string; command?: string; args?: string[] }) {
  return request<AgentCapabilityRunResult>(`/agent/capabilities/${encodeURIComponent(name)}/run`, {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function saveAgentTopic(payload: AgentTopicSnapshot) {
  return request<AgentSnapshot>('/agent/topic', { method: 'PUT', body: JSON.stringify(payload) }).then(normalizeAgentSnapshot);
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
  return request<AgentSnapshot>('/agent/market/portfolio', { method: 'PUT', body: JSON.stringify(payload) }).then(normalizeAgentSnapshot);
}

export function importMarketPortfolioCodes(payload: { codes: string }) {
  return request<Record<string, unknown>>('/agent/market/portfolio/import-codes', { method: 'POST', body: JSON.stringify(payload) });
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

export function renderAgentMarkdown(payload: { markdown: string; mode?: 'long-image' | 'multi-page'; output_dir?: string }) {
  return request<AgentMarkdownRenderResult>('/agent/md2img/render', { method: 'POST', body: JSON.stringify(payload) });
}

export function stableJSON(value: unknown) {
  return JSON.stringify(value, null, 2);
}

export function normalizeAgentSnapshot(input: AgentSnapshot): AgentSnapshot {
  const snapshot = input ?? ({} as AgentSnapshot);
  const settings = snapshot.settings ?? ({} as AgentSettings);
  const directInput = snapshot.direct_input ?? ({} as AgentDirectInputConfig);
  const wecomMenu = snapshot.wecom_menu ?? ({} as AgentWeComMenuSnapshot);
  const wecomConfig = wecomMenu.config ?? ({} as AgentWeComMenuConfig);
  const push = snapshot.push ?? ({} as AgentPushSnapshot);
  const topic = snapshot.topic_summary ?? ({} as AgentTopicSnapshot);
  const market = snapshot.market ?? ({} as AgentMarketSnapshot);
  const portfolio = market.portfolio ?? ({ cash: 0 } as AgentMarketPortfolio);
  const writing = snapshot.writing ?? { topics: [], updated_at: '' };
  const evolution = snapshot.evolution ?? { goals: [], updated_at: '' };

  return {
    ...snapshot,
    settings: {
      ...settings,
      llm_providers: arrayOrEmpty(settings.llm_providers),
      search_engines: arrayOrEmpty(settings.search_engines),
      wecom: settings.wecom ?? { enabled: false },
      terminal: settings.terminal ?? { enabled: false },
      evolution: {
        ...(settings.evolution ?? {}),
        test_commands: arrayOrEmpty(settings.evolution?.test_commands),
      },
    },
    capabilities: arrayOrEmpty(snapshot.capabilities),
    direct_input: {
      ...directInput,
      rules: arrayOrEmpty(directInput.rules),
    },
    wecom_menu: {
      ...wecomMenu,
      config: {
        ...wecomConfig,
        buttons: arrayOrEmpty(wecomConfig.buttons),
      },
      recent_events: arrayOrEmpty(wecomMenu.recent_events),
      validation_errors: arrayOrEmpty(wecomMenu.validation_errors),
    },
    push: {
      ...push,
      users: arrayOrEmpty(push.users),
    },
    conversations: arrayOrEmpty(snapshot.conversations),
    topic_summary: {
      ...topic,
      profiles: arrayOrEmpty(topic.profiles),
      runs: arrayOrEmpty(topic.runs),
    },
    writing: {
      ...writing,
      topics: arrayOrEmpty(writing.topics),
    },
    market: {
      ...market,
      portfolio: {
        ...portfolio,
        funds: arrayOrEmpty(portfolio.funds),
        cash: typeof portfolio.cash === 'number' ? portfolio.cash : 0,
      },
      runs: arrayOrEmpty(market.runs),
    },
    evolution: {
      ...evolution,
      goals: arrayOrEmpty(evolution.goals),
    },
  };
}

function arrayOrEmpty<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : [];
}
