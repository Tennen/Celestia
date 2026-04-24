import { Plus } from 'lucide-react';
import { Badge } from '../../ui/badge';
import { Button } from '../../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../../ui/card';
import type { AgentSnapshot } from '../../../lib/agent';
import { SelectableListItem } from '../shared/SelectableListItem';

type Props = {
  snapshot: AgentSnapshot;
  selectedWorkflowId: string;
  onCreateWorkflow: () => void;
  onOpenWorkflow: (workflowId: string) => void;
};

export function WorkflowDirectoryPanel({ snapshot, selectedWorkflowId, onCreateWorkflow, onOpenWorkflow }: Props) {
  const workflows = snapshot.workflow.workflows;
  const recentRuns = snapshot.workflow.runs.slice(0, 8);

  return (
    <div className="workflow-directory">
      <Card className="panel">
        <CardHeader>
          <CardTitle>Workflow Library</CardTitle>
          <CardDescription>{workflows.length} saved workflows</CardDescription>
        </CardHeader>
        <CardContent className="stack">
          <div className="button-row">
            <Button onClick={onCreateWorkflow}>
              <Plus className="mr-2 h-4 w-4" />
              New Workflow
            </Button>
          </div>
          <div className="list-stack">
            {workflows.map((workflow) => (
              <SelectableListItem
                key={workflow.id}
                title={workflow.name}
                description={`${workflow.nodes.length} nodes · ${workflow.edges.length} links`}
                selected={workflow.id === selectedWorkflowId}
                badges={
                  <Badge tone={workflow.id === snapshot.workflow.active_workflow_id ? 'accent' : 'neutral'} size="xxs">
                    {workflow.id === snapshot.workflow.active_workflow_id ? 'active' : 'saved'}
                  </Badge>
                }
                onClick={() => onOpenWorkflow(workflow.id)}
              />
            ))}
            {workflows.length === 0 ? <div className="detail">No workflow saved yet. Create one to start building on the canvas.</div> : null}
          </div>
        </CardContent>
      </Card>

      <Card className="panel">
        <CardHeader>
          <CardTitle>Recent Runs</CardTitle>
          <CardDescription>{snapshot.workflow.runs.length} recorded executions</CardDescription>
        </CardHeader>
        <CardContent className="workflow-canvas__runs">
          {recentRuns.map((run) => (
            <div key={run.id} className="workflow-canvas__run">
              <div className="workflow-canvas__run-head">
                <strong>{run.workflow_name || run.workflow_id || run.id}</strong>
                <Badge tone={run.status === 'succeeded' ? 'good' : run.status === 'degraded' ? 'warn' : 'bad'} size="xxs">
                  {run.status || 'unknown'}
                </Badge>
              </div>
              <div className="detail">{run.summary}</div>
            </div>
          ))}
          {recentRuns.length === 0 ? <div className="detail">No runs recorded.</div> : null}
        </CardContent>
      </Card>
    </div>
  );
}
