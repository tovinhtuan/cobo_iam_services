package app_test

// Tests for enterprise RBAC scope enforcement:
//   - ListPermissions must not return CMS/Platform permissions.
//   - AssignRolePermission must reject CMS/Platform permission IDs.
//   - ListRolePermissions must not return CMS/Platform permissions even if stored.
//   - Enterprise-scoped permissions pass through unblocked.

import (
	"context"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// cmsBlockedCodes lists permission codes that must not appear in enterprise RBAC.
var cmsBlockedCodes = []string{
	"cms.template.read",
	"cms.template.write",
	"cms.template.activate",
	"cms.template.archive",
	"cms.template.config.write",
	"disclosure_type.config.read",
	"disclosure_type.config.write",
	"platform.cms.view",
}

func buildScopeTestService(t *testing.T) (caapp.AdminService, *cainmem.AdminRepository, caapp.AdminSubject) {
	t.Helper()
	repo := cainmem.NewAdminRepository()

	// Seed CMS permissions with proper module names
	for _, code := range cmsBlockedCodes {
		moduleName := "cms"
		if code == "platform.cms.view" {
			moduleName = "platform"
		}
		repo.SeedPermission(caapp.PermissionListItem{
			PermissionID:   code,
			PermissionCode: code,
			PermissionName: code,
			ModuleName:     moduleName,
		})
	}

	// Seed enterprise permissions
	enterprisePerms := []struct{ code, module string }{
		{"rbac.manage", "admin"},
		{"company.view", "company"},
		{"dept.manage", "org"},
		{"disclosure_type.manage", "disclosure"},
		{"template.workflow.override.read", "template"},
	}
	for _, ep := range enterprisePerms {
		repo.SeedPermission(caapp.PermissionListItem{
			PermissionID:   ep.code,
			PermissionCode: ep.code,
			PermissionName: ep.code,
			ModuleName:     ep.module,
		})
	}

	svc := caapp.NewAdminService(
		repo,
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"rbac.manage", "system.settings"}},
		fixedIDGen("u_owner"),
	)
	sub := caapp.AdminSubject{UserID: "u_owner", MembershipID: "m_owner", CompanyID: "c_001"}
	return svc, repo, sub
}

func seedUnprotectedCustomRole(repo *cainmem.AdminRepository) string {
	roleID := "custom_ops_role"
	repo.SeedRole(caapp.RoleListItem{
		RoleID:      roleID,
		RoleCode:    "custom_ops",
		RoleName:    "Custom Ops",
		Status:      "active",
		Scope:       "company",
		RoleType:    caapp.RoleTypeTenantCustom,
		IsProtected: false,
		IsBuiltin:   false,
	})
	return roleID
}

func assertProtectedRoleReadOnly(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok {
		t.Fatalf("expected HTTPError, got %T: %v", err, err)
	}
	if he.Code != perr.CodeProtectedRoleReadOnly {
		t.Errorf("expected protected_role_read_only, got %s", he.Code)
	}
}

// TestListPermissions_NoCMSModule verifies enterprise RBAC matrix has no 'cms' module.
func TestListPermissions_NoCMSModule(t *testing.T) {
	svc, _, sub := buildScopeTestService(t)
	perms, err := svc.ListPermissions(context.Background(), caapp.AdminSubjectRequest{Subject: sub})
	if err != nil {
		t.Fatalf("ListPermissions: %v", err)
	}
	for _, p := range perms {
		if p.ModuleName == "cms" {
			t.Errorf("ListPermissions returned cms module permission: %s", p.PermissionCode)
		}
	}
}

// TestListPermissions_NoPlatformModule verifies enterprise RBAC matrix has no 'platform' module.
func TestListPermissions_NoPlatformModule(t *testing.T) {
	svc, _, sub := buildScopeTestService(t)
	perms, err := svc.ListPermissions(context.Background(), caapp.AdminSubjectRequest{Subject: sub})
	if err != nil {
		t.Fatalf("ListPermissions: %v", err)
	}
	for _, p := range perms {
		if p.ModuleName == "platform" {
			t.Errorf("ListPermissions returned platform module permission: %s", p.PermissionCode)
		}
	}
}

// TestListPermissions_BlockedCodes verifies each specific blocked code is absent from list.
func TestListPermissions_BlockedCodes(t *testing.T) {
	svc, _, sub := buildScopeTestService(t)
	perms, err := svc.ListPermissions(context.Background(), caapp.AdminSubjectRequest{Subject: sub})
	if err != nil {
		t.Fatalf("ListPermissions: %v", err)
	}
	codeSet := make(map[string]bool, len(perms))
	for _, p := range perms {
		codeSet[p.PermissionCode] = true
	}
	for _, blocked := range cmsBlockedCodes {
		if codeSet[blocked] {
			t.Errorf("ListPermissions returned blocked permission: %s", blocked)
		}
	}
}

// TestListPermissions_EnterprisePermissionsPresent verifies enterprise permissions pass through.
func TestListPermissions_EnterprisePermissionsPresent(t *testing.T) {
	svc, _, sub := buildScopeTestService(t)
	perms, err := svc.ListPermissions(context.Background(), caapp.AdminSubjectRequest{Subject: sub})
	if err != nil {
		t.Fatalf("ListPermissions: %v", err)
	}
	codeSet := make(map[string]bool, len(perms))
	for _, p := range perms {
		codeSet[p.PermissionCode] = true
	}
	keepCodes := []string{"rbac.manage", "company.view", "dept.manage", "disclosure_type.manage", "template.workflow.override.read"}
	for _, code := range keepCodes {
		if !codeSet[code] {
			t.Errorf("ListPermissions missing enterprise permission: %s", code)
		}
	}
}

// TestAssignRolePermission_RejectCMSTemplateRead verifies cms.template.read is rejected.
func TestAssignRolePermission_RejectCMSTemplateRead(t *testing.T) {
	svc, _, sub := buildScopeTestService(t)
	roles, _ := svc.ListRoles(context.Background(), caapp.AdminSubjectRequest{Subject: sub})
	if len(roles) == 0 {
		t.Fatal("no roles in test repo")
	}
	err := svc.AssignRolePermission(context.Background(), caapp.AssignRolePermissionRequest{
		Subject:      sub,
		RoleID:       roles[0].RoleID,
		PermissionID: "cms.template.read",
	})
	assertProtectedRoleReadOnly(t, err)
}

// TestAssignRolePermission_RejectPlatformCmsView verifies platform.cms.view is rejected on protected role.
func TestAssignRolePermission_RejectPlatformCmsView(t *testing.T) {
	svc, _, sub := buildScopeTestService(t)
	roles, _ := svc.ListRoles(context.Background(), caapp.AdminSubjectRequest{Subject: sub})
	if len(roles) == 0 {
		t.Fatal("no roles in test repo")
	}
	err := svc.AssignRolePermission(context.Background(), caapp.AssignRolePermissionRequest{
		Subject:      sub,
		RoleID:       roles[0].RoleID,
		PermissionID: "platform.cms.view",
	})
	assertProtectedRoleReadOnly(t, err)
}

// TestAssignRolePermission_RejectDisclosureTypeConfigWrite verifies disclosure_type.config.write is rejected.
func TestAssignRolePermission_RejectDisclosureTypeConfigWrite(t *testing.T) {
	svc, _, sub := buildScopeTestService(t)
	roles, _ := svc.ListRoles(context.Background(), caapp.AdminSubjectRequest{Subject: sub})
	if len(roles) == 0 {
		t.Fatal("no roles in test repo")
	}
	err := svc.AssignRolePermission(context.Background(), caapp.AssignRolePermissionRequest{
		Subject:      sub,
		RoleID:       roles[0].RoleID,
		PermissionID: "disclosure_type.config.write",
	})
	assertProtectedRoleReadOnly(t, err)
}

// TestAssignRolePermission_AllBlockedCodesRejected checks protected role guard on default roles.
func TestAssignRolePermission_AllBlockedCodesRejected(t *testing.T) {
	svc, _, sub := buildScopeTestService(t)
	roles, _ := svc.ListRoles(context.Background(), caapp.AdminSubjectRequest{Subject: sub})
	if len(roles) == 0 {
		t.Fatal("no roles")
	}
	roleID := roles[0].RoleID
	for _, code := range cmsBlockedCodes {
		err := svc.AssignRolePermission(context.Background(), caapp.AssignRolePermissionRequest{
			Subject:      sub,
			RoleID:       roleID,
			PermissionID: code,
		})
		assertProtectedRoleReadOnly(t, err)
	}
}

// TestAssignRolePermission_RejectRbacManageOnCustomRole verifies rbac.manage is rejected on unprotected custom role.
func TestAssignRolePermission_RejectRbacManageOnCustomRole(t *testing.T) {
	svc, repo, sub := buildScopeTestService(t)
	roleID := seedUnprotectedCustomRole(repo)
	err := svc.AssignRolePermission(context.Background(), caapp.AssignRolePermissionRequest{
		Subject:      sub,
		RoleID:       roleID,
		PermissionID: "rbac.manage",
	})
	if err == nil {
		t.Fatal("expected rejection")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.Code != perr.CodePermissionNotGrantable {
		t.Fatalf("expected permission_not_grantable, got %v", err)
	}
}

// TestAssignRolePermission_AcceptCompanyView verifies company.view is accepted on custom role.
func TestAssignRolePermission_AcceptCompanyView(t *testing.T) {
	svc, repo, sub := buildScopeTestService(t)
	roleID := seedUnprotectedCustomRole(repo)
	err := svc.AssignRolePermission(context.Background(), caapp.AssignRolePermissionRequest{
		Subject:      sub,
		RoleID:       roleID,
		PermissionID: "company.view",
	})
	if err != nil {
		t.Errorf("AssignRolePermission company.view: unexpected error: %v", err)
	}
}

// TestAssignRolePermission_AcceptDeptManage verifies dept.manage is accepted on custom role.
func TestAssignRolePermission_AcceptDeptManage(t *testing.T) {
	svc, repo, sub := buildScopeTestService(t)
	roleID := seedUnprotectedCustomRole(repo)
	err := svc.AssignRolePermission(context.Background(), caapp.AssignRolePermissionRequest{
		Subject:      sub,
		RoleID:       roleID,
		PermissionID: "dept.manage",
	})
	if err != nil {
		t.Errorf("AssignRolePermission dept.manage: unexpected error: %v", err)
	}
}

// TestAssignRolePermission_AcceptDisclosureTypeManage verifies disclosure_type.manage is accepted
// (enterprise permission — manage company's own disclosure types, NOT CMS config).
func TestAssignRolePermission_AcceptDisclosureTypeManage(t *testing.T) {
	svc, repo, sub := buildScopeTestService(t)
	roleID := seedUnprotectedCustomRole(repo)
	err := svc.AssignRolePermission(context.Background(), caapp.AssignRolePermissionRequest{
		Subject:      sub,
		RoleID:       roleID,
		PermissionID: "disclosure_type.manage",
	})
	if err != nil {
		t.Errorf("AssignRolePermission disclosure_type.manage: unexpected error: %v", err)
	}
}

// TestAssignRolePermission_AcceptTemplateWorkflowOverrideRead verifies
// template.workflow.override.read is accepted on custom role.
func TestAssignRolePermission_AcceptTemplateWorkflowOverrideRead(t *testing.T) {
	svc, repo, sub := buildScopeTestService(t)
	roleID := seedUnprotectedCustomRole(repo)
	err := svc.AssignRolePermission(context.Background(), caapp.AssignRolePermissionRequest{
		Subject:      sub,
		RoleID:       roleID,
		PermissionID: "template.workflow.override.read",
	})
	if err != nil {
		t.Errorf("AssignRolePermission template.workflow.override.read: unexpected error: %v", err)
	}
}

// TestListRolePermissions_FiltersCMSFromStoredRole verifies that if a role has CMS permissions
// stored (legacy migration artefact), ListRolePermissions silently filters them out.
func TestListRolePermissions_FiltersCMSFromStoredRole(t *testing.T) {
	repo := cainmem.NewAdminRepository()

	// Seed enterprise and CMS permissions
	repo.SeedPermission(caapp.PermissionListItem{
		PermissionID: "rbac.manage", PermissionCode: "rbac.manage",
		PermissionName: "rbac.manage", ModuleName: "admin",
	})
	repo.SeedPermission(caapp.PermissionListItem{
		PermissionID: "cms.template.read", PermissionCode: "cms.template.read",
		PermissionName: "cms.template.read", ModuleName: "cms",
	})

	svc := caapp.NewAdminService(
		repo,
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"rbac.manage"}},
		fixedIDGen("u_test"),
	)
	sub := caapp.AdminSubject{UserID: "u_test", MembershipID: "m_test", CompanyID: "c_test"}

	// Directly add both permissions to a role at repo level (simulating legacy data)
	roles, _ := svc.ListRoles(context.Background(), caapp.AdminSubjectRequest{Subject: sub})
	if len(roles) == 0 {
		t.Fatal("no roles")
	}
	roleID := roles[0].RoleID
	_ = repo.AddRolePermission(context.Background(), roleID, "rbac.manage")
	_ = repo.AddRolePermission(context.Background(), roleID, "cms.template.read")

	view, err := svc.ListRolePermissions(context.Background(), caapp.ListRolePermissionsRequest{
		Subject: sub, RoleID: roleID,
	})
	if err != nil {
		t.Fatalf("ListRolePermissions: %v", err)
	}
	for _, p := range view.Permissions {
		if p.PermissionCode == "cms.template.read" {
			t.Errorf("ListRolePermissions returned blocked permission cms.template.read")
		}
	}
	found := false
	for _, p := range view.Permissions {
		if p.PermissionCode == "rbac.manage" {
			found = true
		}
	}
	if !found {
		t.Error("ListRolePermissions missing rbac.manage (enterprise permission should pass through)")
	}
}

// TestIsEnterprisePermission_UnitTests verifies the scope helper directly.
func TestIsEnterprisePermission_UnitTests(t *testing.T) {
	cases := []struct {
		code       string
		moduleName string
		want       bool
	}{
		{"cms.template.read", "cms", false},
		{"cms.template.write", "cms", false},
		{"cms.template.activate", "cms", false},
		{"cms.template.archive", "cms", false},
		{"cms.template.config.write", "cms", false},
		{"disclosure_type.config.read", "cms", false},
		{"disclosure_type.config.write", "cms", false},
		{"platform.cms.view", "platform", false},
		// code deny-list even if module drifts to "general"
		{"cms.template.read", "general", false},
		{"platform.cms.view", "general", false},
		// enterprise permissions
		{"rbac.manage", "admin", true},
		{"company.view", "company", true},
		{"dept.manage", "org", true},
		{"disclosure_type.manage", "disclosure", true},
		{"template.workflow.override.read", "template", true},
		{"template.workflow.override.write", "template", true},
		{"template.workflow.override.approve", "template", true},
		{"template.workflow.override.reset", "template", true},
		{"disclosure.view", "disclosure", true},
		{"disclosure.create", "disclosure", true},
		{"disclosure.approve", "disclosure", true},
		{"disclosure.publish", "disclosure", true},
		{"system.settings", "admin", true},
		{"ad_hoc_alert.propose", "ad_hoc", true},
	}
	for _, tc := range cases {
		got := caapp.IsEnterprisePermission(tc.code, tc.moduleName)
		if got != tc.want {
			t.Errorf("IsEnterprisePermission(%q, %q) = %v, want %v", tc.code, tc.moduleName, got, tc.want)
		}
	}
}

func assertPermissionOutOfEnterpriseScope(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected PERMISSION_OUT_OF_ENTERPRISE_SCOPE error, got nil")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok {
		t.Fatalf("expected HTTPError, got %T: %v", err, err)
	}
	if he.Code != perr.CodePermissionOutOfEnterpriseScope {
		t.Errorf("expected PERMISSION_OUT_OF_ENTERPRISE_SCOPE, got %s", he.Code)
	}
}
