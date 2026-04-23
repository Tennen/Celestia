package gateway

import (
	"fmt"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

type AIDevice struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Aliases  []string    `json:"aliases,omitempty"`
	Commands []AICommand `json:"commands"`
}

type AICommand struct {
	Name     string           `json:"name"`
	Aliases  []string         `json:"aliases,omitempty"`
	Action   string           `json:"action,omitempty"`
	Params   []AICommandParam `json:"params,omitempty"`
	Defaults map[string]any   `json:"defaults,omitempty"`
}

type AICommandParam struct {
	Name     string                        `json:"name"`
	Type     models.DeviceCommandParamType `json:"type"`
	Required bool                          `json:"required,omitempty"`
	Default  any                           `json:"default,omitempty"`
	Options  []models.DeviceControlOption  `json:"options,omitempty"`
	Min      *float64                      `json:"min,omitempty"`
	Max      *float64                      `json:"max,omitempty"`
	Step     *float64                      `json:"step,omitempty"`
	Unit     string                        `json:"unit,omitempty"`
}

type AICommandRequest struct {
	Target     string         `json:"target,omitempty"`
	DeviceID   string         `json:"device_id,omitempty"`
	DeviceName string         `json:"device_name,omitempty"`
	Command    string         `json:"command,omitempty"`
	Action     string         `json:"action,omitempty"`
	Actor      string         `json:"actor,omitempty"`
	Params     map[string]any `json:"params,omitempty"`
	Values     []string       `json:"values,omitempty"`
}

type AICommandResult struct {
	Device   AIResolvedDevice       `json:"device"`
	Command  AIResolvedCommand      `json:"command"`
	Decision models.PolicyDecision  `json:"decision"`
	Result   models.CommandResponse `json:"result"`
}

type AIResolvedDevice struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type AIResolvedCommand struct {
	Name   string         `json:"name"`
	Action string         `json:"action"`
	Target string         `json:"target"`
	Params map[string]any `json:"params,omitempty"`
}

type AIResolveMatch struct {
	DeviceID   string `json:"device_id,omitempty"`
	DeviceName string `json:"device_name,omitempty"`
	Room       string `json:"room,omitempty"`
	Command    string `json:"command,omitempty"`
	Action     string `json:"action,omitempty"`
	Target     string `json:"target,omitempty"`
}

type AmbiguousReferenceError struct {
	Field   string           `json:"field"`
	Value   string           `json:"value"`
	Matches []AIResolveMatch `json:"matches"`
}

func (e *AmbiguousReferenceError) Error() string {
	field := strings.TrimSpace(e.Field)
	if field == "" {
		field = "reference"
	}
	value := strings.TrimSpace(e.Value)
	if value == "" {
		return fmt.Sprintf("%s is ambiguous", field)
	}
	return fmt.Sprintf("%s %q is ambiguous", field, value)
}

type ReferenceNotFoundError struct {
	Field string `json:"field"`
	Value string `json:"value"`
	Err   error  `json:"-"`
}

func (e *ReferenceNotFoundError) Error() string {
	if e == nil {
		return "reference not found"
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	field := strings.TrimSpace(e.Field)
	if field == "" {
		field = "reference"
	}
	value := strings.TrimSpace(e.Value)
	if value == "" {
		return field + " not found"
	}
	return fmt.Sprintf("%s %q not found", field, value)
}

func (e *ReferenceNotFoundError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}
