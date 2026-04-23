package control

import (
	"fmt"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

type ValidationError struct {
	Err error
}

func (e *ValidationError) Error() string {
	if e == nil || e.Err == nil {
		return "invalid home request"
	}
	return e.Err.Error()
}

func (e *ValidationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type NotFoundError struct {
	Field string
	Value string
	Err   error
}

func (e *NotFoundError) Error() string {
	if e == nil {
		return "not found"
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

func (e *NotFoundError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type AmbiguousReferenceError struct {
	Field   string
	Value   string
	Matches []HomeResolveMatch
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

type PolicyDeniedError struct {
	Decision models.PolicyDecision
}

func (e *PolicyDeniedError) Error() string {
	if e == nil {
		return "command denied by policy"
	}
	if e.Decision.Reason != "" {
		return e.Decision.Reason
	}
	return "command denied by policy"
}

type CommandExecutionError struct {
	Err error
}

func (e *CommandExecutionError) Error() string {
	if e == nil || e.Err == nil {
		return "command execution failed"
	}
	return e.Err.Error()
}

func (e *CommandExecutionError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}
