import type { ReactNode } from 'react';
import { Handle, NodeResizer, Position, type NodeProps } from '@xyflow/react';

const workflowCanvasGridSize = 18;
const workflowNodeMinSize = workflowCanvasGridSize;

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
  minWidth = workflowNodeMinSize,
  minHeight = workflowNodeMinSize,
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
    case 'timer':
      const timerDescription =
        view.payload?.schedule === 'interval'
          ? 'Trigger: starts on each interval, optionally gated by a sidecar Time Window.'
          : 'Trigger: starts on the configured daily schedule.';
      return (
        <NodeShell title={view.title} description={timerDescription} selected={selected}>
          <Handle className="workflow-node__handle nodrag nopan" type="target" position={Position.Left} id="window" />
          <Handle className="workflow-node__handle nodrag nopan" type="source" position={Position.Bottom} id="trigger" />
        </NodeShell>
      );
    case 'device_state_changed':
      return (
        <NodeShell title={view.title} description="Trigger: starts when the configured state transition occurs." selected={selected}>
          <Handle className="workflow-node__handle nodrag nopan" type="target" position={Position.Left} id="window" />
          <Handle className="workflow-node__handle nodrag nopan" type="source" position={Position.Bottom} id="trigger" />
        </NodeShell>
      );
    case 'device_state_is':
      return (
        <NodeShell title={view.title} description="Trigger or gate: passes when current state matches." selected={selected}>
          <Handle className="workflow-node__handle nodrag nopan" type="target" position={Position.Top} id="trigger" />
          <Handle className="workflow-node__handle nodrag nopan" type="target" position={Position.Left} id="window" />
          <Handle className="workflow-node__handle nodrag nopan" type="source" position={Position.Bottom} id="trigger" />
        </NodeShell>
      );
    case 'time_window':
      return (
        <NodeShell title={view.title} description="Accessory: connect to a trigger's sidecar window port." selected={selected}>
          <Handle className="workflow-node__handle nodrag nopan" type="source" position={Position.Bottom} id="gate" />
        </NodeShell>
      );
    case 'rss_sources':
      return (
        <NodeShell title={view.title} description="Polls RSS/Atom feeds and emits only newly appeared items." selected={selected}>
          <Handle className="workflow-node__handle nodrag nopan" type="target" position={Position.Top} id="trigger" />
          <Handle className="workflow-node__handle nodrag nopan" type="source" position={Position.Bottom} id="content" />
        </NodeShell>
      );
    case 'text':
      return (
        <NodeShell title="" selected={selected}>
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
        <NodeShell title={view.title} description="Consumes prompt, context, and optional search input." selected={selected}>
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
    case 'device_command':
      return (
        <NodeShell title={view.title} description="Action: sends a real command through the gateway." selected={selected}>
          <Handle className="workflow-node__handle nodrag nopan" type="target" position={Position.Top} id="trigger" />
        </NodeShell>
      );
    case 'agent_function':
      return (
        <NodeShell title={view.title} description="Action: runs project input or Agent text." selected={selected}>
          <Handle className="workflow-node__handle nodrag nopan" type="target" position={Position.Top} id="trigger" />
          <Handle className="workflow-node__handle nodrag nopan" type="target" position={Position.Left} id="text" />
          <Handle className="workflow-node__handle nodrag nopan" type="source" position={Position.Bottom} id="text" />
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
        minWidth={workflowNodeMinSize}
        minHeight={workflowNodeMinSize}
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
