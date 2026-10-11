package temper

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Error is an error returned by the control plane (a non-2xx response), or an error from the SDK itself (Status 0,
// e.g. a network error or a failed signature check).
type Error struct {
	// Status is the HTTP status; 0 for network errors, timeouts and invalid webhook signatures.
	Status int
	// Code is the machine-readable code: the error field of the control plane's error body. Non-JSON error
	// responses (e.g. from a proxy in front of the control plane) are recorded as http_<status>.
	Code string
	// Message is a human-readable description.
	Message string
	// Err is the wrapped underlying error (network errors and the like).
	Err error
	// EnvironmentID is the id of the existing environment for environment_exists and idempotency_key_reused.
	EnvironmentID string
}

func (e *Error) Error() string {
	if e.Status != 0 {
		return fmt.Sprintf("temper: %d %s: %s", e.Status, e.Code, e.Message)
	}
	return fmt.Sprintf("temper: %s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Err }

// IsCode reports whether err is (or wraps) an *Error with the given code.
func IsCode(err error, code string) bool {
	var e *Error
	return errors.As(err, &e) && e.Code == code
}

// Common error codes. See the Temper API reference for the full list.
const (
	CodeBadRequest           = "bad_request"
	CodeUnauthorized         = "unauthorized"
	CodeForbidden            = "forbidden"
	CodeNotFound             = "not_found"
	CodeInvalidBody          = "invalid_body"
	CodeEnvironmentExists    = "environment_exists"
	CodeInvalidTransition    = "invalid_state_transition"
	CodeEnvironmentFailed    = "environment_failed"
	CodeEnvironmentNotPlaced = "environment_not_placed"
	CodeEnvironmentNotReady  = "environment_not_running"
	CodeHostUnavailable      = "host_unavailable"
	CodeGuestUnavailable     = "guest_unavailable"
	CodeLimitExceeded        = "limit_exceeded"
	CodeNoOAuthClient        = "no_oauth_client"
	CodeAlreadyDecided       = "already_decided"
	CodeMissingScope         = "missing_scope"
	CodeInvalidScope         = "invalid_scope"
	CodeScopeTooWide         = "scope_too_wide"
	CodeExportTooLarge       = "export_too_large"
	CodeTooManyProcesses     = "too_many_processes"

	// Codes produced by the SDK itself.
	CodeNetwork = "network_error"
	CodeTimeout = "timeout"
	CodeAborted = "aborted"
	// Environments.WaitFor: the environment was destroyed while waiting, or the wait timed out (a failed
	// environment returns CodeEnvironmentFailed).
	CodeEnvironmentDestroyed = "environment_destroyed"
	CodeWaitTimeout          = "wait_timeout"
	// Webhook signature verification failed (VerifyWebhook).
	CodeWebhookSignature = "webhook_signature"
)

func errorFromResponse(status int, contentType string, body []byte) *Error {
	if strings.HasPrefix(contentType, "application/json") {
		var b struct {
			Error         string `json:"error"`
			Message       string `json:"message"`
			EnvironmentID string `json:"environment_id"`
		}
		if json.Unmarshal(body, &b) == nil && b.Error != "" {
			return &Error{Status: status, Code: b.Error, Message: b.Message, EnvironmentID: b.EnvironmentID}
		}
	}
	msg := strings.TrimSpace(string(body))
	if len(msg) > 500 {
		msg = msg[:500]
	}
	return &Error{Status: status, Code: fmt.Sprintf("http_%d", status), Message: msg}
}
