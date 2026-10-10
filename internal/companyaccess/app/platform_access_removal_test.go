package app_test

import (
	"context"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	"github.com/cobo/cobo_iam_services/internal/companyaccess/configversion"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// ROLE-13: a tenant admin cannot grant platform access, so it must not be able to take it
// away from the platform operators either. Only a platform operator may.

func newPlatformRemovalFixture(t *testing.T, callerPerms []string) (*cainmem.AdminRepository, caapp.AdminService, caapp.AdminSubject) {
	t.Helper()
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_adm", MembershipID: "m_adm", CompanyID: "c_001"}
	seedInviteScopedSubject(t, repo, sub)
	seedInviteScopedSubject(t, repo, caapp.AdminSubject{UserID: "u_op", MembershipID: "m_op", CompanyID: "c_001"})
	repo.SeedRoleForCompany(caapp.RoleListItem{RoleID: "r_cms", RoleCode: "cms_ops_custom", RoleName: "CMS ops", Status: "active"}, "c_001")
	_ = repo.AddRolePermission(context.Background(), "r_cms", "platform.cms.view")
	if err := repo.AddRole(context.Background(), "m_op", "r_cms"); err != nil {
		t.Fatal(err)
	}
	if err := repo.InsertDirectPermission(context.Background(), "m_op", "c_001", "platform.cms.view", "u_seed"); err != nil {
		t.Fatal(err)
	}
	svc := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow, permissions: callerPerms}, fixedIDGen("test-id"))
	return repo, svc, sub
}

func TestRemoveRole_TenantAdminCannotRemovePlatformRole(t *testing.T) {
	repo, svc, sub := newPlatformRemovalFixture(t, []string{"rbac.manage"})
	err := svc.RemoveRole(context.Background(), caapp.RemoveRoleRequest{Subject: sub, MembershipID: "m_op", RoleID: "r_cms"})
	requireHTTPCode(t, err, 403, perr.CodePermissionDenied)
	roles, _ := repo.ListMembershipRoles(context.Background(), "m_op")
	if len(roles) == 0 {
		t.Fatal("platform role was removed")
	}
}

func TestRemoveRole_PlatformOperatorCanRemovePlatformRole(t *testing.T) {
	_, svc, sub := newPlatformRemovalFixture(t, []string{"rbac.manage", "platform.cms.view"})
	if err := svc.RemoveRole(context.Background(), caapp.RemoveRoleRequest{Subject: sub, MembershipID: "m_op", RoleID: "r_cms"}); err != nil {
		t.Fatalf("platform operator: %v", err)
	}
}

func TestRemoveDirectPermission_TenantAdminCannotRemovePlatformPermission(t *testing.T) {
	repo, svc, sub := newPlatformRemovalFixture(t, []string{"rbac.manage"})
	err := svc.RemoveDirectPermission(context.Background(), caapp.RemoveDirectPermissionRequest{Subject: sub, MembershipID: "m_op", PermissionCode: "platform.cms.view"})
	requireHTTPCode(t, err, 403, perr.CodePermissionDenied)
	if ok, _ := repo.HasActiveDirectPermission(context.Background(), "m_op", "platform.cms.view"); !ok {
		t.Fatal("platform direct permission was revoked")
	}
}

func TestSubmitDirectRemoveApproval_TenantAdminCannotQueuePlatformPermission(t *testing.T) {
	_, svc, sub := newPlatformRemovalFixture(t, []string{"rbac.manage"})
	_, err := svc.SubmitConfigApproval(context.Background(), caapp.SubmitConfigApprovalRequest{
		Subject: sub, ChangeType: configversion.ChangeTypeRBACDirectPermRemove,
		Proposed: map[string]any{"membership_id": "m_op", "permission_code": "platform.cms.view"},
	})
	requireHTTPCode(t, err, 403, perr.CodePermissionDenied)
}

// BES-21: deactivating or deleting a platform operator's membership removes its platform
// access too, so a tenant admin may not do it.
func newPlatformMemberFixture(t *testing.T, callerPerms []string) (*cainmem.AdminRepository, caapp.AdminService, caapp.AdminSubject) {
	t.Helper()
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_adm", MembershipID: "m_adm", CompanyID: "c_001"}
	seedMem(repo, "m_adm", "u_adm", "c_001")
	seedMem(repo, "m_op", "u_op", "c_001")
	seedMem(repo, "m_staff", "u_staff", "c_001")
	auth := perMemberAuth{byMembership: map[string][]string{
		"m_adm": callerPerms, "m_op": {"platform.cms.view", "rbac.manage"}, "m_staff": {"disclosure.view"},
	}}
	return repo, caapp.NewAdminService(repo, auth, fixedIDGen("test-id")), sub
}

func TestUpdateMembership_TenantAdminCannotDeactivatePlatformOperator(t *testing.T) {
	repo, svc, sub := newPlatformMemberFixture(t, []string{"rbac.manage"})
	_, err := svc.UpdateMembership(context.Background(), caapp.UpdateMembershipRequest{Subject: sub, MembershipID: "m_op", Status: "inactive"})
	requireHTTPCode(t, err, 403, perr.CodePermissionDenied)
	if m, _ := repo.GetMembershipByID(context.Background(), "m_op"); m.Status == "inactive" {
		t.Fatal("platform operator was deactivated")
	}
}

func TestDeleteMembership_TenantAdminCannotDeletePlatformOperator(t *testing.T) {
	_, svc, sub := newPlatformMemberFixture(t, []string{"rbac.manage"})
	err := svc.DeleteMembership(context.Background(), caapp.DeleteMembershipRequest{Subject: sub, MembershipID: "m_op"})
	requireHTTPCode(t, err, 403, perr.CodePermissionDenied)
}

func TestUpdateMembership_TenantAdminCanDeactivateStaff(t *testing.T) {
	_, svc, sub := newPlatformMemberFixture(t, []string{"rbac.manage"})
	if _, err := svc.UpdateMembership(context.Background(), caapp.UpdateMembershipRequest{Subject: sub, MembershipID: "m_staff", Status: "inactive"}); err != nil {
		t.Fatalf("deactivating ordinary staff: %v", err)
	}
}

func TestUpdateMembership_PlatformOperatorCanDeactivatePlatformOperator(t *testing.T) {
	_, svc, sub := newPlatformMemberFixture(t, []string{"rbac.manage", "platform.cms.view"})
	if _, err := svc.UpdateMembership(context.Background(), caapp.UpdateMembershipRequest{Subject: sub, MembershipID: "m_op", Status: "inactive"}); err != nil {
		t.Fatalf("platform operator: %v", err)
	}
}

// ROLE-19: a queued removal of a platform permission cannot be approved by a tenant admin.
func TestApproveDirectRemove_TenantAdminCannotApprovePlatformPermissionRemoval(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	op := caapp.AdminSubject{UserID: "u_op1", MembershipID: "m_op1", CompanyID: "c_001"}
	adm := caapp.AdminSubject{UserID: "u_adm", MembershipID: "m_adm", CompanyID: "c_001"}
	seedInviteScopedSubject(t, repo, op)
	seedInviteScopedSubject(t, repo, adm)
	seedInviteScopedSubject(t, repo, caapp.AdminSubject{UserID: "u_op", MembershipID: "m_op", CompanyID: "c_001"})
	if err := repo.InsertDirectPermission(context.Background(), "m_op", "c_001", "platform.cms.view", "u_seed"); err != nil {
		t.Fatal(err)
	}
	auth := perMemberAuth{byMembership: map[string][]string{
		"m_op1": {"platform.cms.view", "rbac.manage"}, "m_adm": {"rbac.manage"}, "m_op": {"platform.cms.view"},
	}}
	svc := caapp.NewAdminService(repo, auth, &bgIDGen{})
	if _, err := svc.SubmitConfigApproval(context.Background(), caapp.SubmitConfigApprovalRequest{
		Subject: op, ChangeType: configversion.ChangeTypeRBACDirectPermRemove,
		Proposed: map[string]any{"membership_id": "m_op", "permission_code": "platform.cms.view"},
	}); err != nil {
		t.Fatalf("operator queues the removal: %v", err)
	}
	pending, err := svc.ListConfigApprovals(context.Background(), caapp.ListConfigApprovalsRequest{Subject: adm, Status: "pending"})
	if err != nil || len(pending.Items) != 1 {
		t.Fatalf("pending: %v %+v", err, pending)
	}
	_, err = svc.ApproveConfigApproval(context.Background(), caapp.ApproveConfigApprovalRequest{Subject: adm, ApprovalID: pending.Items[0].ApprovalID})
	requireHTTPCode(t, err, 403, perr.CodePermissionDenied)
	if ok, _ := repo.HasActiveDirectPermission(context.Background(), "m_op", "platform.cms.view"); !ok {
		t.Fatal("platform permission was removed by a tenant admin's approval")
	}
}

// Regression from ROLE-13: disclosure_type.manage is tagged module "cms" in the catalog but is a
// tenant-grantable permission. A role carrying it (e.g. admin_doanh_nghiep) must stay removable by
// a company admin, otherwise an owner cannot demote another admin.
func TestRemoveRole_TenantAdminCanRemoveRoleWithCmsModuleTenantPermission(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_adm", MembershipID: "m_adm", CompanyID: "c_001"}
	seedInviteScopedSubject(t, repo, sub)
	seedInviteScopedSubject(t, repo, caapp.AdminSubject{UserID: "u_x", MembershipID: "m_x", CompanyID: "c_001"})
	repo.SeedPermission(caapp.PermissionListItem{PermissionID: "disclosure_type.manage", PermissionCode: "disclosure_type.manage", ModuleName: "cms"})
	repo.SeedRoleForCompany(caapp.RoleListItem{RoleID: "r_adm2", RoleCode: "admin_doanh_nghiep", RoleName: "Admin", Status: "active", RoleType: caapp.RoleTypeTenantCustom}, "c_001")
	_ = repo.AddRolePermission(context.Background(), "r_adm2", "disclosure_type.manage")
	if err := repo.AddRole(context.Background(), "m_x", "r_adm2"); err != nil {
		t.Fatal(err)
	}
	svc := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"rbac.manage"}}, fixedIDGen("test-id"))
	if err := svc.RemoveRole(context.Background(), caapp.RemoveRoleRequest{Subject: sub, MembershipID: "m_x", RoleID: "r_adm2"}); err != nil {
		t.Fatalf("a role with a tenant permission tagged module cms must be removable by a company admin: %v", err)
	}
}
