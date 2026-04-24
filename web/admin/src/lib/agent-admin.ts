export type AgentPanelId = 'llm' | 'conversation' | 'writing' | 'market' | 'evolution' | 'search';

export const agentPanelItems: Array<{ id: AgentPanelId; label: string }> = [
  { id: 'llm', label: 'Runtime' },
  { id: 'conversation', label: 'Conversation' },
  { id: 'writing', label: 'Writing' },
  { id: 'market', label: 'Market' },
  { id: 'evolution', label: 'Evolution' },
  { id: 'search', label: 'Providers' },
];

export function agentPanelLabel(id: AgentPanelId) {
  return agentPanelItems.find((item) => item.id === id)?.label ?? 'Runtime';
}
