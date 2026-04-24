import { useEffect, useRef, useState } from 'react';
import { CircleAlert, CircleCheck, RefreshCw, X } from 'lucide-react';
import { cn } from '../../lib/utils';
import { Button } from '../ui/button';
import { Card, CardContent } from '../ui/card';
import { ScrollArea } from '../ui/scroll-area';
import { fetchAgentSnapshot, type AgentSnapshot } from '../../lib/agent';
import type { WorkflowPageId } from '../../lib/workflow-admin';
import { WorkflowCanvasPanel } from './workflow-canvas/WorkflowCanvasPanel';
import { WorkflowDirectoryPanel } from './workflow-canvas/WorkflowDirectoryPanel';

type WorkflowRunner = (label: string, action: () => Promise<unknown>, refresh?: boolean) => Promise<unknown>;
type WorkflowNoticeTone = 'success' | 'error';
type WorkflowNotice = {
  id: number;
  tone: WorkflowNoticeTone;
  title: string;
  message: string;
};

type Props = {
  activePage: WorkflowPageId;
  onSelectPage: (page: WorkflowPageId) => void;
};

export function WorkflowWorkspace({ activePage, onSelectPage }: Props) {
  const [snapshot, setSnapshot] = useState<AgentSnapshot | null>(null);
  const [busy, setBusy] = useState('load');
  const [loadError, setLoadError] = useState('');
  const [notice, setNotice] = useState<WorkflowNotice | null>(null);
  const [selectedWorkflowId, setSelectedWorkflowId] = useState('');

  const showNotice = (tone: WorkflowNoticeTone, title: string, message: string) => {
    setNotice({
      id: Date.now(),
      tone,
      title,
      message,
    });
  };

  const syncSnapshot = (next: AgentSnapshot) => {
    setSnapshot(next);
    setSelectedWorkflowId((current) => current || next.workflow.active_workflow_id || next.workflow.workflows[0]?.id || '');
  };

  const load = async () => {
    setBusy('load');
    setLoadError('');
    try {
      syncSnapshot(await fetchAgentSnapshot());
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to load workflow state';
      setLoadError(message);
      showNotice('error', 'Load Failed', message);
    } finally {
      setBusy('');
    }
  };

  useEffect(() => {
    void load();
  }, []);

  useEffect(() => {
    setNotice(null);
  }, [activePage]);

  const run: WorkflowRunner = async (label, action, refresh = true) => {
    setBusy(label);
    setNotice(null);
    try {
      const output = await action();
      showNotice('success', label.includes('save') ? 'Saved' : 'Done', label.includes('save') ? 'Workflow changes were saved.' : 'Workflow action completed.');
      if (refresh) {
        syncSnapshot(await fetchAgentSnapshot());
      } else if (output && typeof output === 'object' && 'settings' in output) {
        syncSnapshot(output as AgentSnapshot);
      }
      return output;
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Request failed';
      showNotice('error', 'Error', message);
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
      <div className="workflow-workspace__frame">
        {notice ? <WorkflowFloatingNotice notice={notice} onClose={() => setNotice(null)} /> : null}
        <Card className="panel">
          <CardContent className="flex items-center gap-3 p-6">
            {loadError ? (
              <>
                <CircleAlert className="h-4 w-4 text-destructive" />
                <div className="min-w-0 flex-1 text-sm">{loadError}</div>
                <Button variant="secondary" onClick={() => void load()} disabled={busy === 'load'}>
                  Retry
                </Button>
              </>
            ) : (
              <>
                <RefreshCw className="h-4 w-4 animate-spin" />
                Loading workflow runtime
              </>
            )}
          </CardContent>
        </Card>
      </div>
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
    <div className="workflow-workspace__frame">
      {notice ? <WorkflowFloatingNotice notice={notice} onClose={() => setNotice(null)} /> : null}
      {content}
    </div>
  );
}

function WorkflowFloatingNotice({ notice, onClose }: { notice: WorkflowNotice; onClose: () => void }) {
  const timerRef = useRef<number | null>(null);
  const startedAtRef = useRef(0);
  const remainingRef = useRef(5000);
  const [paused, setPaused] = useState(false);

  const clearTimer = () => {
    if (timerRef.current !== null) {
      window.clearTimeout(timerRef.current);
      timerRef.current = null;
    }
  };

  useEffect(() => {
    remainingRef.current = 5000;
    setPaused(false);
  }, [notice.id]);

  useEffect(() => {
    clearTimer();
    if (paused) {
      return undefined;
    }
    startedAtRef.current = Date.now();
    timerRef.current = window.setTimeout(() => {
      onClose();
    }, remainingRef.current);
    return clearTimer;
  }, [notice.id, onClose, paused]);

  useEffect(() => clearTimer, []);

  const pauseTimer = () => {
    if (paused) {
      return;
    }
    remainingRef.current = Math.max(0, remainingRef.current - (Date.now() - startedAtRef.current));
    clearTimer();
    setPaused(true);
  };

  const resumeTimer = () => {
    if (!paused) {
      return;
    }
    setPaused(false);
  };

  return (
    <div
      className={cn('workflow-workspace__notice', notice.tone === 'error' ? 'is-error' : 'is-success')}
      onMouseEnter={pauseTimer}
      onMouseLeave={resumeTimer}
      role={notice.tone === 'error' ? 'alert' : 'status'}
      aria-live="polite"
    >
      <div className="workflow-workspace__notice-icon">
        {notice.tone === 'error' ? <CircleAlert className="h-4 w-4" /> : <CircleCheck className="h-4 w-4" />}
      </div>
      <div className="workflow-workspace__notice-copy">
        <strong>{notice.title}</strong>
        <span>{notice.message}</span>
      </div>
      <button type="button" className="workflow-workspace__notice-close" onClick={onClose} aria-label="Close notification">
        <X className="h-4 w-4" />
      </button>
    </div>
  );
}
