package applicability_test

import (
	"testing"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
)

// Phase 4 consistency: production ResolveStructure/ResolveDeadlineRule must match
// the documented C1–C9 fixture expectations consumed by FE cross-layer tests.
func TestPhase4_ConsistencyMatrix_ResolveDeadlineRule(t *testing.T) {
	type structureMap map[applicability.StructureCriterion]applicability.StructureDeadlineEntry
	cases := []struct {
		id                   string
		useStructure         bool
		hasSubs              bool
		hasSubUnits          bool
		deadlineDays         int
		dayType              string
		byStructure          structureMap
		wantCode             string
		wantSource           string
		wantDays             int
		wantOK               bool
		wantDuration         string
		wantStructureResolve applicability.StructureCriterion
	}{
		{
			id: "C1", useStructure: false, hasSubs: true, hasSubUnits: true, deadlineDays: 23, dayType: "working",
			byStructure: structureMap{
				applicability.StructureHasSubsidiaries:     {Days: 30},
				applicability.StructureHasSubordinateUnits: {Days: 25},
				applicability.StructureSimpleStructure:     {Days: 15},
			},
			wantCode: applicability.RuleCodeDefault, wantSource: applicability.ResolutionSourceDefaultTemplateRule,
			wantDays: 23, wantOK: true, wantDuration: "WORKING_DAYS",
			wantStructureResolve: applicability.StructureHasSubsidiaries, // ResolveStructure still picks subsidiaries; toggle off ignores for N
		},
		{
			id: "C2", useStructure: true, hasSubs: true, hasSubUnits: false, deadlineDays: 20, dayType: "working",
			byStructure: structureMap{
				applicability.StructureHasSubsidiaries:     {Days: 23},
				applicability.StructureHasSubordinateUnits: {Days: 30},
				applicability.StructureSimpleStructure:     {Days: 15},
			},
			wantCode: string(applicability.StructureHasSubsidiaries), wantSource: applicability.ResolutionSourceStructureOverride,
			wantDays: 23, wantOK: true, wantDuration: "WORKING_DAYS",
			wantStructureResolve: applicability.StructureHasSubsidiaries,
		},
		{
			id: "C3", useStructure: true, hasSubs: false, hasSubUnits: true, deadlineDays: 20, dayType: "working",
			byStructure: structureMap{
				applicability.StructureHasSubsidiaries:     {Days: 23},
				applicability.StructureHasSubordinateUnits: {Days: 30},
				applicability.StructureSimpleStructure:     {Days: 15},
			},
			wantCode: string(applicability.StructureHasSubordinateUnits), wantSource: applicability.ResolutionSourceStructureOverride,
			wantDays: 30, wantOK: true, wantDuration: "WORKING_DAYS",
			wantStructureResolve: applicability.StructureHasSubordinateUnits,
		},
		{
			id: "C4", useStructure: true, hasSubs: true, hasSubUnits: true, deadlineDays: 20, dayType: "working",
			byStructure: structureMap{
				applicability.StructureHasSubsidiaries:     {Days: 23},
				applicability.StructureHasSubordinateUnits: {Days: 30},
				applicability.StructureSimpleStructure:     {Days: 15},
			},
			wantCode: string(applicability.StructureHasSubsidiaries), wantSource: applicability.ResolutionSourceStructureOverride,
			wantDays: 23, wantOK: true, wantDuration: "WORKING_DAYS",
			wantStructureResolve: applicability.StructureHasSubsidiaries,
		},
		{
			id: "C5", useStructure: true, hasSubs: false, hasSubUnits: false, deadlineDays: 20, dayType: "working",
			byStructure: structureMap{
				applicability.StructureHasSubsidiaries:     {Days: 23},
				applicability.StructureHasSubordinateUnits: {Days: 30},
				applicability.StructureSimpleStructure:     {Days: 15},
			},
			wantCode: string(applicability.StructureSimpleStructure), wantSource: applicability.ResolutionSourceStructureOverride,
			wantDays: 15, wantOK: true, wantDuration: "WORKING_DAYS",
			wantStructureResolve: applicability.StructureSimpleStructure,
		},
		{
			id: "C6", useStructure: true, hasSubs: true, hasSubUnits: false, deadlineDays: 23, dayType: "working",
			byStructure: structureMap{
				applicability.StructureHasSubordinateUnits: {Days: 30},
				applicability.StructureSimpleStructure:     {Days: 15},
			},
			wantCode: applicability.RuleCodeDefault, wantSource: applicability.ResolutionSourceStructureFallbackDefault,
			wantDays: 23, wantOK: true, wantDuration: "WORKING_DAYS",
			wantStructureResolve: applicability.StructureHasSubsidiaries,
		},
		{
			id: "C7", useStructure: true, hasSubs: false, hasSubUnits: false, deadlineDays: 0, dayType: "working",
			byStructure:          structureMap{},
			wantCode:             "",
			wantSource:           applicability.ResolutionSourceNoRule,
			wantDays:             0,
			wantOK:               false,
			wantDuration:         "WORKING_DAYS",
			wantStructureResolve: applicability.StructureSimpleStructure,
		},
		{
			id: "C8", useStructure: false, deadlineDays: 10, dayType: "working",
			wantCode: applicability.RuleCodeDefault, wantSource: applicability.ResolutionSourceDefaultTemplateRule,
			wantDays: 10, wantOK: true, wantDuration: "WORKING_DAYS",
			wantStructureResolve: applicability.StructureSimpleStructure,
		},
		{
			id: "C9", useStructure: false, deadlineDays: 10, dayType: "calendar",
			wantCode: applicability.RuleCodeDefault, wantSource: applicability.ResolutionSourceDefaultTemplateRule,
			wantDays: 10, wantOK: true, wantDuration: "CALENDAR_DAYS",
			wantStructureResolve: applicability.StructureSimpleStructure,
		},
	}

	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			profile := applicability.CompanyApplicabilityProfile{
				HasSubsidiaries:               tc.hasSubs,
				HasSubordinateAccountingUnits: tc.hasSubUnits,
			}
			if got := applicability.ResolveStructure(profile); got != tc.wantStructureResolve {
				t.Fatalf("ResolveStructure=%s want %s", got, tc.wantStructureResolve)
			}
			rules := &applicability.TemplateApplicabilityRules{
				UseStructureDeadline: tc.useStructure,
				DeadlineDays:         tc.deadlineDays,
				DeadlineDayType:      tc.dayType,
				DeadlineByStructure:  tc.byStructure,
			}
			res := applicability.ResolveDeadlineRule(rules, profile)
			if res.RuleCode != tc.wantCode || res.ResolutionSource != tc.wantSource || res.OK != tc.wantOK {
				t.Fatalf("ResolveDeadlineRule=%+v want code=%s source=%s ok=%v", res, tc.wantCode, tc.wantSource, tc.wantOK)
			}
			if tc.wantOK && res.ResolvedDays != tc.wantDays {
				t.Fatalf("days=%d want %d", res.ResolvedDays, tc.wantDays)
			}
			days, ok := applicability.ResolveDeadlineDays(rules, profile)
			if ok != tc.wantOK || (tc.wantOK && days != tc.wantDays) {
				t.Fatalf("ResolveDeadlineDays drift days=%d ok=%v", days, ok)
			}
			if got := applicability.ResolveDeadlineDurationType(rules); got != tc.wantDuration {
				t.Fatalf("dayType=%s want %s", got, tc.wantDuration)
			}
		})
	}
}
