package models

import "time"

type AgentScreenshotRequest struct {
	URL       string `json:"url"`
	OutputDir string `json:"output_dir,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	FullPage  bool   `json:"full_page,omitempty"`
	WaitMS    int    `json:"wait_ms,omitempty"`
	TimeoutMS int    `json:"timeout_ms,omitempty"`
}

type AgentScreenshotResult struct {
	URL        string             `json:"url"`
	Image      AgentMarkdownImage `json:"image"`
	Viewport   string             `json:"viewport,omitempty"`
	CapturedAt time.Time          `json:"captured_at"`
}
