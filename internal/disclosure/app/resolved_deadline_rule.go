package app

import (
	"strings"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
)

// buildResolvedDeadlineRuleDTO maps production ResolveDeadlineRule + duration type
// onto the additive type-detail DTO. Does not invent due dates; callers attach due_date
// from DeadlineSummary when calculator output is present.
func buildResolvedDeadlineRuleDTO(
	rules *applicability.TemplateApplicabilityRules,
	profile applicability.CompanyApplicabilityProfile,
	periodicity string,
	deadlineConfig *TemplateDeadlineConfig,
) *ResolvedDeadlineRuleDTO {
	if rules == nil {
		return nil
	}
	res := applicability.ResolveDeadlineRule(rules, profile)
	dto := &ResolvedDeadlineRuleDTO{
		ResolutionSource: res.ResolutionSource,
		DayType:          applicability.ResolveDeadlineDurationType(rules),
		Periodicity:      strings.TrimSpace(periodicity),
	}
	if res.RuleCode != "" {
		dto.RuleCode = res.RuleCode
		dto.RuleLabelKey = applicability.DeadlineRuleLabelKey(res.RuleCode)
	}
	if res.OK {
		days := res.ResolvedDays
		dto.ResolvedDays = &days
	}
	if deadlineConfig != nil && deadlineConfig.DeadlineMode == DeadlineModePeriodic {
		dto.BaseDateSource = BaseDateSourceCycleStart
	}
	return dto
}

func attachResolvedDueDate(rule *ResolvedDeadlineRuleDTO, summary *DeadlineSummaryDTO) {
	if rule == nil || summary == nil || summary.DeadlineDate == nil {
		return
	}
	due := strings.TrimSpace(*summary.DeadlineDate)
	if due == "" {
		return
	}
	rule.DueDate = &due
}
