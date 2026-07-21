package app_test

import (
	"context"
	"net/http"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

func seedPhaseEAdminCapable(t *testing.T, repo *cainmem.AdminRepository, companyID, roleID string) {
	t.Helper()
	repo.SeedRole(caapp.RoleListItem{
		RoleID: roleID, RoleCode: "admin_doanh_nghiep", RoleName: "Admin DN",
		Status: "active", Scope: "company", RoleType: caapp.RoleTypeTenantDefault, IsProtected: true,
	})
	if err := repo.AddRolePermission(context.Background(), roleID, "rbac.manage"); err != nil {
		t.Fatalf("seed rbac.manage: %v", err)
	}
	_ = companyID
}

func seedPhaseECustomRole(t *testing.T, repo *cainmem.AdminRepository, roleID, status string) {
	t.Helper()
	repo.SeedRole(caapp.RoleListItem{
		RoleID: roleID, RoleCode: "custom_viewer_e", RoleName: "Custom Viewer E",
		Status: status, Scope: "company", RoleType: caapp.RoleTypeTenantCustom, IsProtected: false,
	})
	_ = repo.AddRolePermission(context.Background(), roleID, "disclosure.view")
}

func newPhaseEAssignSvc(t *testing.T, repo *cainmem.AdminRepository, sub caapp.AdminSubject) caapp.AdminService {
	t.Helper()
	seedInviteScopedSubject(t, repo, sub)
	return caapp.NewAdminService(
		repo,
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{
			"rbac.manage", "admin.membership.invite", "admin.membership.role.assign", "admin.membership.update", "system.settings",
		}},
		idgen.UUIDv7Generator{},
	)
}

func TestPhaseE_AssignActiveTenantCustomRole(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"}
	svc := newPhaseEAssignSvc(t, repo, sub)
	seedPhaseEAdminCapable(t, repo, sub.CompanyID, "role_admin_e")
	seedPhaseECustomRole(t, repo, "role_custom_e", "active")
	_ = repo.AddRole(context.Background(), sub.MembershipID, "role_admin_e")

	out, err := svc.InviteUser(context.Background(), caapp.InviteUserRequest{
		Subject: sub, Email: "member.e@example.com", FullName: "Member E",
		CompanyID: "c_001", CreatedByUserID: sub.UserID, RoleCode: "user_thuong",
	})
	if err != nil {
		t.Fatalf("InviteUser: %v", err)
	}

	err = svc.ReplaceMembershipPrimaryRole(context.Background(), caapp.ReplaceMembershipPrimaryRoleRequest{
		Subject: sub, MembershipID: out.MembershipID, RoleID: "role_custom_e",
	})
	if err != nil {
		t.Fatalf("assign custom: %v", err)
	}
	roles, err := repo.ListMembershipRoles(context.Background(), out.MembershipID)
	if err != nil {
		t.Fatalf("roles: %v", err)
	}
	if len(roles) != 1 || roles[0].RoleID != "role_custom_e" {
		t.Fatalf("want primary custom only, got %+v", roles)
	}
}

func TestPhaseE_AssignInactiveTenantCustomRoleRejected(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"}
	svc := newPhaseEAssignSvc(t, repo, sub)
	seedPhaseECustomRole(t, repo, "role_custom_inactive", "inactive")

	out, err := svc.InviteUser(context.Background(), caapp.InviteUserRequest{
		Subject: sub, Email: "inactive.target@example.com", FullName: "Inactive Target",
		CompanyID: "c_001", CreatedByUserID: sub.UserID, RoleCode: "user_thuong",
	})
	if err != nil {
		t.Fatalf("InviteUser: %v", err)
	}

	err = svc.ReplaceMembershipPrimaryRole(context.Background(), caapp.ReplaceMembershipPrimaryRoleRequest{
		Subject: sub, MembershipID: out.MembershipID, RoleID: "role_custom_inactive",
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusUnprocessableEntity || he.Code != perr.CodeRoleInactive {
		t.Fatalf("want 422 role_inactive, got %v", err)
	}
}

func TestPhaseE_AssignCrossTenantCustomRoleRejected(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"}
	svc := newPhaseEAssignSvc(t, repo, sub)
	// Seed role only under other company id — GetCompanyRoleByID still returns it in-memory
	// without company scoping; simulate missing by not seeding and using unknown id.
	out, err := svc.InviteUser(context.Background(), caapp.InviteUserRequest{
		Subject: sub, Email: "cross.target@example.com", FullName: "Cross Target",
		CompanyID: "c_001", CreatedByUserID: sub.UserID, RoleCode: "user_thuong",
	})
	if err != nil {
		t.Fatalf("InviteUser: %v", err)
	}

	err = svc.ReplaceMembershipPrimaryRole(context.Background(), caapp.ReplaceMembershipPrimaryRoleRequest{
		Subject: sub, MembershipID: out.MembershipID, RoleID: "role_other_company",
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusNotFound {
		t.Fatalf("want 404 cross-tenant/missing, got %v", err)
	}
}

func TestPhaseE_SelfDemotionBlocked(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"}
	svc := newPhaseEAssignSvc(t, repo, sub)
	seedPhaseEAdminCapable(t, repo, sub.CompanyID, "role_admin_e")
	seedPhaseECustomRole(t, repo, "role_custom_e", "active")
	_ = repo.AddRole(context.Background(), sub.MembershipID, "role_admin_e")

	err := svc.ReplaceMembershipPrimaryRole(context.Background(), caapp.ReplaceMembershipPrimaryRoleRequest{
		Subject: sub, MembershipID: sub.MembershipID, RoleID: "role_custom_e",
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusConflict || he.Code != perr.CodeSelfRoleChangeBlocked {
		t.Fatalf("want 409 self_role_change_blocked, got %v", err)
	}
}

func TestPhaseE_LastAdminDemotionBlocked(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"}
	svc := newPhaseEAssignSvc(t, repo, sub)
	seedPhaseEAdminCapable(t, repo, sub.CompanyID, "role_admin_e")
	seedPhaseECustomRole(t, repo, "role_custom_e", "active")
	_ = repo.AddRole(context.Background(), sub.MembershipID, "role_admin_e")

	// Second admin-capable membership (target). Actor is different membership.
	actor := caapp.AdminSubject{UserID: "u_actor", MembershipID: "m_actor", CompanyID: "c_001"}
	_, _ = repo.CreateMembership(context.Background(), caapp.MembershipView{
		MembershipID: actor.MembershipID, UserID: actor.UserID, CompanyID: "c_001", Status: "active",
	})
	_ = repo.AddRole(context.Background(), actor.MembershipID, "role_admin_e")

	// Only one admin-capable besides... wait: both m_admin and m_actor have admin.
	// Make m_admin the sole admin, actor without admin capability for auth still uses fake allow.
	_ = repo.RemoveRole(context.Background(), actor.MembershipID, "role_admin_e")

	out, err := svc.InviteUser(context.Background(), caapp.InviteUserRequest{
		Subject: sub, Email: "last.admin@example.com", FullName: "Last Admin Twin",
		CompanyID: "c_001", CreatedByUserID: sub.UserID, RoleCode: "user_thuong",
	})
	if err != nil {
		t.Fatalf("InviteUser: %v", err)
	}
	// Promote invited member to be the last admin (and demote m_admin's capability for count).
	_ = repo.RemoveRole(context.Background(), sub.MembershipID, "role_admin_e")
	_ = repo.AddRole(context.Background(), out.MembershipID, "role_admin_e")

	err = svc.ReplaceMembershipPrimaryRole(context.Background(), caapp.ReplaceMembershipPrimaryRoleRequest{
		Subject: actor, MembershipID: out.MembershipID, RoleID: "role_custom_e",
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusConflict || he.Code != perr.CodeLastAdminRoleChangeBlocked {
		t.Fatalf("want 409 last_admin_role_change_blocked, got %v", err)
	}
}

func TestPhaseE_AssignNonLastAdminToCustomSuccess(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"}
	svc := newPhaseEAssignSvc(t, repo, sub)
	seedPhaseEAdminCapable(t, repo, sub.CompanyID, "role_admin_e")
	seedPhaseECustomRole(t, repo, "role_custom_e", "active")
	_ = repo.AddRole(context.Background(), sub.MembershipID, "role_admin_e")

	out, err := svc.InviteUser(context.Background(), caapp.InviteUserRequest{
		Subject: sub, Email: "second.admin@example.com", FullName: "Second Admin",
		CompanyID: "c_001", CreatedByUserID: sub.UserID, RoleCode: "user_thuong",
	})
	if err != nil {
		t.Fatalf("InviteUser: %v", err)
	}
	_ = repo.AddRole(context.Background(), out.MembershipID, "role_admin_e")
	// Remove default user_thuong primary by replacing via service from another admin.
	err = svc.ReplaceMembershipPrimaryRole(context.Background(), caapp.ReplaceMembershipPrimaryRoleRequest{
		Subject: sub, MembershipID: out.MembershipID, RoleID: "role_custom_e",
	})
	if err != nil {
		t.Fatalf("expected success demoting non-last admin: %v", err)
	}
}

func TestPhaseE_InviteWithTenantCustomRole(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"}
	svc := newPhaseEAssignSvc(t, repo, sub)
	seedPhaseECustomRole(t, repo, "role_custom_invite", "active")

	out, err := svc.InviteUser(context.Background(), caapp.InviteUserRequest{
		Subject: sub, Email: "invite.custom@example.com", FullName: "Invite Custom",
		CompanyID: "c_001", CreatedByUserID: sub.UserID, RoleID: "role_custom_invite",
	})
	if err != nil {
		t.Fatalf("InviteUser custom: %v", err)
	}
	roles, err := repo.ListMembershipRoles(context.Background(), out.MembershipID)
	if err != nil {
		t.Fatalf("roles: %v", err)
	}
	if len(roles) != 1 || roles[0].RoleID != "role_custom_invite" {
		t.Fatalf("want custom primary, got %+v", roles)
	}
}

func TestPhaseE_InviteInactiveCustomRejected(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"}
	svc := newPhaseEAssignSvc(t, repo, sub)
	seedPhaseECustomRole(t, repo, "role_custom_dead", "inactive")

	_, err := svc.InviteUser(context.Background(), caapp.InviteUserRequest{
		Subject: sub, Email: "invite.dead@example.com", FullName: "Invite Dead",
		CompanyID: "c_001", CreatedByUserID: sub.UserID, RoleID: "role_custom_dead",
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusUnprocessableEntity || he.Code != perr.CodeRoleInactive {
		t.Fatalf("want 422 role_inactive, got %v", err)
	}
}

func TestPhaseE_ListInviteRolesIncludesCustom(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"}
	svc := newPhaseEAssignSvc(t, repo, sub)
	seedPhaseECustomRole(t, repo, "role_custom_list", "active")
	seedEnterpriseInviteRoles(t, repo)

	items, err := svc.ListInviteRoles(context.Background(), caapp.ListInviteRolesRequest{
		Subject: sub, CompanyID: "c_001",
	})
	if err != nil {
		t.Fatalf("ListInviteRoles: %v", err)
	}
	found := false
	for _, it := range items {
		if it.RoleID == "role_custom_list" && it.RoleType == caapp.RoleTypeTenantCustom {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("custom role missing from invite list: %+v", items)
	}
}

func TestPhaseE_ProtectedTenantDefaultStillAssignable(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"}
	svc := newPhaseEAssignSvc(t, repo, sub)
	seedPhaseEAdminCapable(t, repo, sub.CompanyID, "role_admin_e")
	_ = repo.AddRole(context.Background(), sub.MembershipID, "role_admin_e")

	out, err := svc.InviteUser(context.Background(), caapp.InviteUserRequest{
		Subject: sub, Email: "default.ok@example.com", FullName: "Default OK",
		CompanyID: "c_001", CreatedByUserID: sub.UserID, RoleCode: "user_thuong",
	})
	if err != nil {
		t.Fatalf("InviteUser: %v", err)
	}
	err = svc.ReplaceMembershipPrimaryRole(context.Background(), caapp.ReplaceMembershipPrimaryRoleRequest{
		Subject: sub, MembershipID: out.MembershipID, RoleID: "role_admin_e",
	})
	if err != nil {
		t.Fatalf("assign protected default: %v", err)
	}
}
