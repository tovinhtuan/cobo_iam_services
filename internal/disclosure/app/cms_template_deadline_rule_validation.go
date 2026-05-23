package app

import (
	"net/http"
	"regexp"
	"strings"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func validatePortalDeadlineRule(deadlineRule string, catalog []DeadlineRuleCatalogDTO) error {
	rule := strings.TrimSpace(deadlineRule)
	if rule == "" {
		return nil
	}
	validPatterns := 0
	for _, item := range catalog {
		pattern := strings.TrimSpace(item.Pattern)
		if pattern == "" {
			continue
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}
		validPatterns++
		if re.MatchString(rule) {
			return nil
		}
	}
	if validPatterns == 0 {
		return nil
	}
	return perr.NewHTTPError(
		http.StatusBadRequest,
		perr.CodeInvalidRequest,
		"deadline_rule does not match supported deadline rule formats",
		nil,
	)
}
