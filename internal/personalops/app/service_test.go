package app_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	inappapp "github.com/cobo/cobo_iam_services/internal/inappnotification/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	personalopsapp "github.com/cobo/cobo_iam_services/internal/personalops/app"
	"github.com/cobo/cobo_iam_services/internal/personalops/domain"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type fakeIdentity struct {
	user *iamapp.AuthenticatedUser
	err  error
}

func (f fakeIdentity) GetByUserID(context.Context, string) (*iamapp.AuthenticatedUser, error) {
	return f.user, f.err
}

type fakeMembers struct {
	items []caapp.MembershipView
	roles map[string][]string
	titles map[string][]string
	depts map[string][]caapp.DepartmentView
}

func (f fakeMembers) GetMembershipsByUser(context.Context, string) ([]caapp.MembershipView, error) {
	return f.items, nil
}
func (fakeMembers) GetActiveMembership(context.Context, string, string) (*caapp.MembershipView, error) {
	return nil, nil
}
func (f fakeMembers) GetMembershipRoles(_ context.Context, membershipID string) ([]string, error) {
	return f.roles[membershipID], nil
}
func (f fakeMembers) GetMembershipDepartments(_ context.Context, membershipID string) ([]caapp.DepartmentView, error) {
	return f.depts[membershipID], nil
}
func (f fakeMembers) GetMembershipTitles(_ context.Context, membershipID string) ([]string, error) {
	return f.titles[membershipID], nil
}

type fakeMine struct {
	records []personalopsapp.MineRecord
	tasks   []personalopsapp.MineTask
	adhoc   map[string]string
}

func (f fakeMine) ListMineRecords(context.Context, []string) ([]personalopsapp.MineRecord, error) {
	return f.records, nil
}
func (f fakeMine) ListMineOpenTasks(context.Context, []string, int) ([]personalopsapp.MineTask, error) {
	return f.tasks, nil
}
func (f fakeMine) ListApprovedAdHocDues(context.Context, []string) (map[string]string, error) {
	if f.adhoc == nil {
		return map[string]string{}, nil
	}
	return f.adhoc, nil
}

type fakeAuth struct {
	byMembership map[string]*authapp.EffectiveAccessSummary
}

func (f fakeAuth) GetEffectiveAccess(_ context.Context, membershipID, _ string) (*authapp.EffectiveAccessSummary, error) {
	if s, ok := f.byMembership[membershipID]; ok {
		return s, nil
	}
	return &authapp.EffectiveAccessSummary{Permissions: []string{"disclosure.view"}}, nil
}

type fakeInApp struct {
	items []inappapp.InAppNotification
}

func (f fakeInApp) List(context.Context, string, string) ([]inappapp.InAppNotification, error) {
	return f.items, nil
}

func TestGetOperationalOverview_requiresUser(t *testing.T) {
	svc := personalopsapp.NewService(fakeMembers{}, personalopsapp.EmptyMineRepository{}, fakeIdentity{
		user: &iamapp.AuthenticatedUser{UserID: "u1", FullName: "A", LoginID: "a@x.com"},
	}, nil, nil)
	_, err := svc.GetOperationalOverview(context.Background(), personalopsapp.Subject{})
	if err == nil {
		t.Fatal("expected auth error")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusUnauthorized {
		t.Fatalf("want 401, got %#v", err)
	}
}

func TestGetOperationalOverview_mineOnlyNoCompanyWide(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	members := fakeMembers{
		items: []caapp.MembershipView{
			{MembershipID: "m1", UserID: "u1", CompanyID: "c1", CompanyName: "Co One", Status: "active"},
		},
		roles:  map[string][]string{"m1": {"focal"}},
		titles: map[string][]string{"m1": {"NV"}},
		depts:  map[string][]caapp.DepartmentView{},
	}
	mine := fakeMine{
		records: []personalopsapp.MineRecord{
			{CompanyID: "c1", CompanyName: "Co One", MembershipID: "m1", RecordID: "r1", Title: "Mine overdue", RecordStatus: "in_progress", PlannedDate: "2026-07-01", MatchedViaTask: true},
			{CompanyID: "c1", CompanyName: "Co One", MembershipID: "m1", RecordID: "r2", Title: "Mine upcoming", RecordStatus: "in_progress", PlannedDate: "2026-07-20", MatchedViaAsgn: true},
		},
		tasks: []personalopsapp.MineTask{
			{TaskID: "t1", CompanyID: "c1", CompanyName: "Co One", MembershipID: "m1", RecordID: "r1", Title: "Mine overdue", StepCode: "review", TaskStatus: "pending", PlannedDate: "2026-07-01"},
		},
	}
	auth := fakeAuth{byMembership: map[string]*authapp.EffectiveAccessSummary{
		"m1": {Permissions: []string{"rbac.manage", "disclosure.view"}, DataScope: authapp.EffectiveDataScope{HasCompanyWideAccess: true}},
	}}
	svc := personalopsapp.NewService(members, mine, fakeIdentity{
		user: &iamapp.AuthenticatedUser{UserID: "u1", FullName: "Tuấn", LoginID: "t@x.com", SubscriptionTier: "premium"},
	}, auth, fakeInApp{}, personalopsapp.WithClock(fixedClock{t: now}), personalopsapp.WithLocation(time.UTC))

	resp, err := svc.GetOperationalOverview(context.Background(), personalopsapp.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Kpis.LinkedCompanies.Value == nil || *resp.Kpis.LinkedCompanies.Value != 1 {
		t.Fatalf("linked=%v", resp.Kpis.LinkedCompanies)
	}
	if resp.Kpis.ActiveRoles.Value == nil || *resp.Kpis.ActiveRoles.Value != 1 {
		t.Fatalf("roles=%v", resp.Kpis.ActiveRoles)
	}
	if resp.Kpis.AssignedAlerts.Accuracy != "exact" || resp.Kpis.AssignedAlerts.Value == nil || *resp.Kpis.AssignedAlerts.Value != 2 {
		t.Fatalf("assigned=%v", resp.Kpis.AssignedAlerts)
	}
	if resp.Kpis.OverdueAlerts.Value == nil || *resp.Kpis.OverdueAlerts.Value != 1 {
		t.Fatalf("overdue=%v want 1 (mine only)", resp.Kpis.OverdueAlerts)
	}
	if len(resp.MyTasks) != 1 || resp.MyTasks[0].AlertID != "r1" {
		t.Fatalf("tasks=%v", resp.MyTasks)
	}
	if resp.MyTasks[0].Action.Href != "/app/deadlines/r1" {
		t.Fatalf("href=%s", resp.MyTasks[0].Action.Href)
	}
	if len(resp.CompanyOverviews) != 1 {
		t.Fatalf("overviews=%d", len(resp.CompanyOverviews))
	}
	if resp.CompanyOverviews[0].OnTimeRate.Accuracy != "unavailable" {
		t.Fatalf("on_time should be unavailable")
	}
	if resp.CompanyOverviews[0].OnTimeRate.Reason == nil || *resp.CompanyOverviews[0].OnTimeRate.Reason != "no_completed_items_with_due_and_outcome" {
		t.Fatalf("on_time reason=%v", resp.CompanyOverviews[0].OnTimeRate.Reason)
	}
	if !containsWarningCode(resp.Meta.Warnings, "OUTCOME_TIMESTAMP_FORWARD_ONLY") {
		t.Fatalf("expected FORWARD_ONLY warning, got %#v", resp.Meta.Warnings)
	}
	if !containsWarningCode(resp.Meta.Warnings, "ON_TIME_RATE_SAMPLE_EMPTY") {
		t.Fatalf("expected SAMPLE_EMPTY warning, got %#v", resp.Meta.Warnings)
	}
	if resp.AdminScopes[0].CanManage == nil || !*resp.AdminScopes[0].CanManage {
		t.Fatalf("admin manage expected true from rbac.manage, got %#v", resp.AdminScopes[0])
	}
	// Ensure empty arrays are never null-ish via JSON helpers later; here slices non-nil.
	if resp.RoleAssignments == nil || resp.Activities == nil {
		t.Fatal("nil slices")
	}
}

func containsWarningCode(ws []domain.Warning, code string) bool {
	for _, w := range ws {
		if w.Code == code {
			return true
		}
	}
	return false
}

func TestGetOperationalOverview_zeroAssignmentsExactZero(t *testing.T) {
	members := fakeMembers{
		items: []caapp.MembershipView{
			{MembershipID: "m1", UserID: "u1", CompanyID: "c1", CompanyName: "Co", Status: "active"},
		},
		roles: map[string][]string{"m1": {"member"}},
	}
	svc := personalopsapp.NewService(members, fakeMine{}, fakeIdentity{
		user: &iamapp.AuthenticatedUser{UserID: "u1", FullName: "A", LoginID: "a@x.com"},
	}, fakeAuth{}, fakeInApp{}, personalopsapp.WithClock(fixedClock{t: time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)}), personalopsapp.WithLocation(time.UTC))
	resp, err := svc.GetOperationalOverview(context.Background(), personalopsapp.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Kpis.AssignedAlerts.Accuracy != "exact" || resp.Kpis.AssignedAlerts.Value == nil || *resp.Kpis.AssignedAlerts.Value != 0 {
		t.Fatalf("want exact 0, got %#v", resp.Kpis.AssignedAlerts)
	}
	if len(resp.MyTasks) != 0 {
		t.Fatalf("want empty tasks")
	}
}

func TestClassifyMineAlert_excludesTerminalFromOverdue(t *testing.T) {
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	st := personalopsapp.ClassifyMineAlert("published", "2026-07-01", false, now, time.UTC)
	if st != personalopsapp.AlertPENDINGConfirm {
		t.Fatalf("got %s", st)
	}
	st = personalopsapp.ClassifyMineAlert("in_progress", "2026-07-01", false, now, time.UTC)
	if st != personalopsapp.AlertOVERDUE {
		t.Fatalf("got %s", st)
	}
	st = personalopsapp.ClassifyMineAlert("in_progress", "2026-07-01", true, now, time.UTC)
	if st != personalopsapp.AlertDONE {
		t.Fatalf("got %s", st)
	}
}

func TestGetOperationalOverview_noCrossCompanyLeakIntoOverview(t *testing.T) {
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	members := fakeMembers{
		items: []caapp.MembershipView{
			{MembershipID: "m1", UserID: "u1", CompanyID: "c1", CompanyName: "MineCo", Status: "active"},
		},
		roles: map[string][]string{"m1": {"focal"}},
	}
	mine := fakeMine{
		records: []personalopsapp.MineRecord{
			{CompanyID: "c1", CompanyName: "MineCo", RecordID: "r1", Title: "ok", RecordStatus: "open", PlannedDate: "2026-07-20"},
			{CompanyID: "c_other", CompanyName: "Other", RecordID: "rx", Title: "leak", RecordStatus: "open", PlannedDate: "2026-07-01"},
		},
	}
	svc := personalopsapp.NewService(members, mine, fakeIdentity{
		user: &iamapp.AuthenticatedUser{UserID: "u1", FullName: "A", LoginID: "a@x.com"},
	}, fakeAuth{}, nil, personalopsapp.WithClock(fixedClock{t: now}), personalopsapp.WithLocation(time.UTC))
	resp, err := svc.GetOperationalOverview(context.Background(), personalopsapp.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.CompanyOverviews) != 1 || resp.CompanyOverviews[0].CompanyID != "c1" {
		t.Fatalf("overviews=%v", resp.CompanyOverviews)
	}
	if resp.Kpis.AssignedAlerts.Value == nil || *resp.Kpis.AssignedAlerts.Value != 1 {
		t.Fatalf("assigned should ignore foreign company row: %#v", resp.Kpis.AssignedAlerts)
	}
}

func TestGetOperationalOverview_onTimeRateExactAndExclusions(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	onTime := time.Date(2026, 7, 5, 15, 0, 0, 0, time.UTC)
	late := time.Date(2026, 7, 12, 9, 0, 0, 0, time.UTC)
	members := fakeMembers{
		items: []caapp.MembershipView{
			{MembershipID: "m1", UserID: "u1", CompanyID: "c1", CompanyName: "Co", Status: "active"},
		},
		roles: map[string][]string{"m1": {"focal"}},
	}
	mine := fakeMine{
		records: []personalopsapp.MineRecord{
			// on time with planned_date_fallback
			{CompanyID: "c1", CompanyName: "Co", RecordID: "r1", Title: "A", RecordStatus: "completed", PlannedDate: "2026-07-10", CompletedAt: &onTime, MatchedViaAsgn: true},
			// late with ad-hoc due
			{CompanyID: "c1", CompanyName: "Co", RecordID: "r2", Title: "B", RecordStatus: "published", PlannedDate: "2026-07-01", AdHocDueDate: "2026-07-08", CompletedAt: &late, MatchedViaTask: true},
			// missing outcome — exclude
			{CompanyID: "c1", CompanyName: "Co", RecordID: "r3", Title: "C", RecordStatus: "completed", PlannedDate: "2026-07-10", MatchedViaAsgn: true},
			// missing due — exclude
			{CompanyID: "c1", CompanyName: "Co", RecordID: "r4", Title: "D", RecordStatus: "completed", CompletedAt: &onTime, MatchedViaAsgn: true},
			// non-terminal — exclude from rate
			{CompanyID: "c1", CompanyName: "Co", RecordID: "r5", Title: "E", RecordStatus: "in_progress", PlannedDate: "2026-07-01", CompletedAt: &onTime, MatchedViaAsgn: true},
		},
		adhoc: map[string]string{"c1|r2": "2026-07-08"},
	}
	svc := personalopsapp.NewService(members, mine, fakeIdentity{
		user: &iamapp.AuthenticatedUser{UserID: "u1", FullName: "A", LoginID: "a@x.com"},
	}, fakeAuth{}, nil, personalopsapp.WithClock(fixedClock{t: now}), personalopsapp.WithLocation(time.UTC))
	resp, err := svc.GetOperationalOverview(context.Background(), personalopsapp.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"})
	if err != nil {
		t.Fatal(err)
	}
	rate := resp.CompanyOverviews[0].OnTimeRate
	if rate.Accuracy != "exact" || rate.SampleSize != 2 || rate.CompletedTotal != 2 || rate.CompletedOnTime != 1 {
		t.Fatalf("rate=%#v", rate)
	}
	if rate.Value == nil || *rate.Value != 50 {
		t.Fatalf("value=%v want 50", rate.Value)
	}
	if rate.Source == nil || *rate.Source != "disclosure_records.completed_at" {
		t.Fatalf("source=%v", rate.Source)
	}
	if containsWarningCode(resp.Meta.Warnings, "ON_TIME_RATE_SAMPLE_EMPTY") {
		t.Fatalf("should not warn SAMPLE_EMPTY when sample exists")
	}
}

func TestGetOperationalOverview_onTimeRateZeroPercentExact(t *testing.T) {
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	late := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	members := fakeMembers{
		items: []caapp.MembershipView{
			{MembershipID: "m1", UserID: "u1", CompanyID: "c1", CompanyName: "Co", Status: "active"},
		},
		roles: map[string][]string{"m1": {"x"}},
	}
	mine := fakeMine{
		records: []personalopsapp.MineRecord{
			{CompanyID: "c1", CompanyName: "Co", RecordID: "r1", RecordStatus: "done", PlannedDate: "2026-07-01", CompletedAt: &late, MatchedViaAsgn: true},
		},
	}
	svc := personalopsapp.NewService(members, mine, fakeIdentity{
		user: &iamapp.AuthenticatedUser{UserID: "u1", FullName: "A", LoginID: "a@x.com"},
	}, fakeAuth{}, nil, personalopsapp.WithClock(fixedClock{t: now}), personalopsapp.WithLocation(time.UTC))
	resp, err := svc.GetOperationalOverview(context.Background(), personalopsapp.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"})
	if err != nil {
		t.Fatal(err)
	}
	rate := resp.CompanyOverviews[0].OnTimeRate
	if rate.Accuracy != "exact" || rate.Value == nil || *rate.Value != 0 || rate.SampleSize != 1 {
		t.Fatalf("want exact 0%%, got %#v", rate)
	}
}

func TestGetOperationalOverview_adHocDueIncludedInOnTime(t *testing.T) {
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	completed := time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC)
	members := fakeMembers{
		items: []caapp.MembershipView{
			{MembershipID: "m1", UserID: "u1", CompanyID: "c1", CompanyName: "Co", Status: "active"},
		},
		roles: map[string][]string{"m1": {"x"}},
	}
	mine := fakeMine{
		records: []personalopsapp.MineRecord{
			// planned late relative to planned_date, but ad-hoc due is later → on time via ad-hoc
			{CompanyID: "c1", CompanyName: "Co", RecordID: "r1", RecordStatus: "completed", PlannedDate: "2026-07-01", CompletedAt: &completed, MatchedViaAsgn: true},
		},
		adhoc: map[string]string{"c1|r1": "2026-07-15"},
	}
	svc := personalopsapp.NewService(members, mine, fakeIdentity{
		user: &iamapp.AuthenticatedUser{UserID: "u1", FullName: "A", LoginID: "a@x.com"},
	}, fakeAuth{}, nil, personalopsapp.WithClock(fixedClock{t: now}), personalopsapp.WithLocation(time.UTC))
	resp, err := svc.GetOperationalOverview(context.Background(), personalopsapp.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"})
	if err != nil {
		t.Fatal(err)
	}
	rate := resp.CompanyOverviews[0].OnTimeRate
	if rate.Accuracy != "exact" || rate.CompletedOnTime != 1 || rate.CompletedTotal != 1 {
		t.Fatalf("ad-hoc due should count on time: %#v", rate)
	}
}
