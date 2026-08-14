package app

import (
	"context"
	"testing"
	"time"
)

func TestNormalizeFrequencyUnit(t *testing.T) {
	cases := map[string]string{
		"daily": "daily", "DAY": "daily", "week": "weekly", "weekly": "weekly",
		"month": "monthly", "quarter": "quarterly", "year": "yearly", "annual": "yearly",
		"biweekly": "biweekly", "": "",
	}
	for in, want := range cases {
		if got := NormalizeFrequencyUnit(in); got != want {
			t.Fatalf("NormalizeFrequencyUnit(%q)=%q want %q", in, got, want)
		}
	}
}

func TestComputeCycleLabelAndStart_DailyWeeklyIdentity(t *testing.T) {
	ict := asiaHoChiMinh()
	// Friday 2026-08-14 17:30 UTC = Saturday 2026-08-15 00:30 ICT
	utcBoundary := time.Date(2026, 8, 14, 17, 30, 0, 0, time.UTC)

	dailyLabel, dailyStart := computeCycleLabelAndStart(PeriodicTypeRow{FrequencyUnit: "day"}, utcBoundary)
	if dailyLabel != "2026-08-15" {
		t.Fatalf("daily label=%s want 2026-08-15", dailyLabel)
	}
	if dailyStart.Format("2006-01-02") != "2026-08-15" || dailyStart.Location().String() != ict.String() {
		t.Fatalf("daily start=%v", dailyStart)
	}

	// Saturday 2026-08-15 ICT → week start Sunday 2026-08-09
	weeklyLabel, weeklyStart := computeCycleLabelAndStart(PeriodicTypeRow{FrequencyUnit: "week"}, utcBoundary)
	if weeklyLabel != "2026-08-09" {
		t.Fatalf("weekly label=%s want 2026-08-09", weeklyLabel)
	}
	if weeklyStart.Weekday() != time.Sunday || weeklyStart.Format("2006-01-02") != "2026-08-09" {
		t.Fatalf("weekly start=%v weekday=%s", weeklyStart, weeklyStart.Weekday())
	}

	// Same week, Sunday vs Saturday share identity
	sundayICT := time.Date(2026, 8, 9, 8, 0, 0, 0, ict)
	satICT := time.Date(2026, 8, 15, 22, 0, 0, 0, ict)
	l1, _ := computeCycleLabelAndStart(PeriodicTypeRow{FrequencyUnit: "weekly"}, sundayICT)
	l2, _ := computeCycleLabelAndStart(PeriodicTypeRow{FrequencyUnit: "weekly"}, satICT)
	if l1 != l2 || l1 != "2026-08-09" {
		t.Fatalf("week identity sunday=%s saturday=%s", l1, l2)
	}

	// Year boundary: Thu 2026-01-01 ICT → Sunday 2025-12-28
	newYear := time.Date(2026, 1, 1, 9, 0, 0, 0, ict)
	yl, ys := computeCycleLabelAndStart(PeriodicTypeRow{FrequencyUnit: "weekly"}, newYear)
	if yl != "2025-12-28" {
		t.Fatalf("year-boundary weekly label=%s want 2025-12-28", yl)
	}
	if ys.Year() != 2025 || ys.Month() != time.December || ys.Day() != 28 {
		t.Fatalf("year-boundary weekly start=%v", ys)
	}

	// monthly unchanged (UTC location of input)
	ml, _ := computeCycleLabelAndStart(PeriodicTypeRow{FrequencyUnit: "monthly"}, utcBoundary)
	if ml != "2026-08" {
		t.Fatalf("monthly label=%s", ml)
	}
	ql, _ := computeCycleLabelAndStart(PeriodicTypeRow{FrequencyUnit: "quarterly"}, utcBoundary)
	if ql != "2026-Q3" {
		t.Fatalf("quarterly label=%s", ql)
	}
}

func TestComputeCycleLabelAndStart_NoDuplicateSameDailyPeriod(t *testing.T) {
	ict := asiaHoChiMinh()
	a := time.Date(2026, 8, 14, 1, 0, 0, 0, ict)
	b := time.Date(2026, 8, 14, 23, 0, 0, 0, ict)
	la, _ := computeCycleLabelAndStart(PeriodicTypeRow{FrequencyUnit: "daily"}, a)
	lb, _ := computeCycleLabelAndStart(PeriodicTypeRow{FrequencyUnit: "daily"}, b)
	if la != lb || la != "2026-08-14" {
		t.Fatalf("same-day daily labels %s vs %s", la, lb)
	}
}

func TestDeadlineCalculator_DailyWeeklyT0(t *testing.T) {
	calc := NewDeadlineCalculator(mockHolidayProvider{})
	ctx := context.Background()
	company := CompanyDeadlineContext{}
	utcBoundary := time.Date(2026, 8, 14, 17, 30, 0, 0, time.UTC)

	daily := calc.computeCycleStart(&TemplateDeadlineConfig{FrequencyUnit: "daily"}, company, utcBoundary)
	if daily.Format("2006-01-02") != "2026-08-15" {
		t.Fatalf("daily T0=%s", daily.Format("2006-01-02"))
	}

	weekly := calc.computeCycleStart(&TemplateDeadlineConfig{FrequencyUnit: "weekly"}, company, utcBoundary)
	if weekly.Format("2006-01-02") != "2026-08-09" {
		t.Fatalf("weekly T0=%s", weekly.Format("2006-01-02"))
	}

	summary, err := calc.CalculateDeadlineSummary(ctx, &TemplateDeadlineConfig{
		DeadlineMode:         DeadlineModePeriodic,
		FrequencyUnit:        "daily",
		DeadlineDays:         2,
		DeadlineDurationType: DurationTypeCalendarDays,
	}, company, utcBoundary)
	if err != nil {
		t.Fatalf("daily deadline: %v", err)
	}
	if summary == nil || summary.StartDate == nil || *summary.StartDate != "2026-08-15" {
		t.Fatalf("daily summary start %#v", summary)
	}
	if summary.DeadlineDate == nil || *summary.DeadlineDate != "2026-08-16" {
		t.Fatalf("daily due %#v", summary.DeadlineDate)
	}

	wsummary, err := calc.CalculateDeadlineSummary(ctx, &TemplateDeadlineConfig{
		DeadlineMode:         DeadlineModePeriodic,
		FrequencyUnit:        "week",
		DeadlineDays:         3,
		DeadlineDurationType: DurationTypeCalendarDays,
	}, company, utcBoundary)
	if err != nil {
		t.Fatalf("weekly deadline: %v", err)
	}
	if wsummary == nil || wsummary.StartDate == nil || *wsummary.StartDate != "2026-08-09" {
		t.Fatalf("weekly summary start %#v", wsummary)
	}
	if wsummary.DeadlineDate == nil || *wsummary.DeadlineDate != "2026-08-11" {
		t.Fatalf("weekly due %#v", wsummary.DeadlineDate)
	}
}
