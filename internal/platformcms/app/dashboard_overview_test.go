package app

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/cobo/cobo_iam_services/internal/subscription/companyplan"
)

type fakeDashboardRepo struct {
	mu    sync.Mutex
	calls map[string]int

	companies     DashboardCompanies
	subscriptions []DashboardSubscriptionRow
	users         DashboardUsers
	templates     DashboardTemplates
	disclosures   DashboardDisclosures
	health        DashboardSystemHealth
	growth        DashboardGrowthCounts

	failSection  string
	blockSection string
	panicSection string
	lastToday    string
	// gate, when set, holds every repository call until it is closed.
	gate chan struct{}
}

func (f *fakeDashboardRepo) hit(ctx context.Context, section string) error {
	f.mu.Lock()
	if f.calls == nil {
		f.calls = map[string]int{}
	}
	f.calls[section]++
	gate := f.gate
	f.mu.Unlock()
	if gate != nil {
		<-gate
	}
	if f.panicSection == section {
		panic("boom")
	}
	if f.blockSection == section {
		<-ctx.Done()
		return ctx.Err()
	}
	if f.failSection == section {
		return errors.New("db down")
	}
	return nil
}

func (f *fakeDashboardRepo) callCount(section string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[section]
}

func (f *fakeDashboardRepo) CompanyStats(ctx context.Context, _ time.Time) (DashboardCompanies, error) {
	return f.companies, f.hit(ctx, DashboardSectionCompanies)
}

func (f *fakeDashboardRepo) SubscriptionRows(ctx context.Context, _ time.Time) ([]DashboardSubscriptionRow, error) {
	return f.subscriptions, f.hit(ctx, DashboardSectionSubscriptions)
}

func (f *fakeDashboardRepo) UserStats(ctx context.Context, _ time.Time) (DashboardUsers, error) {
	return f.users, f.hit(ctx, DashboardSectionUsers)
}

func (f *fakeDashboardRepo) TemplateStats(ctx context.Context, _ time.Time) (DashboardTemplates, error) {
	return f.templates, f.hit(ctx, DashboardSectionTemplates)
}

func (f *fakeDashboardRepo) DisclosureStats(ctx context.Context, today string, _ time.Time) (DashboardDisclosures, error) {
	f.mu.Lock()
	f.lastToday = today
	f.mu.Unlock()
	return f.disclosures, f.hit(ctx, DashboardSectionDisclosures)
}

func (f *fakeDashboardRepo) GrowthCounts(ctx context.Context, _ string, _ time.Time) (DashboardGrowthCounts, error) {
	return f.growth, f.hit(ctx, DashboardSectionGrowth)
}

func (f *fakeDashboardRepo) SystemHealthStats(ctx context.Context, _ time.Time, _ int) (DashboardSystemHealth, error) {
	return f.health, f.hit(ctx, DashboardSectionSystemHealth)
}

var allDashboardPerms = []string{"platform.cms.view", "rbac.manage", "system.settings", "cms.template.read"}

func dashboardClockAt(t time.Time) func() time.Time { return func() time.Time { return t } }

func TestDashboardOverview_ForbiddenSectionsSkipRepository(t *testing.T) {
	repo := &fakeDashboardRepo{companies: DashboardCompanies{Unverified: 3}}
	svc := NewDashboardOverviewService(repo)

	got := svc.Overview(context.Background(), DashboardOverviewRequest{UserID: "u1", Permissions: []string{"platform.cms.view"}})

	for _, key := range []string{DashboardSectionCompanies, DashboardSectionSubscriptions, DashboardSectionUsers, DashboardSectionTemplates, DashboardSectionSystemHealth, DashboardSectionGrowth} {
		if got.Sections[key].Status != DashboardStatusForbidden {
			t.Errorf("%s status = %q, want forbidden", key, got.Sections[key].Status)
		}
		if got.Sections[key].Data != nil {
			t.Errorf("%s leaked data for forbidden section", key)
		}
		if n := repo.callCount(key); n != 0 {
			t.Errorf("%s repository called %d times for forbidden section", key, n)
		}
	}
	if got.Sections[DashboardSectionDisclosures].Status != DashboardStatusOK {
		t.Fatalf("disclosures status = %q, want ok (platform.cms.view only)", got.Sections[DashboardSectionDisclosures].Status)
	}
	for _, item := range got.Attention.Items {
		if item.Code == "companies_unverified" {
			t.Fatal("attention leaked a count from a forbidden section")
		}
	}
}

func TestDashboardOverview_TemplateGateMirrorsCMSRead(t *testing.T) {
	for _, perm := range []string{"cms.template.read", "cms.template.write", "cms.template.activate", "cms.template.archive", "cms.template.config.write", "rbac.manage"} {
		if !DashboardSectionAllowed(DashboardSectionTemplates, []string{perm}) {
			t.Errorf("templates section should open with %s", perm)
		}
	}
	if DashboardSectionAllowed(DashboardSectionTemplates, []string{"platform.cms.view", "disclosure_type.publish"}) {
		t.Error("templates section must not open with platform.cms.view / legacy publish only")
	}
	if DashboardSectionAllowed(DashboardSectionSystemHealth, []string{"rbac.manage"}) {
		t.Error("system health requires system.settings")
	}
	if !DashboardSectionAllowed(DashboardSectionCompanies, []string{"SYSTEM.SETTINGS"}) {
		t.Error("permission match should be case-insensitive")
	}
}

func TestDashboardOverview_SectionErrorIsIsolated(t *testing.T) {
	repo := &fakeDashboardRepo{failSection: DashboardSectionUsers, panicSection: DashboardSectionTemplates}
	svc := NewDashboardOverviewService(repo)

	got := svc.Overview(context.Background(), DashboardOverviewRequest{UserID: "u1", Permissions: allDashboardPerms})

	if s := got.Sections[DashboardSectionUsers]; s.Status != DashboardStatusError || s.ErrorCode != "INTERNAL" || s.Data != nil {
		t.Fatalf("users = %+v, want error INTERNAL without data", s)
	}
	if s := got.Sections[DashboardSectionTemplates]; s.Status != DashboardStatusError {
		t.Fatalf("templates (panic) = %+v, want error", s)
	}
	for _, key := range []string{DashboardSectionCompanies, DashboardSectionSubscriptions, DashboardSectionDisclosures, DashboardSectionSystemHealth} {
		if got.Sections[key].Status != DashboardStatusOK {
			t.Errorf("%s status = %q, want ok", key, got.Sections[key].Status)
		}
	}
	// Failed sections are not cached: the next call retries them.
	svc.Overview(context.Background(), DashboardOverviewRequest{UserID: "u1", Permissions: allDashboardPerms})
	if n := repo.callCount(DashboardSectionUsers); n != 2 {
		t.Errorf("users repository calls = %d, want 2 (errors must not be cached)", n)
	}
	if n := repo.callCount(DashboardSectionCompanies); n != 1 {
		t.Errorf("companies repository calls = %d, want 1 (cached)", n)
	}
}

func TestDashboardOverview_SectionTimeout(t *testing.T) {
	repo := &fakeDashboardRepo{blockSection: DashboardSectionDisclosures}
	svc := NewDashboardOverviewService(repo).WithSectionTimeout(20 * time.Millisecond)

	got := svc.Overview(context.Background(), DashboardOverviewRequest{UserID: "u1", Permissions: allDashboardPerms})

	if s := got.Sections[DashboardSectionDisclosures]; s.Status != DashboardStatusError || s.ErrorCode != "TIMEOUT" {
		t.Fatalf("disclosures = %+v, want error TIMEOUT", s)
	}
	if got.Sections[DashboardSectionCompanies].Status != DashboardStatusOK {
		t.Fatal("other sections must still be ok")
	}
}

func TestDashboardOverview_CacheAndRefreshRateLimit(t *testing.T) {
	repo := &fakeDashboardRepo{}
	now := time.Date(2026, 10, 11, 3, 0, 0, 0, time.UTC)
	svc := NewDashboardOverviewService(repo).WithClock(func() time.Time { return now })
	req := DashboardOverviewRequest{UserID: "u1", Permissions: allDashboardPerms}

	if first := svc.Overview(context.Background(), req); first.Cached {
		t.Fatal("first call must not be cached")
	}
	now = now.Add(30 * time.Second)
	if second := svc.Overview(context.Background(), req); !second.Cached {
		t.Fatal("second call within TTL should be served from cache")
	}
	if n := repo.callCount(DashboardSectionCompanies); n != 1 {
		t.Fatalf("companies calls = %d, want 1", n)
	}

	refresh := DashboardOverviewRequest{UserID: "u1", Permissions: allDashboardPerms, Refresh: true}
	svc.Overview(context.Background(), refresh)
	if n := repo.callCount(DashboardSectionCompanies); n != 2 {
		t.Fatalf("refresh should bypass cache: calls = %d, want 2", n)
	}
	now = now.Add(5 * time.Second)
	svc.Overview(context.Background(), refresh)
	if n := repo.callCount(DashboardSectionCompanies); n != 2 {
		t.Fatalf("second refresh within 10s must use cache: calls = %d, want 2", n)
	}
	// Global limit: another user's refresh accepts data computed within the last 10s.
	svc.Overview(context.Background(), DashboardOverviewRequest{UserID: "u2", Permissions: allDashboardPerms, Refresh: true})
	if n := repo.callCount(DashboardSectionCompanies); n != 2 {
		t.Fatalf("u2 refresh within 10s of a recompute must reuse it: calls = %d, want 2", n)
	}
	now = now.Add(10 * time.Second)
	svc.Overview(context.Background(), DashboardOverviewRequest{UserID: "u3", Permissions: allDashboardPerms, Refresh: true})
	if n := repo.callCount(DashboardSectionCompanies); n != 3 {
		t.Fatalf("u3 refresh 15s after the recompute should bypass cache: calls = %d, want 3", n)
	}

	now = now.Add(61 * time.Second)
	svc.Overview(context.Background(), req)
	if n := repo.callCount(DashboardSectionCompanies); n != 4 {
		t.Fatalf("expired cache should recompute: calls = %d, want 4", n)
	}
}

func TestDashboardOverview_BusinessDateIsHCM(t *testing.T) {
	repo := &fakeDashboardRepo{}
	// 17:30 UTC on Oct 11 is already Oct 12 in Ho Chi Minh City.
	svc := NewDashboardOverviewService(repo).WithClock(dashboardClockAt(time.Date(2026, 10, 11, 17, 30, 0, 0, time.UTC)))
	svc.Overview(context.Background(), DashboardOverviewRequest{UserID: "u1", Permissions: allDashboardPerms})
	if repo.lastToday != "2026-10-12" {
		t.Fatalf("business date = %q, want 2026-10-12", repo.lastToday)
	}
}

func TestDashboardOverview_AttentionOrderAndThresholds(t *testing.T) {
	repo := &fakeDashboardRepo{
		companies:   DashboardCompanies{Unverified: 6, Suspended: 1},
		templates:   DashboardTemplates{OpenDrafts: 2, ActiveMissingWorkflow: 0},
		disclosures: DashboardDisclosures{Overdue: 7},
		health: DashboardSystemHealth{
			EmailFailed24h:  5,
			HolidayCalendar: DashboardHolidayCalendar{CurrentYear: true, NextYear: false},
		},
	}
	// October: missing next-year calendar is not flagged yet.
	svc := NewDashboardOverviewService(repo).WithClock(dashboardClockAt(time.Date(2026, 10, 11, 3, 0, 0, 0, time.UTC)))
	got := svc.Overview(context.Background(), DashboardOverviewRequest{UserID: "u1", Permissions: allDashboardPerms})

	want := []DashboardAttentionItem{
		{Code: "disclosures_overdue", Severity: "critical", Count: 7},
		{Code: "email_failed_24h", Severity: "critical", Count: 5},
		{Code: "companies_unverified", Severity: "warning", Count: 6},
		{Code: "companies_suspended", Severity: "info", Count: 1},
		{Code: "templates_open_draft", Severity: "info", Count: 2},
	}
	if len(got.Attention.Items) != len(want) {
		t.Fatalf("attention = %+v, want %+v", got.Attention.Items, want)
	}
	for i := range want {
		if got.Attention.Items[i] != want[i] {
			t.Errorf("attention[%d] = %+v, want %+v", i, got.Attention.Items[i], want[i])
		}
	}

	// From November the next-year calendar is required.
	nov := NewDashboardOverviewService(repo).WithClock(dashboardClockAt(time.Date(2026, 11, 2, 3, 0, 0, 0, time.UTC)))
	got = nov.Overview(context.Background(), DashboardOverviewRequest{UserID: "u1", Permissions: allDashboardPerms})
	found := false
	for _, item := range got.Attention.Items {
		if item.Code == "holiday_calendar_missing" && item.Count == 1 && item.Severity == "warning" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected holiday_calendar_missing in November, got %+v", got.Attention.Items)
	}
}

func TestDashboardOverview_DisclosuresTopListNeverNull(t *testing.T) {
	svc := NewDashboardOverviewService(&fakeDashboardRepo{})
	got := svc.Overview(context.Background(), DashboardOverviewRequest{UserID: "u1", Permissions: []string{"platform.cms.view"}})
	d, ok := got.Sections[DashboardSectionDisclosures].Data.(DashboardDisclosures)
	if !ok || d.TopOverdueCompanies == nil {
		t.Fatalf("top_overdue_companies must be [] not null: %+v", got.Sections[DashboardSectionDisclosures])
	}
}

func TestSummarizeDashboardSubscriptions(t *testing.T) {
	now := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	at := func(days int) *time.Time { t := now.AddDate(0, 0, days); return &t }
	plan := func(id, company string, code companyplan.PlanCode, status companyplan.PlanStatus, fromDays int, expires *time.Time) *companyplan.CompanyPlan {
		return &companyplan.CompanyPlan{ID: id, CompanyID: company, Code: code, Status: status, EffectiveFrom: now.AddDate(0, 0, fromDays), ExpiresAt: expires}
	}
	rows := []DashboardSubscriptionRow{
		{CompanyID: "c_none"}, // no occupying plan
		{CompanyID: "c_premium", Plan: plan("p1", "c_premium", companyplan.PlanCodePremium, companyplan.PlanStatusActive, -30, nil)},
		{CompanyID: "c_trial", Plan: plan("p2", "c_trial", companyplan.PlanCodeEnterprise, companyplan.PlanStatusTrial, -3, at(5))},
		// Expires in 10 days but renewed by a plan starting at expiry → not expiring.
		{CompanyID: "c_renewed", Plan: plan("p3", "c_renewed", companyplan.PlanCodePremium, companyplan.PlanStatusActive, -300, at(10))},
		{CompanyID: "c_renewed", Plan: plan("p4", "c_renewed", companyplan.PlanCodePremium, companyplan.PlanStatusActive, 10, at(375))},
		// Expires in 20 days → outside the 14-day window.
		{CompanyID: "c_later", Plan: plan("p5", "c_later", companyplan.PlanCodeFree, companyplan.PlanStatusActive, -10, at(20))},
		// Only a future plan → no effective plan today.
		{CompanyID: "c_future", Plan: plan("p6", "c_future", companyplan.PlanCodePremium, companyplan.PlanStatusActive, 3, nil)},
	}

	got := SummarizeDashboardSubscriptions(rows, now)

	if got.ByPlan["PREMIUM"] != 2 || got.ByPlan["ENTERPRISE"] != 1 || got.ByPlan["FREE"] != 1 {
		t.Errorf("by_plan = %+v", got.ByPlan)
	}
	if got.NoPlan != 2 {
		t.Errorf("no_plan = %d, want 2 (c_none, c_future)", got.NoPlan)
	}
	if got.Trial != 1 {
		t.Errorf("trial = %d, want 1", got.Trial)
	}
	if got.Expiring14d != 1 {
		t.Errorf("expiring_14d = %d, want 1 (c_trial only)", got.Expiring14d)
	}
}

func TestDashboardOverview_GeneratedAtIsOldestServedData(t *testing.T) {
	repo := &fakeDashboardRepo{}
	t0 := time.Date(2026, 10, 11, 3, 0, 0, 0, time.UTC)
	now := t0
	svc := NewDashboardOverviewService(repo).WithClock(func() time.Time { return now })
	svc.Overview(context.Background(), DashboardOverviewRequest{UserID: "u1", Permissions: []string{"platform.cms.view"}})

	now = t0.Add(40 * time.Second)
	got := svc.Overview(context.Background(), DashboardOverviewRequest{UserID: "u1", Permissions: []string{"platform.cms.view"}})
	if got.GeneratedAt != t0.Format(time.RFC3339) {
		t.Fatalf("generated_at = %s, want %s (time the cached data was computed)", got.GeneratedAt, t0.Format(time.RFC3339))
	}
}

func TestDashboardOverview_ConcurrentCallersShareOneComputation(t *testing.T) {
	repo := &fakeDashboardRepo{gate: make(chan struct{})}
	svc := NewDashboardOverviewService(repo)
	req := DashboardOverviewRequest{UserID: "u1", Permissions: []string{"platform.cms.view"}}

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got := svc.Overview(context.Background(), req)
			if got.Sections[DashboardSectionDisclosures].Status != DashboardStatusOK {
				t.Errorf("disclosures = %+v, want ok", got.Sections[DashboardSectionDisclosures])
			}
		}()
	}
	// No network/state signal to wait on: give all callers time to join the in-flight computation.
	time.Sleep(50 * time.Millisecond)
	close(repo.gate)
	wg.Wait()
	if n := repo.callCount(DashboardSectionDisclosures); n != 1 {
		t.Fatalf("disclosures computed %d times for 5 concurrent callers, want 1", n)
	}
}

func TestDashboardOverview_CallerCancelStillCachesResult(t *testing.T) {
	repo := &fakeDashboardRepo{gate: make(chan struct{})}
	svc := NewDashboardOverviewService(repo)
	req := DashboardOverviewRequest{UserID: "u1", Permissions: []string{"platform.cms.view"}}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan DashboardOverview, 1)
	go func() { done <- svc.Overview(ctx, req) }()
	time.Sleep(20 * time.Millisecond) // let the computation start
	cancel()
	if got := <-done; got.Sections[DashboardSectionDisclosures].Status != DashboardStatusError {
		t.Fatalf("cancelled caller should get an error section, got %+v", got.Sections[DashboardSectionDisclosures])
	}
	close(repo.gate)

	// The detached computation finishes and is cached: the next caller does not recompute.
	deadline := time.Now().Add(time.Second)
	for {
		got := svc.Overview(context.Background(), req)
		if got.Cached {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("result of the cancelled request was never cached")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if n := repo.callCount(DashboardSectionDisclosures); n != 1 {
		t.Fatalf("disclosures computed %d times, want 1", n)
	}
}

func TestBuildDashboardGrowth_DenseSeriesEndingToday(t *testing.T) {
	loc := time.FixedZone("ICT", 7*60*60)
	today := time.Date(2026, 10, 11, 9, 0, 0, 0, loc)
	got := BuildDashboardGrowth(DashboardGrowthCounts{
		Companies:   []DashboardDailyCount{{Date: "2026-10-11", Count: 2}},
		Users:       []DashboardDailyCount{{Date: "2026-09-12", Count: 1}, {Date: "2026-10-01", Count: 3}},
		Disclosures: []DashboardDailyCount{{Date: "2026-09-11", Count: 9}}, // before the window
	}, today, 30)

	if len(got.Days) != 30 || got.Days[0].Date != "2026-09-12" || got.Days[29].Date != "2026-10-11" {
		t.Fatalf("window = %s..%s (%d days), want 2026-09-12..2026-10-11", got.Days[0].Date, got.Days[len(got.Days)-1].Date, len(got.Days))
	}
	if got.Days[29].Companies != 2 || got.Days[0].Users != 1 {
		t.Fatalf("counts not placed on their day: %+v / %+v", got.Days[29], got.Days[0])
	}
	if got.Totals != (DashboardGrowthTotals{Companies: 2, Users: 4, Disclosures: 0}) {
		t.Fatalf("totals = %+v", got.Totals)
	}
}

func TestDashboardOverview_GrowthSectionGate(t *testing.T) {
	repo := &fakeDashboardRepo{}
	svc := NewDashboardOverviewService(repo)
	got := svc.Overview(context.Background(), DashboardOverviewRequest{UserID: "u1", Permissions: []string{"platform.cms.view", "system.settings"}})
	growth, ok := got.Sections[DashboardSectionGrowth].Data.(DashboardGrowth)
	if !ok || len(growth.Days) != DashboardGrowthDays {
		t.Fatalf("growth = %+v, want ok with %d days", got.Sections[DashboardSectionGrowth], DashboardGrowthDays)
	}
	if DashboardSectionAllowed(DashboardSectionGrowth, []string{"platform.cms.view", "cms.template.read"}) {
		t.Fatal("growth must require rbac.manage or system.settings")
	}
}

// Company-directory sections follow the companies screen: platform operator only.
func TestDashboardOverview_CompanySectionsFollowCompaniesScreenGate(t *testing.T) {
	repo := &fakeDashboardRepo{companies: DashboardCompanies{Unverified: 4}}
	svc := NewDashboardOverviewService(repo)

	// cms_operator as seeded: platform.cms.view + rbac.manage, no admin.membership.invite.
	got := svc.Overview(context.Background(), DashboardOverviewRequest{UserID: "u1", Permissions: []string{"platform.cms.view", "rbac.manage"}})
	for _, key := range []string{DashboardSectionCompanies, DashboardSectionSubscriptions, DashboardSectionGrowth} {
		if got.Sections[key].Status != DashboardStatusOK {
			t.Errorf("%s = %+v, want ok for a platform operator", key, got.Sections[key])
		}
	}

	// A tenant permission such as admin.membership.invite does not open them.
	got = svc.Overview(context.Background(), DashboardOverviewRequest{UserID: "u2", Permissions: []string{"platform.cms.view", "admin.membership.invite"}})
	for _, key := range []string{DashboardSectionCompanies, DashboardSectionSubscriptions, DashboardSectionGrowth} {
		if got.Sections[key].Status != DashboardStatusForbidden {
			t.Errorf("%s = %+v, want forbidden without rbac.manage|system.settings", key, got.Sections[key])
		}
	}
	for _, item := range got.Attention.Items {
		if item.Code == "companies_unverified" {
			t.Fatal("attention leaked companies_unverified")
		}
	}
}
