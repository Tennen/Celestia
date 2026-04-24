import { useEffect, useState } from 'react';
import { RefreshCw } from 'lucide-react';
import { Card, CardContent } from '../ui/card';
import { ScrollArea } from '../ui/scroll-area';
import { fetchAgentSnapshot, type AgentSnapshot } from '../../lib/agent';
import type { WorkflowPageId } from '../../lib/workflow-admin';
import { WorkflowCanvasPanel } from './workflow-canvas/WorkflowCanvasPanel';
import { WorkflowDirectoryPanel } from './workflow-canvas/WorkflowDirectoryPanel';

type WorkflowRunner = (label: string, action: () => Promise<unknown>, refresh?: boolean) => Promise<unknown>;

type Props = {
  activePage: WorkflowPageId;
  onSelectPage: (page: WorkflowPageId) => void;
};

export function WorkflowWorkspace({ activePage, onSelectPage }: Props) {
  const [snapshot, setSnapshot] = useState<AgentSnapshot | null>(null);
  const [busy, setBusy] = useState('load');
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [selectedWorkflowId, setSelectedWorkflowId] = useState('');

  const syncSnapshot = (next: AgentSnapshot) => {
    setSnapshot(next);
    setSelectedWorkflowId((current) => current || next.workflow.active_workflow_id || next.workflow.workflows[0]?.id || '');
  };

  const load = async () => {
    setBusy('load');
    setError('');
    try {
      syncSnapshot(await fetchAgentSnapshot());
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load workflow state');
    } finally {
      setBusy('');
    }
  };

  useEffect(() => {
    void load();
  }, []);

  useEffect(() => {
    setNotice('');
  }, [activePage]);

  const run: WorkflowRunner = async (label, action, refresh = true) => {
    setBusy(label);
    setError('');
    setNotice('');
    try {
      const output = await action();
      setNotice(label.includes('save') ? 'Saved' : 'Done');
      if (refresh) {
        syncSnapshot(await fetchAgentSnapshot());
      } else if (output && typeof output === 'object' && 'settings' in output) {
        syncSnapshot(output as AgentSnapshot);
      }
      return output;
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Request failed';
      setError(message);
      throw err;
    } finally {
      setBusy('');
    }
  };

  const openWorkflow = (workflowId: string) => {
    setSelectedWorkflowId(workflowId);
    onSelectPage('builder');
  };

  const createWorkflow = () => {
    setSelectedWorkflowId('');
    onSelectPage('builder');
  };

  if (!snapshot) {
    return (
      <Card className="panel">
        <CardContent className="flex items-center gap-2 p-6">
          <RefreshCw className="h-4 w-4 animate-spin" />
          Loading workflow runtime
        </CardContent>
      </Card>
    );
  }

  const content =
    activePage === 'list' ? (
      <ScrollArea className="detail-scroll">
        <div className="detail-stack agent-detail-stack">
          <WorkflowDirectoryPanel
            snapshot={snapshot}
            selectedWorkflowId={selectedWorkflowId}
            onCreateWorkflow={createWorkflow}
            onOpenWorkflow={openWorkflow}
          />
        </div>
      </ScrollArea>
    ) : (
      <div className="detail-stack agent-detail-stack workflow-workspace">
        <WorkflowCanvasPanel
          snapshot={snapshot}
          busy={busy}
          workflowId={selectedWorkflowId || undefined}
          onRun={run}
          onOpenList={() => onSelectPage('list')}
          onWorkflowSaved={setSelectedWorkflowId}
        />
      </div>
    );

  return (
    <>
      {error ? <div className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm">{error}</div> : null}
      {notice ? <div className="rounded-md border border-primary/20 bg-primary/5 p-3 text-sm">{notice}</div> : null}
      {content}
    </>
  );
}
