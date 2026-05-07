import { PencilLine, Plus } from 'lucide-react';
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
  onEditWorkflow: (workflowId: string) => void;
};

export function WorkflowDirectoryPanel({ snapshot, selectedWorkflowId, onCreateWorkflow, onOpenWorkflow, onEditWorkflow }: Props) {
  const workflows = snapshot.workflow.workflows;
  const selectedWorkflow = workflows.find((workflow) => workflow.id === selectedWorkflowId) ?? workflows[0] ?? null;
  const recentRuns = selectedWorkflow
    ? snapshot.workflow.runs.filter((run) => run.workflow_id === selectedWorkflow.id).slice(0, 8)
    : [];

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

      <div className="stack">
        <Card className="panel">
          <CardHeader>
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0">
                <CardTitle>{selectedWorkflow?.name || 'Select a workflow'}</CardTitle>
                <CardDescription>
                  {selectedWorkflow
                    ? `${selectedWorkflow.nodes.length} nodes · ${selectedWorkflow.edges.length} links`
                    : 'Choose a workflow from the list to inspect its runtime details.'}
                </CardDescription>
              </div>
              {selectedWorkflow ? (
                <Button variant="secondary" onClick={() => onEditWorkflow(selectedWorkflow.id)}>
                  <PencilLine className="mr-2 h-4 w-4" />
                  Edit
                </Button>
              ) : null}
            </div>
          </CardHeader>
          {selectedWorkflow ? (
            <CardContent className="stack">
              <div className="detail">{selectedWorkflow.description || 'No description.'}</div>
              <div className="button-row">
                <Badge tone={selectedWorkflow.id === snapshot.workflow.active_workflow_id ? 'accent' : 'neutral'} size="xxs">
                  {selectedWorkflow.id === snapshot.workflow.active_workflow_id ? 'active' : 'saved'}
                </Badge>
                <Badge tone="neutral" size="xxs">
                  updated {new Date(selectedWorkflow.updated_at).toLocaleString()}
                </Badge>
              </div>
            </CardContent>
          ) : null}
        </Card>

        <Card className="panel">
          <CardHeader>
            <CardTitle>Recent Runs</CardTitle>
            <CardDescription>
              {selectedWorkflow ? `${recentRuns.length} recent executions for this workflow` : 'Select a workflow to view recorded executions'}
            </CardDescription>
          </CardHeader>
          <CardContent className="workflow-canvas__runs">
            {recentRuns.map((run) => (
              <div key={run.id} className="workflow-canvas__run">
                <div className="workflow-canvas__run-head">
                  <strong>{run.summary || run.id}</strong>
                  <Badge tone={run.status === 'succeeded' ? 'good' : run.status === 'degraded' ? 'warn' : 'bad'} size="xxs">
                    {run.status || 'unknown'}
                  </Badge>
                </div>
                <div className="detail">{new Date(run.created_at).toLocaleString()}</div>
              </div>
            ))}
            {selectedWorkflow && recentRuns.length === 0 ? <div className="detail">No runs recorded for this workflow.</div> : null}
            {!selectedWorkflow ? <div className="detail">No workflow selected.</div> : null}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
