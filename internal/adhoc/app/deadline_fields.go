package app

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// normalizeDateOnly coerces MySQL DATE / RFC3339 / datetime strings to YYYY-MM-DD
// before writing into DATE columns. Drivers often scan DATE as "2006-01-02T00:00:00Z".
func normalizeDateOnly(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if len(raw) >= 10 && raw[4] == '-' && raw[7] == '-' {
		candidate := raw[:10]
		if _, err := time.Parse("2006-01-02", candidate); err == nil {
			return candidate
		}
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC().Format("2006-01-02")
	}
	if t, err := time.Parse("2006-01-02 15:04:05", raw); err == nil {
		return t.Format("2006-01-02")
	}
	return raw
}

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
	if d, parseErr := time.Parse("2006-01-02", normalizeDateOnly(raw)); parseErr == nil {
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
