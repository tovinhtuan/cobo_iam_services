package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
)

func TestFormatResolvedDueAtHCMEOD(t *testing.T) {
	loc := asiaHoChiMinh()
	got, ok := FormatResolvedDueAtHCMEOD("2026-09-08", loc)
	if !ok {
		t.Fatal("expected ok")
	}
	want := "2026-09-08T23:59:59+07:00"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if _, ok := FormatResolvedDueAtHCMEOD("", loc); ok {
		t.Fatal("empty must fail")
	}
	if _, ok := FormatResolvedDueAtHCMEOD("not-a-date", loc); ok {
		t.Fatal("invalid must fail")
	}
}

func TestEnrichPortalListResolvedDue_cyclePreferredOverPreview(t *testing.T) {
	repo := &listResolvedDueTestRepo{
		cycles: []PortalListCycleDueRow{{
			TypeID:          "t-periodic",
			CycleLabel:      "2026-09-04",
			DueDateYYYYMMDD: "2026-09-08",
			Source:          resolvedDueSourceCycleDue,
		}},
	}
	svc := &service{
		repo:       repo,
		calculator: NewDeadlineCalculator(nil),
	}
	items := []DisclosureTypeSummaryDTO{{
		TypeID:           "t-periodic",
		TemplateCategory: TemplateCategoryPeriodic,
		Periodicity:      "daily",
		DeadlineConfig: &TemplateDeadlineConfig{
			DeadlineMode:         DeadlineModePeriodic,
			DeadlineDays:         5,
			FrequencyUnit:        "daily",
			TemplateCategory:     TemplateCategoryPeriodic,
			DeadlineDurationType: DurationTypeCalendarDays,
		},
	}}
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, asiaHoChiMinh())
	svc.enrichPortalListResolvedDue(context.Background(), "c_001", items, now)
	if items[0].ResolvedDueAt == nil {
		t.Fatal("expected resolved_due_at")
	}
	if *items[0].ResolvedDueAt != "2026-09-08T23:59:59+07:00" {
		t.Fatalf("due=%q", *items[0].ResolvedDueAt)
	}
	if items[0].ResolvedDueSource != resolvedDueSourceCycleDue {
		t.Fatalf("source=%q", items[0].ResolvedDueSource)
	}
}

func TestEnrichPortalListResolvedDue_companyIsolation(t *testing.T) {
	repo := &listResolvedDueTestRepo{
		cycles: []PortalListCycleDueRow{{
			TypeID:          "t-1",
			CycleLabel:      "2026-09-04",
			DueDateYYYYMMDD: "2026-09-05",
			Source:          resolvedDueSourceCycleDue,
		}},
	}
	svc := &service{repo: repo, calculator: NewDeadlineCalculator(nil)}
	itemsA := []DisclosureTypeSummaryDTO{{
		TypeID:           "t-1",
		TemplateCategory: TemplateCategoryPeriodic,
		Periodicity:      "daily",
		DeadlineConfig: &TemplateDeadlineConfig{
			DeadlineMode:         DeadlineModePeriodic,
			DeadlineDays:         5,
			FrequencyUnit:        "daily",
			DeadlineDurationType: DurationTypeCalendarDays,
		},
	}}
	itemsB := []DisclosureTypeSummaryDTO{{
		TypeID:           "t-1",
		TemplateCategory: TemplateCategoryPeriodic,
		Periodicity:      "daily",
		DeadlineConfig: &TemplateDeadlineConfig{
			DeadlineMode:         DeadlineModePeriodic,
			DeadlineDays:         5,
			FrequencyUnit:        "daily",
			DeadlineDurationType: DurationTypeCalendarDays,
		},
	}}
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, asiaHoChiMinh())
	svc.enrichPortalListResolvedDue(context.Background(), "c_A", itemsA, now)
	repo.cycles = []PortalListCycleDueRow{{
		TypeID: "t-1", CycleLabel: "2026-09-04", DueDateYYYYMMDD: "2026-09-14", Source: resolvedDueSourceCycleDue,
	}}
	svc.enrichPortalListResolvedDue(context.Background(), "c_B", itemsB, now)
	if itemsA[0].ResolvedDueAt == nil || itemsB[0].ResolvedDueAt == nil {
		t.Fatal("both need due")
	}
	if *itemsA[0].ResolvedDueAt == *itemsB[0].ResolvedDueAt {
		t.Fatalf("expected different company dues, both %s", *itemsA[0].ResolvedDueAt)
	}
	if !strings.Contains(*itemsA[0].ResolvedDueAt, "2026-09-05") || !strings.Contains(*itemsB[0].ResolvedDueAt, "2026-09-14") {
		t.Fatalf("A=%s B=%s", *itemsA[0].ResolvedDueAt, *itemsB[0].ResolvedDueAt)
	}
	if repo.lastCompanyID != "c_B" {
		t.Fatalf("last company=%q", repo.lastCompanyID)
	}
}

func TestEnrichPortalListResolvedDue_irregularSkipped(t *testing.T) {
	repo := &listResolvedDueTestRepo{}
	svc := &service{repo: repo, calculator: NewDeadlineCalculator(nil)}
	items := []DisclosureTypeSummaryDTO{{
		TypeID:           "t-irr",
		TemplateCategory: TemplateCategoryIrregular,
		DeadlineRule:     "Trong vòng 24 giờ kể từ sự kiện",
		DeadlineConfig:   &TemplateDeadlineConfig{DeadlineMode: DeadlineModeNone, TemplateCategory: TemplateCategoryIrregular},
	}}
	svc.enrichPortalListResolvedDue(context.Background(), "c_001", items, time.Now())
	if items[0].ResolvedDueAt != nil {
		t.Fatalf("irregular must not get resolved due, got %v", *items[0].ResolvedDueAt)
	}
}

func TestEnrichPortalListResolvedDue_previewFallback(t *testing.T) {
	repo := &listResolvedDueTestRepo{cycles: nil}
	svc := &service{repo: repo, calculator: NewDeadlineCalculator(nil)}
	items := []DisclosureTypeSummaryDTO{{
		TypeID:           "t-preview",
		TemplateCategory: TemplateCategoryPeriodic,
		Periodicity:      "daily",
		DeadlineConfig: &TemplateDeadlineConfig{
			DeadlineMode:         DeadlineModePeriodic,
			DeadlineDays:         5,
			FrequencyUnit:        "daily",
			DeadlineDurationType: DurationTypeCalendarDays,
		},
	}}
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, asiaHoChiMinh())
	svc.enrichPortalListResolvedDue(context.Background(), "c_001", items, now)
	if items[0].ResolvedDueAt == nil {
		t.Fatal("expected preview due")
	}
	if items[0].ResolvedDueSource != resolvedDueSourceDeadlineSummaryPreview {
		t.Fatalf("source=%q", items[0].ResolvedDueSource)
	}
	if !strings.Contains(*items[0].ResolvedDueAt, "2026-09-08") {
		t.Fatalf("due=%s want 2026-09-08 EOD", *items[0].ResolvedDueAt)
	}
}

func TestEnrichPortalListResolvedDue_noOccurrenceNoGeneric(t *testing.T) {
	repo := &listResolvedDueTestRepo{cycles: nil}
	svc := &service{repo: repo, calculator: NewDeadlineCalculator(nil)}
	items := []DisclosureTypeSummaryDTO{{
		TypeID:           "t-none",
		TemplateCategory: TemplateCategoryPeriodic,
		DeadlineRule:     "T+5",
		DeadlineConfig: &TemplateDeadlineConfig{
			DeadlineMode: DeadlineModeNone,
		},
	}}
	svc.enrichPortalListResolvedDue(context.Background(), "c_001", items, time.Now())
	if items[0].ResolvedDueAt != nil {
		t.Fatal("NONE mode must leave resolved due null (FE must not show T+5 as company due)")
	}
}

func TestEnrichPortalListResolvedDue_resolvedDeadlineRuleParity(t *testing.T) {
	repo := &listResolvedDueTestRepo{
		cycles: []PortalListCycleDueRow{{
			TypeID:          "t-periodic",
			CycleLabel:      "2026-09-04",
			DueDateYYYYMMDD: "2026-09-08",
			Source:          resolvedDueSourceCycleDue,
		}},
	}
	svc := &service{repo: repo, calculator: NewDeadlineCalculator(nil)}
	rules := &applicability.TemplateApplicabilityRules{
		DeadlineDays:    5,
		DeadlineDayType: "calendar",
	}
	items := []DisclosureTypeSummaryDTO{{
		TypeID:             "t-periodic",
		TemplateCategory:   TemplateCategoryPeriodic,
		Periodicity:        "daily",
		ApplicabilityRules: rules,
		DeadlineConfig: &TemplateDeadlineConfig{
			DeadlineMode:         DeadlineModePeriodic,
			DeadlineDays:         5,
			FrequencyUnit:        "daily",
			TemplateCategory:     TemplateCategoryPeriodic,
			DeadlineDurationType: DurationTypeCalendarDays,
		},
	}}
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, asiaHoChiMinh())
	svc.enrichPortalListResolvedDue(context.Background(), "c_001", items, now)

	if items[0].ResolvedDeadlineRule == nil {
		t.Fatal("expected resolved_deadline_rule to be enriched on list summary")
	}
	if items[0].ResolvedDeadlineRule.ResolvedDays == nil || *items[0].ResolvedDeadlineRule.ResolvedDays != 5 {
		t.Fatalf("expected resolvedDays=5, got %v", items[0].ResolvedDeadlineRule.ResolvedDays)
	}
	if items[0].ResolvedDeadlineRule.DayType != "CALENDAR_DAYS" {
		t.Fatalf("expected dayType=CALENDAR_DAYS, got %q", items[0].ResolvedDeadlineRule.DayType)
	}
	if items[0].ResolvedDeadlineRule.BaseDateSource != BaseDateSourceCycleStart {
		t.Fatalf("expected baseDateSource=CYCLE_START, got %q", items[0].ResolvedDeadlineRule.BaseDateSource)
	}
	// Runtime resolved_due_at must STILL be preserved!
	if items[0].ResolvedDueAt == nil || *items[0].ResolvedDueAt != "2026-09-08T23:59:59+07:00" {
		t.Fatalf("expected runtime resolvedDueAt preserved, got %v", items[0].ResolvedDueAt)
	}
}

type listResolvedDueTestRepo struct {
	Repository
	cycles        []PortalListCycleDueRow
	prefs         []CompanyTypePreference
	lastCompanyID string
}

func (r *listResolvedDueTestRepo) ListPortalListCycleDues(_ context.Context, companyID string, _ []string) ([]PortalListCycleDueRow, error) {
	r.lastCompanyID = companyID
	out := make([]PortalListCycleDueRow, len(r.cycles))
	copy(out, r.cycles)
	return out, nil
}

func (r *listResolvedDueTestRepo) GetCompanyDeadlineContext(_ context.Context, companyID string) (CompanyDeadlineContext, error) {
	return CompanyDeadlineContext{CompanyID: companyID}, nil
}

func (r *listResolvedDueTestRepo) ListCompanyTypePreferencesByTypeIDs(_ context.Context, _ []string) ([]CompanyTypePreference, error) {
	return r.prefs, nil
}

func (r *listResolvedDueTestRepo) GetCompanyApplicabilityProfile(_ context.Context, _ string) (applicability.CompanyApplicabilityProfile, error) {
	return applicability.CompanyApplicabilityProfile{}, nil
}
