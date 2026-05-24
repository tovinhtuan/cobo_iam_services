package app

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// resolveProposedDeadline maps API input to persisted day-count and optional calendar date.
// FE sends day count via proposed_deadline_days or legacy proposed_deadline_date ("20").
// Absolute dates use proposed_deadline_date as YYYY-MM-DD.
func resolveProposedDeadline(t0, legacyDate string, days int) (dayCount *int, calendarDate *string, err error) {
	if days > 0 {
		v := days
		return &v, nil, nil
	}
	raw := strings.TrimSpace(legacyDate)
	if raw == "" {
		return nil, nil, nil
	}
	if d, parseErr := time.Parse("2006-01-02", raw); parseErr == nil {
		s := d.Format("2006-01-02")
		return nil, &s, nil
	}
	n, convErr := strconv.Atoi(raw)
	if convErr != nil || n <= 0 {
		return nil, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest,
			"proposed_deadline_date must be YYYY-MM-DD or a positive day count", nil)
	}
	v := n
	return &v, nil, nil
}
