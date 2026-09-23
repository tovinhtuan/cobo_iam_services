package app

import (
	"testing"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
)

func TestEnsureCompatibilityDeadlineRule_PeriodicDerivesFromDays(t *testing.T) {
	req := UpsertTypeVersionRequest{
		TemplateCategory: TemplateCategoryPeriodic,
		DeadlineRule:     "Trong vòng 99 ngày (legacy raw)",
		ApplicabilityRules: &applicability.TemplateApplicabilityRules{
			DeadlineDays:         30,
			UseStructureDeadline: true,
			DeadlineByStructure: map[applicability.StructureCriterion]applicability.StructureDeadlineEntry{
				applicability.StructureHasSubsidiaries: {Days: 45},
			},
		},
	}
	EnsureCompatibilityDeadlineRule(&req)
	if req.DeadlineRule != "T+30" {
		t.Fatalf("compatibility must use default deadline_days, got %q", req.DeadlineRule)
	}
}

func TestEnsureCompatibilityDeadlineRule_IrregularUnchanged(t *testing.T) {
	req := UpsertTypeVersionRequest{
		TemplateCategory: TemplateCategoryIrregular,
		DeadlineRule:     "Trong vòng 24 giờ kể từ sự kiện",
		ApplicabilityRules: &applicability.TemplateApplicabilityRules{
			DeadlineDays: 30,
		},
	}
	EnsureCompatibilityDeadlineRule(&req)
	if req.DeadlineRule != "Trong vòng 24 giờ kể từ sự kiện" {
		t.Fatalf("irregular must not derive from applicability, got %q", req.DeadlineRule)
	}
}

func TestEnsureCompatibilityDeadlineRule_PeriodicMissingDaysNoOverwriteEmpty(t *testing.T) {
	req := UpsertTypeVersionRequest{
		TemplateCategory: TemplateCategoryPeriodic,
		DeadlineRule:     "",
		ApplicabilityRules: &applicability.TemplateApplicabilityRules{
			DeadlineDays: 0,
		},
	}
	EnsureCompatibilityDeadlineRule(&req)
	if req.DeadlineRule != "" {
		t.Fatalf("expected empty when days missing, got %q", req.DeadlineRule)
	}
}

func TestDeriveImportCompatibilityDeadlineRule_OverridesRaw(t *testing.T) {
	def := &TemplateImportDefinitionV1{
		TemplateCategory: TemplateCategoryPeriodic,
		DeadlineRule:     "T+5",
		ApplicabilityRules: &applicability.TemplateApplicabilityRules{
			DeadlineDays: 20,
		},
	}
	DeriveImportCompatibilityDeadlineRule(def)
	if def.DeadlineRule != "T+20" {
		t.Fatalf("applicability must win over raw, got %q", def.DeadlineRule)
	}
}

func TestDeriveImportCompatibilityDeadlineRule_MissingRuleWithDays(t *testing.T) {
	def := &TemplateImportDefinitionV1{
		TemplateCategory: TemplateCategoryPeriodic,
		DeadlineRule:     "",
		ApplicabilityRules: &applicability.TemplateApplicabilityRules{
			DeadlineDays: 15,
		},
	}
	DeriveImportCompatibilityDeadlineRule(def)
	if def.DeadlineRule != "T+15" {
		t.Fatalf("got %q", def.DeadlineRule)
	}
}
