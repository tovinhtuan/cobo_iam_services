package app

import (
	"strings"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
)

// coercePeriodicDeadlineEngineMode aligns legacy rows where applicability carries deadline_days
// but deadline_config.deadline_mode was persisted as NONE. Does not mutate deadline_rule display text.
func coercePeriodicDeadlineEngineMode(item *DisclosureTypeDTO) {
	if item == nil || item.DeadlineConfig == nil {
		return
	}
	if !isPeriodicTemplateForDeadline(item.TemplateCategory, item.Periodicity) {
		return
	}
	mode := strings.TrimSpace(item.DeadlineConfig.DeadlineMode)
	if mode != "" && !strings.EqualFold(mode, DeadlineModeNone) {
		return
	}
	days := item.DeadlineConfig.DeadlineDays
	if item.ApplicabilityRules != nil {
		if resolved, ok := applicability.ResolveDeadlineDays(item.ApplicabilityRules, applicability.CompanyApplicabilityProfile{}); ok && resolved > 0 {
			days = resolved
		} else if item.ApplicabilityRules.DeadlineDays > 0 {
			days = item.ApplicabilityRules.DeadlineDays
		}
	}
	if days <= 0 {
		return
	}
	item.DeadlineConfig.DeadlineMode = DeadlineModePeriodic
	if item.DeadlineConfig.DeadlineDays <= 0 {
		item.DeadlineConfig.DeadlineDays = days
	}
}

func isPeriodicTemplateForDeadline(templateCategory, periodicity string) bool {
	cat := strings.ToLower(strings.TrimSpace(templateCategory))
	if cat == TemplateCategoryPeriodic || cat == "định kỳ" || cat == "dinh ky" {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(periodicity)) {
	case "daily", "weekly", "monthly", "quarterly", "yearly", "annual":
		return true
	default:
		return false
	}
}
