package app_test

import (
	"context"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// ROLE-05: removing a role must not strip the primary admin of administration, nor leave the
// company without any admin-capable member.

func newRemoveRoleGuardFixture(t *testing.T) (*cainmem.AdminRepository, caapp.AdminService, caapp.AdminSubject) {
	t.Helper()
	repo := cainmem.NewAdminRepository()
	caller := caapp.AdminSubject{UserID: "u_peer", MembershipID: "m_peer", CompanyID: "c_001"}
	seedMem(repo, "m_peer", "u_peer", "c_001")
	seedMem(repo, "m_owner", "u_owner", "c_001")
	seedMem(repo, "m_staff", "u_staff", "c_001")
	repo.SeedRoleForCompany(caapp.RoleListItem{RoleID: "r_admin", RoleCode: "admin_doanh_nghiep", RoleName: "Admin", Status: "active", RoleType: caapp.RoleTypeTenantCustom}, "c_001")
	repo.SeedRoleForCompany(caapp.RoleListItem{RoleID: "r_viewer", RoleCode: "viewer", RoleName: "Viewer", Status: "active", RoleType: caapp.RoleTypeTenantCustom}, "c_001")
	_ = repo.AddRolePermission(context.Background(), "r_admin", "rbac.manage")
	_ = repo.AddRolePermission(context.Background(), "r_viewer", "disclosure.view")
	if err := repo.SetMembershipPrimaryAdmin(context.Background(), "m_owner"); err != nil {
		t.Fatal(err)
	}
	svc := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"rbac.manage"}}, fixedIDGen("test-id"))
	return repo, svc, caller
}

func TestRemoveRole_CannotRemoveAdminRoleFromPrimaryAdmin(t *testing.T) {
	repo, svc, caller := newRemoveRoleGuardFixture(t)
	_ = repo.AddRole(context.Background(), "m_owner", "r_admin")
	_ = repo.AddRole(context.Background(), "m_peer", "r_admin")
	err := svc.RemoveRole(context.Background(), caapp.RemoveRoleRequest{Subject: caller, MembershipID: "m_owner", RoleID: "r_admin"})
	requireHTTPCode(t, err, 409, perr.CodeStateConflict)
}

func TestRemoveRole_CannotRemoveLastAdmin(t *testing.T) {
	repo, svc, caller := newRemoveRoleGuardFixture(t)
	_ = repo.AddRole(context.Background(), "m_staff", "r_admin") // the only admin-capable member
	err := svc.RemoveRole(context.Background(), caapp.RemoveRoleRequest{Subject: caller, MembershipID: "m_staff", RoleID: "r_admin"})
	requireHTTPCode(t, err, 409, perr.CodeLastAdminRoleChangeBlocked)
}

func TestRemoveRole_AdminRoleRemovableWhenAnotherAdminRemains(t *testing.T) {
	repo, svc, caller := newRemoveRoleGuardFixture(t)
	_ = repo.AddRole(context.Background(), "m_staff", "r_admin")
	_ = repo.AddRole(context.Background(), "m_peer", "r_admin")
	if err := svc.RemoveRole(context.Background(), caapp.RemoveRoleRequest{Subject: caller, MembershipID: "m_staff", RoleID: "r_admin"}); err != nil {
		t.Fatalf("another admin remains: %v", err)
	}
}

func TestRemoveRole_NonAdminRoleFromPrimaryAdminIsAllowed(t *testing.T) {
	repo, svc, caller := newRemoveRoleGuardFixture(t)
	_ = repo.AddRole(context.Background(), "m_owner", "r_admin")
	_ = repo.AddRole(context.Background(), "m_owner", "r_viewer")
	if err := svc.RemoveRole(context.Background(), caapp.RemoveRoleRequest{Subject: caller, MembershipID: "m_owner", RoleID: "r_viewer"}); err != nil {
		t.Fatalf("removing a non-admin role: %v", err)
	}
}

// BES-19: a peer admin cannot demote the primary admin by replacing its primary role either.
func TestReplacePrimaryRole_CannotDemotePrimaryAdmin(t *testing.T) {
	repo, svc, caller := newRemoveRoleGuardFixture(t)
	_ = repo.AddRole(context.Background(), "m_owner", "r_admin")
	_ = repo.AddRole(context.Background(), "m_peer", "r_admin")
	err := svc.ReplaceMembershipPrimaryRole(context.Background(), caapp.ReplaceMembershipPrimaryRoleRequest{
		Subject: caller, MembershipID: "m_owner", RoleID: "r_viewer",
	})
	requireHTTPCode(t, err, 409, perr.CodeStateConflict)
	roles, _ := repo.ListMembershipRoles(context.Background(), "m_owner")
	if len(roles) != 1 || roles[0].RoleID != "r_admin" {
		t.Fatalf("primary admin roles changed: %+v", roles)
	}
}

// API-20: the guard only applies when the member actually holds the role being removed.
func TestRemoveRole_GuardIgnoresRoleNotHeld(t *testing.T) {
	_, svc, caller := newRemoveRoleGuardFixture(t)
	err := svc.RemoveRole(context.Background(), caapp.RemoveRoleRequest{Subject: caller, MembershipID: "m_owner", RoleID: "r_admin"})
	if he, ok := perr.AsHTTPError(err); ok && he.HTTPStatus == 409 {
		t.Fatalf("a role the member does not hold must not trip the admin guard: %v", err)
	}
}
