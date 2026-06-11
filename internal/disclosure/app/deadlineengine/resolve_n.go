package deadlineengine

import (
	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
)

// ResolveEffectiveN returns the effective N (deadline_days) for a template.
//
// Source of truth: rules.DeadlineDays. rules.DeadlineByStructure is a
// conditional override, applied only when rules.UseStructureDeadline is true:
//
//	toggle OFF                           -> deadline_days
//	toggle ON  + structure key hit       -> deadline_by_structure[criterion].days
//	toggle ON  + structure key miss      -> fallback deadline_days
//	toggle ON  + key miss + days invalid -> ErrStructureDeadlineUnresolved (E11)
func ResolveEffectiveN(rules *Rules, profile applicability.CompanyApplicabilityProfile) (int, error) {
	n, _, err := resolveEffectiveN(rules, profile)
	return n, err
}

// resolveEffectiveN is the internal core for ResolveEffectiveN. It additionally
// reports whether the structure-based override was used (needed by
// ResolveDeadline for Resolution.UsedStructureDeadline / hint suffix).
func resolveEffectiveN(rules *Rules, profile applicability.CompanyApplicabilityProfile) (int, bool, error) {
	if rules == nil {
		return 0, false, ErrRulesRequired
	}
	if !rules.UseStructureDeadline {
		if rules.DeadlineDays <= 0 {
			return 0, false, ErrInvalidDeadlineDays
		}
		return rules.DeadlineDays, false, nil
	}

	criterion := applicability.ResolveStructure(profile)
	if entry, ok := rules.DeadlineByStructure[criterion]; ok && entry.Days > 0 {
		return entry.Days, true, nil
	}

	if rules.DeadlineDays > 0 {
		return rules.DeadlineDays, false, nil
	}

	return 0, false, ErrStructureDeadlineUnresolved
}
