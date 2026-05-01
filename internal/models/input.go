package models

import "time"

type ProjectInputRequest struct {
	SessionID string `json:"session_id,omitempty"`
	Input     string `json:"input"`
	Actor     string `json:"actor,omitempty"`
	Source    string `json:"source,omitempty"`
	UserID    string `json:"user_id,omitempty"`
}

type ProjectInputResult struct {
	Handled      bool                 `json:"handled"`
	Conversation AgentConversation    `json:"conversation"`
	ResponseText string               `json:"response_text,omitempty"`
	Images       []ProjectOutputImage `json:"images,omitempty"`
	Slash        *SlashCommandResult  `json:"slash,omitempty"`
}

type SlashCommandResult struct {
	Command    string               `json:"command"`
	Args       []string             `json:"args,omitempty"`
	Output     string               `json:"output"`
	Images     []ProjectOutputImage `json:"images,omitempty"`
	Metadata   map[string]any       `json:"metadata,omitempty"`
	ExecutedAt time.Time            `json:"executed_at"`
}

type ProjectOutputImage struct {
	Path        string `json:"path"`
	ContentType string `json:"content_type,omitempty"`
	Filename    string `json:"filename,omitempty"`
}
