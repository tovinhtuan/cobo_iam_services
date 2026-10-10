package app_test

import (
	"context"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

// ROLE-03: someone who may invite but does not hold rbac.manage may only hand out roles
// (and direct permissions) whose permissions they hold themselves.

func newGrantSubsetFixture(t *testing.T, inviterPerms []string) (*cainmem.AdminRepository, caapp.AdminService, caapp.AdminSubject) {
	t.Helper()
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_inv", MembershipID: "m_inv", CompanyID: "c_001"}
	seedInviteScopedSubject(t, repo, sub)
	seedPhaseEAdminCapable(t, repo, sub.CompanyID, "role_admin_e") // carries rbac.manage
	seedPhaseECustomRole(t, repo, "role_custom_e", "active")       // carries disclosure.view
	repo.SeedRole(caapp.RoleListItem{
		RoleID: "role_audit_e", RoleCode: "custom_audit_e", RoleName: "Custom Audit E",
		Status: "active", Scope: "company", RoleType: caapp.RoleTypeTenantCustom,
	})
	_ = repo.AddRolePermission(context.Background(), "role_audit_e", "audit.view")
	svc := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow, permissions: inviterPerms}, idgen.UUIDv7Generator{})
	return repo, svc, sub
}

func inviteWithRole(svc caapp.AdminService, sub caapp.AdminSubject, email, roleID string) (*caapp.InviteUserResponse, error) {
	return svc.InviteUser(context.Background(), caapp.InviteUserRequest{
		Subject: sub, Email: email, FullName: "Invitee", CompanyID: sub.CompanyID, CreatedByUserID: sub.UserID, RoleID: roleID,
	})
}

func TestInvite_WithoutRbacManage_CannotGrantAdminRole(t *testing.T) {
	_, svc, sub := newGrantSubsetFixture(t, []string{"admin.membership.invite", "disclosure.view"})
	_, err := inviteWithRole(svc, sub, "escalate@example.com", "role_admin_e")
	requireHTTPCode(t, err, 403, perr.CodePermissionDenied)
}

func TestInvite_WithoutRbacManage_CannotGrantRoleWithPermissionNotHeld(t *testing.T) {
	_, svc, sub := newGrantSubsetFixture(t, []string{"admin.membership.invite", "disclosure.view"})
	_, err := inviteWithRole(svc, sub, "audit@example.com", "role_audit_e")
	requireHTTPCode(t, err, 403, perr.CodePermissionDenied)
}

func TestInvite_WithoutRbacManage_CanGrantRoleWithinOwnPermissions(t *testing.T) {
	_, svc, sub := newGrantSubsetFixture(t, []string{"admin.membership.invite", "disclosure.view"})
	if _, err := inviteWithRole(svc, sub, "viewer@example.com", "role_custom_e"); err != nil {
		t.Fatalf("a role within the inviter's own permissions must be grantable: %v", err)
	}
}

func TestInvite_WithRbacManage_CanGrantAdminRole(t *testing.T) {
	_, svc, sub := newGrantSubsetFixture(t, []string{"rbac.manage", "admin.membership.invite"})
	if _, err := inviteWithRole(svc, sub, "admin2@example.com", "role_admin_e"); err != nil {
		t.Fatalf("a company admin may grant any assignable role: %v", err)
	}
}

func TestInvite_PlatformOperator_CanGrantAdminRole(t *testing.T) {
	_, svc, sub := newGrantSubsetFixture(t, []string{"platform.cms.view", "system.settings", "admin.membership.invite"})
	if _, err := inviteWithRole(svc, sub, "client.admin@example.com", "role_admin_e"); err != nil {
		t.Fatalf("a platform operator provisions client admins: %v", err)
	}
}

func TestInvite_SystemSettingsWithoutRbacManage_CannotGrantAdminRole(t *testing.T) {
	_, svc, sub := newGrantSubsetFixture(t, []string{"system.settings", "admin.membership.invite"})
	_, err := inviteWithRole(svc, sub, "escalate2@example.com", "role_admin_e")
	requireHTTPCode(t, err, 403, perr.CodePermissionDenied)
}

func TestInvite_WithoutRbacManage_CannotGrantDirectPermissionNotHeld(t *testing.T) {
	_, svc, sub := newGrantSubsetFixture(t, []string{"admin.membership.invite", "disclosure.view"})
	_, err := svc.InviteUser(context.Background(), caapp.InviteUserRequest{
		Subject: sub, Email: "direct@example.com", FullName: "Invitee", CompanyID: sub.CompanyID, CreatedByUserID: sub.UserID,
		RoleID: "role_custom_e", Permissions: []string{"ad_hoc_alert.propose"},
	})
	requireHTTPCode(t, err, 403, perr.CodePermissionDenied)
}

func TestReplacePrimaryRole_WithoutRbacManage_CannotGrantAdminRole(t *testing.T) {
	repo, svc, sub := newGrantSubsetFixture(t, []string{"admin.membership.invite", "admin.membership.role.assign", "admin.membership.update", "disclosure.view"})
	seedInviteScopedSubject(t, repo, caapp.AdminSubject{UserID: "u_tgt", MembershipID: "m_tgt", CompanyID: "c_001"})
	err := svc.ReplaceMembershipPrimaryRole(context.Background(), caapp.ReplaceMembershipPrimaryRoleRequest{
		Subject: sub, MembershipID: "m_tgt", RoleID: "role_admin_e",
	})
	requireHTTPCode(t, err, 403, perr.CodePermissionDenied)
}

// API-17 / ROLE-22: the invite role picker only offers roles the caller may grant.
func TestListInviteRoles_WithoutRbacManage_HidesRolesNotGrantable(t *testing.T) {
	_, svc, sub := newGrantSubsetFixture(t, []string{"admin.membership.invite", "disclosure.view"})
	items, err := svc.ListInviteRoles(context.Background(), caapp.ListInviteRolesRequest{Subject: sub})
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, it := range items {
		ids[it.RoleID] = true
	}
	if ids["role_admin_e"] || ids["role_audit_e"] {
		t.Fatalf("picker offers roles the caller cannot grant: %v", ids)
	}
	if !ids["role_custom_e"] {
		t.Fatalf("picker must keep grantable roles: %v", ids)
	}
}

func TestListInviteRoles_WithRbacManage_ShowsAll(t *testing.T) {
	_, svc, sub := newGrantSubsetFixture(t, []string{"rbac.manage", "admin.membership.invite"})
	items, err := svc.ListInviteRoles(context.Background(), caapp.ListInviteRolesRequest{Subject: sub})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, it := range items {
		if it.RoleID == "role_admin_e" {
			found = true
		}
	}
	if !found {
		t.Fatal("a company admin sees every assignable role")
	}
}
