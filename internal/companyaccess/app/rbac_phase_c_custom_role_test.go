package app_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func seedPhaseCFixtures(t *testing.T) (caapp.AdminService, *cainmem.AdminRepository, caapp.AdminSubject, string) {
	t.Helper()
	repo := cainmem.NewAdminRepository()
	for _, p := range []caapp.PermissionListItem{
		{PermissionID: "disclosure.view", PermissionCode: "disclosure.view", PermissionName: "View", ModuleName: "disclosure"},
		{PermissionID: "workflow.read", PermissionCode: "workflow.read", PermissionName: "WF Read", ModuleName: "workflow"},
		{PermissionID: "rbac.manage", PermissionCode: "rbac.manage", PermissionName: "RBAC", ModuleName: "admin"},
		{PermissionID: "system.settings", PermissionCode: "system.settings", PermissionName: "Settings", ModuleName: "admin"},
		{PermissionID: "disclosure.publish", PermissionCode: "disclosure.publish", PermissionName: "Publish", ModuleName: "disclosure"},
		{PermissionID: "admin.membership.invite", PermissionCode: "admin.membership.invite", PermissionName: "Invite", ModuleName: "admin"},
		{PermissionID: "cms.template.read", PermissionCode: "cms.template.read", PermissionName: "CMS", ModuleName: "cms"},
		{PermissionID: "ad_hoc_alert.admin_review", PermissionCode: "ad_hoc_alert.admin_review", PermissionName: "Deprecated", ModuleName: "ad_hoc"},
		{PermissionID: "workflow.step.override", PermissionCode: "workflow.step.override", PermissionName: "Override", ModuleName: "workflow"},
		{PermissionID: "template.workflow.override.reset", PermissionCode: "template.workflow.override.reset", PermissionName: "Reset", ModuleName: "template"},
	} {
		repo.SeedPermission(p)
	}

	protectedID := "role_admin_dn"
	repo.SeedRole(caapp.RoleListItem{
		RoleID: protectedID, RoleCode: "admin_doanh_nghiep", RoleName: "Admin DN",
		Status: "active", Scope: "company", RoleType: caapp.RoleTypeTenantDefault, IsProtected: true,
	})
	for _, pid := range []string{
		"disclosure.view", "workflow.read", "rbac.manage", "system.settings",
		"disclosure.publish", "admin.membership.invite", "cms.template.read",
		"ad_hoc_alert.admin_review", "workflow.step.override", "template.workflow.override.reset",
	} {
		_ = repo.AddRolePermission(context.Background(), protectedID, pid)
	}

	svc := caapp.NewAdminService(
		repo,
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"rbac.manage"}},
		fixedIDGen("role_custom_gen"),
	)
	sub := caapp.AdminSubject{UserID: "u_owner", MembershipID: "m_owner", CompanyID: "c_001"}
	return svc, repo, sub, protectedID
}

func TestCreateCustomRole_Success(t *testing.T) {
	svc, _, sub, _ := seedPhaseCFixtures(t)
	role, err := svc.CreateCustomRole(context.Background(), caapp.CreateCustomRoleRequest{
		Subject: sub, RoleName: "Reviewer Custom", Description: "ops",
	})
	if err != nil {
		t.Fatalf("CreateCustomRole: %v", err)
	}
	if role.RoleType != caapp.RoleTypeTenantCustom {
		t.Errorf("role_type=%s", role.RoleType)
	}
	if role.IsProtected {
		t.Error("expected is_protected=false")
	}
	if !role.IsEditable {
		t.Error("expected is_editable=true")
	}
	if !strings.HasPrefix(role.RoleCode, "custom_") {
		t.Errorf("role_code=%s", role.RoleCode)
	}
	if role.PermissionCount != 0 {
		t.Errorf("permission_count=%d", role.PermissionCount)
	}
}

func TestCreateCustomRole_EmptyNameRejected(t *testing.T) {
	svc, _, sub, _ := seedPhaseCFixtures(t)
	_, err := svc.CreateCustomRole(context.Background(), caapp.CreateCustomRoleRequest{Subject: sub, RoleName: "  "})
	assertHTTPCode(t, err, http.StatusUnprocessableEntity, perr.CodeInvalidRoleName)
}

func TestUpdateCustomRole_MetadataSuccess(t *testing.T) {
	svc, _, sub, _ := seedPhaseCFixtures(t)
	created, err := svc.CreateCustomRole(context.Background(), caapp.CreateCustomRoleRequest{
		Subject: sub, RoleName: "Temp", Description: "a",
	})
	if err != nil {
		t.Fatal(err)
	}
	newName := "Temp Updated"
	newDesc := "b"
	updated, err := svc.UpdateCustomRole(context.Background(), caapp.UpdateCustomRoleRequest{
		Subject: sub, RoleID: created.RoleID, RoleName: &newName, Description: &newDesc,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.RoleName != newName || updated.Description != newDesc {
		t.Errorf("got name=%q desc=%q", updated.RoleName, updated.Description)
	}
	if updated.RoleCode != created.RoleCode {
		t.Error("role_code must be immutable")
	}
}

func TestUpdateCustomRole_ProtectedReturns403(t *testing.T) {
	svc, _, sub, protectedID := seedPhaseCFixtures(t)
	name := "Hacked"
	_, err := svc.UpdateCustomRole(context.Background(), caapp.UpdateCustomRoleRequest{
		Subject: sub, RoleID: protectedID, RoleName: &name,
	})
	assertHTTPCode(t, err, http.StatusForbidden, perr.CodeProtectedRoleReadOnly)
}

func TestInactivateCustomRole_Success(t *testing.T) {
	svc, repo, sub, _ := seedPhaseCFixtures(t)
	created, err := svc.CreateCustomRole(context.Background(), caapp.CreateCustomRoleRequest{
		Subject: sub, RoleName: "Disposable",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.InactivateCustomRole(context.Background(), caapp.InactivateCustomRoleRequest{
		Subject: sub, RoleID: created.RoleID,
	}); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.GetCompanyRoleByID(context.Background(), sub.CompanyID, created.RoleID)
	if got == nil || got.Status != "inactive" {
		t.Fatalf("expected inactive, got %+v", got)
	}
}

func TestInactivateCustomRole_ProtectedReturns403(t *testing.T) {
	svc, _, sub, protectedID := seedPhaseCFixtures(t)
	err := svc.InactivateCustomRole(context.Background(), caapp.InactivateCustomRoleRequest{
		Subject: sub, RoleID: protectedID,
	})
	assertHTTPCode(t, err, http.StatusForbidden, perr.CodeProtectedRoleReadOnly)
}

func TestInactivateCustomRole_InUseReturns409(t *testing.T) {
	svc, repo, sub, _ := seedPhaseCFixtures(t)
	created, err := svc.CreateCustomRole(context.Background(), caapp.CreateCustomRoleRequest{
		Subject: sub, RoleName: "In Use Role",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.AddRole(context.Background(), "m_member_1", created.RoleID); err != nil {
		t.Fatal(err)
	}
	err = svc.InactivateCustomRole(context.Background(), caapp.InactivateCustomRoleRequest{
		Subject: sub, RoleID: created.RoleID,
	})
	assertHTTPCode(t, err, http.StatusConflict, perr.CodeRoleInUse)
}

func TestCloneRole_FiltersGrantableOnly(t *testing.T) {
	svc, repo, sub, protectedID := seedPhaseCFixtures(t)
	beforeView, err := repo.ListRolePermissions(context.Background(), sub.CompanyID, protectedID)
	if err != nil {
		t.Fatal(err)
	}
	beforeCount := len(beforeView.Permissions)

	out, err := svc.CloneRole(context.Background(), caapp.CloneRoleRequest{
		Subject: sub, SourceRoleID: protectedID, RoleName: "Clone From Admin", Description: "filtered",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Role.RoleType != caapp.RoleTypeTenantCustom || out.Role.IsProtected {
		t.Fatalf("clone role classification: %+v", out.Role)
	}
	if out.CopySummary.CopiedCount < 1 {
		t.Fatalf("expected copied grantable perms, summary=%+v", out.CopySummary)
	}
	copied := map[string]bool{}
	for _, c := range out.CopySummary.CopiedPermissions {
		copied[c.PermissionCode] = true
		if c.GrantTier != caapp.GrantTierGrantable {
			t.Errorf("copied non-grantable %s", c.PermissionCode)
		}
	}
	if !copied["disclosure.view"] || !copied["workflow.read"] {
		t.Errorf("expected disclosure.view and workflow.read copied: %v", copied)
	}

	skipped := map[string]string{}
	for _, s := range out.CopySummary.SkippedPermissions {
		skipped[s.PermissionCode] = string(s.GrantTier)
	}
	for _, code := range []string{"rbac.manage", "system.settings", "disclosure.publish", "admin.membership.invite", "cms.template.read", "ad_hoc_alert.admin_review", "workflow.step.override", "template.workflow.override.reset"} {
		if _, ok := skipped[code]; !ok {
			t.Errorf("expected skip %s", code)
		}
		if copied[code] {
			t.Errorf("must not copy %s", code)
		}
	}

	afterView, err := repo.ListRolePermissions(context.Background(), sub.CompanyID, protectedID)
	if err != nil {
		t.Fatal(err)
	}
	if len(afterView.Permissions) != beforeCount {
		t.Errorf("source role mutated: before=%d after=%d", beforeCount, len(afterView.Permissions))
	}
}

func TestCloneRole_CrossTenantSourceForbidden(t *testing.T) {
	svc, repo, sub, _ := seedPhaseCFixtures(t)
	// Role not in repo roles map → inaccessible
	otherID := "role_other_company"
	_ = repo // keep for clarity
	_, err := svc.CloneRole(context.Background(), caapp.CloneRoleRequest{
		Subject: sub, SourceRoleID: otherID, RoleName: "Should Fail",
	})
	assertHTTPCode(t, err, http.StatusNotFound, perr.CodeNotFound)
}
