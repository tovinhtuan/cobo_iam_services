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

