package app_test

import (
	"strings"
	"testing"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
)

func TestValidateImportTemplate_PeriodicDeadlineRuleOptionalWithDays(t *testing.T) {
	validDisplayGroups := map[string]struct{}{"display_groups_003": {}}
	day := 31
	month := 3
	tpl := disclosureapp.TemplateImportDefinitionV1{
		Name:             "Periodic no deadline_rule input",
		TemplateCategory: "periodic",
		Periodicity:      "quarterly",
		DeadlineRule:     "", // missing — derive in normalize
		DisplayGroupCodes: []string{"display_groups_003"},
		ApplicabilityRules: &applicability.TemplateApplicabilityRules{
			ApplicableCompanyClasses: []applicability.CompanyClass{applicability.CompanyClassListed},
			ApplicableSectors:        []applicability.BusinessSector{applicability.BusinessSectorCommercial},
			DeadlineDays:             20,
			DeadlineDayType:          "calendar",
			UseStructureDeadline:     false,
			DeadlineByStructure: map[applicability.StructureCriterion]applicability.StructureDeadlineEntry{
				applicability.StructureHasSubsidiaries:     {Days: 30},
				applicability.StructureHasSubordinateUnits: {Days: 25},
				applicability.StructureSimpleStructure:     {Days: 20},
			},
		},
		DeadlineConfig: &disclosureapp.TemplateImportDeadlineConfigV1{
			FrequencyUnit:  "QUARTERLY",
			CycleAnchorDay: &day,
			MonthInQuarter: &month,
			DurationType:   "CALENDAR_DAYS",
		},
		Workflow: &disclosureapp.TemplateImportWorkflowV1{
			Steps: []disclosureapp.TemplateImportWorkflowStepV1{
				{
					Stage:          "Soạn",
					ProcessingDays: 5,
					AssigneeRoles:  []string{"REVIEWER"},
					Department:     &disclosureapp.TemplateImportDepartmentRefV1{Code: "dept-001"},
				},
			},
		},
	}
	norm := disclosureapp.NormalizeTemplateImportV1(tpl)
	if norm.DeadlineRule != "T+20" {
		t.Fatalf("normalize derive got %q", norm.DeadlineRule)
	}
	errs, _, _, _ := disclosureapp.ValidateImportTemplate(
		norm,
		time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC),
		validDisplayGroups,
		map[string]disclosureapp.TemplateDepartmentDTO{"dept-001": {DepartmentCode: "dept-001", DepartmentName: "KT"}},
		map[string]disclosureapp.TemplateDepartmentDTO{"kt": {DepartmentCode: "dept-001", DepartmentName: "KT"}},
	)
	for _, e := range errs {
		if e.Code == "DEADLINE_RULE_REQUIRED" {
			t.Fatalf("periodic with days must not require deadline_rule: %+v", e)
		}
	}
}

func TestValidateImportTemplate_PeriodicRawT99OverriddenByDays20(t *testing.T) {
	tpl := disclosureapp.TemplateImportDefinitionV1{
		Name:             "Periodic raw T+99 override",
		TemplateCategory: "periodic",
		Periodicity:      "monthly",
		DeadlineRule:     "T+99",
		ApplicabilityRules: &applicability.TemplateApplicabilityRules{
			DeadlineDays: 20,
		},
	}
	norm := disclosureapp.NormalizeTemplateImportV1(tpl)
	if norm.DeadlineRule != "T+20" {
		t.Fatalf("applicability must win over raw T+99, got %q", norm.DeadlineRule)
	}
}

func TestValidateImportTemplate_IrregularStillRequiresDeadlineRule(t *testing.T) {
	tpl := disclosureapp.TemplateImportDefinitionV1{
		Name:             "Irregular empty rule",
		TemplateCategory: "irregular",
		Periodicity:      "event_based",
		DeadlineRule:     "",
		ApplicabilityRules: &applicability.TemplateApplicabilityRules{
			DeadlineDays: 30, // must not derive for irregular
		},
	}
	norm := disclosureapp.NormalizeTemplateImportV1(tpl)
	if norm.DeadlineRule != "" {
		t.Fatalf("irregular must not derive from applicability, got %q", norm.DeadlineRule)
	}
	errs, _, _, _ := disclosureapp.ValidateImportTemplate(
		norm,
		time.Now(),
		map[string]struct{}{"display_groups_001": {}},
		nil,
		nil,
	)
	found := false
	for _, e := range errs {
		if e.Code == "DEADLINE_RULE_REQUIRED" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected DEADLINE_RULE_REQUIRED for irregular, errs=%v", errs)
	}
}

func TestValidateImportTemplate_IrregularKeepsFreeTextWithApplicabilityDays(t *testing.T) {
	tpl := disclosureapp.TemplateImportDefinitionV1{
		Name:             "Irregular free text",
		TemplateCategory: "irregular",
		Periodicity:      "event_based",
		DeadlineRule:     "Trong vòng 24 giờ kể từ sự kiện",
		ApplicabilityRules: &applicability.TemplateApplicabilityRules{
			DeadlineDays: 30,
		},
	}
	norm := disclosureapp.NormalizeTemplateImportV1(tpl)
	if norm.DeadlineRule != "Trong vòng 24 giờ kể từ sự kiện" {
		t.Fatalf("irregular must keep free-text, not derive T+N, got %q", norm.DeadlineRule)
	}
}

func TestValidateImportTemplate_PeriodicMissingDaysReportsApplicability(t *testing.T) {
	tpl := disclosureapp.NormalizeTemplateImportV1(disclosureapp.TemplateImportDefinitionV1{
		Name:             "Periodic missing days",
		TemplateCategory: "periodic",
		Periodicity:      "monthly",
		DeadlineRule:     "T+99", // must NOT invent days from raw
		ApplicabilityRules: &applicability.TemplateApplicabilityRules{
			ApplicableCompanyClasses: []applicability.CompanyClass{applicability.CompanyClassListed},
			ApplicableSectors:        []applicability.BusinessSector{applicability.BusinessSectorCommercial},
			DeadlineDays:             0,
			DeadlineByStructure: map[applicability.StructureCriterion]applicability.StructureDeadlineEntry{
				applicability.StructureHasSubsidiaries:     {Days: 30},
				applicability.StructureHasSubordinateUnits: {Days: 25},
				applicability.StructureSimpleStructure:     {Days: 20},
			},
		},
	})
	if tpl.DeadlineRule != "T+99" {
		t.Fatalf("normalize must not overwrite when days<=0, got %q", tpl.DeadlineRule)
	}
	errs, _, _, _ := disclosureapp.ValidateImportTemplate(
		tpl,
		time.Now(),
		map[string]struct{}{"display_groups_003": {}},
		nil,
		nil,
	)
	if len(errs) == 0 {
		t.Fatal("expected domain errors when periodic days missing")
	}
	found := false
	for _, e := range errs {
		if e.Code == "APPLICABILITY_DEADLINE_DAYS_REQUIRED" &&
			e.FieldPath == "template.applicability_rules.deadline_days" {
			found = true
			if strings.Contains(strings.ToLower(e.Message), "stack") ||
				strings.Contains(e.Message, "goroutine") ||
				strings.Contains(e.Message, "runtime.") {
				t.Fatalf("validation error must not expose stack/internal: %+v", e)
			}
		}
	}
	if !found {
		t.Fatalf("expected APPLICABILITY_DEADLINE_DAYS_REQUIRED at applicability_rules.deadline_days, errs=%v", errs)
	}
}

func TestValidateImportTemplate_LegacyPeriodicOmitApplicabilityKeepsRaw(t *testing.T) {
	// Legacy fixture: no applicability_rules, only deadline_rule — must not fail days-required.
	day := 15
	tpl := disclosureapp.NormalizeTemplateImportV1(disclosureapp.TemplateImportDefinitionV1{
		Name:              "Legacy periodic omit rules",
		TemplateCategory:  "periodic",
		Periodicity:       "monthly",
		DeadlineRule:      "T+20",
		DisplayGroupCodes: []string{"display_groups_003"},
		DeadlineConfig: &disclosureapp.TemplateImportDeadlineConfigV1{
			FrequencyUnit:  "MONTHLY",
			CycleAnchorDay: &day,
			DurationType:   "CALENDAR_DAYS",
		},
	})
	if tpl.ApplicabilityRules != nil {
		t.Fatal("legacy omit must not invent applicability_rules")
	}
	if tpl.DeadlineRule != "T+20" {
		t.Fatalf("legacy raw must be preserved, got %q", tpl.DeadlineRule)
	}
	errs, _, _, _ := disclosureapp.ValidateImportTemplate(
		tpl,
		time.Now(),
		map[string]struct{}{"display_groups_003": {}},
		nil,
		nil,
	)
	for _, e := range errs {
		if e.Code == "APPLICABILITY_DEADLINE_DAYS_REQUIRED" {
			t.Fatalf("legacy omit applicability must not require deadline_days: %+v", e)
		}
	}
}

func TestValidateImportTemplate_IrregularOmitApplicabilityUnchanged(t *testing.T) {
	tpl := disclosureapp.NormalizeTemplateImportV1(disclosureapp.TemplateImportDefinitionV1{
		Name:             "Irregular omit rules",
		TemplateCategory: "irregular",
		Periodicity:      "event_based",
		DeadlineRule:     "Trong vòng 24 giờ kể từ sự kiện",
	})
	if tpl.DeadlineRule != "Trong vòng 24 giờ kể từ sự kiện" {
		t.Fatalf("got %q", tpl.DeadlineRule)
	}
	errs, _, _, _ := disclosureapp.ValidateImportTemplate(
		tpl,
		time.Now(),
		map[string]struct{}{"display_groups_001": {}},
		nil,
		nil,
	)
	for _, e := range errs {
		if e.Code == "APPLICABILITY_DEADLINE_DAYS_REQUIRED" {
			t.Fatalf("irregular must not hit periodic days validation: %+v", e)
		}
	}
}
