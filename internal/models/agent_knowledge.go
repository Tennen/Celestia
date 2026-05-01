package models

import "time"

type AgentKnowledgeConfig struct {
	Enabled         bool   `json:"enabled"`
	BaseDir         string `json:"base_dir,omitempty"`
	AgentProviderID string `json:"agent_provider_id,omitempty"`
	TimeoutMS       int    `json:"timeout_ms,omitempty"`
}

type AgentKnowledgeSnapshot struct {
	Sessions  []AgentKnowledgeSession `json:"sessions"`
	UpdatedAt time.Time               `json:"updated_at"`
}

type AgentKnowledgeSession struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id,omitempty"`
	Source         string    `json:"source,omitempty"`
	CodexSessionID string    `json:"codex_session_id,omitempty"`
	Active         bool      `json:"active"`
	Status         string    `json:"status"`
	LastQuestion   string    `json:"last_question,omitempty"`
	LastMarkdown   string    `json:"last_markdown,omitempty"`
	LastError      string    `json:"last_error,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type AgentKnowledgeRequest struct {
	Question   string `json:"question,omitempty"`
	UserID     string `json:"user_id,omitempty"`
	Source     string `json:"source,omitempty"`
	NewSession bool   `json:"new_session,omitempty"`
}

type AgentKnowledgeResult struct {
	MarkdownPath string                `json:"markdown_path"`
	Images       []AgentMarkdownImage  `json:"images"`
	Session      AgentKnowledgeSession `json:"session"`
	Codex        AgentCodexResult      `json:"codex"`
}
