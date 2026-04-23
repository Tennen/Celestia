package models

import "time"

type AutomationLogic string

const (
	AutomationLogicAll AutomationLogic = "all"
	AutomationLogicAny AutomationLogic = "any"
)

type AutomationMatchOperator string

const (
	AutomationMatchAny       AutomationMatchOperator = "any"
	AutomationMatchEquals    AutomationMatchOperator = "equals"
	AutomationMatchNotEquals AutomationMatchOperator = "not_equals"
	AutomationMatchIn        AutomationMatchOperator = "in"
	AutomationMatchNotIn     AutomationMatchOperator = "not_in"
	AutomationMatchExists    AutomationMatchOperator = "exists"
	AutomationMatchMissing   AutomationMatchOperator = "missing"
)

type AutomationRunStatus string

const (
	AutomationRunStatusIdle      AutomationRunStatus = "idle"
	AutomationRunStatusSucceeded AutomationRunStatus = "succeeded"
	AutomationRunStatusFailed    AutomationRunStatus = "failed"
)

type AutomationStateMatch struct {
	Operator AutomationMatchOperator `json:"operator"`
	Value    any                     `json:"value,omitempty"`
}

type AutomationConditionType string

const (
	AutomationConditionTypeStateChanged AutomationConditionType = "state_changed"
	AutomationConditionTypeCurrentState AutomationConditionType = "current_state"
	AutomationConditionTypeTime         AutomationConditionType = "time"
)

type AutomationTimeCondition struct {
	Schedule string `json:"schedule"`
	At       string `json:"at"`
	Timezone string `json:"timezone,omitempty"`
}

type AutomationCondition struct {
	Type     AutomationConditionType  `json:"type,omitempty"`
	DeviceID string                   `json:"device_id,omitempty"`
	StateKey string                   `json:"state_key,omitempty"`
	From     *AutomationStateMatch    `json:"from,omitempty"`
	To       *AutomationStateMatch    `json:"to,omitempty"`
	Match    *AutomationStateMatch    `json:"match,omitempty"`
	Time     *AutomationTimeCondition `json:"time,omitempty"`
}

type AutomationTimeWindow struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type AutomationActionKind string

const (
	AutomationActionKindDevice AutomationActionKind = "device"
	AutomationActionKindAgent  AutomationActionKind = "agent"
)

type AutomationAction struct {
	Kind     AutomationActionKind `json:"kind,omitempty"`
	DeviceID string               `json:"device_id"`
	Label    string               `json:"label,omitempty"`
	Action   string               `json:"action"`
	Params   map[string]any       `json:"params,omitempty"`
}

type Automation struct {
	ID              string                `json:"id"`
	Name            string                `json:"name"`
	Enabled         bool                  `json:"enabled"`
	ConditionLogic  AutomationLogic       `json:"condition_logic"`
	Conditions      []AutomationCondition `json:"conditions,omitempty"`
	TimeWindow      *AutomationTimeWindow `json:"time_window,omitempty"`
	Actions         []AutomationAction    `json:"actions"`
	LastTriggeredAt *time.Time            `json:"last_triggered_at,omitempty"`
	LastRunStatus   AutomationRunStatus   `json:"last_run_status,omitempty"`
	LastError       string                `json:"last_error,omitempty"`
	CreatedAt       time.Time             `json:"created_at"`
	UpdatedAt       time.Time             `json:"updated_at"`
}
