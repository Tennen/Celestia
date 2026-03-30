package gateway

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/chentianyu/celestia/internal/models"
)

// StatusError carries a transport-agnostic status code that callers can map
// to HTTP status or CLI exit behaviour.
type StatusError struct {
	StatusCode int
	Err        error
}

func (e *StatusError) Error() string {
	if e == nil || e.Err == nil {
		return http.StatusText(http.StatusInternalServerError)
	}
	return e.Err.Error()
}

func (e *StatusError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func statusError(statusCode int, err error) error {
	if err == nil {
		err = fmt.Errorf("http %d", statusCode)
	}
	return &StatusError{StatusCode: statusCode, Err: err}
}

func statusErrorf(statusCode int, format string, args ...any) error {
	return statusError(statusCode, fmt.Errorf(format, args...))
}

func StatusCode(err error) int {
	if err == nil {
		return 0
	}
	var typed *StatusError
	if errors.As(err, &typed) {
		return typed.StatusCode
	}
	return 0
}

// PolicyDeniedError preserves policy evaluation details for callers that need
// richer denied responses than a plain error string.
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
