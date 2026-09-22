package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
)

func TestAttachDetailAbsoluteResolvedDue_cycleWinsOverPreview(t *testing.T) {
	repo := &listResolvedDueTestRepo{
		cycles: []PortalListCycleDueRow{{
			TypeID:          "t-periodic",
			CycleLabel:      "2026-09-04",
			DueDateYYYYMMDD: "2026-09-08",
			Source:          resolvedDueSourceCycleDue,
		}},
	}
	svc := &service{repo: repo, calculator: NewDeadlineCalculator(nil)}
	previewDate := "2026-09-20"
	item := &DisclosureTypeDTO{
		TypeID:           "t-periodic",
		TemplateCategory: TemplateCategoryPeriodic,
		Periodicity:      "daily",
		DeadlineConfig: &TemplateDeadlineConfig{
			DeadlineMode:         DeadlineModePeriodic,
			DeadlineDays:         5,
			FrequencyUnit:        "daily",
			DeadlineDurationType: DurationTypeCalendarDays,
		},
		ResolvedDeadlineRule: &ResolvedDeadlineRuleDTO{
			ResolutionSource: applicability.ResolutionSourceDefaultTemplateRule,
			DayType:          "CALENDAR_DAYS",
			DueDate:          &previewDate,
		},
		DeadlineSummary: &DeadlineSummaryDTO{DeadlineDate: &previewDate},
	}
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, asiaHoChiMinh())
	svc.attachDetailAbsoluteResolvedDue(context.Background(), "c_001", item, now, item.DeadlineSummary)

	if item.ResolvedDueAt == nil || *item.ResolvedDueAt != "2026-09-08T23:59:59+07:00" {
		t.Fatalf("resolved_due_at=%v", item.ResolvedDueAt)
	}
	if item.ResolvedDueSource != resolvedDueSourceCycleDue {
		t.Fatalf("source=%q", item.ResolvedDueSource)
	}
	if item.ResolvedDeadlineRule.DueDate == nil || *item.ResolvedDeadlineRule.DueDate != "2026-09-08" {
		t.Fatalf("rule.due_date must sync to cycle, got %v (preview was %s)", item.ResolvedDeadlineRule.DueDate, previewDate)
	}
	if item.DeadlineSummary.DeadlineDate == nil || *item.DeadlineSummary.DeadlineDate != previewDate {
		t.Fatal("deadline_summary must remain preview metadata (not rewritten)")
	}
}

func TestAttachDetailAbsoluteResolvedDue_plannedDateSource(t *testing.T) {
	repo := &listResolvedDueTestRepo{
		cycles: []PortalListCycleDueRow{{
			TypeID:          "t-periodic",
			CycleLabel:      "2026-09-04",
			DueDateYYYYMMDD: "2026-09-10",
			Source:          resolvedDueSourcePlannedDate,
		}},
	}
	svc := &service{repo: repo, calculator: NewDeadlineCalculator(nil)}
	item := &DisclosureTypeDTO{
		TypeID:           "t-periodic",
		TemplateCategory: TemplateCategoryPeriodic,
		Periodicity:      "daily",
		DeadlineConfig: &TemplateDeadlineConfig{
			DeadlineMode:  DeadlineModePeriodic,
			DeadlineDays:  5,
			FrequencyUnit: "daily",
		},
	}
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, asiaHoChiMinh())
	svc.attachDetailAbsoluteResolvedDue(context.Background(), "c_001", item, now, nil)
	if item.ResolvedDueSource != resolvedDueSourcePlannedDate {
		t.Fatalf("source=%q", item.ResolvedDueSource)
	}
	if item.ResolvedDueAt == nil || !strings.Contains(*item.ResolvedDueAt, "2026-09-10") {
		t.Fatalf("due=%v", item.ResolvedDueAt)
	}
}

func TestAttachDetailAbsoluteResolvedDue_previewFallback(t *testing.T) {
	repo := &listResolvedDueTestRepo{cycles: nil}
	svc := &service{repo: repo, calculator: NewDeadlineCalculator(nil)}
	preview := "2026-09-08"
	item := &DisclosureTypeDTO{
		TypeID:           "t-preview",
		TemplateCategory: TemplateCategoryPeriodic,
		Periodicity:      "daily",
		DeadlineConfig: &TemplateDeadlineConfig{
			DeadlineMode:  DeadlineModePeriodic,
			DeadlineDays:  5,
			FrequencyUnit: "daily",
		},
		ResolvedDeadlineRule: &ResolvedDeadlineRuleDTO{ResolutionSource: applicability.ResolutionSourceDefaultTemplateRule},
		DeadlineSummary:      &DeadlineSummaryDTO{DeadlineDate: &preview},
	}
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, asiaHoChiMinh())
	svc.attachDetailAbsoluteResolvedDue(context.Background(), "c_001", item, now, item.DeadlineSummary)
	if item.ResolvedDueSource != resolvedDueSourceDeadlineSummaryPreview {
		t.Fatalf("source=%q", item.ResolvedDueSource)
	}
	if item.ResolvedDueAt == nil || *item.ResolvedDueAt != "2026-09-08T23:59:59+07:00" {
		t.Fatalf("due=%v", item.ResolvedDueAt)
	}
}

func TestAttachDetailAbsoluteResolvedDue_irregularSkipped(t *testing.T) {
	repo := &listResolvedDueTestRepo{
		cycles: []PortalListCycleDueRow{{
			TypeID: "t-irr", CycleLabel: "2026-09-04", DueDateYYYYMMDD: "2026-09-08", Source: resolvedDueSourceCycleDue,
		}},
	}
	svc := &service{repo: repo}
	item := &DisclosureTypeDTO{
		TypeID:           "t-irr",
		TemplateCategory: TemplateCategoryIrregular,
		DeadlineConfig:   &TemplateDeadlineConfig{DeadlineMode: DeadlineModeNone, TemplateCategory: TemplateCategoryIrregular},
	}
	svc.attachDetailAbsoluteResolvedDue(context.Background(), "c_001", item, time.Now(), nil)
	if item.ResolvedDueAt != nil {
		t.Fatalf("irregular must skip absolute due, got %v", *item.ResolvedDueAt)
	}
}

func TestListDetailAbsoluteDueParity_sameCycle(t *testing.T) {
	repo := &listResolvedDueTestRepo{
		cycles: []PortalListCycleDueRow{{
			TypeID:          "t-1",
			CycleLabel:      "2026-09-04",
			DueDateYYYYMMDD: "2026-09-08",
			Source:          resolvedDueSourceCycleDue,
		}},
	}
	svc := &service{repo: repo, calculator: NewDeadlineCalculator(nil)}
	cfg := &TemplateDeadlineConfig{
		DeadlineMode:         DeadlineModePeriodic,
		DeadlineDays:         5,
		FrequencyUnit:        "daily",
		DeadlineDurationType: DurationTypeCalendarDays,
	}
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, asiaHoChiMinh())

	listItems := []DisclosureTypeSummaryDTO{{
		TypeID:           "t-1",
		TemplateCategory: TemplateCategoryPeriodic,
		Periodicity:      "daily",
		DeadlineConfig:   cfg,
	}}
	svc.enrichPortalListResolvedDue(context.Background(), "c_001", listItems, now)

	detail := &DisclosureTypeDTO{
		TypeID:           "t-1",
		TemplateCategory: TemplateCategoryPeriodic,
		Periodicity:      "daily",
		DeadlineConfig:   cfg,
		DeadlineSummary:  &DeadlineSummaryDTO{DeadlineDate: ptrString("2026-09-20")},
	}
	svc.attachDetailAbsoluteResolvedDue(context.Background(), "c_001", detail, now, detail.DeadlineSummary)

	if listItems[0].ResolvedDueAt == nil || detail.ResolvedDueAt == nil {
		t.Fatal("both need resolved_due_at")
	}
	if *listItems[0].ResolvedDueAt != *detail.ResolvedDueAt {
		t.Fatalf("list=%s detail=%s", *listItems[0].ResolvedDueAt, *detail.ResolvedDueAt)
	}
	if listItems[0].ResolvedDueSource != detail.ResolvedDueSource {
		t.Fatalf("sources list=%s detail=%s", listItems[0].ResolvedDueSource, detail.ResolvedDueSource)
	}
}

func TestResolveAbsoluteDueFromCycleOrPreview_empty(t *testing.T) {
	res := resolveAbsoluteDueFromCycleOrPreview(nil, "t", "daily", time.Now(), asiaHoChiMinh(), "")
	if res.DueAt != nil || res.Source != "" {
		t.Fatalf("expected empty, got %+v", res)
	}
}
