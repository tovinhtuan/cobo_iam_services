package app

import (
	"regexp"
	"strings"
)

var (
	deadlineRuleTNValue = regexp.MustCompile(`^T\+(\d+)$`)
	deadlineRuleDMValue = regexp.MustCompile(`^(\d{2}/\d{2})$`)
)

// FormatDeadlineRuleDisplay maps evaluated deadline_rule values (e.g. T+3, 31/03)
// to human-readable Vietnamese labels from deadline_rule_catalog.
// When no catalog pattern matches, the raw value is returned unchanged.
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

func enrichDeadlineRuleDisplay(item *DisclosureTypeDTO, catalog []DeadlineRuleCatalogDTO) {
	if item == nil {
		return
	}
	item.DeadlineRuleDisplay = FormatDeadlineRuleDisplay(item.DeadlineRule, catalog)
}

func enrichDeadlineRuleDisplaySummary(item *DisclosureTypeSummaryDTO, catalog []DeadlineRuleCatalogDTO) {
	if item == nil {
		return
	}
	item.DeadlineRuleDisplay = FormatDeadlineRuleDisplay(item.DeadlineRule, catalog)
}
