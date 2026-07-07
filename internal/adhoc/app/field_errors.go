package app

import (
	"net/http"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func newAdHocFieldError(httpStatus int, code perr.Code, field, message string) error {
	he := perr.NewHTTPError(httpStatus, code, message, nil)
	he.Details = map[string]any{"field": field}
	return he
}

func newAdHocPermissionError(permission, message string) error {
	he := perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, message, nil)
	he.Details = map[string]any{"permission": permission}
	return he
}
