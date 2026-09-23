package app

import (
	"fmt"
	"strings"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
)

// EnsureCompatibilityDeadlineRule normalizes CMS/global upsert deadline_rule for
// periodic templates from applicability_rules.deadline_days (SoT).
//
// Policy (PO 2026-09-23):
//   - periodic: always overwrite with "T+" + deadline_days when days > 0
//   - structure override does NOT change the compatibility string (still default days)
//   - irregular: leave client value unchanged
//
// Call before publication-matrix validation that requires non-empty deadline_rule.
// Does not mutate company-template flows that keep a different contract.
func EnsureCompatibilityDeadlineRule(req *UpsertTypeVersionRequest) {
	if req == nil {
		return
	}
	if !strings.EqualFold(strings.TrimSpace(req.TemplateCategory), TemplateCategoryPeriodic) {
		return
	}
	days := periodicCompatibilityDeadlineDays(req.ApplicabilityRules)
	if days <= 0 {
		return
	}
	req.DeadlineRule = fmt.Sprintf("T+%d", days)
}

func periodicCompatibilityDeadlineDays(rules *applicability.TemplateApplicabilityRules) int {
	if rules == nil {
		return 0
	}
	if rules.DeadlineDays <= 0 {
		return 0
	}
	return rules.DeadlineDays
}

// DeriveImportCompatibilityDeadlineRule applies the same periodic policy on the
// import normalize path (before domain validation). Irregular unchanged.
func DeriveImportCompatibilityDeadlineRule(def *TemplateImportDefinitionV1) {
	if def == nil {
		return
	}
	if def.TemplateCategory != TemplateCategoryPeriodic {
		return
	}
	days := periodicCompatibilityDeadlineDays(def.ApplicabilityRules)
	if days <= 0 {
		return
	}
	def.DeadlineRule = fmt.Sprintf("T+%d", days)
}
