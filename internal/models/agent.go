package models

import (
	"encoding/json"
	"time"
)

type AgentDocument struct {
	Key       string          `json:"key"`
	Domain    string          `json:"domain"`
	Payload   json.RawMessage `json:"payload"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type AgentSnapshot struct {
	Settings      AgentSettings          `json:"settings"`
	DirectInput   AgentDirectInputConfig `json:"direct_input"`
	WeComMenu     AgentWeComMenuSnapshot `json:"wecom_menu"`
	Push          AgentPushSnapshot      `json:"push"`
	Conversations []AgentConversation    `json:"conversations"`
	Memory        AgentMemorySnapshot    `json:"memory"`
	TopicSummary  AgentTopicSnapshot     `json:"topic_summary"`
	Writing       AgentWritingSnapshot   `json:"writing"`
	Market        AgentMarketSnapshot    `json:"market"`
	Evolution     AgentEvolutionSnapshot `json:"evolution"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

type AgentSettings struct {
	RuntimeMode          string                `json:"runtime_mode"`
	DefaultLLMProviderID string                `json:"default_llm_provider_id"`
	LLMProviders         []AgentLLMProvider    `json:"llm_providers"`
	WeCom                AgentWeComConfig      `json:"wecom"`
	Terminal             AgentTerminalConfig   `json:"terminal"`
	Evolution            AgentEvolutionConfig  `json:"evolution"`
	STT                  AgentSpeechConfig     `json:"stt"`
	SearchEngines        []AgentSearchProvider `json:"search_engines"`
	Memory               AgentMemoryConfig     `json:"memory"`
	MD2Img               AgentMD2ImgConfig     `json:"md2img"`
	UpdatedAt            time.Time             `json:"updated_at"`
}

type AgentLLMProvider struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	BaseURL   string `json:"base_url,omitempty"`
	APIKey    string `json:"api_key,omitempty"`
	Model     string `json:"model,omitempty"`
	ChatPath  string `json:"chat_path,omitempty"`
	TimeoutMS int    `json:"timeout_ms,omitempty"`
}

type AgentSearchProvider struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Type    string         `json:"type"`
	Enabled bool           `json:"enabled"`
	Config  map[string]any `json:"config,omitempty"`
}

type AgentSpeechConfig struct {
	Provider  string `json:"provider,omitempty"`
	Command   string `json:"command,omitempty"`
	Enabled   bool   `json:"enabled"`
	TimeoutMS int    `json:"timeout_ms,omitempty"`
}

type AgentMemoryConfig struct {
	Enabled              bool `json:"enabled"`
	CompactEveryRounds   int  `json:"compact_every_rounds,omitempty"`
	CompactMaxBatchSize  int  `json:"compact_max_batch_size,omitempty"`
	SummaryTopK          int  `json:"summary_top_k,omitempty"`
	RawRefLimit          int  `json:"raw_ref_limit,omitempty"`
	RawRecordLimit       int  `json:"raw_record_limit,omitempty"`
	WindowTimeoutSeconds int  `json:"window_timeout_seconds,omitempty"`
	WindowMaxTurns       int  `json:"window_max_turns,omitempty"`
}

type AgentMD2ImgConfig struct {
	Enabled   bool   `json:"enabled"`
	Mode      string `json:"mode,omitempty"`
	Command   string `json:"command,omitempty"`
	OutputDir string `json:"output_dir,omitempty"`
	TimeoutMS int    `json:"timeout_ms,omitempty"`
}

type AgentWeComConfig struct {
	CorpID              string `json:"corp_id,omitempty"`
	CorpSecret          string `json:"corp_secret,omitempty"`
	AgentID             string `json:"agent_id,omitempty"`
	BaseURL             string `json:"base_url,omitempty"`
	BridgeURL           string `json:"bridge_url,omitempty"`
	BridgeToken         string `json:"bridge_token,omitempty"`
	BridgeStreamEnabled bool   `json:"bridge_stream_enabled,omitempty"`
	AudioDir            string `json:"audio_dir,omitempty"`
	TextMaxBytes        int    `json:"text_max_bytes,omitempty"`
	Enabled             bool   `json:"enabled"`
}

type AgentTerminalConfig struct {
	Enabled   bool   `json:"enabled"`
	CWD       string `json:"cwd,omitempty"`
	TimeoutMS int    `json:"timeout_ms,omitempty"`
}

type AgentEvolutionConfig struct {
	Command         string                      `json:"command,omitempty"`
	CWD             string                      `json:"cwd,omitempty"`
	TimeoutMS       int                         `json:"timeout_ms,omitempty"`
	CodexModel      string                      `json:"codex_model,omitempty"`
	CodexReasoning  string                      `json:"codex_reasoning,omitempty"`
	MaxFixAttempts  int                         `json:"max_fix_attempts,omitempty"`
	TestCommands    []AgentEvolutionTestCommand `json:"test_commands,omitempty"`
	AutoCommit      bool                        `json:"auto_commit,omitempty"`
	AutoPush        bool                        `json:"auto_push,omitempty"`
	PushRemote      string                      `json:"push_remote,omitempty"`
	PushBranch      string                      `json:"push_branch,omitempty"`
	StructureReview bool                        `json:"structure_review,omitempty"`
}

type AgentEvolutionTestCommand struct {
	Name      string `json:"name"`
	Command   string `json:"command"`
	TimeoutMS int    `json:"timeout_ms,omitempty"`
}

type AgentDirectInputConfig struct {
	Version   int                    `json:"version"`
	Rules     []AgentDirectInputRule `json:"rules"`
	UpdatedAt time.Time              `json:"updated_at"`
}

type AgentDirectInputRule struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Pattern    string `json:"pattern"`
	TargetText string `json:"target_text"`
	MatchMode  string `json:"match_mode"`
	Enabled    bool   `json:"enabled"`
}

type AgentWeComMenuSnapshot struct {
	Config           AgentWeComMenuConfig    `json:"config"`
	RecentEvents     []AgentWeComEventRecord `json:"recent_events"`
	PublishPayload   map[string]any          `json:"publish_payload,omitempty"`
	ValidationErrors []string                `json:"validation_errors,omitempty"`
}

type AgentWeComMenuConfig struct {
	Version         int                `json:"version"`
	Buttons         []AgentWeComButton `json:"buttons"`
	UpdatedAt       time.Time          `json:"updated_at"`
	LastPublishedAt *time.Time         `json:"last_published_at,omitempty"`
}

type AgentWeComButton struct {
	ID           string             `json:"id"`
	Name         string             `json:"name"`
	Key          string             `json:"key"`
	Enabled      bool               `json:"enabled"`
	DispatchText string             `json:"dispatch_text"`
	SubButtons   []AgentWeComButton `json:"sub_buttons,omitempty"`
}

type AgentWeComEventRecord struct {
	ID                string    `json:"id"`
	EventType         string    `json:"event_type"`
	EventKey          string    `json:"event_key"`
	FromUser          string    `json:"from_user"`
	ToUser            string    `json:"to_user"`
	AgentID           string    `json:"agent_id,omitempty"`
	MatchedButtonID   string    `json:"matched_button_id,omitempty"`
	MatchedButtonName string    `json:"matched_button_name,omitempty"`
	DispatchText      string    `json:"dispatch_text,omitempty"`
	Status            string    `json:"status"`
	Error             string    `json:"error,omitempty"`
	ReceivedAt        time.Time `json:"received_at"`
}

type AgentWeComInboundResult struct {
	Record       AgentWeComEventRecord `json:"record"`
	ResponseText string                `json:"response_text,omitempty"`
	FromUser     string                `json:"from_user,omitempty"`
	ToUser       string                `json:"to_user,omitempty"`
}

type AgentPushSnapshot struct {
	Users     []AgentPushUser `json:"users"`
	Tasks     []AgentPushTask `json:"tasks"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type AgentPushUser struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	WeComUser string    `json:"wecom_user,omitempty"`
	Enabled   bool      `json:"enabled"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AgentPushTask struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	UserID    string     `json:"user_id"`
	Text      string     `json:"text"`
	IntervalM int        `json:"interval_minutes"`
	Enabled   bool       `json:"enabled"`
	LastRunAt *time.Time `json:"last_run_at,omitempty"`
	NextRunAt *time.Time `json:"next_run_at,omitempty"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type AgentConversation struct {
	ID        string         `json:"id"`
	SessionID string         `json:"session_id"`
	Input     string         `json:"input"`
	Resolved  string         `json:"resolved,omitempty"`
	Response  string         `json:"response"`
	Status    string         `json:"status"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

type AgentConversationRequest struct {
	SessionID string `json:"session_id,omitempty"`
	Input     string `json:"input"`
	Actor     string `json:"actor,omitempty"`
}

type AgentTopicSnapshot struct {
	ActiveProfileID string                  `json:"active_profile_id"`
	Profiles        []AgentTopicProfile     `json:"profiles"`
	Runs            []AgentTopicRun         `json:"runs"`
	SentLog         []AgentTopicSentLogItem `json:"sent_log,omitempty"`
	UpdatedAt       time.Time               `json:"updated_at"`
}

type AgentTopicProfile struct {
	ID        string             `json:"id"`
	Name      string             `json:"name"`
	Sources   []AgentTopicSource `json:"sources"`
	UpdatedAt time.Time          `json:"updated_at"`
}

type AgentTopicSource struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Category string  `json:"category"`
	FeedURL  string  `json:"feed_url"`
	Weight   float64 `json:"weight"`
	Enabled  bool    `json:"enabled"`
}

type AgentTopicRun struct {
	ID          string           `json:"id"`
	ProfileID   string           `json:"profile_id"`
	CreatedAt   time.Time        `json:"created_at"`
	Summary     string           `json:"summary"`
	Items       []AgentTopicItem `json:"items"`
	FetchErrors []AgentRunError  `json:"fetch_errors,omitempty"`
}

type AgentTopicItem struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	SourceID    string `json:"source_id"`
	SourceName  string `json:"source_name"`
	PublishedAt string `json:"published_at,omitempty"`
	Summary     string `json:"summary,omitempty"`
}

type AgentRunError struct {
	Target string `json:"target"`
	Error  string `json:"error"`
}

type AgentWritingSnapshot struct {
	Topics    []AgentWritingTopic `json:"topics"`
	UpdatedAt time.Time           `json:"updated_at"`
}

type AgentWritingTopic struct {
	ID               string                 `json:"id"`
	Title            string                 `json:"title"`
	Status           string                 `json:"status"`
	Materials        []AgentWritingMaterial `json:"materials"`
	State            AgentWritingState      `json:"state"`
	Backup           AgentWritingState      `json:"backup"`
	ArtifactRoot     string                 `json:"artifact_root,omitempty"`
	RawFiles         []AgentWritingRawFile  `json:"raw_files,omitempty"`
	Artifacts        AgentWritingArtifacts  `json:"artifacts,omitempty"`
	LastSummarizedAt *time.Time             `json:"last_summarized_at,omitempty"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
}

type AgentWritingMaterial struct {
	ID           string         `json:"id"`
	Title        string         `json:"title"`
	Content      string         `json:"content"`
	Type         string         `json:"type,omitempty"`
	Source       string         `json:"source,omitempty"`
	InputMode    string         `json:"input_mode,omitempty"`
	URLs         []string       `json:"urls,omitempty"`
	RawFile      string         `json:"raw_file,omitempty"`
	ArtifactPath string         `json:"artifact_path,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}

type AgentWritingState struct {
	Summary string `json:"summary"`
	Outline string `json:"outline"`
	Draft   string `json:"draft"`
}

type AgentWritingRawFile struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	LineCount int    `json:"line_count"`
}

type AgentWritingArtifacts struct {
	MaterialCount  int                   `json:"material_count"`
	InsightCount   int                   `json:"insight_count"`
	DocumentCount  int                   `json:"document_count"`
	LatestInsight  *AgentWritingInsight  `json:"latest_insight,omitempty"`
	LatestDocument *AgentWritingDocument `json:"latest_document,omitempty"`
}

type AgentWritingInsight struct {
	ID           string    `json:"id"`
	TopicID      string    `json:"topic_id"`
	MaterialIDs  []string  `json:"material_ids"`
	Summary      string    `json:"summary"`
	KeyPoints    []string  `json:"key_points"`
	Tags         []string  `json:"tags"`
	Entities     []string  `json:"entities"`
	QualityScore float64   `json:"quality_score"`
	Path         string    `json:"path,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type AgentWritingDocument struct {
	ID          string    `json:"id"`
	TopicID     string    `json:"topic_id"`
	MaterialIDs []string  `json:"material_ids"`
	InsightID   string    `json:"insight_id"`
	Mode        string    `json:"mode"`
	Title       string    `json:"title"`
	Path        string    `json:"path"`
	Version     int       `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
}

type AgentMarketSnapshot struct {
	Portfolio AgentMarketPortfolio `json:"portfolio"`
	Config    AgentMarketConfig    `json:"config"`
	Runs      []AgentMarketRun     `json:"runs"`
	UpdatedAt time.Time            `json:"updated_at"`
}

type AgentMarketPortfolio struct {
	Funds []AgentMarketHolding `json:"funds"`
	Cash  float64              `json:"cash"`
}

type AgentMarketHolding struct {
	Code     string  `json:"code"`
	Name     string  `json:"name"`
	Quantity float64 `json:"quantity,omitempty"`
	AvgCost  float64 `json:"avg_cost,omitempty"`
}

type AgentMarketConfig struct {
	AnalysisEngine string `json:"analysis_engine"`
	SearchEngine   string `json:"search_engine"`
}

type AgentMarketRun struct {
	ID          string                    `json:"id"`
	CreatedAt   time.Time                 `json:"created_at"`
	Phase       string                    `json:"phase"`
	MarketState string                    `json:"market_state"`
	Summary     string                    `json:"summary"`
	Images      []AgentMarkdownImage      `json:"images,omitempty"`
	Assets      []AgentMarketAssetContext `json:"assets,omitempty"`
	SourceChain []string                  `json:"source_chain,omitempty"`
	Errors      []AgentRunError           `json:"errors,omitempty"`
}

type AgentEvolutionSnapshot struct {
	Goals     []AgentEvolutionGoal `json:"goals"`
	UpdatedAt time.Time            `json:"updated_at"`
}

type AgentEvolutionGoal struct {
	ID              string                     `json:"id"`
	Goal            string                     `json:"goal"`
	CommitMessage   string                     `json:"commit_message,omitempty"`
	Status          string                     `json:"status"`
	Stage           string                     `json:"stage"`
	Plan            AgentEvolutionPlan         `json:"plan"`
	FixAttempts     int                        `json:"fix_attempts"`
	StartedFromRef  string                     `json:"started_from_ref,omitempty"`
	CompletedCommit string                     `json:"completed_commit,omitempty"`
	TestResults     []AgentEvolutionTestResult `json:"test_results,omitempty"`
	RawTail         []AgentEvolutionRawLine    `json:"raw_tail,omitempty"`
	LastCodexOutput string                     `json:"last_codex_output,omitempty"`
	Events          []AgentEvolutionEvent      `json:"events"`
	CreatedAt       time.Time                  `json:"created_at"`
	UpdatedAt       time.Time                  `json:"updated_at"`
	StartedAt       *time.Time                 `json:"started_at,omitempty"`
	CompletedAt     *time.Time                 `json:"completed_at,omitempty"`
	LastError       string                     `json:"last_error,omitempty"`
}

type AgentEvolutionPlan struct {
	Steps       []string `json:"steps"`
	CurrentStep int      `json:"current_step"`
}

type AgentEvolutionTestResult struct {
	Name       string    `json:"name"`
	Command    string    `json:"command"`
	OK         bool      `json:"ok"`
	ExitCode   int       `json:"exit_code"`
	Output     string    `json:"output"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
}

type AgentEvolutionRawLine struct {
	At   time.Time `json:"at"`
	Line string    `json:"line"`
}

type AgentEvolutionEvent struct {
	At      time.Time `json:"at"`
	Stage   string    `json:"stage"`
	Message string    `json:"message"`
}

type AgentTerminalRequest struct {
	Command string `json:"command"`
	CWD     string `json:"cwd,omitempty"`
}

type AgentTerminalResult struct {
	Command    string    `json:"command"`
	CWD        string    `json:"cwd,omitempty"`
	ExitCode   int       `json:"exit_code"`
	Output     string    `json:"output"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
}
