package app_test

import (
	"context"
	"testing"
	"time"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	inappapp "github.com/cobo/cobo_iam_services/internal/inappnotification/app"
	personalopsapp "github.com/cobo/cobo_iam_services/internal/personalops/app"
)

type reportInApp struct {
	byCompany map[string][]inappapp.InAppNotification
}

func (f reportInApp) List(_ context.Context, _, companyID string) ([]inappapp.InAppNotification, error) {
	return f.byCompany[companyID], nil
}

type actorAudit struct {
	entries []auditapp.Entry
}

func (f actorAudit) ListFiltered(_ context.Context, filter auditapp.ListFilter) ([]auditapp.Entry, error) {
	out := make([]auditapp.Entry, 0)
	for _, e := range f.entries {
		if filter.ActorUserID != "" && e.ActorUserID != filter.ActorUserID {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

func TestOperationalOverview_SeparatesReportActivitiesAndActivityLog(t *testing.T) {
	now := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	rt := inappapp.ResourceTypeDisclosure
	svc := personalopsapp.NewService(
		fakeMembers{items: []caapp.MembershipView{{
			MembershipID: "m1", UserID: "u1", CompanyID: "c1", CompanyName: "Co", Status: "active",
		}}},
		fakeMine{},
		fakeIdentity{user: &iamapp.AuthenticatedUser{UserID: "u1", FullName: "User", LoginID: "u@x.com"}},
		nil,
		reportInApp{byCompany: map[string][]inappapp.InAppNotification{
			"c1": {
				{ID: "n1", Kind: inappapp.KindReminderDeadline, Title: "Deadline soon", Body: "Q2", ResourceType: &rt, ResourceID: strPtr("r1"), CreatedAt: now},
				{ID: "n2", Kind: inappapp.KindAuthEmailVerif, Title: "Verify email", Body: "auth", CreatedAt: now},
			},
		}},
		personalopsapp.WithAuditLister(actorAudit{entries: []auditapp.Entry{
			{EventID: "e1", ActorUserID: "u1", Action: "admin.user.create", OccurredAt: now.Format(time.RFC3339)},
			{EventID: "e2", ActorUserID: "other", Action: "admin.user.create", OccurredAt: now.Format(time.RFC3339)},
		}}),
		personalopsapp.WithClock(fixedClock{t: now}),
	)

	resp, err := svc.GetOperationalOverview(context.Background(), personalopsapp.Subject{UserID: "u1", CompanyID: "c1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Activities) != 1 || resp.Activities[0].ID != "n1" {
		t.Fatalf("report activities should exclude auth notif, got %+v", resp.Activities)
	}
	if len(resp.ActivityLog) != 1 || resp.ActivityLog[0].ID != "e1" {
		t.Fatalf("activity log must be actor-scoped, got %+v", resp.ActivityLog)
	}
	if resp.Activities[0].Source != "in_app_notifications" {
		t.Fatalf("unexpected activity source %q", resp.Activities[0].Source)
	}
	if resp.ActivityLog[0].Source != "audit_logs" {
		t.Fatalf("unexpected activity_log source %q", resp.ActivityLog[0].Source)
	}
}

func strPtr(s string) *string { return &s }
