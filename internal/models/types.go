package models

import "time"

type PluginStatus string

const (
	PluginStatusInstalled PluginStatus = "installed"
	PluginStatusEnabled   PluginStatus = "enabled"
	PluginStatusDisabled  PluginStatus = "disabled"
)

type HealthState string

const (
	HealthStateUnknown   HealthState = "unknown"
	HealthStateHealthy   HealthState = "healthy"
	HealthStateDegraded  HealthState = "degraded"
	HealthStateUnhealthy HealthState = "unhealthy"
	HealthStateStopped   HealthState = "stopped"
)

type RiskLevel string

const (
	RiskLevelLow    RiskLevel = "low"
	RiskLevelMedium RiskLevel = "medium"
	RiskLevelHigh   RiskLevel = "high"
)

type EventType string

const (
	EventDeviceDiscovered     EventType = "device.discovered"
	EventDeviceStateChanged   EventType = "device.state.changed"
	EventDeviceOccurred       EventType = "device.event.occurred"
	EventDeviceCommandAccept  EventType = "device.command.accepted"
	EventDeviceCommandFailed  EventType = "device.command.failed"
	EventPluginHealthChanged  EventType = "plugin.health.changed"
	EventPluginLifecycleState EventType = "plugin.lifecycle.changed"
)

type DeviceKind string

const (
	DeviceKindLight        DeviceKind = "light"
	DeviceKindSwitch       DeviceKind = "switch"
	DeviceKindSensor       DeviceKind = "sensor"
	DeviceKindClimate      DeviceKind = "climate"
	DeviceKindWasher       DeviceKind = "washer"
	DeviceKindPetFeeder    DeviceKind = "pet_feeder"
	DeviceKindPetFountain  DeviceKind = "pet_fountain"
	DeviceKindPetLitterBox DeviceKind = "pet_litter_box"
	DeviceKindAquarium     DeviceKind = "aquarium"
	DeviceKindSpeaker      DeviceKind = "speaker"
	DeviceKindCameraLike   DeviceKind = "camera_like"
)

type PluginManifest struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Version      string         `json:"version"`
	Vendor       string         `json:"vendor"`
	Capabilities []string       `json:"capabilities"`
	ConfigSchema map[string]any `json:"config_schema,omitempty"`
	DeviceKinds  []DeviceKind   `json:"device_kinds"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type PluginInstallRecord struct {
	PluginID         string         `json:"plugin_id"`
	Version          string         `json:"version"`
	Status           PluginStatus   `json:"status"`
	BinaryPath       string         `json:"binary_path"`
	Config           map[string]any `json:"config"`
	ConfigRef        string         `json:"config_ref,omitempty"`
	InstalledAt      time.Time      `json:"installed_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	LastHeartbeatAt  *time.Time     `json:"last_heartbeat_at,omitempty"`
	LastHealthStatus HealthState    `json:"last_health_status"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

type PluginHealth struct {
	PluginID   string      `json:"plugin_id"`
	Status     HealthState `json:"status"`
	Message    string      `json:"message"`
	CheckedAt  time.Time   `json:"checked_at"`
	Manifest   string      `json:"manifest_version,omitempty"`
	ProcessPID int         `json:"process_pid,omitempty"`
}

type Device struct {
	ID             string         `json:"id"`
	PluginID       string         `json:"plugin_id"`
	VendorDeviceID string         `json:"vendor_device_id"`
	Kind           DeviceKind     `json:"kind"`
	Name           string         `json:"name"`
	Room           string         `json:"room,omitempty"`
	Online         bool           `json:"online"`
	Capabilities   []string       `json:"capabilities"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type DeviceStateSnapshot struct {
	DeviceID string         `json:"device_id"`
	PluginID string         `json:"plugin_id"`
	TS       time.Time      `json:"ts"`
	State    map[string]any `json:"state"`
}

type DeviceControlKind string

const (
	DeviceControlKindToggle DeviceControlKind = "toggle"
	DeviceControlKindAction DeviceControlKind = "action"
)

type DeviceControl struct {
	ID          string            `json:"id"`
	Kind        DeviceControlKind `json:"kind"`
	Label       string            `json:"label"`
	Description string            `json:"description,omitempty"`
	State       *bool             `json:"state,omitempty"`
}

type DeviceView struct {
	Device   Device              `json:"device"`
	State    DeviceStateSnapshot `json:"state"`
	Controls []DeviceControl     `json:"controls,omitempty"`
}

type Event struct {
	ID       string         `json:"id"`
	Type     EventType      `json:"type"`
	PluginID string         `json:"plugin_id,omitempty"`
	DeviceID string         `json:"device_id,omitempty"`
	TS       time.Time      `json:"ts"`
	Payload  map[string]any `json:"payload,omitempty"`
}

type CommandRequest struct {
	DeviceID  string         `json:"device_id"`
	Action    string         `json:"action"`
	Params    map[string]any `json:"params,omitempty"`
	RequestID string         `json:"request_id"`
}

type CommandResponse struct {
	Accepted bool   `json:"accepted"`
	JobID    string `json:"job_id,omitempty"`
	Message  string `json:"message,omitempty"`
}

type AuditRecord struct {
	ID        string         `json:"id"`
	Actor     string         `json:"actor"`
	DeviceID  string         `json:"device_id"`
	Action    string         `json:"action"`
	Params    map[string]any `json:"params,omitempty"`
	Result    string         `json:"result"`
	RiskLevel RiskLevel      `json:"risk_level"`
	Allowed   bool           `json:"allowed"`
	CreatedAt time.Time      `json:"created_at"`
}

type PolicyDecision struct {
	Allowed   bool      `json:"allowed"`
	RiskLevel RiskLevel `json:"risk_level"`
	Reason    string    `json:"reason,omitempty"`
}

type CatalogPlugin struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	BinaryName  string         `json:"binary_name"`
	Manifest    PluginManifest `json:"manifest"`
}

type PluginRuntimeView struct {
	Record     PluginInstallRecord `json:"record"`
	Manifest   *PluginManifest     `json:"manifest,omitempty"`
	Health     PluginHealth        `json:"health"`
	Running    bool                `json:"running"`
	LastError  string              `json:"last_error,omitempty"`
	RecentLogs []string            `json:"recent_logs,omitempty"`
	ProcessPID int                 `json:"process_pid,omitempty"`
	ListenAddr string              `json:"listen_addr,omitempty"`
}

type DashboardSummary struct {
	Plugins        int `json:"plugins"`
	EnabledPlugins int `json:"enabled_plugins"`
	Devices        int `json:"devices"`
	OnlineDevices  int `json:"online_devices"`
	Events         int `json:"events"`
	Audits         int `json:"audits"`
}

type OAuthProvider string

const (
	OAuthProviderXiaomi OAuthProvider = "xiaomi"
)

type OAuthSessionStatus string

const (
	OAuthSessionPending   OAuthSessionStatus = "pending"
	OAuthSessionCompleted OAuthSessionStatus = "completed"
	OAuthSessionFailed    OAuthSessionStatus = "failed"
	OAuthSessionExpired   OAuthSessionStatus = "expired"
)

type OAuthSession struct {
	ID             string             `json:"id"`
	Provider       OAuthProvider      `json:"provider"`
	PluginID       string             `json:"plugin_id,omitempty"`
	AccountName    string             `json:"account_name,omitempty"`
	Region         string             `json:"region,omitempty"`
	ClientID       string             `json:"client_id,omitempty"`
	RedirectURL    string             `json:"redirect_url,omitempty"`
	DeviceID       string             `json:"device_id,omitempty"`
	State          string             `json:"state,omitempty"`
	AuthURL        string             `json:"auth_url,omitempty"`
	Status         OAuthSessionStatus `json:"status"`
	Error          string             `json:"error,omitempty"`
	AccountConfig  map[string]any     `json:"account_config,omitempty"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
	CompletedAt    *time.Time         `json:"completed_at,omitempty"`
	StateExpiresAt *time.Time         `json:"state_expires_at,omitempty"`
	TokenExpiresAt *time.Time         `json:"token_expires_at,omitempty"`
}
