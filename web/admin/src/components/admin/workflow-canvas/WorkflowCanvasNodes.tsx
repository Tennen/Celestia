import type { ReactNode } from 'react';
import { Handle, NodeResizer, Position, type NodeProps } from '@xyflow/react';

type WorkflowNodeData = {
  title: string;
  nodeType: string;
  description?: string;
  payload?: Record<string, unknown>;
  onTextChange?: (value: string) => void;
};

function NodeShell({
  title,
  description,
  selected,
  minWidth = 190,
  minHeight = 108,
  children,
}: {
  title: string;
  description?: string;
  selected: boolean;
  minWidth?: number;
  minHeight?: number;
  children?: ReactNode;
}) {
  return (
    <div className={`workflow-node${selected ? ' is-selected' : ''}`}>
      <NodeResizer
        isVisible={selected}
        minWidth={minWidth}
        minHeight={minHeight}
        lineClassName="workflow-node__resize-line"
        handleClassName="workflow-node__resize-handle nodrag nopan"
      />
      {title ? <div className="workflow-node__title">{title}</div> : null}
      {description ? <div className="workflow-node__desc">{description}</div> : null}
      {children}
    </div>
  );
}

export function WorkflowCanvasNode({ data, selected }: NodeProps) {
  const view = (data ?? {}) as WorkflowNodeData;
  switch (view.nodeType) {
    case 'rss_sources':
      return (
        <NodeShell title={view.title} description="Fetches feed items and emits workflow context." selected={selected}>
          <Handle className="workflow-node__handle nodrag nopan" type="source" position={Position.Bottom} id="content" />
        </NodeShell>
      );
    case 'text':
      return (
        <NodeShell title="" selected={selected} minWidth={240} minHeight={148}>
          <Handle className="workflow-node__handle nodrag nopan" type="target" position={Position.Top} id="text" />
          <textarea
            className="workflow-node__text-editor nodrag nopan"
            value={String(view.payload?.text ?? view.payload?.prompt ?? '')}
            onChange={(event) => view.onTextChange?.(event.target.value)}
            placeholder="Write text here..."
          />
          <Handle className="workflow-node__handle nodrag nopan" type="source" position={Position.Bottom} id="text" />
        </NodeShell>
      );
    case 'search_provider':
      return (
        <NodeShell title={view.title} description="Runs a configured search provider." selected={selected}>
          <Handle className="workflow-node__handle nodrag nopan" type="source" position={Position.Bottom} id="search" />
        </NodeShell>
      );
    case 'llm':
      return (
        <NodeShell title={view.title} description="Consumes prompt, context, and optional search input." selected={selected} minHeight={128}>
          <Handle className="workflow-node__handle nodrag nopan" type="target" position={Position.Top} id="prompt" />
          <Handle className="workflow-node__handle nodrag nopan" type="target" position={Position.Left} id="context" />
          <Handle className="workflow-node__handle nodrag nopan" type="target" position={Position.Right} id="search" />
          <Handle className="workflow-node__handle nodrag nopan" type="target" position={Position.Left} id="skill" style={{ top: '72%' }} />
          <Handle className="workflow-node__handle nodrag nopan" type="target" position={Position.Right} id="tool" style={{ top: '72%' }} />
          <Handle className="workflow-node__handle nodrag nopan" type="source" position={Position.Bottom} id="text" />
          <div className="workflow-node__ports">
            <span>skill</span>
            <span>tool</span>
          </div>
        </NodeShell>
      );
    case 'wecom_output':
      return (
        <NodeShell title={view.title} description="Delivers the generated text to WeCom." selected={selected}>
          <Handle className="workflow-node__handle nodrag nopan" type="target" position={Position.Top} id="text" />
        </NodeShell>
      );
    default:
      return <NodeShell title={view.title} description={view.description} selected={selected} />;
  }
}

export function WorkflowCanvasGroupNode({ data, selected }: NodeProps) {
  const view = (data ?? {}) as WorkflowNodeData;
  return (
    <div className={`workflow-group-node${selected ? ' is-selected' : ''}`}>
      <NodeResizer
        isVisible={selected}
        minWidth={260}
        minHeight={180}
        lineClassName="workflow-node__resize-line"
        handleClassName="workflow-node__resize-handle nodrag nopan"
      />
      <div className="workflow-group-node__title">{view.title}</div>
    </div>
  );
}

export const workflowCanvasNodeTypes = {
  workflowCanvasNode: WorkflowCanvasNode,
  workflowGroupNode: WorkflowCanvasGroupNode,
};
