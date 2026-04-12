package models

import "time"

const VisionCapabilityID = "vision_entity_stay_zone"

const DefaultVisionEventCaptureRetentionHours = 168

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
	ServiceWSURL               string       `json:"service_ws_url"`
	ModelName                  string       `json:"model_name,omitempty"`
	RecognitionEnabled         bool         `json:"recognition_enabled"`
	EventCaptureRetentionHours int          `json:"event_capture_retention_hours"`
	Rules                      []VisionRule `json:"rules,omitempty"`
	UpdatedAt                  time.Time    `json:"updated_at"`
}

type VisionEntityCatalog struct {
	ServiceWSURL   string                   `json:"service_ws_url"`
	SchemaVersion  string                   `json:"schema_version"`
	ServiceVersion string                   `json:"service_version,omitempty"`
	ModelName      string                   `json:"model_name,omitempty"`
	FetchedAt      time.Time                `json:"fetched_at"`
	Entities       []VisionEntityDescriptor `json:"entities"`
}

type VisionEntityCatalogRefreshRequest struct {
	ServiceWSURL string `json:"service_ws_url,omitempty"`
	ModelName    string `json:"model_name,omitempty"`
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
)

type VisionServiceEvent struct {
	EventID        string                   `json:"event_id,omitempty"`
	RuleID         string                   `json:"rule_id"`
	CameraDeviceID string                   `json:"camera_device_id,omitempty"`
	Status         VisionServiceEventStatus `json:"status"`
	ObservedAt     time.Time                `json:"observed_at"`
	DwellSeconds   int                      `json:"dwell_seconds"`
	EntityValue    string                   `json:"entity_value,omitempty"`
	Entities       []VisionEntityDescriptor `json:"entities,omitempty"`
	Metadata       map[string]any           `json:"metadata,omitempty"`
}

type VisionServiceEventBatch struct {
	Events []VisionServiceEvent `json:"events"`
}

type VisionEventCapturePhase string

const (
	VisionEventCapturePhaseStart  VisionEventCapturePhase = "start"
	VisionEventCapturePhaseMiddle VisionEventCapturePhase = "middle"
	VisionEventCapturePhaseEnd    VisionEventCapturePhase = "end"
)

type VisionEventCapture struct {
	CaptureID      string                  `json:"capture_id"`
	EventID        string                  `json:"event_id"`
	RuleID         string                  `json:"rule_id,omitempty"`
	CameraDeviceID string                  `json:"camera_device_id,omitempty"`
	Phase          VisionEventCapturePhase `json:"phase"`
	CapturedAt     time.Time               `json:"captured_at"`
	ContentType    string                  `json:"content_type"`
	SizeBytes      int                     `json:"size_bytes"`
	Metadata       map[string]any          `json:"metadata,omitempty"`
}

type VisionEventCaptureAsset struct {
	Capture VisionEventCapture `json:"capture"`
	Data    []byte             `json:"-"`
}

type VisionServiceEventCapture struct {
	CaptureID      string                  `json:"capture_id,omitempty"`
	EventID        string                  `json:"event_id"`
	RuleID         string                  `json:"rule_id,omitempty"`
	CameraDeviceID string                  `json:"camera_device_id,omitempty"`
	Phase          VisionEventCapturePhase `json:"phase"`
	CapturedAt     time.Time               `json:"captured_at"`
	ContentType    string                  `json:"content_type,omitempty"`
	ImageBase64    string                  `json:"image_base64"`
	Metadata       map[string]any          `json:"metadata,omitempty"`
}

type VisionServiceEventCaptureBatch struct {
	Captures []VisionServiceEventCapture `json:"captures"`
}

type VisionServiceEntityCatalog struct {
	SchemaVersion  string                   `json:"schema_version"`
	ServiceVersion string                   `json:"service_version,omitempty"`
	ModelName      string                   `json:"model_name,omitempty"`
	FetchedAt      time.Time                `json:"fetched_at"`
	Entities       []VisionEntityDescriptor `json:"entities"`
}

type VisionServiceSyncPayload struct {
	SchemaVersion      string              `json:"schema_version"`
	SentAt             time.Time           `json:"sent_at"`
	RecognitionEnabled bool                `json:"recognition_enabled"`
	Rules              []VisionServiceRule `json:"rules"`
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
