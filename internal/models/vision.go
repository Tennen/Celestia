package models

import "time"

const VisionCapabilityID = "vision_entity_stay_zone"

type VisionEntitySelector struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type VisionEntityDescriptor struct {
	Kind        string `json:"kind"`
	Value       string `json:"value"`
	DisplayName string `json:"display_name,omitempty"`
}

type VisionZoneBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type VisionRTSPSource struct {
	URL string `json:"url"`
}

type VisionRule struct {
	ID                   string               `json:"id"`
	Name                 string               `json:"name"`
	Enabled              bool                 `json:"enabled"`
	CameraDeviceID       string               `json:"camera_device_id"`
	RecognitionEnabled   bool                 `json:"recognition_enabled"`
	RTSPSource           VisionRTSPSource     `json:"rtsp_source"`
	EntitySelector       VisionEntitySelector `json:"entity_selector"`
	Zone                 VisionZoneBox        `json:"zone"`
	StayThresholdSeconds int                  `json:"stay_threshold_seconds"`
}

type VisionCapabilityConfig struct {
	ServiceURL         string       `json:"service_url"`
	RecognitionEnabled bool         `json:"recognition_enabled"`
	Rules              []VisionRule `json:"rules,omitempty"`
	UpdatedAt          time.Time    `json:"updated_at"`
}

type VisionEntityCatalog struct {
	ServiceURL     string                   `json:"service_url"`
	SchemaVersion  string                   `json:"schema_version"`
	ServiceVersion string                   `json:"service_version,omitempty"`
	ModelName      string                   `json:"model_name,omitempty"`
	FetchedAt      time.Time                `json:"fetched_at"`
	Entities       []VisionEntityDescriptor `json:"entities"`
}

type VisionEntityCatalogRefreshRequest struct {
	ServiceURL string `json:"service_url,omitempty"`
}

type VisionCapabilityStatus struct {
	Status         HealthState    `json:"status"`
	Message        string         `json:"message,omitempty"`
	ServiceVersion string         `json:"service_version,omitempty"`
	LastSyncedAt   *time.Time     `json:"last_synced_at,omitempty"`
	LastReportedAt *time.Time     `json:"last_reported_at,omitempty"`
	LastEventAt    *time.Time     `json:"last_event_at,omitempty"`
	Runtime        map[string]any `json:"runtime,omitempty"`
	SyncError      string         `json:"sync_error,omitempty"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type VisionServiceStatusReport struct {
	Status         HealthState    `json:"status"`
	Message        string         `json:"message,omitempty"`
	ServiceVersion string         `json:"service_version,omitempty"`
	ReportedAt     time.Time      `json:"reported_at"`
	Runtime        map[string]any `json:"runtime,omitempty"`
}

type VisionServiceEventStatus string

const (
	VisionServiceEventStatusThresholdMet VisionServiceEventStatus = "threshold_met"
	VisionServiceEventStatusCleared      VisionServiceEventStatus = "cleared"
)

type VisionServiceEvent struct {
	EventID        string                   `json:"event_id,omitempty"`
	RuleID         string                   `json:"rule_id"`
	CameraDeviceID string                   `json:"camera_device_id,omitempty"`
	Status         VisionServiceEventStatus `json:"status"`
	ObservedAt     time.Time                `json:"observed_at"`
	DwellSeconds   int                      `json:"dwell_seconds"`
	EntityValue    string                   `json:"entity_value,omitempty"`
	Metadata       map[string]any           `json:"metadata,omitempty"`
}

type VisionServiceEventBatch struct {
	Events []VisionServiceEvent `json:"events"`
}

type VisionServiceEntityCatalog struct {
	SchemaVersion  string                   `json:"schema_version"`
	ServiceVersion string                   `json:"service_version,omitempty"`
	ModelName      string                   `json:"model_name,omitempty"`
	FetchedAt      time.Time                `json:"fetched_at"`
	Entities       []VisionEntityDescriptor `json:"entities"`
}

type VisionServiceSyncPayload struct {
	SchemaVersion      string                     `json:"schema_version"`
	SentAt             time.Time                  `json:"sent_at"`
	RecognitionEnabled bool                       `json:"recognition_enabled"`
	Callbacks          VisionServiceSyncCallbacks `json:"callbacks"`
	Rules              []VisionServiceRule        `json:"rules"`
}

type VisionServiceSyncCallbacks struct {
	StatusPath string `json:"status_path"`
	EventPath  string `json:"event_path"`
}

type VisionServiceRule struct {
	ID                   string               `json:"id"`
	Name                 string               `json:"name"`
	Enabled              bool                 `json:"enabled"`
	Camera               VisionServiceCamera  `json:"camera"`
	RTSPSource           VisionRTSPSource     `json:"rtsp_source"`
	EntitySelector       VisionEntitySelector `json:"entity_selector"`
	Zone                 VisionZoneBox        `json:"zone"`
	StayThresholdSeconds int                  `json:"stay_threshold_seconds"`
}

type VisionServiceCamera struct {
	DeviceID       string `json:"device_id"`
	PluginID       string `json:"plugin_id"`
	VendorDeviceID string `json:"vendor_device_id"`
	Name           string `json:"name"`
	EntryID        string `json:"entry_id,omitempty"`
}
