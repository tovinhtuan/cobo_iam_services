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

func TestInviteUser_RoleIDsRejected(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"}
	svc := newEnterpriseInviteSvc(t, repo, sub)

	_, err := svc.InviteUser(context.Background(), caapp.InviteUserRequest{
		Subject:         sub,
		Email:           "multi.role@example.com",
		FullName:        "Multi Role",
		CompanyID:       "c_001",
		CreatedByUserID: sub.UserID,
		RoleIDs:         []string{"r_invite_user_thuong", "r_invite_company_admin"},
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("want 400 invalid_request, got %v", err)
	}
}

func TestInviteUser_SingleRoleSuccess(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"}
	svc := newEnterpriseInviteSvc(t, repo, sub)

	out, err := svc.InviteUser(context.Background(), caapp.InviteUserRequest{
		Subject:         sub,
		Email:           "single.role@example.com",
		FullName:        "Single Role",
		CompanyID:       "c_001",
		CreatedByUserID: sub.UserID,
		RoleCode:        "user_thuong",
	})
	if err != nil {
		t.Fatalf("InviteUser: %v", err)
	}
	roles, err := repo.ListMembershipRoles(context.Background(), out.MembershipID)
	if err != nil {
		t.Fatalf("ListMembershipRoles: %v", err)
	}
	if len(roles) != 1 || roles[0].RoleCode != "user_thuong" {
		t.Fatalf("roles=%+v want single user_thuong", roles)
	}
}

func TestReplaceMembershipPrimaryRole_Success(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"}
	svc := newEnterpriseInviteSvc(t, repo, sub)

	out, err := svc.InviteUser(context.Background(), caapp.InviteUserRequest{
		Subject:         sub,
		Email:           "replace.role@example.com",
		FullName:        "Replace Role",
		CompanyID:       "c_001",
		CreatedByUserID: sub.UserID,
		RoleCode:        "user_thuong",
	})
	if err != nil {
		t.Fatalf("InviteUser: %v", err)
	}

	err = svc.ReplaceMembershipPrimaryRole(context.Background(), caapp.ReplaceMembershipPrimaryRoleRequest{
		Subject: sub, MembershipID: out.MembershipID, RoleID: "r_invite_company_admin",
	})
	if err != nil {
		t.Fatalf("ReplaceMembershipPrimaryRole: %v", err)
	}

	roles, err := repo.ListMembershipRoles(context.Background(), out.MembershipID)
	if err != nil {
		t.Fatalf("ListMembershipRoles: %v", err)
	}
	codes := make([]string, 0, len(roles))
	for _, r := range roles {
		codes = append(codes, r.RoleCode)
	}
	if len(codes) != 1 || codes[0] != "company_admin" {
		t.Fatalf("roles=%v want [company_admin]", codes)
	}
}

func TestAssignRole_SecondPrimaryRejected(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"}
	svc := newEnterpriseInviteSvc(t, repo, sub)

	out, err := svc.InviteUser(context.Background(), caapp.InviteUserRequest{
		Subject:         sub,
		Email:           "second.role@example.com",
		FullName:        "Second Role",
		CompanyID:       "c_001",
		CreatedByUserID: sub.UserID,
		RoleCode:        "user_thuong",
	})
	if err != nil {
		t.Fatalf("InviteUser: %v", err)
	}

	err = svc.AssignRole(context.Background(), caapp.AssignRoleRequest{
		Subject: sub, MembershipID: out.MembershipID, RoleID: "r_invite_company_admin",
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("want 400, got %v", err)
	}
}

func TestReplaceMembershipPrimaryRole_DeptLeadRejected(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"}
	svc := newEnterpriseInviteSvc(t, repo, sub)

	out, err := svc.InviteUser(context.Background(), caapp.InviteUserRequest{
		Subject:         sub,
		Email:           "legacy.role@example.com",
		FullName:        "Legacy Role",
		CompanyID:       "c_001",
		CreatedByUserID: sub.UserID,
		RoleCode:        "user_thuong",
	})
	if err != nil {
		t.Fatalf("InviteUser: %v", err)
	}

	err = svc.ReplaceMembershipPrimaryRole(context.Background(), caapp.ReplaceMembershipPrimaryRoleRequest{
		Subject: sub, MembershipID: out.MembershipID, RoleID: "r_invite_dept_lead",
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("want 400, got %v", err)
	}
}

func TestReplaceMembershipPrimaryRole_KeepsLegacyRoles(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"}
	svc := newEnterpriseInviteSvc(t, repo, sub)

	out, err := svc.InviteUser(context.Background(), caapp.InviteUserRequest{
		Subject:         sub,
		Email:           "legacy.keep@example.com",
		FullName:        "Legacy Keep",
		CompanyID:       "c_001",
		CreatedByUserID: sub.UserID,
		RoleCode:        "user_thuong",
	})
	if err != nil {
		t.Fatalf("InviteUser: %v", err)
	}
	if err := repo.AddRole(context.Background(), out.MembershipID, "r_invite_dept_lead"); err != nil {
		t.Fatalf("AddRole legacy: %v", err)
	}
	if err := repo.AddRole(context.Background(), out.MembershipID, "r_invite_company_admin"); err != nil {
		t.Fatalf("AddRole extra: %v", err)
	}

	err = svc.ReplaceMembershipPrimaryRole(context.Background(), caapp.ReplaceMembershipPrimaryRoleRequest{
		Subject: sub, MembershipID: out.MembershipID, RoleID: "r_invite_company_admin",
	})
	if err != nil {
		t.Fatalf("ReplaceMembershipPrimaryRole: %v", err)
	}

	roles, err := repo.ListMembershipRoles(context.Background(), out.MembershipID)
	if err != nil {
		t.Fatalf("ListMembershipRoles: %v", err)
	}
	codes := map[string]struct{}{}
	for _, r := range roles {
		codes[r.RoleCode] = struct{}{}
	}
	if _, ok := codes["dept_lead"]; !ok {
		t.Fatal("legacy dept_lead should remain")
	}
	if _, ok := codes["company_admin"]; !ok {
		t.Fatal("company_admin should remain as primary")
	}
}

func TestInviteUser_AdditionalPermissionsIndependent(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"}
	svc := caapp.NewAdminService(
		repo,
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"system.settings", "admin.membership.invite"}},
		idgen.UUIDv7Generator{},
	)
	seedInviteScopedSubject(t, repo, sub)
	seedEnterpriseInviteRoles(t, repo)
	if err := repo.AddRolePermission(context.Background(), "ad_hoc_alert.propose", "ad_hoc_alert.propose"); err != nil {
		t.Fatalf("seed permission: %v", err)
	}

	out, err := svc.InviteUser(context.Background(), caapp.InviteUserRequest{
		Subject:         sub,
		Email:           "perm.only@example.com",
		FullName:        "Perm Only",
		CompanyID:       "c_001",
		CreatedByUserID: sub.UserID,
		RoleCode:        "user_thuong",
		Permissions:     []string{"ad_hoc_alert.propose"},
	})
	if err != nil {
		t.Fatalf("InviteUser: %v", err)
	}
	found, err := repo.HasActiveDirectPermission(context.Background(), out.MembershipID, "ad_hoc_alert.propose")
	if err != nil {
		t.Fatalf("HasActiveDirectPermission: %v", err)
	}
	if !found {
		t.Fatal("expected ad_hoc_alert.propose direct permission")
	}
}
