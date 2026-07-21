package app_test

import (
	"context"
	"net/http"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func seedPhaseBTestFixtures(t *testing.T) (caapp.AdminService, *cainmem.AdminRepository, caapp.AdminSubject, string, string) {
	t.Helper()
	repo := cainmem.NewAdminRepository()

	perms := []caapp.PermissionListItem{
		{PermissionID: "disclosure.view", PermissionCode: "disclosure.view", PermissionName: "View", ModuleName: "disclosure"},
		{PermissionID: "rbac.manage", PermissionCode: "rbac.manage", PermissionName: "RBAC", ModuleName: "admin"},
		{PermissionID: "disclosure.publish", PermissionCode: "disclosure.publish", PermissionName: "Publish", ModuleName: "disclosure"},
		{PermissionID: "cms.template.read", PermissionCode: "cms.template.read", PermissionName: "CMS Read", ModuleName: "cms"},
	}
	for _, p := range perms {
		repo.SeedPermission(p)
	}

	protectedRoleID := "role_admin_dn"
	repo.SeedRole(caapp.RoleListItem{
		RoleID:      protectedRoleID,
		RoleCode:    "admin_doanh_nghiep",
		RoleName:    "Admin DN",
		Status:      "active",
		Scope:       "company",
		RoleType:    caapp.RoleTypeTenantDefault,
		IsProtected: true,
		IsBuiltin:   false,
	})

	customRoleID := "role_custom_ops"
	repo.SeedRole(caapp.RoleListItem{
		RoleID:      customRoleID,
		RoleCode:    "custom_ops",
		RoleName:    "Custom Ops",
		Status:      "active",
		Scope:       "company",
		RoleType:    caapp.RoleTypeTenantCustom,
		IsProtected: false,
		IsBuiltin:   false,
	})

	svc := caapp.NewAdminService(
		repo,
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"rbac.manage", "system.settings"}},
		fixedIDGen("u_owner"),
	)
	sub := caapp.AdminSubject{UserID: "u_owner", MembershipID: "m_owner", CompanyID: "c_001"}
	return svc, repo, sub, protectedRoleID, customRoleID
}

func assertHTTPCode(t *testing.T, err error, wantStatus int, wantCode perr.Code) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok {
		t.Fatalf("expected HTTPError, got %T: %v", err, err)
	}
	if he.HTTPStatus != wantStatus {
		t.Errorf("status=%d want %d", he.HTTPStatus, wantStatus)
	}
	if he.Code != wantCode {
		t.Errorf("code=%s want %s", he.Code, wantCode)
	}
}

func TestAssignRolePermission_ProtectedRoleReturns403(t *testing.T) {
	svc, _, sub, protectedRoleID, _ := seedPhaseBTestFixtures(t)
	err := svc.AssignRolePermission(context.Background(), caapp.AssignRolePermissionRequest{
		Subject: sub, RoleID: protectedRoleID, PermissionID: "disclosure.view",
	})
	assertHTTPCode(t, err, http.StatusForbidden, perr.CodeProtectedRoleReadOnly)
}

func TestRemoveRolePermission_ProtectedRoleReturns403(t *testing.T) {
	svc, _, sub, protectedRoleID, _ := seedPhaseBTestFixtures(t)
	err := svc.RemoveRolePermission(context.Background(), caapp.RemoveRolePermissionRequest{
		Subject: sub, RoleID: protectedRoleID, PermissionID: "rbac.manage",
	})
	assertHTTPCode(t, err, http.StatusForbidden, perr.CodeProtectedRoleReadOnly)
}

func TestAssignRolePermission_RejectTenantAdminOnlyOnCustomRole(t *testing.T) {
	svc, _, sub, _, customRoleID := seedPhaseBTestFixtures(t)
	err := svc.AssignRolePermission(context.Background(), caapp.AssignRolePermissionRequest{
		Subject: sub, RoleID: customRoleID, PermissionID: "rbac.manage",
	})
	assertHTTPCode(t, err, http.StatusUnprocessableEntity, perr.CodePermissionNotGrantable)
}

func TestAssignRolePermission_RejectHighRiskOnCustomRole(t *testing.T) {
	svc, _, sub, _, customRoleID := seedPhaseBTestFixtures(t)
	err := svc.AssignRolePermission(context.Background(), caapp.AssignRolePermissionRequest{
		Subject: sub, RoleID: customRoleID, PermissionID: "disclosure.publish",
	})
	assertHTTPCode(t, err, http.StatusUnprocessableEntity, perr.CodeHighRiskPermissionRequiresApproval)
}

func TestAssignRolePermission_RejectSystemOnlyOnCustomRole(t *testing.T) {
	svc, _, sub, _, customRoleID := seedPhaseBTestFixtures(t)
	err := svc.AssignRolePermission(context.Background(), caapp.AssignRolePermissionRequest{
		Subject: sub, RoleID: customRoleID, PermissionID: "cms.template.read",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok {
		t.Fatalf("expected HTTPError: %v", err)
	}
	if he.Code != perr.CodePermissionOutOfEnterpriseScope && he.Code != perr.CodePermissionNotGrantable {
		t.Fatalf("unexpected code %s", he.Code)
	}
}

func TestAssignRolePermission_AllowGrantableOnCustomRole(t *testing.T) {
	svc, _, sub, _, customRoleID := seedPhaseBTestFixtures(t)
	err := svc.AssignRolePermission(context.Background(), caapp.AssignRolePermissionRequest{
		Subject: sub, RoleID: customRoleID, PermissionID: "disclosure.view",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListGrantablePermissions_IncludesBlockedTiers(t *testing.T) {
	svc, _, sub, _, _ := seedPhaseBTestFixtures(t)
	items, err := svc.ListGrantablePermissions(context.Background(), caapp.AdminSubjectRequest{Subject: sub})
	if err != nil {
		t.Fatalf("ListGrantablePermissions: %v", err)
	}
	foundGrantable := false
	foundBlocked := false
	for _, it := range items {
		if it.PermissionCode == "disclosure.view" && it.GrantTier == caapp.GrantTierGrantable {
			foundGrantable = true
		}
		if it.PermissionCode == "rbac.manage" && it.GrantTier == caapp.GrantTierTenantAdminOnly {
			foundBlocked = true
		}
	}
	if !foundGrantable {
		t.Fatal("disclosure.view grantable missing")
	}
	if !foundBlocked {
		t.Fatal("rbac.manage blocked tier missing")
	}
}
