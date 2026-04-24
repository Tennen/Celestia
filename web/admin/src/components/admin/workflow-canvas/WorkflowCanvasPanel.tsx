import { useEffect, useState } from 'react';
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
import { ArrowLeft, Play, Plus, Save, Trash2 } from 'lucide-react';
import { Badge } from '../../ui/badge';
import { Button } from '../../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../../ui/card';
import { ScrollArea } from '../../ui/scroll-area';
import { Textarea } from '../../ui/textarea';
import { Field } from '../AgentFormFields';
import type { AgentSnapshot } from '../../../lib/agent';
import {
  runAgentWorkflow,
  saveAgentWorkflow,
  type AgentWorkflowDefinition,
  type AgentWorkflowNode,
} from '../../../lib/agent-workflow';
import {
  cloneWorkflow,
  createWorkflowDefinition,
  createWorkflowNode,
  removeWorkflowEdgesForNode,
  removeWorkflowNode,
  removeWorkflowDefinition,
  replaceWorkflowDefinition,
  replaceWorkflowNode,
  workflowGroups,
  workflowNodeCatalog,
  type WorkflowNodeType,
} from '../../../lib/workflow-canvas';
import { WorkflowCanvasInspector } from './WorkflowCanvasInspector';
import { workflowCanvasNodeTypes } from './WorkflowCanvasNodes';

type WorkflowRunner = (label: string, action: () => Promise<unknown>, refresh?: boolean) => Promise<unknown>;

type Props = {
  snapshot: AgentSnapshot;
  busy: string;
  workflowId?: string;
  onRun: WorkflowRunner;
  onOpenList: () => void;
  onWorkflowSaved: (workflowId: string) => void;
};

export function WorkflowCanvasPanel({ snapshot, busy, workflowId, onRun, onOpenList, onWorkflowSaved }: Props) {
  const [draft, setDraft] = useState<AgentWorkflowDefinition>(() => buildDraft(snapshot, workflowId));
  const [flowNodes, setFlowNodes] = useState<Node[]>(() => buildDraft(snapshot, workflowId).nodes.map((node) => toFlowNode(node)));
  const [flowEdges, setFlowEdges] = useState<Edge[]>(() => buildDraft(snapshot, workflowId).edges.map((edge) => toFlowEdge(edge)));
  const [selectedNodeId, setSelectedNodeId] = useState('');

  useEffect(() => {
    const nextDraft = buildDraft(snapshot, workflowId);
    setDraft(nextDraft);
    setFlowNodes(nextDraft.nodes.map((node) => toFlowNode(node)));
    setFlowEdges(nextDraft.edges.map((edge) => toFlowEdge(edge)));
    setSelectedNodeId('');
  }, [snapshot, workflowId]);

  const persisted = snapshot.workflow.workflows.some((workflow) => workflow.id === draft.id);
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

  const saveWorkflow = async () => {
    onWorkflowSaved(draft.id);
    await onRun('workflow-save', () =>
      saveAgentWorkflow({
        ...snapshot.workflow,
        active_workflow_id: draft.id,
        workflows: replaceWorkflowDefinition(snapshot.workflow.workflows, draft),
      }),
    );
  };

  const runWorkflow = async () => {
    await onRun('workflow-run', () => runAgentWorkflow(draft.id));
  };

  const deleteWorkflow = async () => {
    const workflows = removeWorkflowDefinition(snapshot.workflow.workflows, draft.id);
    const nextActive = workflows[0]?.id ?? '';
    await onRun('workflow-save', () =>
      saveAgentWorkflow({
        ...snapshot.workflow,
        active_workflow_id: nextActive,
        workflows,
      }),
    );
    onOpenList();
  };

  const addNode = (type: WorkflowNodeType) => {
    const node = createWorkflowNode(type, draft.nodes.length + 1);
    if (selectedNode?.type === 'group' && type !== 'group') {
      node.parent_id = selectedNode.id;
      node.position = { x: 24, y: 48 + draft.nodes.filter((item) => item.parent_id === selectedNode.id).length * 92 };
    }
    setDraft((current) => ({ ...current, nodes: [...current.nodes, node] }));
    setFlowNodes((current) => [
      ...current.map((item) => ({ ...item, selected: false })),
      {
        ...toFlowNode(node),
        selected: true,
      },
    ]);
    setSelectedNodeId(node.id);
  };

  const onNodesChange: OnNodesChange = (changes) => {
    setFlowNodes((current) => {
      const nextNodes = applyNodeChanges(changes, current);
      setDraft((draftState) => ({ ...draftState, nodes: nextNodes.map((node) => fromFlowNode(node)) }));
      setSelectedNodeId(nextNodes.find((node) => node.selected)?.id ?? '');
      return nextNodes;
    });
  };

  const onEdgesChange: OnEdgesChange = (changes) => {
    setFlowEdges((current) => {
      const nextEdges = applyEdgeChanges(changes, current);
      setDraft((draftState) => ({ ...draftState, edges: nextEdges.map((edge) => fromFlowEdge(edge)) }));
      return nextEdges;
    });
  };

  const onConnect = (connection: Connection) => {
    setFlowEdges((current) => {
      const nextEdges = addEdge(
        {
          ...connection,
          id: `edge-${Date.now()}`,
        },
        current,
      );
      setDraft((draftState) => ({ ...draftState, edges: nextEdges.map((edge) => fromFlowEdge(edge)) }));
      return nextEdges;
    });
  };

  const updateSelectedNode = (nextNode: AgentWorkflowNode) => {
    setDraft((current) => ({
      ...current,
      nodes: replaceWorkflowNode(current.nodes, nextNode),
    }));
    setFlowNodes((current) =>
      current.map((node) =>
        node.id === nextNode.id
          ? {
              ...toFlowNode(nextNode),
              selected: node.selected,
            }
          : node,
      ),
    );
  };

  return (
    <div className="workflow-builder">
      <Card className="panel workflow-builder__toolbar-card">
        <CardContent className="workflow-builder__toolbar-content">
          <div className="workflow-builder__header">
            <div className="stack">
              <div className="workflow-builder__headline">
                <div>
                  <p className="eyebrow">Workflow Builder</p>
                  <CardTitle>{persisted ? draft.name || 'Workflow Builder' : 'New Workflow'}</CardTitle>
                </div>
                <div className="workflow-builder__status">
                  <Badge tone={persisted ? 'accent' : 'neutral'} size="xxs">
                    {persisted ? 'saved' : 'draft'}
                  </Badge>
                  <span className="detail">{draft.nodes.length} nodes · {draft.edges.length} links</span>
                </div>
              </div>
              <CardDescription>Build the graph here, then save it as a reusable workflow definition.</CardDescription>
            </div>
            <div className="button-row workflow-builder__actions">
              <Button variant="secondary" onClick={onOpenList}>
                <ArrowLeft className="mr-2 h-4 w-4" />
                Back to List
              </Button>
              <Button onClick={() => void saveWorkflow()} disabled={busy === 'workflow-save' || !draft.name.trim()}>
                <Save className="mr-2 h-4 w-4" />
                Save
              </Button>
              <Button variant="secondary" onClick={() => void runWorkflow()} disabled={!persisted || busy === 'workflow-run'}>
                <Play className="mr-2 h-4 w-4" />
                Run
              </Button>
              <Button variant="danger" onClick={() => void deleteWorkflow()} disabled={!persisted || busy === 'workflow-save'}>
                <Trash2 className="mr-2 h-4 w-4" />
                Delete
              </Button>
            </div>
          </div>

          <div className="workflow-builder__meta">
            <Field label="Workflow Name" value={draft.name} onChange={(name) => setDraft((current) => ({ ...current, name }))} />
            <label className="stack text-sm font-medium">
              <span>Description</span>
              <Textarea
                value={draft.description ?? ''}
                onChange={(event) => setDraft((current) => ({ ...current, description: event.target.value }))}
                placeholder="Describe what this workflow does."
              />
            </label>
          </div>
        </CardContent>
      </Card>

      <div className="workflow-builder__workspace">
        <Card className="panel workflow-builder__canvas-card">
          <CardHeader>
            <CardTitle>Canvas</CardTitle>
            <CardDescription>Connect inputs, prompts, models, providers, and outputs into one reusable workflow.</CardDescription>
          </CardHeader>
          <CardContent className="workflow-builder__canvas-content">
            <div className="workflow-builder__canvas">
              {flowNodes.length === 0 ? (
                <div className="workflow-builder__canvas-empty">
                  <strong>Start from the right palette</strong>
                  <span>Add RSS, prompt, model, provider, and output nodes to begin wiring this workflow.</span>
                </div>
              ) : null}
              <ReactFlow
                nodes={flowNodes}
                edges={flowEdges}
                onNodesChange={onNodesChange}
                onEdgesChange={onEdgesChange}
                onConnect={onConnect}
                onNodeClick={(_, node) => setSelectedNodeId(node.id)}
                onSelectionChange={({ nodes }) => setSelectedNodeId(nodes[0]?.id ?? '')}
                onPaneClick={() => setSelectedNodeId('')}
                panOnDrag={[1, 2]}
                connectionRadius={28}
                fitView
                nodeTypes={workflowCanvasNodeTypes}
              >
                <MiniMap pannable zoomable />
                <Controls />
                <Background gap={18} />
              </ReactFlow>
            </div>
          </CardContent>
        </Card>

        <div className="workflow-builder__aside">
          <Card className="panel workflow-builder__library-card">
            <CardHeader>
              <CardTitle>Components</CardTitle>
              <CardDescription>Add modules directly from the palette on the right of the canvas.</CardDescription>
            </CardHeader>
            <CardContent className="workflow-builder__library-content">
              <ScrollArea className="workflow-builder__library-scroll">
                <div className="workflow-builder__library">
                  {workflowNodeCatalog.map((item) => (
                    <button key={item.type} type="button" className="workflow-builder__library-item" onClick={() => addNode(item.type)}>
                      <div className="workflow-builder__library-item-head">
                        <Plus className="h-3.5 w-3.5" />
                        <span>{item.label}</span>
                      </div>
                      <span className="workflow-builder__library-item-desc">{item.description}</span>
                    </button>
                  ))}
                </div>
              </ScrollArea>
            </CardContent>
          </Card>

          <Card className="panel workflow-builder__inspector-card">
            <CardHeader>
              <CardTitle>{selectedNode ? selectedNode.label || selectedNode.type : 'Inspector'}</CardTitle>
              <CardDescription>
                {selectedNode ? `Editing ${selectedNode.type}` : 'Select a node to configure its inputs, provider selection, and output target.'}
              </CardDescription>
            </CardHeader>
            <CardContent className="stack workflow-builder__inspector-content">
              {selectedNode ? (
                <WorkflowCanvasInspector
                  node={selectedNode}
                  groups={groups}
                  providerOptions={providerOptions}
                  searchProviderOptions={searchProviderOptions}
                  wecomOptions={wecomOptions}
                  onChange={updateSelectedNode}
                  onDelete={() => {
                    setDraft((current) => {
                      const nextNodes = removeWorkflowNode(current.nodes, selectedNode.id);
                      const nextEdges = removeWorkflowEdgesForNode(current.edges, selectedNode.id);
                      setFlowNodes(nextNodes.map((node) => toFlowNode(node)));
                      setFlowEdges(nextEdges.map((edge) => toFlowEdge(edge)));
                      return {
                        ...current,
                        nodes: nextNodes,
                        edges: nextEdges,
                      };
                    });
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
    </div>
  );
}

function buildDraft(snapshot: AgentSnapshot, workflowId?: string) {
  const current = snapshot.workflow.workflows.find((workflow) => workflow.id === workflowId);
  return current ? cloneWorkflow(current) : createWorkflowDefinition();
}

function toFlowNode(node: AgentWorkflowNode): Node {
  const isGroup = node.type === 'group';
  const style =
    typeof node.width === 'number' || typeof node.height === 'number'
      ? {
          width: node.width,
          height: node.height,
        }
      : undefined;
  return {
    id: node.id,
    type: isGroup ? 'workflowGroupNode' : 'workflowCanvasNode',
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
    style: isGroup ? { width: node.width ?? 360, height: node.height ?? 240 } : style,
  };
}

function toFlowEdge(edge: { id: string; source: string; source_handle?: string; target: string; target_handle?: string; label?: string }) {
  return {
    id: edge.id,
    source: edge.source,
    sourceHandle: edge.source_handle ?? undefined,
    target: edge.target,
    targetHandle: edge.target_handle ?? undefined,
    label: edge.label ?? '',
  } satisfies Edge;
}

function fromFlowNode(node: Node): AgentWorkflowNode {
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
