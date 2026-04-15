package errors

import (
	"errors"
	"fmt"
)

// Code is a stable machine-readable error code (see docs/api-contracts-json.md).
type Code string

const (
	CodeInvalidCredentials     Code = "INVALID_CREDENTIALS"
	CodeAccountLocked          Code = "ACCOUNT_LOCKED"
	CodeSessionExpired         Code = "SESSION_EXPIRED"
	CodePasswordResetTokenInvalid Code = "PASSWORD_RESET_TOKEN_INVALID_OR_EXPIRED"
	CodeEmailVerificationTokenInvalid Code = "EMAIL_VERIFICATION_TOKEN_INVALID_OR_EXPIRED"
	CodeNoActiveCompanyAccess  Code = "NO_ACTIVE_COMPANY_ACCESS"
	CodeMembershipNotFound     Code = "MEMBERSHIP_NOT_FOUND"
	CodeCompanyContextRequired Code = "COMPANY_CONTEXT_REQUIRED"
	CodeCompanyScopeMismatch   Code = "COMPANY_SCOPE_MISMATCH"
	CodePermissionDenied       Code = "PERMISSION_DENIED"
	CodeDataScopeDenied        Code = "DATA_SCOPE_DENIED"
	CodeResponsibilityRequired Code = "RESPONSIBILITY_REQUIRED"
	CodeStateConflict          Code = "STATE_CONFLICT"
	CodeInvalidRequest         Code = "INVALID_REQUEST"
	CodeMFARequired            Code = "MFA_REQUIRED"
	CodeInternal               Code = "INTERNAL_ERROR"
)

// HTTPError is returned to clients as JSON { "error": { ... } }.
type HTTPError struct {
	Code       Code
	Message    string
	HTTPStatus int
	Details    map[string]any
	Cause      error
}

func (e *HTTPError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Code, e.Cause)
	}
	return string(e.Code) + ": " + e.Message
}

func (e *HTTPError) Unwrap() error { return e.Cause }

// NewHTTPError builds an HTTPError with optional wrapped cause.
func NewHTTPError(httpStatus int, code Code, message string, cause error) *HTTPError {
	return &HTTPError{
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
		Details:    nil,
		Cause:      cause,
	}
}

// AsHTTPError returns (*HTTPError, true) if err wraps HTTPError.
func AsHTTPError(err error) (*HTTPError, bool) {
	var he *HTTPError
	if errors.As(err, &he) {
		return he, true
	}
	return nil, false
}
