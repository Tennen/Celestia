import { request } from './api';

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
  wecom_user?: string;
  wecom_chat_id?: string;
  enabled: boolean;
  updated_at?: string;
};

export type AgentPushSnapshot = {
  users: AgentWeComUser[];
  updated_at: string;
};

export function fetchAgentWeComMenu() {
  return request<AgentWeComMenuSnapshot>('/touchpoints/wecom/menu').then(normalizeWeComMenuSnapshot);
}

export function saveAgentWeComMenu(payload: AgentWeComMenuConfig) {
  return request<AgentWeComMenuSnapshot>('/touchpoints/wecom/menu', { method: 'PUT', body: JSON.stringify(payload) }).then(normalizeWeComMenuSnapshot);
}

export function deleteAgentWeComMenu() {
  return request<AgentWeComMenuSnapshot>('/touchpoints/wecom/menu', { method: 'DELETE' }).then(normalizeWeComMenuSnapshot);
}

export function normalizeWeComMenuSnapshot(input: AgentWeComMenuSnapshot): AgentWeComMenuSnapshot {
  const menu = input ?? ({} as AgentWeComMenuSnapshot);
  const config = menu.config ?? ({} as AgentWeComMenuConfig);
  return {
    ...menu,
    config: {
      ...config,
      buttons: arrayOrEmpty(config.buttons).map(normalizeWeComButton),
    },
    recent_events: arrayOrEmpty(menu.recent_events),
    validation_errors: arrayOrEmpty(menu.validation_errors),
  };
}

function normalizeWeComButton(button: AgentWeComButton): AgentWeComButton {
  return {
    ...button,
    sub_buttons: arrayOrEmpty(button.sub_buttons).map(normalizeWeComButton),
  };
}

function arrayOrEmpty<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : [];
}
