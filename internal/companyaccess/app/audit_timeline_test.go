package app_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	auditinmem "github.com/cobo/cobo_iam_services/internal/audit/infra/inmemory"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func newTimelineTestService(repo *cainmem.AdminRepository, auditRepo *auditinmem.Repository, perms []string) caapp.AdminService {
	return caapp.NewAdminService(
		repo,
		healthAuthService{permissions: perms},
		fixedIDGen("id-1"),
		caapp.WithAuditRepository(auditRepo),
		caapp.WithConflictSnapshotReader(cainmem.NewConflictSnapshotReader(repo)),
		caapp.WithDependencyReader(cainmem.NewDependencyReader(repo)),
	)
}

func TestListChangeTimelineNormalized(t *testing.T) {
	auditRepo := auditinmem.NewRepository()
	_ = auditRepo.Append(context.Background(), auditapp.Entry{
		EventID: "e1", OccurredAt: time.Now().UTC().Format(time.RFC3339),
		CompanyID: "co-1", Action: "admin.department.create",
		ActorUserID: "u1", ResourceType: "department", ResourceID: "d1",
	})
	_ = auditRepo.Append(context.Background(), auditapp.Entry{
		EventID: "e2", OccurredAt: time.Now().UTC().Format(time.RFC3339),
		CompanyID: "co-1", Action: "cms.entry.create",
		ActorUserID: "u1",
	})
	svc := newTimelineTestService(cainmem.NewAdminRepository(), auditRepo, []string{"rbac.manage"})
	sub := caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "co-1"}
	out, err := svc.ListChangeTimeline(context.Background(), caapp.ListChangeTimelineRequest{Subject: sub, Limit: 10})
	if err != nil {
		t.Fatalf("ListChangeTimeline: %v", err)
	}
	if len(out.Items) != 1 {
		t.Fatalf("expected admin-only event, got %d", len(out.Items))
	}
	if out.Items[0].Summary != "Tạo phòng ban" {
		t.Fatalf("summary: %s", out.Items[0].Summary)
	}
}

func TestListChangeTimelineForbidden(t *testing.T) {
	svc := newTimelineTestService(cainmem.NewAdminRepository(), auditinmem.NewRepository(), []string{"disclosure.view"})
	sub := caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "co-1"}
	_, err := svc.ListChangeTimeline(context.Background(), caapp.ListChangeTimelineRequest{Subject: sub})
	if err == nil {
		t.Fatal("expected error")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusForbidden {
		t.Fatalf("expected 403, got %v", err)
	}
}

func TestListAuditLogsCompanyScoped(t *testing.T) {
	auditRepo := auditinmem.NewRepository()
	_ = auditRepo.Append(context.Background(), auditapp.Entry{
		EventID: "e1", OccurredAt: time.Now().UTC().Format(time.RFC3339),
		CompanyID: "co-1", Action: "admin.user.invite", ActorUserID: "u1",
	})
	svc := newTimelineTestService(cainmem.NewAdminRepository(), auditRepo, []string{"rbac.manage"})
	sub := caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "co-1"}
	out, err := svc.ListAuditLogs(context.Background(), caapp.ListAuditLogsRequest{Subject: sub})
	if err != nil {
		t.Fatalf("ListAuditLogs: %v", err)
	}
	if len(out.Items) != 1 {
		t.Fatalf("items: %d", len(out.Items))
	}
}
