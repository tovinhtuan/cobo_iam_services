package app_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	auditinmem "github.com/cobo/cobo_iam_services/internal/audit/infra/inmemory"
	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type delIDGen struct{ n int }

func (g *delIDGen) NewUUID() string {
	g.n++
	return fmt.Sprintf("del-%d", g.n)
}

func newDelegationSvc(t *testing.T, repo *cainmem.AdminRepository, perms []string) caapp.AdminService {
	t.Helper()
	return caapp.NewAdminService(repo, fakeAuthService{
		decision:    authapp.DecisionAllow,
		permissions: perms,
	}, &delIDGen{},
		caapp.WithAuditRepository(auditinmem.NewRepository()),
	)
}

func seedDelegator(t *testing.T, repo *cainmem.AdminRepository) caapp.AdminSubject {
	t.Helper()
	sub := caapp.AdminSubject{UserID: "u_del_admin", MembershipID: "m_del_admin", CompanyID: "c_del"}
	seedInviteScopedSubject(t, repo, sub)
	_ = repo.AddRolePermission(context.Background(), "company_admin", "rbac.manage")
	_ = repo.AddRole(context.Background(), sub.MembershipID, "company_admin")
	return sub
}

func seedDelegatee(t *testing.T, repo *cainmem.AdminRepository, membershipID, deptID string) {
	t.Helper()
	_, err := repo.CreateUser(context.Background(), caapp.UserView{
		UserID: membershipID + "_u", LoginID: membershipID + "@example.com",
		FullName: "Delegatee", AccountStatus: "active",
	}, "hash", caapp.CreateUserOptions{
		MembershipID: membershipID, CompanyID: "c_del", MembershipStatus: "active",
	})
	if err != nil {
		t.Fatalf("seed delegatee: %v", err)
	}
	_ = repo.AddDepartment(context.Background(), membershipID, deptID)
}

func seedMemberInDept(t *testing.T, repo *cainmem.AdminRepository, membershipID, deptID string) {
	t.Helper()
	_, err := repo.CreateUser(context.Background(), caapp.UserView{
		UserID: membershipID + "_u", LoginID: membershipID + "@example.com",
		FullName: "Member", AccountStatus: "active",
	}, "hash", caapp.CreateUserOptions{
		MembershipID: membershipID, CompanyID: "c_del", MembershipStatus: "active",
	})
	if err != nil {
		t.Fatalf("seed member: %v", err)
	}
	_ = repo.AddDepartment(context.Background(), membershipID, deptID)
}

func TestDelegation_CreateSuccess(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	repo.SeedDepartment(caapp.DepartmentView{DepartmentID: "dep_a", Name: "Dept A"})
	delegator := seedDelegator(t, repo)
	seedDelegatee(t, repo, "m_delegatee", "dep_a")
	svc := newDelegationSvc(t, repo, []string{"rbac.manage", "admin.membership.invite", "admin.membership.update"})

	grant, err := svc.CreateDelegation(context.Background(), caapp.CreateDelegationRequest{
		Subject: delegator, DelegateeMembershipID: "m_delegatee",
		ScopeType: "department", ScopeID: "dep_a",
		PermissionSet: []string{"admin.membership.invite", "admin.membership.update"},
	})
	if err != nil {
		t.Fatalf("CreateDelegation: %v", err)
	}
	if grant.Status != caapp.DelegationStatusActive {
		t.Fatalf("status=%q", grant.Status)
	}
}

func TestDelegation_Duplicate409(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	repo.SeedDepartment(caapp.DepartmentView{DepartmentID: "dep_a", Name: "Dept A"})
	delegator := seedDelegator(t, repo)
	seedDelegatee(t, repo, "m_delegatee", "dep_a")
	svc := newDelegationSvc(t, repo, []string{"rbac.manage", "admin.membership.invite"})

	req := caapp.CreateDelegationRequest{
		Subject: delegator, DelegateeMembershipID: "m_delegatee",
		ScopeType: "department", ScopeID: "dep_a",
		PermissionSet: []string{"admin.membership.invite"},
	}
	if _, err := svc.CreateDelegation(context.Background(), req); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := svc.CreateDelegation(context.Background(), req)
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusConflict {
		t.Fatalf("want 409, got %v", err)
	}
}

func TestDelegation_ForbiddenRbacManage(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	repo.SeedDepartment(caapp.DepartmentView{DepartmentID: "dep_a", Name: "Dept A"})
	delegator := seedDelegator(t, repo)
	seedDelegatee(t, repo, "m_delegatee", "dep_a")
	svc := newDelegationSvc(t, repo, []string{"rbac.manage"})

	_, err := svc.CreateDelegation(context.Background(), caapp.CreateDelegationRequest{
		Subject: delegator, DelegateeMembershipID: "m_delegatee",
		ScopeType: "department", ScopeID: "dep_a",
		PermissionSet: []string{"rbac.manage"},
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusForbidden {
		t.Fatalf("want 403, got %v", err)
	}
}

func TestDelegation_ScopedInviteInScope(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	repo.SeedDepartment(caapp.DepartmentView{DepartmentID: "dep_a", Name: "Dept A"})
	delegator := seedDelegator(t, repo)
	seedDelegatee(t, repo, "m_delegatee", "dep_a")
	svcAdmin := newDelegationSvc(t, repo, []string{"rbac.manage", "admin.membership.invite"})
	delegatee := caapp.AdminSubject{UserID: "m_delegatee_u", MembershipID: "m_delegatee", CompanyID: "c_del"}

	_, err := svcAdmin.CreateDelegation(context.Background(), caapp.CreateDelegationRequest{
		Subject: delegator, DelegateeMembershipID: "m_delegatee",
		ScopeType: "department", ScopeID: "dep_a",
		PermissionSet: []string{"admin.membership.invite"},
	})
	if err != nil {
		t.Fatalf("grant: %v", err)
	}

	svcDel := newDelegationSvc(t, repo, []string{"admin.membership.create", "admin.membership.invite"})
	_, err = svcDel.InviteUser(context.Background(), caapp.InviteUserRequest{
		Subject: delegatee, Email: "newscoped@example.com", FullName: "Scoped",
		CompanyID: "c_del", DepartmentID: "dep_a",
	})
	if err != nil {
		t.Fatalf("invite in scope: %v", err)
	}
}

func TestDelegation_ScopedInviteOutsideScopeDenied(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	repo.SeedDepartment(caapp.DepartmentView{DepartmentID: "dep_a", Name: "Dept A"})
	repo.SeedDepartment(caapp.DepartmentView{DepartmentID: "dep_b", Name: "Dept B"})
	delegator := seedDelegator(t, repo)
	seedDelegatee(t, repo, "m_delegatee", "dep_a")
	svcAdmin := newDelegationSvc(t, repo, []string{"rbac.manage", "admin.membership.invite"})
	delegatee := caapp.AdminSubject{UserID: "m_delegatee_u", MembershipID: "m_delegatee", CompanyID: "c_del"}

	_, _ = svcAdmin.CreateDelegation(context.Background(), caapp.CreateDelegationRequest{
		Subject: delegator, DelegateeMembershipID: "m_delegatee",
		ScopeType: "department", ScopeID: "dep_a",
		PermissionSet: []string{"admin.membership.invite"},
	})
	svcDel := newDelegationSvc(t, repo, []string{"admin.membership.create", "admin.membership.invite"})
	_, err := svcDel.InviteUser(context.Background(), caapp.InviteUserRequest{
		Subject: delegatee, Email: "out@example.com", FullName: "Out",
		CompanyID: "c_del", DepartmentID: "dep_b",
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusForbidden {
		t.Fatalf("want 403 outside scope, got %v", err)
	}
}

func TestDelegation_UpdateOutsideScopeDenied(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	repo.SeedDepartment(caapp.DepartmentView{DepartmentID: "dep_a", Name: "Dept A"})
	repo.SeedDepartment(caapp.DepartmentView{DepartmentID: "dep_b", Name: "Dept B"})
	delegator := seedDelegator(t, repo)
	seedDelegatee(t, repo, "m_delegatee", "dep_a")
	seedMemberInDept(t, repo, "m_outside", "dep_b")
	svcAdmin := newDelegationSvc(t, repo, []string{"rbac.manage", "admin.membership.update"})
	delegatee := caapp.AdminSubject{UserID: "m_delegatee_u", MembershipID: "m_delegatee", CompanyID: "c_del"}

	_, _ = svcAdmin.CreateDelegation(context.Background(), caapp.CreateDelegationRequest{
		Subject: delegator, DelegateeMembershipID: "m_delegatee",
		ScopeType: "department", ScopeID: "dep_a",
		PermissionSet: []string{"admin.membership.update"},
	})
	svcDel := newDelegationSvc(t, repo, []string{"admin.membership.update"})
	_, err := svcDel.UpdateMembership(context.Background(), caapp.UpdateMembershipRequest{
		Subject: delegatee, MembershipID: "m_outside", Status: "inactive",
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusForbidden {
		t.Fatalf("want 403, got %v", err)
	}
}

func TestDelegation_RevokeStopsAuthorization(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	repo.SeedDepartment(caapp.DepartmentView{DepartmentID: "dep_a", Name: "Dept A"})
	delegator := seedDelegator(t, repo)
	seedDelegatee(t, repo, "m_delegatee", "dep_a")
	svcAdmin := newDelegationSvc(t, repo, []string{"rbac.manage", "admin.membership.invite"})
	delegatee := caapp.AdminSubject{UserID: "m_delegatee_u", MembershipID: "m_delegatee", CompanyID: "c_del"}

	grant, _ := svcAdmin.CreateDelegation(context.Background(), caapp.CreateDelegationRequest{
		Subject: delegator, DelegateeMembershipID: "m_delegatee",
		ScopeType: "department", ScopeID: "dep_a",
		PermissionSet: []string{"admin.membership.invite"},
	})
	if _, err := svcAdmin.RevokeDelegation(context.Background(), caapp.RevokeDelegationRequest{
		Subject: delegator, DelegationID: grant.DelegationID,
	}); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	svcDel := newDelegationSvc(t, repo, []string{"admin.membership.create", "admin.membership.invite"})
	_, err := svcDel.InviteUser(context.Background(), caapp.InviteUserRequest{
		Subject: delegatee, Email: "after@example.com", FullName: "After",
		CompanyID: "c_del", DepartmentID: "dep_a",
	})
	if err == nil {
		t.Fatal("expected error after revoke")
	}
}

func TestDelegation_GlobalAdminUnchanged(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	repo.SeedDepartment(caapp.DepartmentView{DepartmentID: "dep_a", Name: "Dept A"})
	repo.SeedDepartment(caapp.DepartmentView{DepartmentID: "dep_b", Name: "Dept B"})
	admin := seedDelegator(t, repo)
	seedMemberInDept(t, repo, "m_any", "dep_b")
	svc := newDelegationSvc(t, repo, []string{"rbac.manage", "admin.membership.update"})
	_, err := svc.UpdateMembership(context.Background(), caapp.UpdateMembershipRequest{
		Subject: admin, MembershipID: "m_any", Status: "inactive",
	})
	if err != nil {
		t.Fatalf("global admin update: %v", err)
	}
}

func TestDelegation_AuditOnGrant(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	repo.SeedDepartment(caapp.DepartmentView{DepartmentID: "dep_a", Name: "Dept A"})
	delegator := seedDelegator(t, repo)
	seedDelegatee(t, repo, "m_delegatee", "dep_a")
	auditRepo := auditinmem.NewRepository()
	svc := caapp.NewAdminService(repo, fakeAuthService{
		decision: authapp.DecisionAllow, permissions: []string{"rbac.manage", "admin.membership.invite"},
	}, &delIDGen{}, caapp.WithAuditRepository(auditRepo))

	_, err := svc.CreateDelegation(context.Background(), caapp.CreateDelegationRequest{
		Subject: delegator, DelegateeMembershipID: "m_delegatee",
		ScopeType: "department", ScopeID: "dep_a",
		PermissionSet: []string{"admin.membership.invite"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	entries, _ := auditRepo.ListByCompany(context.Background(), "c_del", "", "", "", "", "", "", 10)
	found := false
	for _, e := range entries {
		if e.Action == "delegated.admin.granted" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected delegated.admin.granted audit")
	}
}
