package models

import "time"

type AgentSearchPlan struct {
	Label   string   `json:"label"`
	Query   string   `json:"query"`
	Sites   []string `json:"sites,omitempty"`
	Recency string   `json:"recency,omitempty"`
}

type AgentSearchRequest struct {
	EngineSelector string            `json:"engine_selector,omitempty"`
	TimeoutMS      int               `json:"timeout_ms,omitempty"`
	MaxItems       int               `json:"max_items,omitempty"`
	Plans          []AgentSearchPlan `json:"plans"`
	LogContext     string            `json:"log_context,omitempty"`
}

type AgentSearchResultItem struct {
	Title       string `json:"title"`
	Source      string `json:"source"`
	Link        string `json:"link,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
	Snippet     string `json:"snippet,omitempty"`
}

type AgentSearchResult struct {
	Items       []AgentSearchResultItem `json:"items"`
	SourceChain []string                `json:"source_chain"`
	Errors      []string                `json:"errors"`
}

type AgentSpeechRequest struct {
	AudioPath string `json:"audio_path"`
}

type AgentSpeechResult struct {
	Text     string    `json:"text"`
	Provider string    `json:"provider"`
	At       time.Time `json:"at"`
}

type AgentCodexRequest struct {
	TaskID          string `json:"task_id,omitempty"`
	Prompt          string `json:"prompt"`
	Model           string `json:"model,omitempty"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
	TimeoutMS       int    `json:"timeout_ms,omitempty"`
	CWD             string `json:"cwd,omitempty"`
}

type AgentCodexResult struct {
	TaskID     string    `json:"task_id"`
	OK         bool      `json:"ok"`
	OutputFile string    `json:"output_file,omitempty"`
	Output     string    `json:"output,omitempty"`
	Error      string    `json:"error,omitempty"`
	ExitCode   int       `json:"exit_code"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
}

type AgentMarkdownRenderRequest struct {
	Markdown  string `json:"markdown"`
	Mode      string `json:"mode,omitempty"`
	OutputDir string `json:"output_dir,omitempty"`
}

type AgentMarkdownImage struct {
	Path        string `json:"path"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
}

type AgentMarkdownRenderResult struct {
	Mode        string               `json:"mode"`
	Images      []AgentMarkdownImage `json:"images"`
	OutputDir   string               `json:"output_dir"`
	SourceChars int                  `json:"source_chars"`
	RenderedAt  time.Time            `json:"rendered_at"`
}

type AgentTopicSentLogItem struct {
	URLNormalized string    `json:"url_normalized"`
	SentAt        time.Time `json:"sent_at"`
	Title         string    `json:"title"`
}

type AgentMarketAssetContext struct {
	Code        string                  `json:"code"`
	Name        string                  `json:"name"`
	EstimateNAV float64                 `json:"estimate_nav,omitempty"`
	ChangePct   float64                 `json:"change_pct,omitempty"`
	AsOf        string                  `json:"as_of,omitempty"`
	News        []AgentSearchResultItem `json:"news,omitempty"`
	SourceChain []string                `json:"source_chain,omitempty"`
	Errors      []string                `json:"errors,omitempty"`
}

type AgentMarketImportCodesRequest struct {
	Codes string `json:"codes"`
}

type AgentMarketImportCodeResult struct {
	Code    string `json:"code"`
	Name    string `json:"name,omitempty"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type AgentMarketImportCodesResponse struct {
	OK        bool                          `json:"ok"`
	Portfolio AgentMarketPortfolio          `json:"portfolio"`
	Results   []AgentMarketImportCodeResult `json:"results"`
	Summary   map[string]int                `json:"summary"`
}
