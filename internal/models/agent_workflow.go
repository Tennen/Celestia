package models

import "time"

type AgentWorkflowSnapshot struct {
	ActiveWorkflowID string                     `json:"active_workflow_id,omitempty"`
	Workflows        []AgentWorkflow            `json:"workflows"`
	Runs             []AgentWorkflowRun         `json:"runs"`
	SentLog          []AgentWorkflowSentLogItem `json:"sent_log,omitempty"`
	UpdatedAt        time.Time                  `json:"updated_at"`
}

type AgentWorkflow struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Nodes       []AgentWorkflowNode `json:"nodes"`
	Edges       []AgentWorkflowEdge `json:"edges"`
	UpdatedAt   time.Time           `json:"updated_at"`
}

type AgentWorkflowNode struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Label    string         `json:"label,omitempty"`
	ParentID string         `json:"parent_id,omitempty"`
	Position AgentNodePoint `json:"position"`
	Width    float64        `json:"width,omitempty"`
	Height   float64        `json:"height,omitempty"`
	Data     map[string]any `json:"data,omitempty"`
}

type AgentNodePoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type AgentWorkflowEdge struct {
	ID           string `json:"id"`
	Source       string `json:"source"`
	SourceHandle string `json:"source_handle,omitempty"`
	Target       string `json:"target"`
	TargetHandle string `json:"target_handle,omitempty"`
	Label        string `json:"label,omitempty"`
}

type AgentWorkflowSource struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Category string  `json:"category"`
	FeedURL  string  `json:"feed_url"`
	Weight   float64 `json:"weight"`
	Enabled  bool    `json:"enabled"`
}

type AgentWorkflowRun struct {
	ID             string                    `json:"id"`
	WorkflowID     string                    `json:"workflow_id,omitempty"`
	WorkflowName   string                    `json:"workflow_name,omitempty"`
	CreatedAt      time.Time                 `json:"created_at"`
	StartedAt      time.Time                 `json:"started_at,omitempty"`
	FinishedAt     time.Time                 `json:"finished_at,omitempty"`
	Status         string                    `json:"status,omitempty"`
	Summary        string                    `json:"summary"`
	OutputText     string                    `json:"output_text,omitempty"`
	Items          []AgentWorkflowItem       `json:"items"`
	NodeResults    []AgentWorkflowNodeResult `json:"node_results,omitempty"`
	FetchErrors    []AgentRunError           `json:"fetch_errors,omitempty"`
	DeliveryErrors []AgentRunError           `json:"delivery_errors,omitempty"`
}

type AgentWorkflowNodeResult struct {
	NodeID   string         `json:"node_id"`
	NodeType string         `json:"node_type"`
	Status   string         `json:"status"`
	Summary  string         `json:"summary,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type AgentWorkflowItem struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	SourceID    string `json:"source_id"`
	SourceName  string `json:"source_name"`
	PublishedAt string `json:"published_at,omitempty"`
	Summary     string `json:"summary,omitempty"`
}

type AgentWorkflowSentLogItem struct {
	URLNormalized string    `json:"url_normalized"`
	SentAt        time.Time `json:"sent_at"`
	Title         string    `json:"title"`
}

type AgentRunError struct {
	Target string `json:"target"`
	Error  string `json:"error"`
}
