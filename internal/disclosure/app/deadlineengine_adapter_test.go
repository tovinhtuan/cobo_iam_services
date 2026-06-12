package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
	"github.com/cobo/cobo_iam_services/internal/disclosure/app/deadlineengine"
)

func boolPtr(v bool) *bool { return &v }

func TestDeadlineEngineAdapter_ResolveDeadline_PeriodicMonthly(t *testing.T) {
	adapter := NewDeadlineEngineAdapter(mockHolidayProvider{})
	refTime := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)

	res, err := adapter.ResolveDeadline(context.Background(), DeadlineEngineAdapterInput{
		TemplateCategory: "periodic",
		Config: &TemplateDeadlineConfig{
			DeadlineMode:    DeadlineModePeriodic,
			FrequencyUnit:   "monthly",
			CycleAnchorDay:  5,
			AllowT0Override: boolPtr(false),
		},
		Rules:   &applicability.TemplateApplicabilityRules{DeadlineDays: 20, DeadlineDayType: "calendar"},
		RefTime: refTime,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantT0 := time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC)
	if !res.ResolvedT0.Equal(wantT0) {
		t.Fatalf("expected T0=%v, got %v", wantT0, res.ResolvedT0)
	}
	wantPlanned := wantT0.AddDate(0, 0, 19) // inclusive counting, calendar
	if !res.PlannedDate.Equal(wantPlanned) {
		t.Fatalf("expected planned=%v, got %v", wantPlanned, res.PlannedDate)
	}
	if res.UsedCompanyT0Override {
		t.Fatalf("expected no company override")
	}
}

func TestDeadlineEngineAdapter_ResolveDeadline_CompanyT0Override(t *testing.T) {
	adapter := NewDeadlineEngineAdapter(nil)
	refTime := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)

	res, err := adapter.ResolveDeadline(context.Background(), DeadlineEngineAdapterInput{
		TemplateCategory: "periodic",
		Config: &TemplateDeadlineConfig{
			DeadlineMode:    DeadlineModePeriodic,
			FrequencyUnit:   "monthly",
			CycleAnchorDay:  1,
			AllowT0Override: boolPtr(true),
		},
		Rules: &applicability.TemplateApplicabilityRules{DeadlineDays: 10, DeadlineDayType: "calendar"},
		Company: CompanyDeadlineContext{
			CycleAnchorDay: 10,
		},
		RefTime: refTime,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantT0 := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	if !res.ResolvedT0.Equal(wantT0) {
		t.Fatalf("expected T0=%v (company override applied), got %v", wantT0, res.ResolvedT0)
	}
	if !res.UsedCompanyT0Override {
		t.Fatalf("expected company override flag to be set")
	}
}

func TestDeadlineEngineAdapter_ResolveDeadline_CycleCtxShortCircuit(t *testing.T) {
	adapter := NewDeadlineEngineAdapter(nil)
	refTime := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)
	existingCycleStart := time.Date(2025, 4, 4, 0, 0, 0, 0, time.UTC)

	res, err := adapter.ResolveDeadline(context.Background(), DeadlineEngineAdapterInput{
		TemplateCategory: "periodic",
		Config: &TemplateDeadlineConfig{
			DeadlineMode:     DeadlineModePeriodic,
			FrequencyUnit:    "yearly",
			CycleAnchorMonth: 4,
			CycleAnchorDay:   4,
			AllowT0Override:  boolPtr(false),
		},
		Rules:    &applicability.TemplateApplicabilityRules{DeadlineDays: 90, DeadlineDayType: "calendar"},
		CycleCtx: &deadlineengine.PeriodicCycleContext{CycleStart: existingCycleStart},
		RefTime:  refTime,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// I-21 short-circuit: ResolvedT0 must be the existing cycle's start, not a
	// fresh computeCycleStart result.
	if !res.ResolvedT0.Equal(existingCycleStart) {
		t.Fatalf("expected T0=%v (cycleCtx short-circuit), got %v", existingCycleStart, res.ResolvedT0)
	}
}

func TestDeadlineEngineAdapter_ResolveDeadline_Irregular(t *testing.T) {
	adapter := NewDeadlineEngineAdapter(nil)
	refTime := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)

	res, err := adapter.ResolveDeadline(context.Background(), DeadlineEngineAdapterInput{
		TemplateCategory: "irregular",
		Config: &TemplateDeadlineConfig{
			DeadlineMode: DeadlineModeNone,
			T0Policy:     "system_date",
		},
		Rules:   &applicability.TemplateApplicabilityRules{DeadlineDays: 5, DeadlineDayType: "calendar"},
		RefTime: refTime,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.ResolvedT0.Equal(refTime) {
		t.Fatalf("expected T0=refTime=%v, got %v", refTime, res.ResolvedT0)
	}
}

func TestDeadlineEngineAdapter_ResolveDeadline_UserDefinedT0(t *testing.T) {
	adapter := NewDeadlineEngineAdapter(nil)
	refTime := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)

	res, err := adapter.ResolveDeadline(context.Background(), DeadlineEngineAdapterInput{
		TemplateCategory: "irregular",
		Config: &TemplateDeadlineConfig{
			DeadlineMode: DeadlineModeNone,
			T0Policy:     "user_defined",
		},
		Rules:   &applicability.TemplateApplicabilityRules{DeadlineDays: 5, DeadlineDayType: "calendar"},
		Runtime: deadlineengine.RuntimeInput{T0Date: "2026-05-01"},
		RefTime: refTime,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	if !res.ResolvedT0.Equal(want) {
		t.Fatalf("expected T0=%v, got %v", want, res.ResolvedT0)
	}
}

func TestDeadlineEngineAdapter_ResolveDeadline_WorkingDaysWithHolidays(t *testing.T) {
	adapter := NewDeadlineEngineAdapter(mockHolidayProvider{
		days: map[string]string{
			"2026-06-15": "HOLIDAY",
		},
	})
	refTime := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC) // Friday

	res, err := adapter.ResolveDeadline(context.Background(), DeadlineEngineAdapterInput{
		TemplateCategory: "irregular",
		Config: &TemplateDeadlineConfig{
			DeadlineMode: DeadlineModeNone,
			T0Policy:     "system_date",
		},
		Rules:   &applicability.TemplateApplicabilityRules{DeadlineDays: 3, DeadlineDayType: "working"},
		RefTime: refTime,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// T0=Fri 06-12 (day1). Working days: 06-12(1), skip Sat/Sun, 06-15 is a
	// configured holiday(skip), 06-16(2), 06-17(3) -> planned = 06-17.
	want := time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC)
	if !res.PlannedDate.Equal(want) {
		t.Fatalf("expected planned=%v, got %v", want, res.PlannedDate)
	}
}

func TestDeadlineEngineAdapter_ResolveDeadline_NilConfig(t *testing.T) {
	adapter := NewDeadlineEngineAdapter(nil)
	_, err := adapter.ResolveDeadline(context.Background(), DeadlineEngineAdapterInput{})
	if !errors.Is(err, ErrMissingDeadlineConfig) {
		t.Fatalf("expected ErrMissingDeadlineConfig, got %v", err)
	}
}

func TestDeadlineEngineAdapter_ResolveDeadline_RulesRequired(t *testing.T) {
	adapter := NewDeadlineEngineAdapter(nil)
	_, err := adapter.ResolveDeadline(context.Background(), DeadlineEngineAdapterInput{
		Config: &TemplateDeadlineConfig{
			DeadlineMode: DeadlineModeNone,
			T0Policy:     "system_date",
		},
	})
	if !errors.Is(err, deadlineengine.ErrRulesRequired) {
		t.Fatalf("expected ErrRulesRequired (error propagation from deadlineengine), got %v", err)
	}
}

func TestDeadlineEngineAdapter_ResolveDeadline_InvalidT0Policy(t *testing.T) {
	adapter := NewDeadlineEngineAdapter(nil)
	_, err := adapter.ResolveDeadline(context.Background(), DeadlineEngineAdapterInput{
		Config: &TemplateDeadlineConfig{
			DeadlineMode: DeadlineModeNone,
			T0Policy:     "not_a_real_policy",
		},
		Rules: &applicability.TemplateApplicabilityRules{DeadlineDays: 5, DeadlineDayType: "calendar"},
	})
	if !errors.Is(err, deadlineengine.ErrInvalidT0Configuration) {
		t.Fatalf("expected ErrInvalidT0Configuration (error propagation from deadlineengine), got %v", err)
	}
}

func TestDeadlineEngineAdapter_ResolveDeadline_DefaultsRefTimeToNow(t *testing.T) {
	adapter := NewDeadlineEngineAdapter(nil)
	res, err := adapter.ResolveDeadline(context.Background(), DeadlineEngineAdapterInput{
		Config: &TemplateDeadlineConfig{
			DeadlineMode: DeadlineModeNone,
			T0Policy:     "system_date",
		},
		Rules: &applicability.TemplateApplicabilityRules{DeadlineDays: 1, DeadlineDayType: "calendar"},
		// RefTime intentionally zero.
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ResolvedT0.IsZero() {
		t.Fatalf("expected non-zero T0 when RefTime defaults to time.Now()")
	}
}

// TestDeadlineEngineAdapter_ResolveDeadline_LegacyDeadlineByStructureFallback
// reproduces the Batch 5C.1 DEV input contract gap: applicability_rules_json
// rows authored before the V2 use_structure_deadline flag existed populate
// deadline_by_structure (required by CMS validation E01-E06 for periodic
// templates) but leave deadline_days=0 and use_structure_deadline=false.
// Without normalizeRulesForEngine, ResolveEffectiveN would return
// ErrInvalidDeadlineDays even though deadline_by_structure has a usable entry
// for the resolved structure criterion (simple_structure, for a zero-value
// CompanyApplicabilityProfile).
func TestDeadlineEngineAdapter_ResolveDeadline_LegacyDeadlineByStructureFallback(t *testing.T) {
	adapter := NewDeadlineEngineAdapter(nil)
	refTime := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)

	res, err := adapter.ResolveDeadline(context.Background(), DeadlineEngineAdapterInput{
		TemplateCategory: "periodic",
		Config: &TemplateDeadlineConfig{
			DeadlineMode:    DeadlineModePeriodic,
			FrequencyUnit:   "quarterly",
			AllowT0Override: boolPtr(true),
		},
		Rules: &applicability.TemplateApplicabilityRules{
			DeadlineDayType: "calendar",
			DeadlineByStructure: map[applicability.StructureCriterion]applicability.StructureDeadlineEntry{
				applicability.StructureSimpleStructure:     {Days: 20},
				applicability.StructureHasSubsidiaries:     {Days: 30},
				applicability.StructureHasSubordinateUnits: {Days: 30},
			},
		},
		RefTime: refTime,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantPlanned := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, 19) // cycle_start 2026-04-01 + 19 (inclusive 20 calendar days)
	if !res.PlannedDate.Equal(wantPlanned) {
		t.Fatalf("expected planned=%v (simple_structure N=20), got %v", wantPlanned, res.PlannedDate)
	}
}

func TestNonTradingDayCheckerAdapter_IsHoliday(t *testing.T) {
	adapter := &nonTradingDayCheckerAdapter{provider: mockHolidayProvider{
		days: map[string]string{"2026-06-15": "TET"},
	}}
	holiday, err := adapter.IsHoliday(context.Background(), time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !holiday {
		t.Fatalf("expected 2026-06-15 to be flagged as a holiday")
	}
	holiday, err = adapter.IsHoliday(context.Background(), time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if holiday {
		t.Fatalf("expected 2026-06-16 to not be flagged as a holiday")
	}
}
