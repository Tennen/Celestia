package models

import "time"

type AgentMemorySnapshot struct {
	RawRecords []AgentRawMemoryRecord     `json:"raw_records"`
	Summaries  []AgentSummaryMemoryRecord `json:"summaries"`
	Windows    []AgentConversationWindow  `json:"windows"`
	UpdatedAt  time.Time                  `json:"updated_at"`
}

type AgentRawMemoryRecord struct {
	ID           string         `json:"id"`
	SessionID    string         `json:"session_id"`
	RequestID    string         `json:"request_id"`
	Source       string         `json:"source"`
	User         string         `json:"user"`
	Assistant    string         `json:"assistant"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	SummarizedAt *time.Time     `json:"summarized_at,omitempty"`
}

type AgentSummaryMemoryRecord struct {
	ID                  string    `json:"id"`
	SessionID           string    `json:"session_id"`
	UserFacts           []string  `json:"user_facts"`
	Environment         []string  `json:"environment"`
	LongTermPreferences []string  `json:"long_term_preferences"`
	TaskResults         []string  `json:"task_results"`
	RawRefs             []string  `json:"raw_refs"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type AgentConversationWindow struct {
	SessionID        string            `json:"session_id"`
	WindowID         string            `json:"window_id"`
	StartedAt        time.Time         `json:"started_at"`
	LastUserAt       *time.Time        `json:"last_user_at,omitempty"`
	LastAssistantAt  *time.Time        `json:"last_assistant_at,omitempty"`
	Turns            []AgentMemoryTurn `json:"turns"`
	ActiveCapability map[string]any    `json:"active_capability,omitempty"`
}

type AgentMemoryTurn struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}
