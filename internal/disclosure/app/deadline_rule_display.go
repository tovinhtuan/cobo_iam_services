package app

import (
	"regexp"
	"strings"
)

var (
	deadlineRuleTNValue = regexp.MustCompile(`^T\+(\d+)$`)
	deadlineRuleDMValue = regexp.MustCompile(`^(\d{2}/\d{2})$`)
)

// FormatDeadlineRuleDisplay maps compact legacy codes (e.g. T+3, 31/03) via catalog.
// Deprecated for Portal display enrichment: product treats deadline_rule as free text.
// Kept for reference-data / admin tooling compatibility.
func FormatDeadlineRuleDisplay(deadlineRule string, catalog []DeadlineRuleCatalogDTO) string {
	rule := strings.TrimSpace(deadlineRule)
	if rule == "" {
		return ""
	}
	for _, item := range catalog {
		pattern := strings.TrimSpace(item.Pattern)
		if pattern == "" {
			continue
		}
		re, err := regexp.Compile(pattern)
		if err != nil || !re.MatchString(rule) {
			continue
		}
		label := strings.TrimSpace(item.LabelVI)
		if label == "" {
			return rule
		}
		switch strings.TrimSpace(item.InputType) {
		case "number":
			if m := deadlineRuleTNValue.FindStringSubmatch(rule); len(m) == 2 {
				return strings.ReplaceAll(label, "N", m[1])
			}
		case "date_dm":
			if m := deadlineRuleDMValue.FindStringSubmatch(rule); len(m) == 2 {
				return strings.ReplaceAll(label, "dd/mm", m[1])
			}
		}
		return label
	}
	return rule
}

// T0PolicyBasisLabelVI returns the honest Vietnamese basis label for t0_policy (engine context only).
func T0PolicyBasisLabelVI(t0Policy string) string {
	switch strings.ToLower(strings.TrimSpace(t0Policy)) {
	case "system_date":
		return "Ngày hệ thống"
	case "event_date":
		return "Ngày sự kiện"
	case "user_defined":
		return "Ngày do người dùng xác định"
	default:
		return ""
	}
}

// enrichDeadlineRuleDisplay sets deadline_rule_display to the raw admin text (display-only SoT).
// Does not expand catalog phrases, does not use deadline_config / t0_policy for Portal copy.
func enrichDeadlineRuleDisplay(item *DisclosureTypeDTO, _ []DeadlineRuleCatalogDTO) {
	if item == nil {
		return
	}
	item.DeadlineRuleDisplay = strings.TrimSpace(item.DeadlineRule)
	// Do not populate TimeCalculationBasis from engine t0 — Portal display-only UX hides basis.
}

func enrichDeadlineRuleDisplaySummary(item *DisclosureTypeSummaryDTO, _ []DeadlineRuleCatalogDTO) {
	if item == nil {
		return
	}
	item.DeadlineRuleDisplay = strings.TrimSpace(item.DeadlineRule)
}
