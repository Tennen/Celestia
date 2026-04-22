package gateway

import (
	"context"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

type Service interface {
	Health(ctx context.Context) (HealthStatus, error)
	Dashboard(ctx context.Context) (models.DashboardSummary, error)

	ListCatalogPlugins(ctx context.Context) ([]models.CatalogPlugin, error)
	ListPlugins(ctx context.Context) ([]models.PluginRuntimeView, error)
	ListCapabilities(ctx context.Context) ([]models.Capability, error)
	GetCapability(ctx context.Context, id string) (models.CapabilityDetail, error)
	InstallPlugin(ctx context.Context, req InstallPluginRequest) (models.PluginInstallRecord, error)
	UpdatePluginConfig(ctx context.Context, req UpdatePluginConfigRequest) (models.PluginInstallRecord, error)
	EnablePlugin(ctx context.Context, pluginID string) error
	DisablePlugin(ctx context.Context, pluginID string) error
	DiscoverPlugin(ctx context.Context, pluginID string) error
	DeletePlugin(ctx context.Context, pluginID string) error
	GetPluginLogs(ctx context.Context, pluginID string) (PluginLogsView, error)
	SaveVisionCapabilityConfig(ctx context.Context, config models.VisionCapabilityConfig) (models.CapabilityDetail, error)
	RefreshVisionEntityCatalog(ctx context.Context, req models.VisionEntityCatalogRefreshRequest) (models.VisionEntityCatalog, error)
	ListVisionRuleEvents(ctx context.Context, ruleID string, filter VisionRuleEventFilter) ([]models.Event, error)
	DeleteVisionRuleEvent(ctx context.Context, ruleID string, eventID string) error
	GetVisionEventCapture(ctx context.Context, captureID string) (models.VisionEventCaptureAsset, error)

	ListAutomations(ctx context.Context) ([]models.Automation, error)
	SaveAutomation(ctx context.Context, automation models.Automation) (models.Automation, error)
	DeleteAutomation(ctx context.Context, id string) error

	GetAgentSnapshot(ctx context.Context) (models.AgentSnapshot, error)
	SaveAgentSettings(ctx context.Context, settings models.AgentSettings) (models.AgentSnapshot, error)
	SaveAgentDirectInput(ctx context.Context, config models.AgentDirectInputConfig) (models.AgentSnapshot, error)
	SaveAgentPush(ctx context.Context, push models.AgentPushSnapshot) (models.AgentSnapshot, error)
	SaveAgentWeComMenu(ctx context.Context, config models.AgentWeComMenuConfig) (models.AgentSnapshot, error)
	PublishAgentWeComMenu(ctx context.Context) (models.AgentWeComMenuSnapshot, error)
	SendAgentWeComMessage(ctx context.Context, req AgentWeComSendRequest) error
	SendAgentWeComImage(ctx context.Context, req AgentWeComImageRequest) error
	RecordAgentWeComCallback(ctx context.Context, raw []byte) (models.AgentWeComEventRecord, error)
	HandleAgentWeComIngress(ctx context.Context, raw []byte) (models.AgentWeComInboundResult, error)
	RunAgentConversation(ctx context.Context, req models.AgentConversationRequest) (models.AgentConversation, error)
	SaveAgentTopic(ctx context.Context, topic models.AgentTopicSnapshot) (models.AgentSnapshot, error)
	RunAgentTopicSummary(ctx context.Context, profileID string) (models.AgentTopicRun, error)
	SaveAgentWritingTopic(ctx context.Context, req AgentWritingTopicRequest) (models.AgentWritingTopic, error)
	AddAgentWritingMaterial(ctx context.Context, topicID string, req AgentWritingMaterialRequest) (models.AgentWritingTopic, error)
	SummarizeAgentWritingTopic(ctx context.Context, topicID string) (models.AgentWritingTopic, error)
	SaveAgentMarketPortfolio(ctx context.Context, portfolio models.AgentMarketPortfolio) (models.AgentSnapshot, error)
	RunAgentMarketAnalysis(ctx context.Context, req AgentMarketRunRequest) (models.AgentMarketRun, error)
	CreateAgentEvolutionGoal(ctx context.Context, req AgentEvolutionGoalRequest) (models.AgentEvolutionGoal, error)
	RunAgentEvolutionGoal(ctx context.Context, goalID string) (models.AgentEvolutionGoal, error)
	RunAgentTerminal(ctx context.Context, req models.AgentTerminalRequest) (models.AgentTerminalResult, error)
	RunAgentSearch(ctx context.Context, req models.AgentSearchRequest) (models.AgentSearchResult, error)
	TranscribeAgentSpeech(ctx context.Context, req models.AgentSpeechRequest) (models.AgentSpeechResult, error)
	RunAgentCodex(ctx context.Context, req models.AgentCodexRequest) (models.AgentCodexResult, error)
	RunAgentMarkdownRender(ctx context.Context, req models.AgentMarkdownRenderRequest) (models.AgentMarkdownRenderResult, error)

	ListDevices(ctx context.Context, filter DeviceFilter) ([]models.DeviceView, error)
	GetDevice(ctx context.Context, deviceID string) (models.DeviceView, error)
	ListAIDevices(ctx context.Context, filter DeviceFilter) ([]AIDevice, error)
	UpdateDevicePreference(ctx context.Context, req UpdateDevicePreferenceRequest) (models.DevicePreference, error)
	UpdateControlPreference(ctx context.Context, req UpdateControlPreferenceRequest) (models.DeviceControlPreference, error)
	SendDeviceCommand(ctx context.Context, req DeviceCommandRequest) (CommandExecutionResult, error)
	ExecuteAICommand(ctx context.Context, req AICommandRequest) (AICommandResult, error)
	ToggleControl(ctx context.Context, req ToggleControlRequest) (CommandExecutionResult, error)
	RunActionControl(ctx context.Context, req ActionControlRequest) (CommandExecutionResult, error)

	ListEvents(ctx context.Context, filter EventFilter) ([]models.Event, error)
	ListAudits(ctx context.Context, filter AuditFilter) ([]models.AuditRecord, error)
}

type HealthStatus struct {
	Status string    `json:"status"`
	Time   time.Time `json:"time"`
}

type InstallPluginRequest struct {
	PluginID   string         `json:"plugin_id"`
	BinaryPath string         `json:"binary_path,omitempty"`
	Config     map[string]any `json:"config,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type UpdatePluginConfigRequest struct {
	PluginID string         `json:"plugin_id"`
	Config   map[string]any `json:"config"`
}

type PluginLogsView struct {
	PluginID string   `json:"plugin_id"`
	Logs     []string `json:"logs"`
}

type AgentWeComSendRequest struct {
	ToUser string `json:"to_user"`
	Text   string `json:"text"`
}

type AgentWeComImageRequest struct {
	ToUser      string `json:"to_user"`
	Base64      string `json:"base64"`
	Filename    string `json:"filename,omitempty"`
	ContentType string `json:"content_type,omitempty"`
}

type AgentWritingTopicRequest struct {
	ID    string `json:"id,omitempty"`
	Title string `json:"title"`
}

type AgentWritingMaterialRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type AgentMarketRunRequest struct {
	Phase string `json:"phase"`
	Notes string `json:"notes,omitempty"`
}

type AgentEvolutionGoalRequest struct {
	Goal          string `json:"goal"`
	CommitMessage string `json:"commit_message,omitempty"`
}

type DeviceFilter struct {
	PluginID string
	Kind     string
	Query    string
}

type UpdateDevicePreferenceRequest struct {
	DeviceID string `json:"device_id"`
	Alias    string `json:"alias"`
}

type UpdateControlPreferenceRequest struct {
	DeviceID  string `json:"device_id"`
	ControlID string `json:"control_id"`
	Alias     string `json:"alias"`
	Visible   *bool  `json:"visible,omitempty"`
}

type DeviceCommandRequest struct {
	DeviceID string         `json:"device_id"`
	Actor    string         `json:"actor,omitempty"`
	Action   string         `json:"action"`
	Params   map[string]any `json:"params,omitempty"`
}

type ToggleControlRequest struct {
	CompoundControlID string `json:"compound_control_id"`
	Actor             string `json:"actor,omitempty"`
	On                bool   `json:"on"`
}

type ActionControlRequest struct {
	CompoundControlID string `json:"compound_control_id"`
	Actor             string `json:"actor,omitempty"`
}

type CommandExecutionResult struct {
	Decision models.PolicyDecision  `json:"decision"`
	Result   models.CommandResponse `json:"result"`
}

type EventFilter struct {
	PluginID string
	DeviceID string
	FromTS   *time.Time
	ToTS     *time.Time
	BeforeTS *time.Time
	BeforeID string
	Limit    int
}

type VisionRuleEventFilter struct {
	FromTS   *time.Time
	ToTS     *time.Time
	BeforeTS *time.Time
	BeforeID string
	Limit    int
}

type AuditFilter struct {
	DeviceID string
	Limit    int
}
