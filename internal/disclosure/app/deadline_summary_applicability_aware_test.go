package app

import (
	"context"
	"testing"
	"time"
)

func hcmDate(y int, m time.Month, d int) time.Time {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		loc = time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)
	}
	return time.Date(y, m, d, 12, 0, 0, 0, loc)
}

func activeTrue() *bool {
	v := true
	return &v
}

// D1 — candidate before AF specific → clamp forward to AF.
func TestSourceA_Preview_D1_CandidateBeforeSpecificAF(t *testing.T) {
	calc := NewDeadlineCalculator(nil)
	now := hcmDate(2026, 9, 9)
	sum, err := calc.CalculateDeadlineSummary(context.Background(), &TemplateDeadlineConfig{
		DeadlineMode:          DeadlineModePeriodic,
		FrequencyUnit:         "monthly",
		CycleAnchorDay:        1,
		DeadlineDays:          5,
		DeadlineDurationType:  DurationTypeWorkingDays,
		ApplicableFromMode:    ApplicableFromModeSpecific,
		ApplicableFromSlot:    "2026-10",
	}, CompanyDeadlineContext{}, now)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if sum == nil || sum.StartDate == nil || *sum.StartDate != "2026-10-01" {
		t.Fatalf("T want 2026-10-01 got %#v", sum)
	}
	if sum.DeadlineDate == nil || *sum.DeadlineDate != "2026-10-07" {
		t.Fatalf("DueAt want 2026-10-07 got %#v", sum.DeadlineDate)
	}
	if sum.Status == "OVERDUE" {
		t.Fatalf("must not be OVERDUE for Oct preview from Sep clock, status=%s", sum.Status)
	}
}

// D2 — candidate equal AF → unchanged.
func TestSourceA_Preview_D2_CandidateEqualAF(t *testing.T) {
	calc := NewDeadlineCalculator(nil)
	now := hcmDate(2026, 10, 5)
	sum, err := calc.CalculateDeadlineSummary(context.Background(), &TemplateDeadlineConfig{
		DeadlineMode:         DeadlineModePeriodic,
		FrequencyUnit:        "monthly",
		CycleAnchorDay:       1,
		DeadlineDays:         5,
		DeadlineDurationType: DurationTypeWorkingDays,
		ApplicableFromMode:   ApplicableFromModeSpecific,
		ApplicableFromSlot:   "2026-10",
	}, CompanyDeadlineContext{}, now)
	if err != nil || sum == nil || sum.StartDate == nil || *sum.StartDate != "2026-10-01" {
		t.Fatalf("want Oct T, got %#v err=%v", sum, err)
	}
}

// D3 — candidate after AF → unchanged (no unnecessary forward).
func TestSourceA_Preview_D3_CandidateAfterAF(t *testing.T) {
	calc := NewDeadlineCalculator(nil)
	now := hcmDate(2026, 11, 5)
	sum, err := calc.CalculateDeadlineSummary(context.Background(), &TemplateDeadlineConfig{
		DeadlineMode:         DeadlineModePeriodic,
		FrequencyUnit:        "monthly",
		CycleAnchorDay:       1,
		DeadlineDays:         5,
		DeadlineDurationType: DurationTypeWorkingDays,
		ApplicableFromMode:   ApplicableFromModeSpecific,
		ApplicableFromSlot:   "2026-08",
	}, CompanyDeadlineContext{}, now)
	if err != nil || sum == nil || sum.StartDate == nil || *sum.StartDate != "2026-11-01" {
		t.Fatalf("want Nov T (candidate), got %#v err=%v", sum, err)
	}
}

// D4 — frozen NEXT_SLOT lower bound.
func TestSourceA_Preview_D4_FrozenNextSlot(t *testing.T) {
	calc := NewDeadlineCalculator(nil)
	now := hcmDate(2026, 9, 9)
	sum, err := calc.CalculateDeadlineSummary(context.Background(), &TemplateDeadlineConfig{
		DeadlineMode:         DeadlineModePeriodic,
		FrequencyUnit:        "monthly",
		CycleAnchorDay:       1,
		DeadlineDays:         5,
		DeadlineDurationType: DurationTypeWorkingDays,
		ApplicableFromMode:   ApplicableFromModeNext,
		ApplicableFromSlot:   "2026-10",
	}, CompanyDeadlineContext{}, now)
	if err != nil || sum == nil || sum.DeadlineDate == nil || *sum.DeadlineDate != "2026-10-07" {
		t.Fatalf("frozen NEXT must clamp to Oct Due, got %#v err=%v", sum, err)
	}
	if sum.Status == "OVERDUE" {
		t.Fatalf("status OVERDUE unexpected")
	}
}

// D5 — frozen CURRENT_SLOT lower bound.
func TestSourceA_Preview_D5_FrozenCurrentSlot(t *testing.T) {
	calc := NewDeadlineCalculator(nil)
	now := hcmDate(2026, 9, 9)
	sum, err := calc.CalculateDeadlineSummary(context.Background(), &TemplateDeadlineConfig{
		DeadlineMode:         DeadlineModePeriodic,
		FrequencyUnit:        "monthly",
		CycleAnchorDay:       1,
		DeadlineDays:         5,
		DeadlineDurationType: DurationTypeWorkingDays,
		ApplicableFromMode:   ApplicableFromModeCurrent,
		ApplicableFromSlot:   "2026-10",
	}, CompanyDeadlineContext{}, now)
	if err != nil || sum == nil || sum.StartDate == nil || *sum.StartDate != "2026-10-01" {
		t.Fatalf("frozen CURRENT must clamp to Oct, got %#v err=%v", sum, err)
	}
}

// D6 — null AF → legacy clock-based unchanged.
func TestSourceA_Preview_D6_LegacyNullAF(t *testing.T) {
	calc := NewDeadlineCalculator(nil)
	now := hcmDate(2026, 9, 9)
	sum, err := calc.CalculateDeadlineSummary(context.Background(), &TemplateDeadlineConfig{
		DeadlineMode:         DeadlineModePeriodic,
		FrequencyUnit:        "monthly",
		CycleAnchorDay:       1,
		DeadlineDays:         5,
		DeadlineDurationType: DurationTypeWorkingDays,
	}, CompanyDeadlineContext{}, now)
	if err != nil || sum == nil || sum.DeadlineDate == nil || *sum.DeadlineDate != "2026-09-07" {
		t.Fatalf("legacy null AF want Sep Due 2026-09-07, got %#v err=%v", sum, err)
	}
}

// A1 — candidate before/equal AT → valid.
func TestSourceA_Preview_A1_CandidateWithinApplicableTo(t *testing.T) {
	calc := NewDeadlineCalculator(nil)
	now := hcmDate(2026, 9, 9)
	sum, err := calc.CalculateDeadlineSummary(context.Background(), &TemplateDeadlineConfig{
		DeadlineMode:         DeadlineModePeriodic,
		FrequencyUnit:        "monthly",
		CycleAnchorDay:       1,
		DeadlineDays:         5,
		DeadlineDurationType: DurationTypeWorkingDays,
		ApplicableFromMode:   ApplicableFromModeSpecific,
		ApplicableFromSlot:   "2026-08",
		ApplicableTo:         "2026-09-30",
	}, CompanyDeadlineContext{}, now)
	if err != nil || sum == nil || sum.StartDate == nil || *sum.StartDate != "2026-09-01" {
		t.Fatalf("want Sep within AT, got %#v err=%v", sum, err)
	}
}

// A2 — candidate after AT → no out-of-window deadline.
func TestSourceA_Preview_A2_CandidateAfterApplicableTo(t *testing.T) {
	calc := NewDeadlineCalculator(nil)
	now := hcmDate(2026, 10, 5)
	sum, err := calc.CalculateDeadlineSummary(context.Background(), &TemplateDeadlineConfig{
		DeadlineMode:         DeadlineModePeriodic,
		FrequencyUnit:        "monthly",
		CycleAnchorDay:       1,
		DeadlineDays:         5,
		DeadlineDurationType: DurationTypeWorkingDays,
		ApplicableFromMode:   ApplicableFromModeSpecific,
		ApplicableFromSlot:   "2026-08",
		ApplicableTo:         "2026-09-30",
	}, CompanyDeadlineContext{}, now)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if sum != nil {
		t.Fatalf("expected nil summary after AT, got %#v", sum)
	}
}

// A3 — invalid AF>AT range / invalid AT → fail-safe nil, no panic.
func TestSourceA_Preview_A3_InvalidATFailSafe(t *testing.T) {
	calc := NewDeadlineCalculator(nil)
	now := hcmDate(2026, 9, 9)
	sum, err := calc.CalculateDeadlineSummary(context.Background(), &TemplateDeadlineConfig{
		DeadlineMode:         DeadlineModePeriodic,
		FrequencyUnit:        "monthly",
		CycleAnchorDay:       1,
		DeadlineDays:         5,
		DeadlineDurationType: DurationTypeWorkingDays,
		ApplicableFromMode:   ApplicableFromModeSpecific,
		ApplicableFromSlot:   "2026-10",
		ApplicableTo:         "2026-02-30",
	}, CompanyDeadlineContext{}, now)
	if err != nil {
		t.Fatalf("must not return err/panic, got %v", err)
	}
	if sum != nil {
		t.Fatalf("invalid AT fail-safe want nil, got %#v", sum)
	}
}

// Target October regression — bang-tinh-luong-nhan-vien-thang shape.
func TestSourceA_Preview_TargetOctober_BangTinhLuong(t *testing.T) {
	calc := NewDeadlineCalculator(nil)
	now := hcmDate(2026, 9, 9)
	sum, err := calc.CalculateDeadlineSummary(context.Background(), &TemplateDeadlineConfig{
		DeadlineMode:         DeadlineModePeriodic,
		FrequencyUnit:        "monthly",
		CycleAnchorDay:       1,
		DeadlineDays:         5,
		DeadlineDurationType: DurationTypeWorkingDays,
		ApplicableFromMode:   ApplicableFromModeSpecific,
		ApplicableFromSlot:   "2026-10",
		T0Policy:             "system_date",
		TemplateCategory:     "periodic",
	}, CompanyDeadlineContext{CompanyID: "c_001"}, now)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if sum == nil {
		t.Fatal("summary nil")
	}
	if sum.StartDate == nil || *sum.StartDate != "2026-10-01" {
		t.Fatalf("EXPECTED_T=2026-10-01 got %v", sum.StartDate)
	}
	if sum.DeadlineDate == nil || *sum.DeadlineDate != "2026-10-07" {
		t.Fatalf("EXPECTED_DUE_AT=2026-10-07 got %v", sum.DeadlineDate)
	}
	if sum.Status == "OVERDUE" {
		t.Fatalf("status must not be OVERDUE, got %s remaining=%v", sum.Status, sum.RemainingDays)
	}
}

// Company override still wins for Effective T after AF clamp.
func TestSourceA_Preview_CompanyOverrideEffectiveT(t *testing.T) {
	calc := NewDeadlineCalculator(nil)
	now := hcmDate(2026, 9, 9)
	active := activeTrue()
	sum, err := calc.CalculateDeadlineSummary(context.Background(), &TemplateDeadlineConfig{
		DeadlineMode:         DeadlineModePeriodic,
		FrequencyUnit:        "monthly",
		CycleAnchorDay:       1, // CMS
		DeadlineDays:         5,
		DeadlineDurationType: DurationTypeCalendarDays,
		ApplicableFromMode:   ApplicableFromModeSpecific,
		ApplicableFromSlot:   "2026-10",
	}, CompanyDeadlineContext{
		CompanyID:         "c_001",
		CycleAnchorDay:    15, // company
		OverrideActive:    active,
		OverrideFrequency: "monthly",
	}, now)
	if err != nil || sum == nil || sum.StartDate == nil || *sum.StartDate != "2026-10-15" {
		t.Fatalf("company override T want 2026-10-15 got %#v err=%v", sum, err)
	}
}

// Unfrozen relative (empty slot) → clock candidate, no invented freeze.
func TestSourceA_Preview_UnfrozenRelativeKeepsClock(t *testing.T) {
	calc := NewDeadlineCalculator(nil)
	now := hcmDate(2026, 9, 9)
	sum, err := calc.CalculateDeadlineSummary(context.Background(), &TemplateDeadlineConfig{
		DeadlineMode:         DeadlineModePeriodic,
		FrequencyUnit:        "monthly",
		CycleAnchorDay:       1,
		DeadlineDays:         5,
		DeadlineDurationType: DurationTypeWorkingDays,
		ApplicableFromMode:   ApplicableFromModeNext,
		ApplicableFromSlot:   "",
	}, CompanyDeadlineContext{}, now)
	if err != nil || sum == nil || sum.StartDate == nil || *sum.StartDate != "2026-09-01" {
		t.Fatalf("unfrozen NEXT empty slot keep Sep clock, got %#v err=%v", sum, err)
	}
}

// Quarterly AF clamp (non-monthly coverage).
func TestSourceA_Preview_QuarterlyAFClamp(t *testing.T) {
	calc := NewDeadlineCalculator(nil)
	now := hcmDate(2026, 8, 15) // Q3
	miq := 1
	sum, err := calc.CalculateDeadlineSummary(context.Background(), &TemplateDeadlineConfig{
		DeadlineMode:         DeadlineModePeriodic,
		FrequencyUnit:        "quarterly",
		MonthInQuarter:       &miq,
		DeadlineDays:         5,
		DeadlineDurationType: DurationTypeCalendarDays,
		ApplicableFromMode:   ApplicableFromModeSpecific,
		ApplicableFromSlot:   "2026-Q4",
	}, CompanyDeadlineContext{}, now)
	if err != nil || sum == nil || sum.StartDate == nil || *sum.StartDate != "2026-10-01" {
		t.Fatalf("quarterly AF Q4 want T 2026-10-01 got %#v err=%v", sum, err)
	}
}

// AF > AT: clamp to AF then AT rejects → nil.
func TestSourceA_Preview_AFgtAT_NoSynthetic(t *testing.T) {
	calc := NewDeadlineCalculator(nil)
	now := hcmDate(2026, 9, 9)
	sum, err := calc.CalculateDeadlineSummary(context.Background(), &TemplateDeadlineConfig{
		DeadlineMode:         DeadlineModePeriodic,
		FrequencyUnit:        "monthly",
		CycleAnchorDay:       1,
		DeadlineDays:         5,
		DeadlineDurationType: DurationTypeWorkingDays,
		ApplicableFromMode:   ApplicableFromModeSpecific,
		ApplicableFromSlot:   "2026-10",
		ApplicableTo:         "2026-09-30",
	}, CompanyDeadlineContext{}, now)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if sum != nil {
		t.Fatalf("AF>AT window empty want nil, got %#v", sum)
	}
}
