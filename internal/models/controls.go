package models

import "time"

type DeviceControlKind string

const (
	DeviceControlKindToggle DeviceControlKind = "toggle"
	DeviceControlKindAction DeviceControlKind = "action"
	DeviceControlKindSelect DeviceControlKind = "select"
	DeviceControlKindNumber DeviceControlKind = "number"
)

type DeviceControlOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type DeviceCommandParamType string

const (
	DeviceCommandParamTypeBoolean DeviceCommandParamType = "boolean"
	DeviceCommandParamTypeNumber  DeviceCommandParamType = "number"
	DeviceCommandParamTypeString  DeviceCommandParamType = "string"
)

type DeviceCommandParamSpec struct {
	Name     string                 `json:"name"`
	Type     DeviceCommandParamType `json:"type"`
	Required bool                   `json:"required,omitempty"`
	Default  any                    `json:"default,omitempty"`
	Options  []DeviceControlOption  `json:"options,omitempty"`
	Min      *float64               `json:"min,omitempty"`
	Max      *float64               `json:"max,omitempty"`
	Step     *float64               `json:"step,omitempty"`
	Unit     string                 `json:"unit,omitempty"`
}

type DeviceControlCommand struct {
	Action     string                   `json:"action"`
	Params     map[string]any           `json:"params,omitempty"`
	ValueParam string                   `json:"value_param,omitempty"`
	ParamsSpec []DeviceCommandParamSpec `json:"params_spec,omitempty"`
}

// DeviceControlSpec is plugin-declared control metadata persisted inside device metadata.
type DeviceControlSpec struct {
	ID             string                `json:"id"`
	Kind           DeviceControlKind     `json:"kind"`
	Label          string                `json:"label"`
	Disabled       bool                  `json:"disabled,omitempty"`
	DisabledReason string                `json:"disabled_reason,omitempty"`
	StateKey       string                `json:"state_key,omitempty"`
	Min            *float64              `json:"min,omitempty"`
	Max            *float64              `json:"max,omitempty"`
	Step           *float64              `json:"step,omitempty"`
	Unit           string                `json:"unit,omitempty"`
	Options        []DeviceControlOption `json:"options,omitempty"`
	Command        *DeviceControlCommand `json:"command,omitempty"`
	OnCommand      *DeviceControlCommand `json:"on_command,omitempty"`
	OffCommand     *DeviceControlCommand `json:"off_command,omitempty"`
}

type DeviceControl struct {
	ID             string                `json:"id"`
	Kind           DeviceControlKind     `json:"kind"`
	Label          string                `json:"label"`
	DefaultLabel   string                `json:"default_label,omitempty"`
	Alias          string                `json:"alias,omitempty"`
	Disabled       bool                  `json:"disabled,omitempty"`
	DisabledReason string                `json:"disabled_reason,omitempty"`
	State          *bool                 `json:"state,omitempty"`
	Value          any                   `json:"value,omitempty"`
	Min            *float64              `json:"min,omitempty"`
	Max            *float64              `json:"max,omitempty"`
	Step           *float64              `json:"step,omitempty"`
	Unit           string                `json:"unit,omitempty"`
	Options        []DeviceControlOption `json:"options,omitempty"`
	Command        *DeviceControlCommand `json:"command,omitempty"`
	Visible        bool                  `json:"visible"`
}

type DeviceControlPreference struct {
	DeviceID  string    `json:"device_id"`
	ControlID string    `json:"control_id"`
	Alias     string    `json:"alias,omitempty"`
	Visible   bool      `json:"visible"`
	UpdatedAt time.Time `json:"updated_at"`
}
