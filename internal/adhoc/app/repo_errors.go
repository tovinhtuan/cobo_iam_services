package app

import (
	"errors"
	"net/http"
	"strings"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func mapRepositoryError(err error) error {
	if err == nil {
		return nil
	}
	var he *perr.HTTPError
	if errors.As(err, &he) {
		return err
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "Incorrect date value"),
		strings.Contains(msg, "Unknown column 'proposed_deadline_days'"),
		strings.Contains(msg, "cannot be null"),
		strings.Contains(msg, "foreign key constraint"):
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest,
			"could not save ad-hoc proposal: "+msg, err)
	default:
		return err
	}
}
