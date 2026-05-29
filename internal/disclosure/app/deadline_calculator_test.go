package app

import (
	"context"
	"testing"
	"time"
)

type mockHolidayProvider struct {
	days map[string]string
}

func (m mockHolidayProvider) IsNonTradingDay(_ context.Context, date time.Time) (bool, string, error) {
	reason, ok := m.days[date.Format("2006-01-02")]
	return ok, reason, nil
}

func TestCalculateFixedDateWarnOnlyKeepsDate(t *testing.T) {
	calc := NewDeadlineCalculator(mockHolidayProvider{})
	now := time.Date(2026, 4, 10, 0, 0, 0, 0, time.FixedZone("ICT", 7*60*60))
	summary, err := calc.CalculateDeadlineSummary(context.Background(), &TemplateDeadlineConfig{
		DeadlineMode: DeadlineModeFixedDate,
		FixedDeadline: &FixedDeadlineConfig{
			Date:             "2026-04-11", // Saturday
			NonTradingPolicy: NonTradingPolicyWarnOnlyKeepDate,
		},
	}, CompanyDeadlineContext{}, now)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if summary == nil || summary.ActualDeadline == nil || *summary.ActualDeadline != "2026-04-11" {
		t.Fatalf("expected fixed date keep, got %#v", summary)
	}
	if summary.AdjustedBecauseNonTradingDay == nil || *summary.AdjustedBecauseNonTradingDay {
		t.Fatalf("expected warn-only without adjust, got %#v", summary.AdjustedBecauseNonTradingDay)
	}
}

func TestCalculateFixedDateMoveNextWorkingDay(t *testing.T) {
	calc := NewDeadlineCalculator(mockHolidayProvider{})
	now := time.Date(2026, 4, 10, 0, 0, 0, 0, time.FixedZone("ICT", 7*60*60))
	summary, err := calc.CalculateDeadlineSummary(context.Background(), &TemplateDeadlineConfig{
		DeadlineMode: DeadlineModeFixedDate,
		FixedDeadline: &FixedDeadlineConfig{
			Date:             "2026-04-11", // Saturday
			NonTradingPolicy: NonTradingPolicyMoveNextWorking,
		},
	}, CompanyDeadlineContext{}, now)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if summary == nil || summary.ActualDeadline == nil || *summary.ActualDeadline != "2026-04-13" {
		t.Fatalf("expected move to monday, got %#v", summary)
	}
	if summary.AdjustedBecauseNonTradingDay == nil || !*summary.AdjustedBecauseNonTradingDay {
		t.Fatalf("expected adjusted true, got %#v", summary.AdjustedBecauseNonTradingDay)
	}
}

func TestCalculateDynamicWorkingDaysFromWeekendStart(t *testing.T) {
	calc := NewDeadlineCalculator(mockHolidayProvider{
		days: map[string]string{
			"2026-01-08": "PUBLIC_HOLIDAY",
		},
	})
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.FixedZone("ICT", 7*60*60))
	summary, err := calc.CalculateDeadlineSummary(context.Background(), &TemplateDeadlineConfig{
		DeadlineMode: DeadlineModeDynamicRule,
		DynamicRule: &DynamicDeadlineRule{
			RuleType:              "DISCLOSURE_DEADLINE",
			BaseDateSource:        BaseDateSourceCompanyEstablished,
			Duration:              3,
			DurationType:          DurationTypeWorkingDays,
			InclusiveStart:        true,
			AdjustIfNonTradingDay: true,
			HolidayCalendarSource: HolidayCalendarSourceByYear,
		},
	}, CompanyDeadlineContext{
		CurrentYear:      2026,
		EstablishedMonth: 1,
		EstablishedDay:   3, // Saturday -> first working day is Monday Jan 5
	}, now)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if summary == nil || summary.ActualDeadline == nil || *summary.ActualDeadline != "2026-01-07" {
		t.Fatalf("expected actual deadline 2026-01-07, got %#v", summary)
	}
}

func TestCalculateDynamicCalendarDaysAdjustsWeekend(t *testing.T) {
	calc := NewDeadlineCalculator(mockHolidayProvider{})
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.FixedZone("ICT", 7*60*60))
	summary, err := calc.CalculateDeadlineSummary(context.Background(), &TemplateDeadlineConfig{
		DeadlineMode: DeadlineModeDynamicRule,
		DynamicRule: &DynamicDeadlineRule{
			RuleType:              "DISCLOSURE_DEADLINE",
			BaseDateSource:        BaseDateSourceCompanyEstablished,
			Duration:              2,
			DurationType:          DurationTypeCalendarDays,
			InclusiveStart:        true,
			AdjustIfNonTradingDay: true,
			HolidayCalendarSource: HolidayCalendarSourceByYear,
		},
	}, CompanyDeadlineContext{
		CurrentYear:      2026,
		EstablishedMonth: 1,
		EstablishedDay:   9, // Friday, tentative Jan 10 Saturday -> adjust to Jan 12
	}, now)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if summary == nil || summary.ActualDeadline == nil || *summary.ActualDeadline != "2026-01-12" {
		t.Fatalf("expected adjusted monday, got %#v", summary)
	}
}

// ─── PERIODIC mode tests ──────────────────────────────────────────────────────

func TestCalculatePeriodicQuarterly_StandardAnchor(t *testing.T) {
	// Quarterly standard (anchor 01/01): Q2 starts 01/04/2026
	// now = 2026-05-01 (in Q2), deadline_days=3 working days
	// 2026-04-01 = Wednesday → d1=04-01, d2=04-02, d3=04-03 = 2026-04-03
	calc := NewDeadlineCalculator(mockHolidayProvider{})
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.FixedZone("ICT", 7*60*60))
	summary, err := calc.CalculateDeadlineSummary(context.Background(), &TemplateDeadlineConfig{
		DeadlineMode:     DeadlineModePeriodic,
		FrequencyUnit:    "quarterly",
		CycleAnchorMonth: 1,
		CycleAnchorDay:   1,
		DeadlineDays:     3,
	}, CompanyDeadlineContext{}, now)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if summary == nil {
		t.Fatal("expected summary, got nil")
	}
	if summary.DeadlineMode != DeadlineModePeriodic {
		t.Fatalf("mode=%q want PERIODIC", summary.DeadlineMode)
	}
	if summary.StartDate == nil || *summary.StartDate != "2026-04-01" {
		t.Fatalf("cycleStart=%v want 2026-04-01", summary.StartDate)
	}
	if summary.DeadlineDate == nil || *summary.DeadlineDate != "2026-04-03" {
		t.Fatalf("deadlineDate=%v want 2026-04-03", summary.DeadlineDate)
	}
}

func TestCalculatePeriodicQuarterly_CustomAnchorApril(t *testing.T) {
	// Fiscal year starts April (anchor_month=4)
	// Fiscal Q1=Apr-Jun, Q2=Jul-Sep, Q3=Oct-Dec, Q4=Jan-Mar
	// now = 2026-08-01 (in Fiscal Q2) → cycleStart = 2026-07-01 (Wednesday)
	// deadline_days=3 → d1=07-01, d2=07-02, d3=07-03 = 2026-07-03
	calc := NewDeadlineCalculator(mockHolidayProvider{})
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.FixedZone("ICT", 7*60*60))
	summary, err := calc.CalculateDeadlineSummary(context.Background(), &TemplateDeadlineConfig{
		DeadlineMode:     DeadlineModePeriodic,
		FrequencyUnit:    "quarterly",
		CycleAnchorMonth: 4,
		CycleAnchorDay:   1,
		DeadlineDays:     3,
	}, CompanyDeadlineContext{}, now)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if summary.StartDate == nil || *summary.StartDate != "2026-07-01" {
		t.Fatalf("cycleStart=%v want 2026-07-01", summary.StartDate)
	}
	if summary.DeadlineDate == nil || *summary.DeadlineDate != "2026-07-03" {
		t.Fatalf("deadlineDate=%v want 2026-07-03", summary.DeadlineDate)
	}
}

func TestCalculatePeriodicMonthly_StandardAnchor(t *testing.T) {
	// Monthly: cycleStart = 1st of current month
	// now = 2026-05-15, cycleStart = 2026-05-01 (Friday)
	// deadline_days=3: d1=05-01(Fri), [05-02 Sat, 05-03 Sun], d2=05-04(Mon), d3=05-05(Tue) = 2026-05-05
	calc := NewDeadlineCalculator(mockHolidayProvider{})
	now := time.Date(2026, 5, 15, 0, 0, 0, 0, time.FixedZone("ICT", 7*60*60))
	summary, err := calc.CalculateDeadlineSummary(context.Background(), &TemplateDeadlineConfig{
		DeadlineMode:  DeadlineModePeriodic,
		FrequencyUnit: "monthly",
		DeadlineDays:  3,
	}, CompanyDeadlineContext{}, now)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if summary.StartDate == nil || *summary.StartDate != "2026-05-01" {
		t.Fatalf("cycleStart=%v want 2026-05-01", summary.StartDate)
	}
	if summary.DeadlineDate == nil || *summary.DeadlineDate != "2026-05-05" {
		t.Fatalf("deadlineDate=%v want 2026-05-05", summary.DeadlineDate)
	}
}

func TestCalculatePeriodicYearly_CustomAnchorJuly(t *testing.T) {
	// Yearly with anchor 01/07: fiscal year starts July
	// now = 2026-10-01, cycleStart = 2026-07-01 (Wednesday)
	// deadline_days=3 → d1=07-01, d2=07-02, d3=07-03 = 2026-07-03
	calc := NewDeadlineCalculator(mockHolidayProvider{})
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.FixedZone("ICT", 7*60*60))
	summary, err := calc.CalculateDeadlineSummary(context.Background(), &TemplateDeadlineConfig{
		DeadlineMode:     DeadlineModePeriodic,
		FrequencyUnit:    "yearly",
		CycleAnchorMonth: 7,
		CycleAnchorDay:   1,
		DeadlineDays:     3,
	}, CompanyDeadlineContext{}, now)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if summary.StartDate == nil || *summary.StartDate != "2026-07-01" {
		t.Fatalf("cycleStart=%v want 2026-07-01", summary.StartDate)
	}
	if summary.DeadlineDate == nil || *summary.DeadlineDate != "2026-07-03" {
		t.Fatalf("deadlineDate=%v want 2026-07-03", summary.DeadlineDate)
	}
}

func TestCalculatePeriodicYearly_FutureAnchorUsesLastYear(t *testing.T) {
	// Yearly with anchor 01/07, now = 2026-05-01 (before anchor)
	// Most recent past anchor = 2025-07-01 (Wednesday)
	// deadline_days=3 → 2025-07-03
	calc := NewDeadlineCalculator(mockHolidayProvider{})
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.FixedZone("ICT", 7*60*60))
	summary, err := calc.CalculateDeadlineSummary(context.Background(), &TemplateDeadlineConfig{
		DeadlineMode:     DeadlineModePeriodic,
		FrequencyUnit:    "yearly",
		CycleAnchorMonth: 7,
		CycleAnchorDay:   1,
		DeadlineDays:     3,
	}, CompanyDeadlineContext{}, now)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if summary.StartDate == nil || *summary.StartDate != "2025-07-01" {
		t.Fatalf("cycleStart=%v want 2025-07-01", summary.StartDate)
	}
	if summary.DeadlineDate == nil || *summary.DeadlineDate != "2025-07-03" {
		t.Fatalf("deadlineDate=%v want 2025-07-03", summary.DeadlineDate)
	}
}

func TestCalculatePeriodic_ZeroDeadlineDays_ReturnsNil(t *testing.T) {
	// deadline_days=0 → no deadline configured → nil summary (same as NONE mode)
	calc := NewDeadlineCalculator(mockHolidayProvider{})
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.FixedZone("ICT", 7*60*60))
	summary, err := calc.CalculateDeadlineSummary(context.Background(), &TemplateDeadlineConfig{
		DeadlineMode:  DeadlineModePeriodic,
		FrequencyUnit: "quarterly",
		DeadlineDays:  0,
	}, CompanyDeadlineContext{}, now)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if summary != nil {
		t.Fatalf("expected nil summary for deadline_days=0, got %+v", summary)
	}
}

func TestCalculatePeriodic_CompanyCycleAnchorOverride(t *testing.T) {
	// Template anchor: quarterly standard (anchor_month=1)
	// Company override: CycleAnchorMonth=4 (fiscal year starts April)
	// now = 2026-08-01 → company fiscal Q2 = Jul → cycleStart = 2026-07-01
	// deadline_days=3 → 2026-07-03
	calc := NewDeadlineCalculator(mockHolidayProvider{})
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.FixedZone("ICT", 7*60*60))
	summary, err := calc.CalculateDeadlineSummary(context.Background(), &TemplateDeadlineConfig{
		DeadlineMode:     DeadlineModePeriodic,
		FrequencyUnit:    "quarterly",
		CycleAnchorMonth: 1, // template default: Jan
		CycleAnchorDay:   1,
		DeadlineDays:     3,
	}, CompanyDeadlineContext{
		CycleAnchorMonth: 4, // company override: April
	}, now)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if summary.StartDate == nil || *summary.StartDate != "2026-07-01" {
		t.Fatalf("cycleStart=%v want 2026-07-01 (company override)", summary.StartDate)
	}
	if summary.DeadlineDate == nil || *summary.DeadlineDate != "2026-07-03" {
		t.Fatalf("deadlineDate=%v want 2026-07-03", summary.DeadlineDate)
	}
}

func TestCalculatePeriodicQuarterly_PreviousYearRollback(t *testing.T) {
	// anchor_month=11 (Nov), now = Jan 2026
	// Q1=Nov-Jan, so Jan 2026 is in Q1 that started Nov 2025
	// cycleStart = 2025-11-01 (Saturday) → actual cycle start as-is
	// deadline_days=3 → working: 11-01 is Sat, next working = Mon 11-03 (d1), 11-04(d2), 11-05(d3) = 2025-11-05
	calc := NewDeadlineCalculator(mockHolidayProvider{})
	now := time.Date(2026, 1, 15, 0, 0, 0, 0, time.FixedZone("ICT", 7*60*60))
	summary, err := calc.CalculateDeadlineSummary(context.Background(), &TemplateDeadlineConfig{
		DeadlineMode:     DeadlineModePeriodic,
		FrequencyUnit:    "quarterly",
		CycleAnchorMonth: 11,
		CycleAnchorDay:   1,
		DeadlineDays:     3,
	}, CompanyDeadlineContext{}, now)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if summary.StartDate == nil || *summary.StartDate != "2025-11-01" {
		t.Fatalf("cycleStart=%v want 2025-11-01", summary.StartDate)
	}
}

// ─── End PERIODIC mode tests ──────────────────────────────────────────────────

func TestCalculateDynamicFallbackEstablishedDateToFirstOfYear(t *testing.T) {
	calc := NewDeadlineCalculator(mockHolidayProvider{})
	now := time.Date(2026, 2, 1, 0, 0, 0, 0, time.FixedZone("ICT", 7*60*60))
	summary, err := calc.CalculateDeadlineSummary(context.Background(), &TemplateDeadlineConfig{
		DeadlineMode: DeadlineModeDynamicRule,
		DynamicRule: &DynamicDeadlineRule{
			RuleType:              "DISCLOSURE_DEADLINE",
			BaseDateSource:        BaseDateSourceCompanyEstablished,
			Duration:              1,
			DurationType:          DurationTypeCalendarDays,
			InclusiveStart:        true,
			AdjustIfNonTradingDay: true,
			HolidayCalendarSource: HolidayCalendarSourceByYear,
		},
	}, CompanyDeadlineContext{
		CurrentYear: 2026,
	}, now)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if summary == nil || summary.StartDate == nil || *summary.StartDate != "2026-01-01" {
		t.Fatalf("expected fallback start date 2026-01-01, got %#v", summary)
	}
}

