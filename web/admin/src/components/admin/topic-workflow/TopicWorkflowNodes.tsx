import type { ReactNode } from 'react';
import { Handle, Position, type NodeProps } from '@xyflow/react';

type WorkflowNodeData = {
  title: string;
  nodeType: string;
  description?: string;
};

function NodeShell({ title, description, children }: { title: string; description?: string; children?: ReactNode }) {
  return (
    <div className="topic-node">
      <div className="topic-node__title">{title}</div>
      {description ? <div className="topic-node__desc">{description}</div> : null}
      {children}
    </div>
  );
}

export function TopicWorkflowNode({ data }: NodeProps) {
  const view = (data ?? {}) as WorkflowNodeData;
  switch (view.nodeType) {
    case 'rss_sources':
      return (
        <NodeShell title={view.title} description="Fetches feed items and emits workflow context.">
          <Handle type="source" position={Position.Bottom} id="content" />
        </NodeShell>
      );
    case 'prompt_unit':
      return (
        <NodeShell title={view.title} description="Reusable instruction block for downstream LLM nodes.">
          <Handle type="source" position={Position.Bottom} id="prompt" />
        </NodeShell>
      );
    case 'search_provider':
      return (
        <NodeShell title={view.title} description="Runs a configured search provider.">
          <Handle type="source" position={Position.Bottom} id="search" />
        </NodeShell>
      );
    case 'llm':
      return (
        <NodeShell title={view.title} description="Consumes prompt, context, and optional search input.">
          <Handle type="target" position={Position.Top} id="prompt" />
          <Handle type="target" position={Position.Left} id="context" />
          <Handle type="target" position={Position.Right} id="search" />
          <Handle type="target" position={Position.Left} id="skill" style={{ top: '72%' }} />
          <Handle type="target" position={Position.Right} id="tool" style={{ top: '72%' }} />
          <Handle type="source" position={Position.Bottom} id="text" />
          <div className="topic-node__ports">
            <span>skill</span>
            <span>tool</span>
          </div>
        </NodeShell>
      );
    case 'wecom_output':
      return (
        <NodeShell title={view.title} description="Delivers the generated text to WeCom.">
          <Handle type="target" position={Position.Top} id="text" />
        </NodeShell>
      );
    default:
      return <NodeShell title={view.title} description={view.description} />;
  }
}

export function TopicWorkflowGroupNode({ data }: NodeProps) {
  const view = (data ?? {}) as WorkflowNodeData;
  return (
    <div className="topic-group-node">
      <div className="topic-group-node__title">{view.title}</div>
    </div>
  );
}

export const topicWorkflowNodeTypes = {
  topicWorkflowNode: TopicWorkflowNode,
  topicGroupNode: TopicWorkflowGroupNode,
};
