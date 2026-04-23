export type AgentPanelId = 'llm' | 'conversation' | 'content' | 'market' | 'evolution' | 'search';

export const agentPanelItems: Array<{ id: AgentPanelId; label: string }> = [
  { id: 'llm', label: 'LLM' },
  { id: 'conversation', label: 'Conversation' },
  { id: 'content', label: 'Content' },
  { id: 'market', label: 'Market' },
  { id: 'evolution', label: 'Evolution' },
  { id: 'search', label: 'Search' },
];

export function agentPanelLabel(id: AgentPanelId) {
  return agentPanelItems.find((item) => item.id === id)?.label ?? 'LLM';
}
