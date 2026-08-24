package app

import (
	"context"
	"testing"
	"time"
)

func monthlyCfg(mode, slot string, day, n int) *TemplateDeadlineConfig {
	return &TemplateDeadlineConfig{
		FrequencyUnit:        "monthly",
		CycleAnchorDay:       day,
		DeadlineDays:         n,
		DeadlineDurationType: DurationTypeCalendarDays,
		ApplicableFromMode:   mode,
		ApplicableFromSlot:   slot,
		OpenDaysBeforeT:      0,
	}
}

func TestResolveFirstMaterializableSlot_PastEqualFuture(t *testing.T) {
	first, err := ResolveFirstMaterializableSlot("monthly", "2026-08", "2026-04", false)
	if err != nil || first != "2026-08" {
		t.Fatalf("past boundary → current, got %q err=%v", first, err)
	}
	first, err = ResolveFirstMaterializableSlot("monthly", "2026-08", "2026-08", false)
	if err != nil || first != "2026-08" {
		t.Fatalf("equal → %q", first)
	}
	first, err = ResolveFirstMaterializableSlot("monthly", "2026-08", "2026-11", false)
	if err != nil || first != "2026-11" {
		t.Fatalf("future → %q", first)
	}
	first, err = ResolveFirstMaterializableSlot("monthly", "2026-08", "", true)
	if err != nil || first != "2026-08" {
		t.Fatalf("legacy → %q", first)
	}
}

func TestBuildFirstOccurrencePreview_ModesMonthly(t *testing.T) {
	loc := asiaHoChiMinh()
	eval := time.Date(2026, 8, 15, 12, 0, 0, 0, loc)
	ctx := context.Background()

	t.Run("specific_past", func(t *testing.T) {
		p, _ := BuildFirstOccurrencePreview(ctx, monthlyCfg(ApplicableFromModeSpecific, "2026-04", 5, 20), eval, nil)
		if p.FirstOccurrenceSlot != "2026-08" {
			t.Fatalf("first=%s want 2026-08 (anti-backfill)", p.FirstOccurrenceSlot)
		}
		if p.ProspectiveApplicableFromSlot == nil || *p.ProspectiveApplicableFromSlot != "2026-04" {
			t.Fatalf("boundary=%v", p.ProspectiveApplicableFromSlot)
		}
		if !p.FirstOccurrenceIsCurrentCandidate {
			t.Fatal("expected current candidate")
		}
	})
	t.Run("current", func(t *testing.T) {
		p, _ := BuildFirstOccurrencePreview(ctx, monthlyCfg(ApplicableFromModeCurrent, "", 5, 20), eval, nil)
		if p.FirstOccurrenceSlot != "2026-08" || p.ProspectiveApplicableFromSlot == nil || *p.ProspectiveApplicableFromSlot != "2026-08" {
			t.Fatalf("%+v", p)
		}
	})
	t.Run("next", func(t *testing.T) {
		p, _ := BuildFirstOccurrencePreview(ctx, monthlyCfg(ApplicableFromModeNext, "", 5, 20), eval, nil)
		if p.FirstOccurrenceSlot != "2026-09" {
			t.Fatalf("first=%s", p.FirstOccurrenceSlot)
		}
		if p.FirstOccurrenceIsCurrentCandidate {
			t.Fatal("next must not be current candidate")
		}
	})
	t.Run("specific_future", func(t *testing.T) {
		p, _ := BuildFirstOccurrencePreview(ctx, monthlyCfg(ApplicableFromModeSpecific, "2026-11", 5, 20), eval, nil)
		if p.FirstOccurrenceSlot != "2026-11" {
			t.Fatalf("first=%s", p.FirstOccurrenceSlot)
		}
	})
	t.Run("legacy", func(t *testing.T) {
		p, _ := BuildFirstOccurrencePreview(ctx, monthlyCfg("", "", 5, 20), eval, nil)
		if p.FirstOccurrenceSlot != "2026-08" || p.ProspectiveApplicableFromSlot != nil {
			t.Fatalf("%+v", p)
		}
	})
}

func TestBuildFirstOccurrencePreview_AllFrequenciesPastEqualFuture(t *testing.T) {
	loc := asiaHoChiMinh()
	eval := time.Date(2026, 8, 24, 10, 0, 0, 0, loc) // Mon; weekly Sunday=2026-08-23
	ctx := context.Background()

	cases := []struct {
		freq, past, equal, future string
	}{
		{"daily", "2026-08-20", "2026-08-24", "2026-08-26"},
		{"weekly", "2026-08-16", "2026-08-23", "2026-08-30"},
		{"monthly", "2026-04", "2026-08", "2026-11"},
		{"quarterly", "2026-Q1", "2026-Q3", "2027-Q1"},
		{"yearly", "2025", "2026", "2028"},
	}
	for _, tc := range cases {
		cfg := &TemplateDeadlineConfig{
			FrequencyUnit: tc.freq, DeadlineDays: 10, DeadlineDurationType: DurationTypeCalendarDays,
			CycleAnchorDay: 1, CycleAnchorMonth: 1,
		}
		if tc.freq == "weekly" {
			wd := int(time.Friday)
			cfg.CycleAnchorWeekday = &wd
		}
		if tc.freq == "quarterly" {
			miq := 1
			cfg.MonthInQuarter = &miq
		}
		cfg.ApplicableFromMode = ApplicableFromModeSpecific
		cfg.ApplicableFromSlot = tc.past
		p, _ := BuildFirstOccurrencePreview(ctx, cfg, eval, nil)
		if p.FirstOccurrenceSlot != tc.equal {
			t.Fatalf("%s past: first=%s want %s", tc.freq, p.FirstOccurrenceSlot, tc.equal)
		}
		cfg.ApplicableFromSlot = tc.equal
		p, _ = BuildFirstOccurrencePreview(ctx, cfg, eval, nil)
		if p.FirstOccurrenceSlot != tc.equal {
			t.Fatalf("%s equal: first=%s", tc.freq, p.FirstOccurrenceSlot)
		}
		cfg.ApplicableFromSlot = tc.future
		p, _ = BuildFirstOccurrencePreview(ctx, cfg, eval, nil)
		if p.FirstOccurrenceSlot != tc.future {
			t.Fatalf("%s future: first=%s want %s", tc.freq, p.FirstOccurrenceSlot, tc.future)
		}
	}
}

func TestBuildFirstOccurrencePreview_WeeklyTWeekdayDoesNotChangeFirstSlot(t *testing.T) {
	loc := asiaHoChiMinh()
	eval := time.Date(2026, 8, 24, 10, 0, 0, 0, loc)
	ctx := context.Background()
	for _, wd := range []int{int(time.Sunday), int(time.Monday), int(time.Friday)} {
		w := wd
		cfg := &TemplateDeadlineConfig{
			FrequencyUnit: "weekly", DeadlineDays: 5, DeadlineDurationType: DurationTypeCalendarDays,
			CycleAnchorWeekday: &w, ApplicableFromMode: ApplicableFromModeSpecific, ApplicableFromSlot: "2026-08-16",
		}
		p, _ := BuildFirstOccurrencePreview(ctx, cfg, eval, nil)
		if p.FirstOccurrenceSlot != "2026-08-23" {
			t.Fatalf("weekday=%d first=%s", wd, p.FirstOccurrenceSlot)
		}
	}
}

func TestBuildFirstOccurrencePreview_WeeklyCrossYear(t *testing.T) {
	loc := asiaHoChiMinh()
	eval := time.Date(2027, 1, 2, 10, 0, 0, 0, loc) // Sat; Sunday week start 2026-12-27
	ctx := context.Background()
	cfg := &TemplateDeadlineConfig{
		FrequencyUnit: "weekly", DeadlineDays: 5, DeadlineDurationType: DurationTypeCalendarDays,
		ApplicableFromMode: ApplicableFromModeSpecific, ApplicableFromSlot: "2026-12-20",
	}
	p, _ := BuildFirstOccurrencePreview(ctx, cfg, eval, nil)
	if p.CurrentLogicalSlot != "2026-12-27" || p.FirstOccurrenceSlot != "2026-12-27" {
		t.Fatalf("current=%s first=%s", p.CurrentLogicalSlot, p.FirstOccurrenceSlot)
	}
}

func TestClassifyFirstOccurrenceScheduleStatus(t *testing.T) {
	loc := asiaHoChiMinh()
	open := time.Date(2026, 9, 1, 0, 0, 0, 0, loc)
	due := time.Date(2026, 9, 24, 0, 0, 0, 0, loc)
	if got := ClassifyFirstOccurrenceScheduleStatus(time.Date(2026, 8, 31, 12, 0, 0, 0, loc), open, due, loc); got != FirstOccurrenceStatusFuture {
		t.Fatalf("future=%s", got)
	}
	if got := ClassifyFirstOccurrenceScheduleStatus(time.Date(2026, 9, 10, 12, 0, 0, 0, loc), open, due, loc); got != FirstOccurrenceStatusOpen {
		t.Fatalf("open=%s", got)
	}
	if got := ClassifyFirstOccurrenceScheduleStatus(time.Date(2026, 9, 24, 12, 0, 0, 0, loc), open, due, loc); got != FirstOccurrenceStatusDueToday {
		t.Fatalf("due today=%s", got)
	}
	if got := ClassifyFirstOccurrenceScheduleStatus(time.Date(2026, 9, 26, 12, 0, 0, 0, loc), open, due, loc); got != FirstOccurrenceStatusOverdue {
		t.Fatalf("overdue=%s", got)
	}
}

func TestBuildFirstOccurrencePreview_HCMMidnight(t *testing.T) {
	loc := asiaHoChiMinh()
	ctx := context.Background()
	cfg := monthlyCfg(ApplicableFromModeCurrent, "", 5, 20)
	before := time.Date(2026, 8, 31, 23, 59, 0, 0, loc)
	after := time.Date(2026, 9, 1, 0, 1, 0, 0, loc)
	p1, _ := BuildFirstOccurrencePreview(ctx, cfg, before, nil)
	p2, _ := BuildFirstOccurrencePreview(ctx, cfg, after, nil)
	if p1.FirstOccurrenceSlot != "2026-08" || p2.FirstOccurrenceSlot != "2026-09" {
		t.Fatalf("before=%s after=%s", p1.FirstOccurrenceSlot, p2.FirstOccurrenceSlot)
	}
}

func TestBuildFirstOccurrencePreview_DeadlineInclusiveAndLateOverdue(t *testing.T) {
	loc := asiaHoChiMinh()
	ctx := context.Background()
	cfg := monthlyCfg(ApplicableFromModeCurrent, "", 5, 20)
	eval := time.Date(2026, 9, 26, 10, 0, 0, 0, loc)
	p, w := BuildFirstOccurrencePreview(ctx, cfg, eval, nil)
	if p.T == nil || *p.T != "2026-09-05" {
		t.Fatalf("T=%v", p.T)
	}
	if p.DueAt == nil || *p.DueAt != "2026-09-24" {
		t.Fatalf("DueAt=%v want 2026-09-24 (inclusive T+N-1)", p.DueAt)
	}
	if p.Status != FirstOccurrenceStatusOverdue || len(w) == 0 {
		t.Fatalf("status=%s warnings=%v", p.Status, w)
	}
	if p.FirstOccurrenceSlot != "2026-09" {
		t.Fatalf("must not shift to next slot, got %s", p.FirstOccurrenceSlot)
	}
}

func TestBuildFirstOccurrencePreview_IdempotentNoMutation(t *testing.T) {
	loc := asiaHoChiMinh()
	eval := time.Date(2026, 8, 15, 12, 0, 0, 0, loc)
	cfg := monthlyCfg(ApplicableFromModeNext, "", 5, 20)
	modeBefore, slotBefore := cfg.ApplicableFromMode, cfg.ApplicableFromSlot
	p1, _ := BuildFirstOccurrencePreview(context.Background(), cfg, eval, nil)
	p2, _ := BuildFirstOccurrencePreview(context.Background(), cfg, eval, nil)
	if p1.FirstOccurrenceSlot != p2.FirstOccurrenceSlot || p1.Status != p2.Status {
		t.Fatal("preview not idempotent")
	}
	if cfg.ApplicableFromMode != modeBefore || cfg.ApplicableFromSlot != slotBefore {
		t.Fatal("preview must not mutate draft config")
	}
}

func TestBuildFirstOccurrencePreview_PreviewDoesNotFreezeRelative(t *testing.T) {
	loc := asiaHoChiMinh()
	eval := time.Date(2026, 8, 15, 12, 0, 0, 0, loc)
	cfg := monthlyCfg(ApplicableFromModeNext, "", 5, 20)
	_, _ = BuildFirstOccurrencePreview(context.Background(), cfg, eval, nil)
	if cfg.ApplicableFromSlot != "" {
		t.Fatalf("preview froze slot unexpectedly: %q", cfg.ApplicableFromSlot)
	}
}
