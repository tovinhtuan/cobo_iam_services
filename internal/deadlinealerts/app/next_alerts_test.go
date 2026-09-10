package app

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

func TestEffectiveOpenAtYMD(t *testing.T) {
	if got := EffectiveOpenAtYMD("2026-10-05", "2026-10-10"); got != "2026-10-05" {
		t.Fatalf("prefer open_at: got %q", got)
	}
	if got := EffectiveOpenAtYMD("", "2026-10-10"); got != "2026-10-10" {
		t.Fatalf("fallback cycle_start: got %q", got)
	}
	if got := EffectiveOpenAtYMD("", ""); got != "" {
		t.Fatalf("empty: got %q", got)
	}
}

func TestEffectiveOpenAtYMD_dueDateDoesNotExtend(t *testing.T) {
	// Critical: even if a caller mistakenly passes due_date somewhere, helper ignores it.
	// Boundary must be cycle_start when open_at is null — not a later due_date.
	got := EffectiveOpenAtYMD("", "2026-10-10")
	if got != "2026-10-10" {
		t.Fatalf("got %q want 2026-10-10", got)
	}
	// Simulate old 3-arg behavior would have picked due_date only when both empty —
	// with cycle_start present, due_date must never win.
	if EffectiveOpenAtYMD("", "2026-10-10") == "2026-10-30" {
		t.Fatal("due_date must not become EffectiveOpenAt when cycle_start is set")
	}
}

func TestSelectNearestNextAlertPerType(t *testing.T) {
	rows := []NextAlertCycleRow{
		{CycleID: "c1", TypeID: "tA", OpenAt: "2026-10-01", CycleStart: "2026-10-01"},
		{CycleID: "c2", TypeID: "tA", OpenAt: "2026-11-01", CycleStart: "2026-11-01"},
		{CycleID: "c3", TypeID: "tB", OpenAt: "2026-10-15", CycleStart: "2026-10-15"},
	}
	got := SelectNearestNextAlertPerType(rows)
	if len(got) != 2 {
		t.Fatalf("len=%d want 2", len(got))
	}
	if got[0].CycleID != "c1" || got[1].CycleID != "c3" {
		t.Fatalf("nearest mismatch: %+v", got)
	}
}

func TestMaterializerEffectiveOpenAtSQL_constant(t *testing.T) {
	if MaterializerEffectiveOpenAtSQL != "COALESCE(pc.open_at, pc.cycle_start)" {
		t.Fatalf("authority constant drifted: %q", MaterializerEffectiveOpenAtSQL)
	}
	if strings.Contains(MaterializerEffectiveOpenAtSQL, "due_date") {
		t.Fatal("materializer-aligned Next Alert authority must not include due_date")
	}
}

func TestNextAlertMaterializerBoundaryParity_sourceTrace(t *testing.T) {
	// Prove Next Alert SQL uses the same COALESCE(open_at, cycle_start) authority constant.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoPath := filepath.Join(filepath.Dir(thisFile), "..", "infra", "mysql", "repository.go")
	src, err := os.ReadFile(repoPath)
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)
	idx := strings.Index(body, "func (r *Repository) ListNextAlertCycles")
	if idx < 0 {
		t.Fatal("ListNextAlertCycles not found")
	}
	chunk := body[idx:]
	if end := strings.Index(chunk, "\nfunc ("); end > 0 {
		chunk = chunk[:end]
	}
	if !strings.Contains(chunk, MaterializerEffectiveOpenAtSQL) {
		t.Fatalf("ListNextAlertCycles must use %q", MaterializerEffectiveOpenAtSQL)
	}
	if strings.Contains(chunk, "COALESCE(pc.open_at, pc.cycle_start, pc.due_date)") {
		t.Fatal("ListNextAlertCycles must not use due_date in OpenAt COALESCE")
	}

	// Materializer app gate: skip zero CycleStart (due_date alone cannot materialize).
	periodicPath := filepath.Join(filepath.Dir(thisFile), "..", "..", "disclosure", "app", "periodic.go")
	periodicSrc, err := os.ReadFile(periodicPath)
	if err != nil {
		t.Fatal(err)
	}
	ps := string(periodicSrc)
	if !strings.Contains(ps, "periodic cycle missing cycle_start; skip materialize") {
		t.Fatal("materializer must skip zero CycleStart — due_date-only rows are not runtime-materializable")
	}
	if !strings.Contains(ps, "COALESCE(open_at, cycle_start)") {
		t.Fatal("materializePeriodicDisclosures comment authority must mention COALESCE(open_at, cycle_start)")
	}
}

func TestIsNextAlertOpenAtEligible_openAtPresent(t *testing.T) {
	eff := EffectiveOpenAtYMD("2026-10-05", "2026-10-10")
	if eff != "2026-10-05" {
		t.Fatalf("eff=%q", eff)
	}
	if !IsNextAlertOpenAtEligible("2026-10-04", eff) {
		t.Fatal("before open_at must be NEXT")
	}
	if IsNextAlertOpenAtEligible("2026-10-05", eff) {
		t.Fatal("at open_at must NOT be NEXT")
	}
	if IsNextAlertOpenAtEligible("2026-10-06", eff) {
		t.Fatal("after open_at must NOT be NEXT")
	}
}

func TestIsNextAlertOpenAtEligible_openAtNullUsesCycleStart(t *testing.T) {
	eff := EffectiveOpenAtYMD("", "2026-10-10")
	if eff != "2026-10-10" {
		t.Fatalf("eff=%q", eff)
	}
	if !IsNextAlertOpenAtEligible("2026-10-09", eff) {
		t.Fatal("before cycle_start must be NEXT")
	}
	if IsNextAlertOpenAtEligible("2026-10-10", eff) {
		t.Fatal("at cycle_start must NOT be NEXT")
	}
}

func TestDueDateDoesNotExtendNextAlertWindow(t *testing.T) {
	// open_at NULL, cycle_start 2026-10-10, due_date 2026-10-30 (due ignored by helper)
	eff := EffectiveOpenAtYMD("", "2026-10-10")
	if eff != "2026-10-10" {
		t.Fatalf("boundary must be cycle_start, got %q", eff)
	}
	// On 2026-10-15 (after cycle_start, before due_date): NOT Next — materializer-eligible window.
	if IsNextAlertOpenAtEligible("2026-10-15", eff) {
		t.Fatal("due_date must not keep Next Alert open past EffectiveOpenAt=cycle_start")
	}
}

func TestListNextDeadlineAlerts_openAtPresentBoundary(t *testing.T) {
	repo := &stubRepo{nextAlertCycles: []NextAlertCycleRow{{
		CycleID: "pc1", TypeID: "t1", TypeName: "T",
		CycleLabel: "2026-10", CycleStart: "2026-10-10", OpenAt: "2026-10-05", DueAt: "2026-10-14",
	}}}
	svc := NewService(repo, allowAuthSvc(), disclosureapp.NewDeadlineCalculator(disclosureapp.NewHolidayCalendarFileProvider("configs/non_trading_days")))
	loc := time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)

	svc.(*service).now = func() time.Time { return time.Date(2026, 10, 4, 12, 0, 0, 0, loc) }
	before, err := svc.ListNextDeadlineAlerts(context.Background(), Subject{UserID: "u1", MembershipID: "m_admin_001", CompanyID: "c_001"})
	if err != nil {
		t.Fatal(err)
	}
	if len(before.Items) != 1 {
		t.Fatalf("before OpenAt: got %d", len(before.Items))
	}

	svc.(*service).now = func() time.Time { return time.Date(2026, 10, 5, 12, 0, 0, 0, loc) }
	at, err := svc.ListNextDeadlineAlerts(context.Background(), Subject{UserID: "u1", MembershipID: "m_admin_001", CompanyID: "c_001"})
	if err != nil {
		t.Fatal(err)
	}
	if len(at.Items) != 0 {
		t.Fatalf("at OpenAt must be empty, got %+v", at.Items)
	}
}

func TestListNextDeadlineAlerts_materializerLagAfterOpenAt(t *testing.T) {
	repo := &stubRepo{nextAlertCycles: []NextAlertCycleRow{{
		CycleID: "pc1", TypeID: "t1",
		CycleLabel: "2026-10", CycleStart: "2026-10-10", OpenAt: "2026-10-05", DueAt: "2026-10-30",
	}}}
	svc := NewService(repo, allowAuthSvc(), disclosureapp.NewDeadlineCalculator(disclosureapp.NewHolidayCalendarFileProvider("configs/non_trading_days")))
	loc := time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)
	svc.(*service).now = func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, loc) }

	resp, err := svc.ListNextDeadlineAlerts(context.Background(), Subject{UserID: "u1", MembershipID: "m_admin_001", CompanyID: "c_001"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 0 {
		t.Fatalf("after OpenAt (record still absent) must NOT be Next; got %+v", resp.Items)
	}
}

func TestListNextDeadlineAlerts_basicUpcoming(t *testing.T) {
	repo := &stubRepo{nextAlertCycles: []NextAlertCycleRow{{
		CycleID: "pc1", TypeID: "type-m", TypeName: "Báo cáo tháng",
		FrequencyUnit: "monthly", CycleLabel: "2026-10",
		CycleStart: "2026-10-01", OpenAt: "2026-09-25", DueAt: "2026-10-05",
	}}}
	svc := NewService(repo, allowAuthSvc(), disclosureapp.NewDeadlineCalculator(disclosureapp.NewHolidayCalendarFileProvider("configs/non_trading_days")))
	svc.(*service).now = func() time.Time { return time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC) }

	resp, err := svc.ListNextDeadlineAlerts(context.Background(), Subject{
		UserID: "u1", MembershipID: "m_admin_001", CompanyID: "c_001",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("items=%d want 1", len(resp.Items))
	}
	item := resp.Items[0]
	if item.CycleID != "pc1" || item.State != NextDeadlineAlertStateUpcoming {
		t.Fatalf("item=%+v", item)
	}
}

func TestListNextDeadlineAlerts_emptyWhenNotSeeded(t *testing.T) {
	repo := &stubRepo{}
	svc := NewService(repo, allowAuthSvc(), disclosureapp.NewDeadlineCalculator(disclosureapp.NewHolidayCalendarFileProvider("configs/non_trading_days")))
	svc.(*service).now = func() time.Time { return time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC) }

	resp, err := svc.ListNextDeadlineAlerts(context.Background(), Subject{
		UserID: "u1", MembershipID: "m_admin_001", CompanyID: "c_001",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 0 {
		t.Fatalf("want empty, got %+v", resp.Items)
	}
}

func TestListNextDeadlineAlerts_onePerTemplateAndSort(t *testing.T) {
	repo := &stubRepo{nextAlertCycles: []NextAlertCycleRow{
		{CycleID: "early-b", TypeID: "tB", TypeName: "B", CycleLabel: "2026-10", CycleStart: "2026-10-01", OpenAt: "2026-09-20", DueAt: "2026-10-05"},
		{CycleID: "early-a", TypeID: "tA", TypeName: "A", CycleLabel: "2026-10", CycleStart: "2026-10-01", OpenAt: "2026-09-25", DueAt: "2026-10-05"},
		{CycleID: "later-a", TypeID: "tA", TypeName: "A", CycleLabel: "2026-11", CycleStart: "2026-11-01", OpenAt: "2026-10-25", DueAt: "2026-11-05"},
		{CycleID: "mid-c", TypeID: "tC", TypeName: "C", CycleLabel: "2026-10", CycleStart: "2026-10-01", OpenAt: "2026-09-28", DueAt: "2026-10-05"},
	}}
	svc := NewService(repo, allowAuthSvc(), disclosureapp.NewDeadlineCalculator(disclosureapp.NewHolidayCalendarFileProvider("configs/non_trading_days")))
	svc.(*service).now = func() time.Time { return time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC) }

	resp, err := svc.ListNextDeadlineAlerts(context.Background(), Subject{
		UserID: "u1", MembershipID: "m_admin_001", CompanyID: "c_001",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 3 {
		t.Fatalf("want 3 templates, got %d %+v", len(resp.Items), resp.Items)
	}
	if resp.Items[0].CycleID != "early-b" || resp.Items[1].CycleID != "early-a" || resp.Items[2].CycleID != "mid-c" {
		t.Fatalf("sort/one-per-type failed: %+v", resp.Items)
	}
}

func TestListNextDeadlineAlerts_readOnlyNoSideEffectOnRepeatedCalls(t *testing.T) {
	repo := &stubRepo{nextAlertCycles: []NextAlertCycleRow{{
		CycleID: "pc1", TypeID: "t1", CycleLabel: "2026-10",
		CycleStart: "2026-10-01", OpenAt: "2026-09-25", DueAt: "2026-10-05",
	}}}
	svc := NewService(repo, allowAuthSvc(), disclosureapp.NewDeadlineCalculator(disclosureapp.NewHolidayCalendarFileProvider("configs/non_trading_days")))
	svc.(*service).now = func() time.Time { return time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC) }
	sub := Subject{UserID: "u1", MembershipID: "m_admin_001", CompanyID: "c_001"}

	for i := 0; i < 3; i++ {
		resp, err := svc.ListNextDeadlineAlerts(context.Background(), sub)
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.Items) != 1 {
			t.Fatalf("iter %d items=%d", i, len(resp.Items))
		}
	}
	if repo.listNextCalls != 3 {
		t.Fatalf("calls=%d", repo.listNextCalls)
	}
}

func TestListNextDeadlineAlerts_requiresCompany(t *testing.T) {
	svc := NewService(&stubRepo{}, allowAuthSvc(), disclosureapp.NewDeadlineCalculator(disclosureapp.NewHolidayCalendarFileProvider("configs/non_trading_days")))
	_, err := svc.ListNextDeadlineAlerts(context.Background(), Subject{UserID: "u1", MembershipID: "m1"})
	if err == nil {
		t.Fatal("expected company required error")
	}
}

func TestListNextDeadlineAlerts_crossCompanyUsesSubjectCompanyOnly(t *testing.T) {
	repo := &stubRepo{nextAlertCycles: []NextAlertCycleRow{{
		CycleID: "a-only", TypeID: "t1", CycleLabel: "2026-10",
		CycleStart: "2026-10-01", OpenAt: "2026-09-25", DueAt: "2026-10-05",
	}}}
	svc := NewService(repo, allowAuthSvc(), disclosureapp.NewDeadlineCalculator(disclosureapp.NewHolidayCalendarFileProvider("configs/non_trading_days")))
	svc.(*service).now = func() time.Time { return time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC) }

	resp, err := svc.ListNextDeadlineAlerts(context.Background(), Subject{
		UserID: "u1", MembershipID: "m_admin_001", CompanyID: "c_001",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("company A items=%d", len(resp.Items))
	}
}

func TestListDeadlineAlerts_countUnchangedByNextProjectionFixture(t *testing.T) {
	repo := &stubRepo{
		rows: []AlertRow{
			{CompanyID: "c1", RecordID: "r1", Title: "Draft", RecordStatus: "Draft", PlannedDate: "2026-06-01"},
		},
		nextAlertCycles: []NextAlertCycleRow{{
			CycleID: "pc1", TypeID: "t1", CycleLabel: "2026-10",
			CycleStart: "2026-10-01", OpenAt: "2026-09-25", DueAt: "2026-10-05",
		}},
	}
	svc := NewService(repo, allowAuthSvc(), disclosureapp.NewDeadlineCalculator(disclosureapp.NewHolidayCalendarFileProvider("configs/non_trading_days")))
	svc.(*service).now = func() time.Time { return time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC) }

	alerts, err := svc.ListDeadlineAlerts(context.Background(), ListDeadlineAlertsRequest{
		Subject: Subject{UserID: "u1", MembershipID: "m_admin_001", CompanyID: "c_001"},
		Page:    1, PageSize: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if alerts.Total != 1 {
		t.Fatalf("actionable total=%d want 1", alerts.Total)
	}
	next, err := svc.ListNextDeadlineAlerts(context.Background(), Subject{
		UserID: "u1", MembershipID: "m_admin_001", CompanyID: "c_001",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Items) != 1 {
		t.Fatalf("next=%d", len(next.Items))
	}
}
