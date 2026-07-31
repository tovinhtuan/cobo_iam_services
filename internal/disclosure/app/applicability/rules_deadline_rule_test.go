package applicability

import "testing"

func TestResolveDeadlineRule_ToggleOffBothFlags(t *testing.T) {
	rules := &TemplateApplicabilityRules{
		UseStructureDeadline: false,
		DeadlineDays:         23,
		DeadlineDayType:      "working",
		DeadlineByStructure: map[StructureCriterion]StructureDeadlineEntry{
			StructureHasSubsidiaries: {Days: 30},
		},
	}
	profile := CompanyApplicabilityProfile{HasSubsidiaries: true, HasSubordinateAccountingUnits: true}
	res := ResolveDeadlineRule(rules, profile)
	if !res.OK || res.RuleCode != RuleCodeDefault || res.ResolutionSource != ResolutionSourceDefaultTemplateRule || res.ResolvedDays != 23 {
		t.Fatalf("got %+v", res)
	}
	days, ok := ResolveDeadlineDays(rules, profile)
	if !ok || days != 23 {
		t.Fatalf("ResolveDeadlineDays drift: days=%d ok=%v", days, ok)
	}
}

func TestResolveDeadlineRule_StructureOverrides(t *testing.T) {
	rules := &TemplateApplicabilityRules{
		UseStructureDeadline: true,
		DeadlineDays:         20,
		DeadlineByStructure: map[StructureCriterion]StructureDeadlineEntry{
			StructureHasSubsidiaries:     {Days: 30},
			StructureHasSubordinateUnits: {Days: 25},
			StructureSimpleStructure:     {Days: 15},
		},
	}
	cases := []struct {
		name    string
		profile CompanyApplicabilityProfile
		code    string
		days    int
	}{
		{"subsidiaries_only", CompanyApplicabilityProfile{HasSubsidiaries: true}, string(StructureHasSubsidiaries), 30},
		{"subordinate_only", CompanyApplicabilityProfile{HasSubordinateAccountingUnits: true}, string(StructureHasSubordinateUnits), 25},
		{"both_flags_subsidiaries_wins", CompanyApplicabilityProfile{HasSubsidiaries: true, HasSubordinateAccountingUnits: true}, string(StructureHasSubsidiaries), 30},
		{"simple", CompanyApplicabilityProfile{}, string(StructureSimpleStructure), 15},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := ResolveDeadlineRule(rules, tc.profile)
			if !res.OK || res.RuleCode != tc.code || res.ResolutionSource != ResolutionSourceStructureOverride || res.ResolvedDays != tc.days {
				t.Fatalf("got %+v want code=%s days=%d OVERRIDE", res, tc.code, tc.days)
			}
			days, ok := ResolveDeadlineDays(rules, tc.profile)
			if !ok || days != tc.days {
				t.Fatalf("ResolveDeadlineDays drift: days=%d ok=%v", days, ok)
			}
		})
	}
}

func TestResolveDeadlineRule_StructureFallbackDefault(t *testing.T) {
	rules := &TemplateApplicabilityRules{
		UseStructureDeadline: true,
		DeadlineDays:         23,
		DeadlineByStructure: map[StructureCriterion]StructureDeadlineEntry{
			// Missing has_subsidiaries entry → miss for subsidiaries profile
			StructureHasSubordinateUnits: {Days: 25},
			StructureSimpleStructure:     {Days: 15},
		},
	}
	res := ResolveDeadlineRule(rules, CompanyApplicabilityProfile{HasSubsidiaries: true})
	if !res.OK || res.RuleCode != RuleCodeDefault || res.ResolutionSource != ResolutionSourceStructureFallbackDefault || res.ResolvedDays != 23 {
		t.Fatalf("got %+v", res)
	}
}

func TestResolveDeadlineRule_NoRule(t *testing.T) {
	rules := &TemplateApplicabilityRules{
		UseStructureDeadline: true,
		DeadlineDays:         0,
		DeadlineByStructure:  map[StructureCriterion]StructureDeadlineEntry{},
	}
	res := ResolveDeadlineRule(rules, CompanyApplicabilityProfile{})
	if res.OK || res.ResolutionSource != ResolutionSourceNoRule {
		t.Fatalf("got %+v", res)
	}
	res = ResolveDeadlineRule(nil, CompanyApplicabilityProfile{})
	if res.OK || res.ResolutionSource != ResolutionSourceNoRule {
		t.Fatalf("nil rules: %+v", res)
	}
}

func TestResolveDeadlineDurationType_WorkingAndCalendar(t *testing.T) {
	if got := ResolveDeadlineDurationType(&TemplateApplicabilityRules{DeadlineDayType: "working"}); got != "WORKING_DAYS" {
		t.Fatalf("working: %s", got)
	}
	if got := ResolveDeadlineDurationType(&TemplateApplicabilityRules{DeadlineDayType: ""}); got != "CALENDAR_DAYS" {
		t.Fatalf("empty: %s", got)
	}
}

func TestDeadlineRuleLabelKey(t *testing.T) {
	if DeadlineRuleLabelKey(RuleCodeDefault) != "deadline.rule.default" {
		t.Fatal("default key")
	}
	if DeadlineRuleLabelKey(string(StructureHasSubsidiaries)) != "deadline.rule.has_subsidiaries" {
		t.Fatal("subsidiaries key")
	}
}
