export type AgentPanelId = 'llm' | 'conversation' | 'content' | 'market' | 'evolution' | 'search';

export const agentPanelItems: Array<{ id: AgentPanelId; label: string }> = [
  { id: 'llm', label: 'Runtime' },
  { id: 'conversation', label: 'Conversation' },
  { id: 'content', label: 'Skills' },
  { id: 'market', label: 'Workflows' },
  { id: 'evolution', label: 'Evolution' },
  { id: 'search', label: 'Providers' },
];

export function agentPanelLabel(id: AgentPanelId) {
  return agentPanelItems.find((item) => item.id === id)?.label ?? 'Runtime';
}
