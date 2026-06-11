package deadlineengine

import (
	"context"
	"errors"
	"testing"
	"time"
)

var refTime2026 = time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC) // Wednesday, Q2 (Apr-Jun)

// --- DeriveDeadlineBehavior ---------------------------------------------

func TestDeriveDeadlineBehavior(t *testing.T) {
	cases := []struct {
		name     string
		category string
		freq     string
		t0Policy string
		want     DeadlineBehavior
		wantErr  error
	}{
		{"global periodic monthly", "periodic", "monthly", "", DeadlineBehaviorPeriodic, nil},
		{"global periodic quarterly", "periodic", "quarterly", "", DeadlineBehaviorPeriodic, nil},
		{"global periodic yearly", "periodic", "yearly", "", DeadlineBehaviorPeriodic, nil},
		{"global irregular system_date", "irregular", "", "system_date", DeadlineBehaviorIrregular, nil},

		// PD-DE-CUSTOM-01 cases
		{"custom + monthly -> periodic", "custom", "monthly", "", DeadlineBehaviorPeriodic, nil},
		{"custom + quarterly -> periodic", "custom", "quarterly", "", DeadlineBehaviorPeriodic, nil},
		{"custom + yearly -> periodic", "custom", "yearly", "", DeadlineBehaviorPeriodic, nil},
		{"custom + user_defined -> irregular", "custom", "", "user_defined", DeadlineBehaviorIrregular, nil},
		{"custom + no frequency, no t0_policy -> error", "custom", "", "", "", ErrInvalidT0Configuration},

		{"unsupported frequency_unit", "periodic", "weekly", "", "", ErrUnsupportedFrequency},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := TemplateInput{
				TemplateCategory: tc.category,
				T0Config: T0ConfigInput{
					FrequencyUnit: tc.freq,
					T0Policy:      tc.t0Policy,
				},
			}
			got, err := DeriveDeadlineBehavior(input)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected behavior %q, got %q", tc.want, got)
			}
		})
	}
}

// --- ResolveT0 — periodic (R-P) ------------------------------------------

func TestResolveT0_Periodic_Monthly(t *testing.T) {
	template := TemplateInput{
		TemplateCategory: "periodic",
		T0Config: T0ConfigInput{
			FrequencyUnit:    "monthly",
			CycleAnchorMonth: 1,
			CycleAnchorDay:   10,
		},
	}
	got, err := ResolveT0(context.Background(), template, CompanyInput{}, nil, RuntimeInput{}, refTime2026)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestResolveT0_Periodic_Quarterly(t *testing.T) {
	// anchor_month=1 -> fiscal quarters start Jan/Apr/Jul/Oct. refTime is in
	// April -> current quarter starts 01/04/2026.
	template := TemplateInput{
		TemplateCategory: "periodic",
		T0Config: T0ConfigInput{
			FrequencyUnit:    "quarterly",
			CycleAnchorMonth: 1,
			CycleAnchorDay:   1,
		},
	}
	got, err := ResolveT0(context.Background(), template, CompanyInput{}, nil, RuntimeInput{}, refTime2026)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestResolveT0_Periodic_Yearly_PastOccurrence(t *testing.T) {
	// anchor_month/day = 01/01 -> already passed this year -> 01/01/2026.
	template := TemplateInput{
		TemplateCategory: "periodic",
		T0Config: T0ConfigInput{
			FrequencyUnit:    "yearly",
			CycleAnchorMonth: 1,
			CycleAnchorDay:   1,
		},
	}
	got, err := ResolveT0(context.Background(), template, CompanyInput{}, nil, RuntimeInput{}, refTime2026)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestResolveT0_Periodic_Yearly_FutureRollsToPreviousYear(t *testing.T) {
	// anchor_month/day = 12/12 -> not yet occurred this year (refTime=15/04)
	// -> most recent past occurrence = 12/12/2025.
	template := TemplateInput{
		TemplateCategory: "periodic",
		T0Config: T0ConfigInput{
			FrequencyUnit:    "yearly",
			CycleAnchorMonth: 12,
			CycleAnchorDay:   12,
		},
	}
	got, err := ResolveT0(context.Background(), template, CompanyInput{}, nil, RuntimeInput{}, refTime2026)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2025, 12, 12, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

// --- ResolveT0 — PD-DE-CUSTOM-01 cases ------------------------------------

func TestResolveT0_CustomPeriodic_Quarterly(t *testing.T) {
	// Case 3: custom + quarterly -> periodic behavior, R-P.
	template := TemplateInput{
		TemplateCategory: "custom",
		T0Config: T0ConfigInput{
			FrequencyUnit:    "quarterly",
			CycleAnchorMonth: 1,
			CycleAnchorDay:   1,
		},
	}
	t0, _, err := resolveT0(context.Background(), template, CompanyInput{}, nil, RuntimeInput{}, refTime2026)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	if !t0.Equal(want) {
		t.Fatalf("expected %v, got %v", want, t0)
	}
}

func TestResolveT0_CustomIrregular_UserDefined(t *testing.T) {
	// Case 4: custom + t0_policy=user_defined -> irregular behavior, R-I.
	template := TemplateInput{
		TemplateCategory: "custom",
		T0Config: T0ConfigInput{
			T0Policy: "user_defined",
		},
	}
	got, err := ResolveT0(context.Background(), template, CompanyInput{}, nil, RuntimeInput{T0Date: "2026-05-20"}, refTime2026)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestResolveT0_CustomIrregular_SystemDate(t *testing.T) {
	// custom + t0_policy=system_date -> irregular, R-I.
	template := TemplateInput{
		TemplateCategory: "custom",
		T0Config: T0ConfigInput{
			T0Policy: "system_date",
		},
	}
	got, err := ResolveT0(context.Background(), template, CompanyInput{}, nil, RuntimeInput{}, refTime2026)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Equal(refTime2026) {
		t.Fatalf("expected %v, got %v", refTime2026, got)
	}
}

func TestResolveT0_CustomEventDate_NotImplemented(t *testing.T) {
	template := TemplateInput{
		TemplateCategory: "custom",
		T0Config: T0ConfigInput{
			T0Policy: "event_date",
		},
	}
	_, err := ResolveT0(context.Background(), template, CompanyInput{}, nil, RuntimeInput{}, refTime2026)
	if !errors.Is(err, ErrEventDateNotImplemented) {
		t.Fatalf("expected ErrEventDateNotImplemented, got %v", err)
	}
}

func TestResolveT0_CustomNoFrequencyNoPolicy_Error(t *testing.T) {
	// Case 5: custom, no frequency_unit, no t0_policy -> ErrInvalidT0Configuration.
	template := TemplateInput{
		TemplateCategory: "custom",
		T0Config:         T0ConfigInput{},
	}
	_, err := ResolveT0(context.Background(), template, CompanyInput{}, nil, RuntimeInput{}, refTime2026)
	if !errors.Is(err, ErrInvalidT0Configuration) {
		t.Fatalf("expected ErrInvalidT0Configuration, got %v", err)
	}
}

// --- Company override (R-C) -----------------------------------------------

func TestResolveT0_CompanyOverride_Applied(t *testing.T) {
	// custom + monthly + allow_t0_override=true + company month override -> applied.
	template := TemplateInput{
		TemplateCategory: "custom",
		T0Config: T0ConfigInput{
			FrequencyUnit:    "monthly",
			CycleAnchorMonth: 1,
			CycleAnchorDay:   5,
			AllowT0Override:  true,
		},
	}
	company := CompanyInput{CycleAnchorDay: 20}
	t0, used, err := resolveT0(context.Background(), template, company, nil, RuntimeInput{}, refTime2026)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !used {
		t.Fatalf("expected usedOverride=true")
	}
	want := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
	if !t0.Equal(want) {
		t.Fatalf("expected %v, got %v", want, t0)
	}
}

func TestResolveT0_CompanyOverride_IgnoredWhenNotAllowed(t *testing.T) {
	template := TemplateInput{
		TemplateCategory: "custom",
		T0Config: T0ConfigInput{
			FrequencyUnit:    "monthly",
			CycleAnchorMonth: 1,
			CycleAnchorDay:   5,
			AllowT0Override:  false,
		},
	}
	company := CompanyInput{CycleAnchorDay: 20}
	t0, used, err := resolveT0(context.Background(), template, company, nil, RuntimeInput{}, refTime2026)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if used {
		t.Fatalf("expected usedOverride=false")
	}
	want := time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC)
	if !t0.Equal(want) {
		t.Fatalf("expected %v, got %v", want, t0)
	}
}

func TestResolveT0_CompanyOverride_IgnoredWhenAllowFlagMissing(t *testing.T) {
	// AllowT0Override unset (false zero-value, mapped from nil per
	// PD: nil/missing = false) -> override ignored even though company set it.
	template := TemplateInput{
		TemplateCategory: "custom",
		T0Config: T0ConfigInput{
			FrequencyUnit:    "monthly",
			CycleAnchorMonth: 1,
			CycleAnchorDay:   5,
			// AllowT0Override left as zero value (false)
		},
	}
	company := CompanyInput{CycleAnchorDay: 20}
	t0, used, err := resolveT0(context.Background(), template, company, nil, RuntimeInput{}, refTime2026)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if used {
		t.Fatalf("expected usedOverride=false")
	}
	want := time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC)
	if !t0.Equal(want) {
		t.Fatalf("expected %v, got %v", want, t0)
	}
}

func TestResolveT0_CycleCtxShortCircuit(t *testing.T) {
	template := TemplateInput{
		TemplateCategory: "periodic",
		T0Config: T0ConfigInput{
			FrequencyUnit:    "monthly",
			CycleAnchorMonth: 1,
			CycleAnchorDay:   5,
			AllowT0Override:  true,
		},
	}
	cycleCtx := &PeriodicCycleContext{CycleStart: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)}
	company := CompanyInput{CycleAnchorDay: 20}
	t0, used, err := resolveT0(context.Background(), template, company, cycleCtx, RuntimeInput{}, refTime2026)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if used {
		t.Fatalf("expected usedOverride=false on short-circuit")
	}
	if !t0.Equal(cycleCtx.CycleStart) {
		t.Fatalf("expected %v, got %v", cycleCtx.CycleStart, t0)
	}
}

// --- Irregular errors -------------------------------------------------------

func TestResolveT0_Irregular_UserDefinedMissingDate(t *testing.T) {
	template := TemplateInput{
		TemplateCategory: "irregular",
		T0Config:         T0ConfigInput{T0Policy: "user_defined"},
	}
	_, err := ResolveT0(context.Background(), template, CompanyInput{}, nil, RuntimeInput{}, refTime2026)
	if !errors.Is(err, ErrT0DateRequired) {
		t.Fatalf("expected ErrT0DateRequired, got %v", err)
	}
}

func TestResolveT0_Irregular_InvalidDateFormat(t *testing.T) {
	template := TemplateInput{
		TemplateCategory: "irregular",
		T0Config:         T0ConfigInput{T0Policy: "user_defined"},
	}
	_, err := ResolveT0(context.Background(), template, CompanyInput{}, nil, RuntimeInput{T0Date: "20-05-2026"}, refTime2026)
	if !errors.Is(err, ErrInvalidT0Date) {
		t.Fatalf("expected ErrInvalidT0Date, got %v", err)
	}
}

// --- Invalid anchor validation ----------------------------------------------

func TestNormalizeAnchor_Yearly(t *testing.T) {
	// frequency_unit=yearly: anchor_day validated against anchor_month.
	refTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC) // 2024 = leap year

	cases := []struct {
		name       string
		month, day int
		wantErr    bool
		wantMonth  int
		wantDay    int
	}{
		{"month=2 day=31 invalid", 2, 31, true, 0, 0},
		{"month=4 day=31 invalid", 4, 31, true, 0, 0},
		{"month=13 day=1 out of range", 13, 1, true, 0, 0},
		{"month=0 day=1 defaults to 1/1", 0, 1, false, 1, 1},
		{"month=0 day=0 defaults to 1/1", 0, 0, false, 1, 1},
		{"valid month=4 day=30", 4, 30, false, 4, 30},
		{"month=2 day=29 valid in leap year", 2, 29, false, 2, 29},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotMonth, gotDay, err := normalizeAnchor(tc.month, tc.day, "yearly", refTime)
			if tc.wantErr {
				if !errors.Is(err, ErrInvalidT0Configuration) {
					t.Fatalf("expected ErrInvalidT0Configuration, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotMonth != tc.wantMonth || gotDay != tc.wantDay {
				t.Fatalf("expected (%d,%d), got (%d,%d)", tc.wantMonth, tc.wantDay, gotMonth, gotDay)
			}
		})
	}
}

func TestNormalizeAnchor_Quarterly_NoDayValidation(t *testing.T) {
	// frequency_unit=quarterly: anchor_day is never used by computeCycleStart,
	// so an "invalid" day (e.g. 31) must not error.
	refTime := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	gotMonth, gotDay, err := normalizeAnchor(2, 31, "quarterly", refTime)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMonth != 2 || gotDay != 31 {
		t.Fatalf("expected (2,31), got (%d,%d)", gotMonth, gotDay)
	}
}

func TestResolveT0_InvalidCompanyAnchor_Day31InFebruary(t *testing.T) {
	template := TemplateInput{
		TemplateCategory: "custom",
		T0Config: T0ConfigInput{
			FrequencyUnit:    "monthly",
			CycleAnchorMonth: 1,
			CycleAnchorDay:   5,
			AllowT0Override:  true,
		},
	}
	// February ref time + company override day=31 -> invalid for February.
	febRef := time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC)
	company := CompanyInput{CycleAnchorDay: 31}
	_, _, err := resolveT0(context.Background(), template, company, nil, RuntimeInput{}, febRef)
	if !errors.Is(err, ErrInvalidT0Configuration) {
		t.Fatalf("expected ErrInvalidT0Configuration, got %v", err)
	}
}
