package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func monthlyDetailItem(typeID string) *DisclosureTypeDTO {
	preview := "2026-10-07"
	return &DisclosureTypeDTO{
		TypeID:           typeID,
		TemplateCategory: TemplateCategoryPeriodic,
		Periodicity:      "monthly",
		DeadlineConfig: &TemplateDeadlineConfig{
			DeadlineMode:  DeadlineModePeriodic,
			DeadlineDays:  5,
			FrequencyUnit: "monthly",
		},
		DeadlineSummary: &DeadlineSummaryDTO{DeadlineDate: &preview, Status: "OVERDUE"},
	}
}

func TestAttachDetailCurrentCycle_materializedRecord(t *testing.T) {
	repo := &listResolvedDueTestRepo{cycles: []PortalListCycleDueRow{
		{TypeID: "t-pay", CycleLabel: "2026-09", DueDateYYYYMMDD: "2026-09-07", Source: resolvedDueSourceCycleDue, RecordID: "rec-sep"},
		{TypeID: "t-pay", CycleLabel: "2026-10", DueDateYYYYMMDD: "2026-10-07", Source: resolvedDueSourceCycleDue, RecordID: "rec-oct"},
	}}
	svc := &service{repo: repo, calculator: NewDeadlineCalculator(nil)}
	item := monthlyDetailItem("t-pay")
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, asiaHoChiMinh())

	svc.attachDetailAbsoluteResolvedDue(context.Background(), "c_001", item, now, item.DeadlineSummary)

	if repo.lastCompanyID != "c_001" {
		t.Fatalf("cycle lookup must be scoped to the subject company, got %q", repo.lastCompanyID)
	}
	s := item.DeadlineSummary
	if s.CurrentCycleLabel == nil || *s.CurrentCycleLabel != "2026-10" {
		t.Fatalf("current_cycle_label=%v, want 2026-10 (current slot only)", s.CurrentCycleLabel)
	}
	if s.CurrentRecordID == nil || *s.CurrentRecordID != "rec-oct" {
		t.Fatalf("current_record_id=%v, want rec-oct (not the previous cycle)", s.CurrentRecordID)
	}
	if s.Status != "OVERDUE" || s.DeadlineDate == nil || *s.DeadlineDate != "2026-10-07" {
		t.Fatal("existing deadline_summary fields must stay unchanged")
	}
}

func TestAttachDetailCurrentCycle_notMaterializedOnlyLabel(t *testing.T) {
	repo := &listResolvedDueTestRepo{cycles: []PortalListCycleDueRow{
		{TypeID: "t-pay", CycleLabel: "2026-10", DueDateYYYYMMDD: "2026-10-07", Source: resolvedDueSourceCycleDue},
	}}
	svc := &service{repo: repo, calculator: NewDeadlineCalculator(nil)}
	item := monthlyDetailItem("t-pay")
	now := time.Date(2026, 10, 2, 9, 0, 0, 0, asiaHoChiMinh())

	svc.attachDetailAbsoluteResolvedDue(context.Background(), "c_001", item, now, item.DeadlineSummary)

	if item.DeadlineSummary.CurrentCycleLabel == nil || *item.DeadlineSummary.CurrentCycleLabel != "2026-10" {
		t.Fatalf("label=%v", item.DeadlineSummary.CurrentCycleLabel)
	}
	if item.DeadlineSummary.CurrentRecordID != nil {
		t.Fatalf("record id must be omitted before materialization, got %q", *item.DeadlineSummary.CurrentRecordID)
	}
}

func TestAttachDetailCurrentCycle_noCycleRowOmitsFields(t *testing.T) {
	repo := &listResolvedDueTestRepo{cycles: []PortalListCycleDueRow{
		{TypeID: "t-pay", CycleLabel: "2026-09", DueDateYYYYMMDD: "2026-09-07", Source: resolvedDueSourceCycleDue, RecordID: "rec-sep"},
		{TypeID: "t-other", CycleLabel: "2026-10", DueDateYYYYMMDD: "2026-10-07", Source: resolvedDueSourceCycleDue, RecordID: "rec-other"},
	}}
	svc := &service{repo: repo, calculator: NewDeadlineCalculator(nil)}
	item := monthlyDetailItem("t-pay")
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, asiaHoChiMinh())

	svc.attachDetailAbsoluteResolvedDue(context.Background(), "c_001", item, now, item.DeadlineSummary)

	if item.DeadlineSummary.CurrentCycleLabel != nil || item.DeadlineSummary.CurrentRecordID != nil {
		t.Fatalf("no row for this type+slot → no fields, got label=%v record=%v",
			item.DeadlineSummary.CurrentCycleLabel, item.DeadlineSummary.CurrentRecordID)
	}
	raw, err := json.Marshal(item.DeadlineSummary)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "current_record_id") || strings.Contains(string(raw), "current_cycle_label") {
		t.Fatalf("absent fields must be omitted from JSON: %s", raw)
	}
}

func TestAttachDetailCurrentCycle_irregularSkipped(t *testing.T) {
	repo := &listResolvedDueTestRepo{cycles: []PortalListCycleDueRow{
		{TypeID: "t-irr", CycleLabel: "2026-10-13", DueDateYYYYMMDD: "2026-10-20", Source: resolvedDueSourceCycleDue, RecordID: "rec-irr"},
	}}
	svc := &service{repo: repo}
	item := &DisclosureTypeDTO{
		TypeID:           "t-irr",
		TemplateCategory: TemplateCategoryIrregular,
		DeadlineConfig:   &TemplateDeadlineConfig{DeadlineMode: DeadlineModeNone, TemplateCategory: TemplateCategoryIrregular},
		DeadlineSummary:  &DeadlineSummaryDTO{Status: "UNKNOWN"},
	}
	svc.attachDetailAbsoluteResolvedDue(context.Background(), "c_001", item, time.Date(2026, 10, 13, 9, 0, 0, 0, asiaHoChiMinh()), nil)
	if item.DeadlineSummary.CurrentRecordID != nil || item.DeadlineSummary.CurrentCycleLabel != nil {
		t.Fatal("irregular templates must not expose a current cycle record")
	}
}

type failingCycleReaderRepo struct {
	listResolvedDueTestRepo
}

func (r *failingCycleReaderRepo) ListPortalListCycleDues(context.Context, string, []string) ([]PortalListCycleDueRow, error) {
	return nil, errors.New("db down")
}

func TestAttachDetailCurrentCycle_readerErrorKeepsResponse(t *testing.T) {
	svc := &service{repo: &failingCycleReaderRepo{}, calculator: NewDeadlineCalculator(nil)}
	item := monthlyDetailItem("t-pay")
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, asiaHoChiMinh())

	svc.attachDetailAbsoluteResolvedDue(context.Background(), "c_001", item, now, item.DeadlineSummary)

	if item.DeadlineSummary.CurrentRecordID != nil || item.DeadlineSummary.CurrentCycleLabel != nil {
		t.Fatal("reader error must only drop the new fields")
	}
	if item.ResolvedDueSource != resolvedDueSourceDeadlineSummaryPreview {
		t.Fatalf("preview fallback must still apply, source=%q", item.ResolvedDueSource)
	}
}

func TestAttachCurrentCycleRecord_nilSummaryNoop(t *testing.T) {
	cycles := map[string]PortalListCycleDueRow{"t|2026-10": {TypeID: "t", CycleLabel: "2026-10", RecordID: "r"}}
	attachCurrentCycleRecord(nil, cycles, "t", "monthly", time.Date(2026, 10, 13, 9, 0, 0, 0, asiaHoChiMinh()), nil)
}

func TestAttachDetailCurrentCycle_unknownSummarySkipped(t *testing.T) {
	repo := &listResolvedDueTestRepo{cycles: []PortalListCycleDueRow{
		{TypeID: "t-pay", CycleLabel: "2026-10", DueDateYYYYMMDD: "2026-10-07", Source: resolvedDueSourceCycleDue, RecordID: "rec-oct"},
	}}
	svc := &service{repo: repo, calculator: NewDeadlineCalculator(nil)}
	item := monthlyDetailItem("t-pay")
	item.DeadlineSummary = &DeadlineSummaryDTO{Status: "UNKNOWN"}
	svc.attachDetailAbsoluteResolvedDue(context.Background(), "c_001", item, time.Date(2026, 10, 13, 9, 0, 0, 0, asiaHoChiMinh()), nil)
	if item.DeadlineSummary.CurrentCycleLabel != nil || item.DeadlineSummary.CurrentRecordID != nil {
		t.Fatal("UNKNOWN summaries must not carry current cycle fields")
	}
}

func TestLimitCurrentRecordToDeadlineViewers(t *testing.T) {
	sub := Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c_001"}
	newItem := func() *DisclosureTypeDTO {
		label, rec := "2026-10", "rec-oct"
		return &DisclosureTypeDTO{DeadlineSummary: &DeadlineSummaryDTO{CurrentCycleLabel: &label, CurrentRecordID: &rec}}
	}

	viewer := &service{auth: &catalogAuthStub{perms: []string{"deadline.view"}}}
	item := newItem()
	viewer.limitCurrentRecordToDeadlineViewers(context.Background(), sub, item)
	if item.DeadlineSummary.CurrentRecordID == nil {
		t.Fatal("deadline.view users keep the record id")
	}

	proposer := &service{auth: &catalogAuthStub{perms: []string{"ad_hoc_alert.propose"}}}
	item = newItem()
	proposer.limitCurrentRecordToDeadlineViewers(context.Background(), sub, item)
	if item.DeadlineSummary.CurrentRecordID != nil {
		t.Fatal("catalog-only readers must not receive the record id")
	}
	if item.DeadlineSummary.CurrentCycleLabel == nil {
		t.Fatal("cycle label stays available to catalog readers")
	}

	proposer.limitCurrentRecordToDeadlineViewers(context.Background(), sub, &DisclosureTypeDTO{})
	proposer.limitCurrentRecordToDeadlineViewers(context.Background(), sub, nil)
}
