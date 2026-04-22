export type AgentPanelId = 'runtime' | 'wecom' | 'content' | 'ops';

export const agentPanelItems: Array<{ id: AgentPanelId; label: string; badge: string }> = [
  { id: 'runtime', label: 'Runtime', badge: 'Core' },
  { id: 'wecom', label: 'WeCom', badge: 'Msg' },
  { id: 'content', label: 'Content', badge: 'Data' },
  { id: 'ops', label: 'Ops', badge: 'Ops' },
];

export function agentPanelLabel(id: AgentPanelId) {
  return agentPanelItems.find((item) => item.id === id)?.label ?? 'Runtime';
}
