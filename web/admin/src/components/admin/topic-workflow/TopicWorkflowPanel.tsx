import { useEffect, useMemo, useState } from 'react';
import '@xyflow/react/dist/style.css';
import {
  addEdge,
  applyEdgeChanges,
  applyNodeChanges,
  Background,
  Controls,
  MiniMap,
  ReactFlow,
  type Connection,
  type Edge,
  type Node,
  type OnEdgesChange,
  type OnNodesChange,
} from '@xyflow/react';
import { Play, Plus, Save, Trash2 } from 'lucide-react';
import type { AgentRunner } from '../AgentWorkspace';
import { SelectableListItem } from '../shared/SelectableListItem';
import { Badge } from '../../ui/badge';
import { Button } from '../../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../../ui/card';
import { Textarea } from '../../ui/textarea';
import { Field } from '../AgentFormFields';
import type { AgentSnapshot } from '../../../lib/agent';
import {
  runAgentTopic,
  saveAgentTopic,
  type AgentTopicNode,
  type AgentTopicWorkflow,
} from '../../../lib/agent-topic';
import {
  cloneTopicWorkflow,
  createTopicNode,
  createTopicWorkflow,
  removeTopicEdgesForNode,
  removeTopicNode,
  removeTopicWorkflow,
  replaceTopicNode,
  replaceTopicWorkflow,
  topicWorkflowNodeCatalog,
  updateNodeData,
  workflowGroups,
  type TopicWorkflowNodeType,
} from '../../../lib/topic-workflow';
import { TopicWorkflowInspector } from './TopicWorkflowInspector';
import { topicWorkflowNodeTypes } from './TopicWorkflowNodes';

type Props = {
  snapshot: AgentSnapshot;
  busy: string;
  onRun: AgentRunner;
};

export function TopicWorkflowPanel({ snapshot, busy, onRun }: Props) {
  const firstWorkflow = snapshot.topic_summary.workflows[0] ?? createTopicWorkflow();
  const [workflowId, setWorkflowId] = useState(snapshot.topic_summary.active_workflow_id || firstWorkflow.id);
  const [draft, setDraft] = useState<AgentTopicWorkflow>(cloneTopicWorkflow(firstWorkflow));
  const [selectedNodeId, setSelectedNodeId] = useState('');

  useEffect(() => {
    const nextWorkflow =
      snapshot.topic_summary.workflows.find((workflow) => workflow.id === snapshot.topic_summary.active_workflow_id) ??
      snapshot.topic_summary.workflows[0] ??
      createTopicWorkflow();
    setWorkflowId(nextWorkflow.id);
    setDraft(cloneTopicWorkflow(nextWorkflow));
    setSelectedNodeId('');
  }, [snapshot]);

  const persisted = snapshot.topic_summary.workflows.some((workflow) => workflow.id === draft.id);
  const groups = workflowGroups(draft);
  const selectedNode = draft.nodes.find((node) => node.id === selectedNodeId) ?? null;

  const providerOptions = [
    { value: '', label: 'Default Provider' },
    ...snapshot.settings.llm_providers.map((provider) => ({
      value: provider.id,
      label: provider.name || provider.id,
    })),
  ];
  const searchProviderOptions = [
    { value: '', label: 'Default Search Profile' },
    ...(snapshot.settings.search_engines ?? []).map((provider) => ({
      value: String(provider.id ?? ''),
      label: String(provider.name ?? provider.id ?? provider.type ?? 'provider'),
    })),
  ];
  const wecomOptions = [
    { value: '', label: 'Select User' },
    ...snapshot.push.users
      .filter((user) => user.enabled)
      .map((user) => ({
        value: user.id,
        label: user.name || user.wecom_user || user.id,
      })),
  ];

  const flowNodes = useMemo(() => draft.nodes.map((node) => toFlowNode(node)), [draft.nodes]);
  const flowEdges = useMemo(() => draft.edges.map((edge) => ({ ...edge, id: edge.id } satisfies Edge)), [draft.edges]);

  const saveWorkflow = () => {
    const topic = {
      ...snapshot.topic_summary,
      active_workflow_id: draft.id,
      workflows: replaceTopicWorkflow(snapshot.topic_summary.workflows, draft),
    };
    onRun('topic-save', () => saveAgentTopic(topic));
  };

  const runWorkflow = () => {
    onRun('topic-run', () => runAgentTopic(draft.id));
  };

  const deleteWorkflow = () => {
    const workflows = removeTopicWorkflow(snapshot.topic_summary.workflows, draft.id);
    const nextActive = workflows[0]?.id ?? '';
    onRun('topic-save', () =>
      saveAgentTopic({
        ...snapshot.topic_summary,
        active_workflow_id: nextActive,
        workflows,
      }),
    );
  };

  const addNode = (type: TopicWorkflowNodeType) => {
    const node = createTopicNode(type, draft.nodes.length + 1);
    if (selectedNode?.type === 'group' && type !== 'group') {
      node.parent_id = selectedNode.id;
      node.position = { x: 24, y: 48 + draft.nodes.filter((item) => item.parent_id === selectedNode.id).length * 92 };
    }
    setDraft((current) => ({ ...current, nodes: [...current.nodes, node] }));
    setSelectedNodeId(node.id);
  };

  const onNodesChange: OnNodesChange = (changes) => {
    const nextNodes = applyNodeChanges(changes, flowNodes);
    setDraft((current) => ({ ...current, nodes: nextNodes.map((node) => fromFlowNode(node)) }));
  };

  const onEdgesChange: OnEdgesChange = (changes) => {
    const nextEdges = applyEdgeChanges(changes, flowEdges);
    setDraft((current) => ({ ...current, edges: nextEdges.map((edge) => fromFlowEdge(edge)) }));
  };

  const onConnect = (connection: Connection) => {
    const nextEdges = addEdge(
      {
        ...connection,
        id: `edge-${Date.now()}`,
      },
      flowEdges,
    );
    setDraft((current) => ({ ...current, edges: nextEdges.map((edge) => fromFlowEdge(edge)) }));
  };

  const updateSelectedNode = (nextNode: AgentTopicNode) => {
    setDraft((current) => ({
      ...current,
      nodes: replaceTopicNode(current.nodes, nextNode),
    }));
  };

  const latestRuns = snapshot.topic_summary.runs.slice(0, 6);

  return (
    <div className="topic-workflow">
      <div className="topic-workflow__grid">
        <Card className="panel">
          <CardHeader>
            <CardTitle>Topic Workflows</CardTitle>
            <CardDescription>{snapshot.topic_summary.workflows.length} saved workflows</CardDescription>
          </CardHeader>
          <CardContent className="stack">
            <div className="list-stack">
              {snapshot.topic_summary.workflows.map((workflow) => (
                <SelectableListItem
                  key={workflow.id}
                  title={workflow.name}
                  description={`${workflow.nodes.length} nodes · ${workflow.edges.length} links`}
                  selected={workflow.id === workflowId}
                  badges={
                    <Badge tone={workflow.id === snapshot.topic_summary.active_workflow_id ? 'accent' : 'neutral'} size="xxs">
                      {workflow.id === snapshot.topic_summary.active_workflow_id ? 'active' : 'saved'}
                    </Badge>
                  }
                  onClick={() => {
                    setWorkflowId(workflow.id);
                    setDraft(cloneTopicWorkflow(workflow));
                    setSelectedNodeId('');
                  }}
                />
              ))}
              {snapshot.topic_summary.workflows.length === 0 ? <div className="detail">No workflow saved yet.</div> : null}
            </div>
            <div className="button-row">
              <Button
                variant="secondary"
                onClick={() => {
                  const next = createTopicWorkflow();
                  setWorkflowId(next.id);
                  setDraft(next);
                  setSelectedNodeId('');
                }}
              >
                <Plus className="mr-2 h-4 w-4" />
                New Workflow
              </Button>
              <Button onClick={saveWorkflow} disabled={busy === 'topic-save' || !draft.name.trim()}>
                <Save className="mr-2 h-4 w-4" />
                Save
              </Button>
              <Button variant="secondary" onClick={runWorkflow} disabled={!persisted || busy === 'topic-run'}>
                <Play className="mr-2 h-4 w-4" />
                Run
              </Button>
              <Button variant="danger" onClick={deleteWorkflow} disabled={!persisted}>
                <Trash2 className="mr-2 h-4 w-4" />
                Delete
              </Button>
            </div>

            <Field label="Workflow Name" value={draft.name} onChange={(name) => setDraft((current) => ({ ...current, name }))} />
            <label className="stack text-sm font-medium">
              <span>Description</span>
              <Textarea
                value={draft.description ?? ''}
                onChange={(event) => setDraft((current) => ({ ...current, description: event.target.value }))}
                placeholder="Describe what this workflow is meant to do."
              />
            </label>

            <div className="stack">
              <div className="text-sm font-medium">Node Library</div>
              <div className="topic-workflow__library">
                {topicWorkflowNodeCatalog.map((item) => (
                  <button key={item.type} type="button" className="topic-workflow__library-item" onClick={() => addNode(item.type)}>
                    <strong>{item.label}</strong>
                    <span>{item.description}</span>
                  </button>
                ))}
              </div>
            </div>
          </CardContent>
        </Card>

        <div className="topic-workflow__workspace">
          <Card className="panel topic-workflow__canvas-card">
            <CardHeader>
              <CardTitle>Canvas</CardTitle>
              <CardDescription>
                Connect `RSS Sources` to `LLM` through `Prompt Unit` and optional `Search Provider`, then route `LLM` output into `WeCom Output`.
              </CardDescription>
            </CardHeader>
            <CardContent className="topic-workflow__canvas-content">
              <div className="topic-workflow__canvas">
                <ReactFlow
                  nodes={flowNodes}
                  edges={flowEdges}
                  onNodesChange={onNodesChange}
                  onEdgesChange={onEdgesChange}
                  onConnect={onConnect}
                  onNodeClick={(_, node) => setSelectedNodeId(node.id)}
                  onPaneClick={() => setSelectedNodeId('')}
                  fitView
                  nodeTypes={topicWorkflowNodeTypes}
                >
                  <MiniMap pannable zoomable />
                  <Controls />
                  <Background />
                </ReactFlow>
              </div>
            </CardContent>
          </Card>

          <Card className="panel topic-workflow__inspector-card">
            <CardHeader>
              <CardTitle>{selectedNode ? selectedNode.label || selectedNode.type : 'Inspector'}</CardTitle>
              <CardDescription>
                {selectedNode ? `Editing ${selectedNode.type}` : 'Select a node to configure its inputs, provider selection, and output target.'}
              </CardDescription>
            </CardHeader>
            <CardContent className="stack">
              {selectedNode ? (
                <TopicWorkflowInspector
                  node={selectedNode}
                  groups={groups}
                  providerOptions={providerOptions}
                  searchProviderOptions={searchProviderOptions}
                  wecomOptions={wecomOptions}
                  onChange={updateSelectedNode}
                  onDelete={() => {
                    setDraft((current) => ({
                      ...current,
                      nodes: removeTopicNode(current.nodes, selectedNode.id),
                      edges: removeTopicEdgesForNode(current.edges, selectedNode.id),
                    }));
                    setSelectedNodeId('');
                  }}
                />
              ) : (
                <div className="detail">Select a node on the canvas to edit it.</div>
              )}
            </CardContent>
          </Card>
        </div>
      </div>

      <Card className="panel">
        <CardHeader>
          <CardTitle>Recent Runs</CardTitle>
          <CardDescription>{snapshot.topic_summary.runs.length} recorded executions</CardDescription>
        </CardHeader>
        <CardContent className="topic-workflow__runs">
          {latestRuns.map((run) => (
            <div key={run.id} className="topic-workflow__run">
              <div className="topic-workflow__run-head">
                <strong>{run.workflow_name || run.workflow_id || run.id}</strong>
                <Badge tone={run.status === 'succeeded' ? 'good' : run.status === 'degraded' ? 'warn' : 'bad'} size="xxs">
                  {run.status || 'unknown'}
                </Badge>
              </div>
              <div className="detail">{run.summary}</div>
              {run.output_text ? <Textarea readOnly value={run.output_text} /> : null}
            </div>
          ))}
          {latestRuns.length === 0 ? <div className="detail">No runs recorded.</div> : null}
        </CardContent>
      </Card>
    </div>
  );
}

function toFlowNode(node: AgentTopicNode): Node {
  const isGroup = node.type === 'group';
  return {
    id: node.id,
    type: isGroup ? 'topicGroupNode' : 'topicWorkflowNode',
    position: node.position,
    parentId: node.parent_id || undefined,
    extent: node.parent_id ? 'parent' : undefined,
    width: node.width,
    height: node.height,
    data: {
      title: node.label || node.type,
      nodeType: node.type,
      payload: node.data ?? {},
    },
    style: isGroup
      ? {
          width: node.width ?? 360,
          height: node.height ?? 240,
        }
      : undefined,
  };
}

function fromFlowNode(node: Node): AgentTopicNode {
  return {
    id: node.id,
    type: String(node.data?.nodeType ?? ''),
    label: String(node.data?.title ?? ''),
    parent_id: node.parentId,
    position: node.position,
    width: typeof node.width === 'number' ? node.width : undefined,
    height: typeof node.height === 'number' ? node.height : undefined,
    data: (node.data?.payload as Record<string, unknown>) ?? {},
  };
}

function fromFlowEdge(edge: Edge) {
  return {
    id: edge.id,
    source: edge.source,
    source_handle: edge.sourceHandle ?? undefined,
    target: edge.target,
    target_handle: edge.targetHandle ?? undefined,
    label: typeof edge.label === 'string' ? edge.label : '',
  };
}
