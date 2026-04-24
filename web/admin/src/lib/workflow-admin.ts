export type WorkflowPageId = 'list' | 'builder';

export const workflowPageItems: Array<{ id: WorkflowPageId; label: string }> = [
  { id: 'list', label: 'List' },
  { id: 'builder', label: 'Builder' },
];

export function workflowPageLabel(id: WorkflowPageId) {
  return workflowPageItems.find((item) => item.id === id)?.label ?? 'List';
}
