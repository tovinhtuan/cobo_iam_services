package app_test

import (
	"context"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

// ROLE-07 / RP-05: platform.cms.view alone does not make a platform operator. A member that
// holds it plus the invite permission is a tenant inviter: platform roles stay out of reach.

func newWeakCMSFixture(t *testing.T, callerPerms []string) caapp.AdminService {
	t.Helper()
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_inv", MembershipID: "m_inv", CompanyID: "c_001"}
	seedInviteScopedSubject(t, repo, sub)
	repo.SeedRole(caapp.RoleListItem{
		RoleID: "r_cmsview", RoleCode: "custom_cms_view", RoleName: "CMS viewer",
		Status: "active", Scope: "company", RoleType: caapp.RoleTypeTenantCustom,
	})
	_ = repo.AddRolePermission(context.Background(), "r_cmsview", "platform.cms.view")
	return caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow, permissions: callerPerms}, idgen.UUIDv7Generator{})
}

var weakCMSSubject = caapp.AdminSubject{UserID: "u_inv", MembershipID: "m_inv", CompanyID: "c_001"}

func TestInvite_CMSViewOnly_CannotGrantPlatformRole(t *testing.T) {
	svc := newWeakCMSFixture(t, []string{"platform.cms.view", "admin.membership.invite"})
	_, err := svc.InviteUser(context.Background(), caapp.InviteUserRequest{
		Subject: weakCMSSubject, Email: "cms.escalate@example.com", FullName: "X", CompanyID: "c_001",
		CreatedByUserID: "u_inv", RoleID: "r_cmsview",
	})
	if err == nil {
		t.Fatal("a member with only platform.cms.view must not hand out a platform role")
	}
}

func TestListInviteRoles_CMSViewOnly_HidesPlatformRoles(t *testing.T) {
	svc := newWeakCMSFixture(t, []string{"platform.cms.view", "admin.membership.invite"})
	items, err := svc.ListInviteRoles(context.Background(), caapp.ListInviteRolesRequest{Subject: weakCMSSubject})
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if it.RoleID == "r_cmsview" {
			t.Fatalf("picker offers a platform role to a non-operator: %+v", it)
		}
	}
}

func TestInvite_PlatformOperator_StillGrantsPlatformRole(t *testing.T) {
	svc := newWeakCMSFixture(t, []string{"platform.cms.view", "rbac.manage", "admin.membership.invite"})
	if _, err := svc.InviteUser(context.Background(), caapp.InviteUserRequest{
		Subject: weakCMSSubject, Email: "cms.op@example.com", FullName: "Y", CompanyID: "c_001",
		CreatedByUserID: "u_inv", RoleID: "r_cmsview",
	}); err != nil {
		t.Fatalf("a platform operator (platform.cms.view + rbac.manage) keeps provisioning platform roles: %v", err)
	}
}
