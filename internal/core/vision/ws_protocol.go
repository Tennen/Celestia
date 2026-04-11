package vision

import (
	"encoding/json"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

const (
	visionWSSchemaVersion      = "celestia.vision.ws.v1"
	visionControlSchemaVersion = "celestia.vision.control.ws.v1"
	visionModelsSchemaVersion  = "celestia.vision.models.v1"

	visionMessageTypeHello         = "hello"
	visionMessageTypeError         = "error"
	visionMessageTypeRuntimeStatus = "runtime_status"
	visionMessageTypeRuleEvents    = "rule_events"
	visionMessageTypeEvidence      = "evidence"
	visionMessageTypeGetModels     = "get_models"
	visionMessageTypeModels        = "models"
	visionMessageTypeSelectModel   = "select_model"
	visionMessageTypeModelSelected = "model_selected"
	visionMessageTypeGetEntities   = "get_entities"
	visionMessageTypeEntityCatalog = "entity_catalog"
	visionMessageTypeSyncConfig    = "sync_config"
	visionMessageTypeSyncApplied   = "sync_applied"
)

type wsEnvelope struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type wsHelloPayload struct {
	SchemaVersion  string    `json:"schema_version"`
	ServiceVersion string    `json:"service_version,omitempty"`
	ConnectedAt    time.Time `json:"connected_at"`
}

type wsErrorPayload struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

type wsRuleEventsPayload struct {
	Events []models.VisionServiceEvent `json:"events"`
}

type wsEvidencePayload struct {
	Captures []models.VisionServiceEventCapture `json:"captures"`
}

type wsSelectModelPayload struct {
	ModelName *string `json:"model_name"`
}

type wsGetEntitiesPayload struct {
	ModelName string `json:"model_name,omitempty"`
}

type wsModelDescriptor struct {
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"created_at"`
	IsSelected bool      `json:"is_selected"`
	IsDefault  bool      `json:"is_default"`
}

type wsModelsPayload struct {
	SchemaVersion    string              `json:"schema_version"`
	ServiceVersion   string              `json:"service_version,omitempty"`
	CurrentModelName string              `json:"current_model_name,omitempty"`
	DefaultModelName string              `json:"default_model_name,omitempty"`
	FetchedAt        time.Time           `json:"fetched_at"`
	Models           []wsModelDescriptor `json:"models"`
}

type wsModelSelectedPayload struct {
	OK        bool      `json:"ok"`
	ModelName string    `json:"model_name,omitempty"`
	ChangedAt time.Time `json:"changed_at"`
}

type wsSyncAppliedPayload struct {
	OK        bool      `json:"ok"`
	AppliedAt time.Time `json:"applied_at"`
}
