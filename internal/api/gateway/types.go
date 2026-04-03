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
	InstallPlugin(ctx context.Context, req InstallPluginRequest) (models.PluginInstallRecord, error)
	UpdatePluginConfig(ctx context.Context, req UpdatePluginConfigRequest) (models.PluginInstallRecord, error)
	EnablePlugin(ctx context.Context, pluginID string) error
	DisablePlugin(ctx context.Context, pluginID string) error
	DiscoverPlugin(ctx context.Context, pluginID string) error
	DeletePlugin(ctx context.Context, pluginID string) error
	GetPluginLogs(ctx context.Context, pluginID string) (PluginLogsView, error)

	ListAutomations(ctx context.Context) ([]models.Automation, error)
	SaveAutomation(ctx context.Context, automation models.Automation) (models.Automation, error)
	DeleteAutomation(ctx context.Context, id string) error

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
	Limit    int
}

type AuditFilter struct {
	DeviceID string
	Limit    int
}
