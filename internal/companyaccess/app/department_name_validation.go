package app

import (
	"net/http"
	"strings"
	"unicode/utf8"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

const maxDepartmentNameBytes = 255

// validateDepartmentName ensures department names are plain business text (no HTML/script).
func validateDepartmentName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", departmentNameFieldError("name is required")
	}
	if len([]byte(trimmed)) > maxDepartmentNameBytes {
		return "", departmentNameFieldError("name must be at most 255 UTF-8 bytes")
	}
	if !utf8.ValidString(trimmed) {
		return "", departmentNameFieldError("name must be valid UTF-8 text")
	}
	lower := strings.ToLower(trimmed)
	if strings.Contains(trimmed, "<") || strings.Contains(trimmed, ">") {
		return "", departmentNameFieldError("name must not contain HTML markup")
	}
	if strings.Contains(lower, "javascript:") {
		return "", departmentNameFieldError("name must not contain javascript URLs")
	}
	if strings.Contains(lower, "onerror") {
		return "", departmentNameFieldError("name must not contain script event handlers")
	}
	return trimmed, nil
}

func departmentNameFieldError(message string) error {
	return &perr.HTTPError{
		Code:       perr.CodeInvalidRequest,
		Message:    message,
		HTTPStatus: http.StatusBadRequest,
		Details: map[string]any{
			"field_errors": map[string]string{
				"department_name": message,
			},
		},
	}
}
