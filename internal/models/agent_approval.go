package models

import "time"

type AgentApprovalSnapshot struct {
	Requests  []AgentApprovalRequest `json:"requests"`
	UpdatedAt time.Time              `json:"updated_at"`
}

type AgentApprovalRequest struct {
	ID           string                    `json:"id"`
	Kind         string                    `json:"kind"`
	Action       string                    `json:"action"`
	GoalID       string                    `json:"goal_id,omitempty"`
	Title        string                    `json:"title"`
	Detail       string                    `json:"detail,omitempty"`
	Status       string                    `json:"status"`
	RequestedBy  string                    `json:"requested_by,omitempty"`
	DecisionBy   string                    `json:"decision_by,omitempty"`
	DecisionNote string                    `json:"decision_note,omitempty"`
	Result       *AgentEvolutionTestResult `json:"result,omitempty"`
	Error        string                    `json:"error,omitempty"`
	CreatedAt    time.Time                 `json:"created_at"`
	UpdatedAt    time.Time                 `json:"updated_at"`
	DecidedAt    *time.Time                `json:"decided_at,omitempty"`
	ExecutedAt   *time.Time                `json:"executed_at,omitempty"`
}

type AgentApprovalCreateRequest struct {
	Kind        string `json:"kind"`
	Action      string `json:"action"`
	GoalID      string `json:"goal_id,omitempty"`
	Title       string `json:"title,omitempty"`
	Detail      string `json:"detail,omitempty"`
	RequestedBy string `json:"requested_by,omitempty"`
}

type AgentApprovalDecisionRequest struct {
	Actor string `json:"actor,omitempty"`
	Note  string `json:"note,omitempty"`
}
