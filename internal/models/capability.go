package models

import "time"

type CapabilityKind string

const (
	CapabilityKindAutomation           CapabilityKind = "automation"
	CapabilityKindVisionEntityStayZone CapabilityKind = "vision_entity_stay_zone"
)

type Capability struct {
	ID          string         `json:"id"`
	Kind        CapabilityKind `json:"kind"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Enabled     bool           `json:"enabled"`
	Status      HealthState    `json:"status"`
	Summary     map[string]any `json:"summary,omitempty"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type CapabilityDetail struct {
	Capability
	Automation *AutomationCapabilityDetail `json:"automation,omitempty"`
	Vision     *VisionCapabilityDetail     `json:"vision,omitempty"`
}

type AutomationCapabilityDetail struct {
	Total           int        `json:"total"`
	EnabledCount    int        `json:"enabled_count"`
	LastTriggeredAt *time.Time `json:"last_triggered_at,omitempty"`
}

type VisionCapabilityDetail struct {
	Config       VisionCapabilityConfig `json:"config"`
	Runtime      VisionCapabilityStatus `json:"runtime"`
	Catalog      *VisionEntityCatalog   `json:"catalog,omitempty"`
	RecentEvents []Event                `json:"recent_events,omitempty"`
}
