package app

import (
	"net/http"
	"strings"
	"unicode/utf8"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// MaxDeadlineDisplayTextRunes caps Portal/CMS deadline_rule as free-form display text.
const MaxDeadlineDisplayTextRunes = 1000

// validatePortalDeadlineRule treats deadline_rule as display-only free text for Portal.
// It is NOT matched against deadline_rule_catalog patterns and is NOT parsed for runtime.
func validatePortalDeadlineRule(deadlineRule string, _ []DeadlineRuleCatalogDTO) error {
	rule := strings.TrimSpace(deadlineRule)
	if rule == "" {
		return nil
	}
	if utf8.RuneCountInString(rule) > MaxDeadlineDisplayTextRunes {
		return perr.NewHTTPError(
			http.StatusBadRequest,
			perr.CodeInvalidRequest,
			"deadline_rule exceeds maximum length of 1000 characters",
			nil,
		)
	}
	return nil
}
