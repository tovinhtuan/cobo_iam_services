package deadlineengine

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
)

// refTime used across E2E tests: 2026-04-15 (Wednesday).
var engineRefTime = date(2026, 4, 15)

// --- PD-DE-CUSTOM-01 mandatory examples (Cases 1-5), end-to-end -----------------

func TestResolveDeadline_Case1_GlobalPeriodic_Monthly(t *testing.T) {
	template := TemplateInput{
		TemplateCategory: "periodic",
		T0Config: T0ConfigInput{
			FrequencyUnit:  "monthly",
			CycleAnchorDay: 5,
		},
		Rules: &Rules{DeadlineDays: 20, DeadlineDayType: "calendar"},
	}

	res, err := ResolveDeadline(context.Background(), template, CompanyInput{}, nil, RuntimeInput{}, engineRefTime, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.ResolvedT0.Equal(date(2026, 4, 5)) {
		t.Fatalf("expected T0=2026-04-05, got %v", res.ResolvedT0)
	}
	// Inclusive counting: T0 + (N-1) = 2026-04-05 + 19 = 2026-04-24.
	if !res.PlannedDate.Equal(date(2026, 4, 24)) {
		t.Fatalf("expected planned=2026-04-24, got %v", res.PlannedDate)
	}
	if res.UsedCompanyT0Override {
		t.Fatalf("expected no company override")
	}
}

func TestResolveDeadline_Case2_GlobalIrregular_SystemDate(t *testing.T) {
	template := TemplateInput{
		TemplateCategory: "irregular",
		T0Config: T0ConfigInput{
			T0Policy: "system_date",
		},
		Rules: &Rules{DeadlineDays: 10, DeadlineDayType: "calendar"},
	}

	res, err := ResolveDeadline(context.Background(), template, CompanyInput{}, nil, RuntimeInput{}, engineRefTime, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.ResolvedT0.Equal(date(2026, 4, 15)) {
		t.Fatalf("expected T0=2026-04-15, got %v", res.ResolvedT0)
	}
	if !res.PlannedDate.Equal(date(2026, 4, 24)) {
		t.Fatalf("expected planned=2026-04-24, got %v", res.PlannedDate)
	}
}

func TestResolveDeadline_Case3_CompanyCustomPeriodic_Quarterly(t *testing.T) {
	// PD-DE-CUSTOM-01 critical case: template_category="custom" +
	// frequency_unit="quarterly" -> behavior=periodic (R-P), NOT irregular.
	template := TemplateInput{
		TemplateCategory: "custom",
		T0Config: T0ConfigInput{
			FrequencyUnit:    "quarterly",
			CycleAnchorMonth: 1,
		},
		Rules: &Rules{DeadlineDays: 15, DeadlineDayType: "calendar"},
	}

	behavior, err := DeriveDeadlineBehavior(template)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if behavior != DeadlineBehaviorPeriodic {
		t.Fatalf("expected periodic behavior for custom+quarterly, got %v", behavior)
	}

	res, err := ResolveDeadline(context.Background(), template, CompanyInput{}, nil, RuntimeInput{}, engineRefTime, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// anchorMonth=1, refTime=2026-04-15 -> fiscal quarter starting 2026-04-01.
	if !res.ResolvedT0.Equal(date(2026, 4, 1)) {
		t.Fatalf("expected T0=2026-04-01, got %v", res.ResolvedT0)
	}
	if !res.PlannedDate.Equal(date(2026, 4, 15)) {
		t.Fatalf("expected planned=2026-04-15, got %v", res.PlannedDate)
	}
}

func TestResolveDeadline_Case4_CompanyCustomIrregular_UserDefined(t *testing.T) {
	// PD-DE-CUSTOM-01: template_category="custom" + t0_policy="user_defined"
	// (no frequency_unit) -> behavior=irregular (R-I).
	template := TemplateInput{
		TemplateCategory: "custom",
		T0Config: T0ConfigInput{
			T0Policy: "user_defined",
		},
		Rules: &Rules{DeadlineDays: 5, DeadlineDayType: "calendar"},
	}

	behavior, err := DeriveDeadlineBehavior(template)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if behavior != DeadlineBehaviorIrregular {
		t.Fatalf("expected irregular behavior for custom+user_defined, got %v", behavior)
	}

	res, err := ResolveDeadline(context.Background(), template, CompanyInput{}, nil, RuntimeInput{T0Date: "2026-05-01"}, engineRefTime, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.ResolvedT0.Equal(date(2026, 5, 1)) {
		t.Fatalf("expected T0=2026-05-01, got %v", res.ResolvedT0)
	}
	if !res.PlannedDate.Equal(date(2026, 5, 5)) {
		t.Fatalf("expected planned=2026-05-05, got %v", res.PlannedDate)
	}
}

func TestResolveDeadline_Case5_CompanyCustom_NoFrequency_NoT0Policy_Errors(t *testing.T) {
	// PD-DE-CUSTOM-01: template_category="custom", neither frequency_unit nor
	// t0_policy set -> MUST NOT default to periodic or irregular.
	template := TemplateInput{
		TemplateCategory: "custom",
		T0Config:         T0ConfigInput{},
		Rules:            &Rules{DeadlineDays: 5, DeadlineDayType: "calendar"},
	}

	_, err := DeriveDeadlineBehavior(template)
	if !errors.Is(err, ErrInvalidT0Configuration) {
		t.Fatalf("expected ErrInvalidT0Configuration from DeriveDeadlineBehavior, got %v", err)
	}

	_, err = ResolveDeadline(context.Background(), template, CompanyInput{}, nil, RuntimeInput{}, engineRefTime, nil)
	if !errors.Is(err, ErrInvalidT0Configuration) {
		t.Fatalf("expected ErrInvalidT0Configuration from ResolveDeadline, got %v", err)
	}
}

func TestResolveDeadline_Case5b_CompanyCustom_NoFrequency_WithT0Policy_Irregular(t *testing.T) {
	// custom, no frequency_unit, but t0_policy=system_date set -> irregular,
	// must NOT error.
	template := TemplateInput{
		TemplateCategory: "custom",
		T0Config: T0ConfigInput{
			T0Policy: "system_date",
		},
		Rules: &Rules{DeadlineDays: 5, DeadlineDayType: "calendar"},
	}

	behavior, err := DeriveDeadlineBehavior(template)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if behavior != DeadlineBehaviorIrregular {
		t.Fatalf("expected irregular, got %v", behavior)
	}
}

// --- Error propagation -------------------------------------------------------

func TestResolveDeadline_RulesRequired(t *testing.T) {
	template := TemplateInput{
		TemplateCategory: "periodic",
		T0Config:         T0ConfigInput{FrequencyUnit: "monthly", CycleAnchorDay: 5},
		Rules:            nil,
	}

	_, err := ResolveDeadline(context.Background(), template, CompanyInput{}, nil, RuntimeInput{}, engineRefTime, nil)
	if !errors.Is(err, ErrRulesRequired) {
		t.Fatalf("expected ErrRulesRequired, got %v", err)
	}
}

func TestResolveDeadline_EventDateNotImplemented(t *testing.T) {
	template := TemplateInput{
		TemplateCategory: "irregular",
		T0Config:         T0ConfigInput{T0Policy: "event_date"},
		Rules:            &Rules{DeadlineDays: 10, DeadlineDayType: "calendar"},
	}

	_, err := ResolveDeadline(context.Background(), template, CompanyInput{}, nil, RuntimeInput{}, engineRefTime, nil)
	if !errors.Is(err, ErrEventDateNotImplemented) {
		t.Fatalf("expected ErrEventDateNotImplemented, got %v", err)
	}
}

func TestResolveDeadline_StructureDeadlineUnresolved_WrappedAsRuleUnresolved(t *testing.T) {
	template := TemplateInput{
		TemplateCategory: "periodic",
		T0Config:         T0ConfigInput{FrequencyUnit: "monthly", CycleAnchorDay: 5},
		Rules: &Rules{
			DeadlineDays:         0,
			UseStructureDeadline: true,
			DeadlineByStructure:  map[applicability.StructureCriterion]applicability.StructureDeadlineEntry{},
		},
	}

	_, err := ResolveDeadline(context.Background(), template, CompanyInput{}, nil, RuntimeInput{}, engineRefTime, nil)
	if !errors.Is(err, ErrDeadlineRuleUnresolved) {
		t.Fatalf("expected ErrDeadlineRuleUnresolved, got %v", err)
	}
	if !errors.Is(err, ErrStructureDeadlineUnresolved) {
		t.Fatalf("expected wrapped ErrStructureDeadlineUnresolved, got %v", err)
	}
}

// --- Structure-based N (UsedStructureDeadline) -------------------------------

func TestResolveDeadline_UsedStructureDeadline(t *testing.T) {
	template := TemplateInput{
		TemplateCategory: "periodic",
		T0Config:         T0ConfigInput{FrequencyUnit: "monthly", CycleAnchorDay: 5},
		Rules: &Rules{
			DeadlineDays:         20,
			DeadlineDayType:      "calendar",
			UseStructureDeadline: true,
			DeadlineByStructure: map[applicability.StructureCriterion]applicability.StructureDeadlineEntry{
				applicability.StructureHasSubsidiaries: {Days: 30},
			},
		},
	}
	company := CompanyInput{
		Profile: applicability.CompanyApplicabilityProfile{HasSubsidiaries: true},
	}

	res, err := ResolveDeadline(context.Background(), template, company, nil, RuntimeInput{}, engineRefTime, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.UsedStructureDeadline {
		t.Fatalf("expected UsedStructureDeadline=true")
	}
	if res.EffectiveDeadlineDays != 30 {
		t.Fatalf("expected 30, got %d", res.EffectiveDeadlineDays)
	}
	// T0=2026-04-05, N=30 calendar -> 2026-04-05 + 29 = 2026-05-04.
	if !res.PlannedDate.Equal(date(2026, 5, 4)) {
		t.Fatalf("expected planned=2026-05-04, got %v", res.PlannedDate)
	}
}

// --- Company T0 override (R-C) ------------------------------------------------

func TestResolveDeadline_CompanyT0Override_Applied(t *testing.T) {
	template := TemplateInput{
		TemplateCategory: "custom",
		T0Config: T0ConfigInput{
			FrequencyUnit:    "monthly",
			CycleAnchorDay:   5,
			AllowT0Override:  true,
		},
		Rules: &Rules{DeadlineDays: 10, DeadlineDayType: "calendar"},
	}
	company := CompanyInput{CycleAnchorDay: 20}

	res, err := ResolveDeadline(context.Background(), template, company, nil, RuntimeInput{}, engineRefTime, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.UsedCompanyT0Override {
		t.Fatalf("expected UsedCompanyT0Override=true")
	}
	if !res.ResolvedT0.Equal(date(2026, 4, 20)) {
		t.Fatalf("expected T0=2026-04-20, got %v", res.ResolvedT0)
	}
	if !strings.Contains(res.HintLabelVI, "theo T0 doanh nghiệp đã cấu hình") {
		t.Fatalf("expected hint to mention company T0 override, got %q", res.HintLabelVI)
	}
}

// --- Working day type integration --------------------------------------------

func TestResolveDeadline_WorkingDayType(t *testing.T) {
	// 2026-04-03 is a Friday.
	template := TemplateInput{
		TemplateCategory: "irregular",
		T0Config:         T0ConfigInput{T0Policy: "user_defined"},
		Rules:            &Rules{DeadlineDays: 2, DeadlineDayType: "working"},
	}

	res, err := ResolveDeadline(context.Background(), template, CompanyInput{}, nil, RuntimeInput{T0Date: "2026-04-03"}, engineRefTime, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.EffectiveDayType != DayTypeWorking {
		t.Fatalf("expected working day type, got %v", res.EffectiveDayType)
	}
	// Friday + N=2 working (inclusive) -> skip weekend -> Monday 2026-04-06.
	if !res.PlannedDate.Equal(date(2026, 4, 6)) {
		t.Fatalf("expected planned=2026-04-06, got %v", res.PlannedDate)
	}
}

// --- I-21 short-circuit (PeriodicCycleContext) --------------------------------

func TestResolveDeadline_CycleContextShortCircuit(t *testing.T) {
	template := TemplateInput{
		TemplateCategory: "periodic",
		T0Config:         T0ConfigInput{FrequencyUnit: "monthly", CycleAnchorDay: 5, AllowT0Override: true},
		Rules:            &Rules{DeadlineDays: 5, DeadlineDayType: "calendar"},
	}
	cycleCtx := &PeriodicCycleContext{CycleStart: date(2026, 3, 1), CycleLabel: "2026-03"}
	company := CompanyInput{CycleAnchorDay: 20}

	res, err := ResolveDeadline(context.Background(), template, company, cycleCtx, RuntimeInput{}, engineRefTime, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.ResolvedT0.Equal(date(2026, 3, 1)) {
		t.Fatalf("expected short-circuited T0=2026-03-01, got %v", res.ResolvedT0)
	}
	if res.UsedCompanyT0Override {
		t.Fatalf("expected UsedCompanyT0Override=false on short-circuit")
	}
}
