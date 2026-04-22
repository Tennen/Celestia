export type AgentPanelId = 'runtime' | 'wecom' | 'content' | 'ops';

export const agentPanelItems: Array<{ id: AgentPanelId; label: string }> = [
  { id: 'runtime', label: 'Runtime' },
  { id: 'wecom', label: 'WeCom' },
  { id: 'content', label: 'Content' },
  { id: 'ops', label: 'Ops' },
];

export function agentPanelLabel(id: AgentPanelId) {
  return agentPanelItems.find((item) => item.id === id)?.label ?? 'Runtime';
}
