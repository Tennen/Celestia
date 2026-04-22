export type AgentPanelId = 'llm' | 'wecom' | 'input' | 'content' | 'market' | 'evolution' | 'search';

export const agentPanelItems: Array<{ id: AgentPanelId; label: string }> = [
  { id: 'llm', label: 'LLM' },
  { id: 'wecom', label: 'WeCom' },
  { id: 'input', label: 'Input' },
  { id: 'content', label: 'Content' },
  { id: 'market', label: 'Market' },
  { id: 'evolution', label: 'Evolution' },
  { id: 'search', label: 'Search' },
];

export function agentPanelLabel(id: AgentPanelId) {
  return agentPanelItems.find((item) => item.id === id)?.label ?? 'LLM';
}
