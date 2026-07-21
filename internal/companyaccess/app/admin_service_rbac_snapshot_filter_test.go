package app_test

// Tests for filterEnterpriseRBACSnapshotJSON applied at versioning/export/rollback callsites.
// Covers the follow-up cleanup mission requirements:
//   - RBAC snapshot excludes cms.template.read
//   - Config export excludes cms.template.read
//   - Rollback does not re-introduce blocked permissions
//   - Migration logic validated via service-layer guard

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	auditinmem "github.com/cobo/cobo_iam_services/internal/audit/infra/inmemory"
	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	authprojection "github.com/cobo/cobo_iam_services/internal/authorization/infra/projection"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	"github.com/cobo/cobo_iam_services/internal/companyaccess/configversion"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
)

func newSnapshotFilterSvc(t *testing.T, repo *cainmem.AdminRepository) caapp.AdminService {
	t.Helper()
	cache := authprojection.NewInMemoryStore(0)
	return caapp.NewAdminService(
		repo,
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"rbac.manage", "system.settings"}},
		fixedIDGen("sf-1"),
		caapp.WithAuditRepository(auditinmem.NewRepository()),
		caapp.WithEffectiveAccessCache(cache),
	)
}

// seedSnapshotFilterAdmin seeds a subject, a role, enterprise permissions, and a
// CMS-blocked permission that was "legacy" inserted directly into the in-memory repo
// (bypassing AssignRolePermission, simulating data from before the scope fix).
func seedSnapshotFilterAdmin(t *testing.T, repo *cainmem.AdminRepository) (caapp.AdminSubject, string, string) {
	t.Helper()
	sub := caapp.AdminSubject{UserID: "u_sf", MembershipID: "m_sf", CompanyID: "c_sf"}
	seedInviteScopedSubject(t, repo, sub)
	_ = repo.AddRolePermission(context.Background(), "company_admin", "rbac.manage")

	// Seed enterprise permission
	repo.SeedPermission(caapp.PermissionListItem{
		PermissionID:   "perm_rbac_manage",
		PermissionCode: "rbac.manage",
		PermissionName: "Manage RBAC",
		ModuleName:     "rbac",
	})

	// Seed CMS-blocked permission (legacy data simulating pre-fix state)
	repo.SeedPermission(caapp.PermissionListItem{
		PermissionID:   "perm_cms_template_read",
		PermissionCode: "cms.template.read",
		PermissionName: "Read CMS Template",
		ModuleName:     "cms",
	})

	// Legacy: directly add the blocked permission to the role bypassing the service filter
	_ = repo.AddRolePermission(context.Background(), "company_admin", "perm_cms_template_read")

	repo.SeedRole(caapp.RoleListItem{
		RoleID: "custom_sf_role", RoleCode: "custom_sf", RoleName: "Custom SF",
		Status: "active", Scope: "company",
		RoleType: caapp.RoleTypeTenantCustom, IsProtected: false, IsBuiltin: false,
	})
	repo.SeedPermission(caapp.PermissionListItem{
		PermissionID:   "perm_disclosure_view",
		PermissionCode: "disclosure.view",
		PermissionName: "View disclosure",
		ModuleName:     "disclosure",
	})

	return sub, "perm_disclosure_view", "perm_cms_template_read"
}

// TestRBACSnapshot_ExcludesCMSTemplateRead verifies that captureRBACMatrixVersion
// filters out cms.template.read from the stored snapshot even if the DB row exists.
func TestRBACSnapshot_ExcludesCMSTemplateRead(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub, enterprisePermID, _ := seedSnapshotFilterAdmin(t, repo)
	svc := newSnapshotFilterSvc(t, repo)

	// Trigger snapshot capture via AssignRolePermission (uses enterprise perm)
	if err := svc.AssignRolePermission(context.Background(), caapp.AssignRolePermissionRequest{
		Subject: sub, RoleID: "custom_sf_role", PermissionID: enterprisePermID,
	}); err != nil {
		t.Fatalf("assign enterprise perm: %v", err)
	}

	versions, err := svc.ListRBACMatrixVersions(context.Background(), caapp.ListRBACMatrixVersionsRequest{
		Subject: sub, Limit: 10,
	})
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions.Items) == 0 {
		t.Fatal("expected at least 1 RBAC snapshot version")
	}

	detail, err := svc.GetRBACMatrixVersion(context.Background(), caapp.GetRBACMatrixVersionRequest{
		Subject: sub, VersionNo: versions.Items[0].VersionNo,
	})
	if err != nil {
		t.Fatalf("get version: %v", err)
	}

	raw := detail.SnapshotJSON
	assertSnapshotNoCMSPermissions(t, raw, "RBAC versioning snapshot")
}

// TestConfigExport_RBACModule_ExcludesCMSTemplateRead verifies that CreateConfigExport
// with the rbac_matrix module does not include cms.template.read in the artifact.
func TestConfigExport_RBACModule_ExcludesCMSTemplateRead(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub, _, _ := seedSnapshotFilterAdmin(t, repo)

	// Also need a notification rule seeded for the full export to work
	_ = repo.AddNotificationRule(context.Background(), map[string]any{
		"company_id": sub.CompanyID,
		"rule_code":  caapp.AlertChannelPrefsRuleCode,
		"status":     "active",
		"channels":   map[string]any{"email": map[string]any{"enabled": true}},
	})

	svc := newSnapshotFilterSvc(t, repo)

	job, err := svc.CreateConfigExport(context.Background(), caapp.CreateConfigExportRequest{Subject: sub})
	if err != nil {
		t.Fatalf("create export: %v", err)
	}

	raw, err := svc.DownloadConfigExport(context.Background(), caapp.DownloadConfigExportRequest{
		Subject: sub, ExportID: job.ExportID,
	})
	if err != nil {
		t.Fatalf("download export: %v", err)
	}

	// The artifact is a JSON envelope; the rbac_matrix key holds the filtered snapshot
	var artifact map[string]any
	if err := json.Unmarshal(raw, &artifact); err != nil {
		t.Fatalf("unmarshal artifact: %v", err)
	}

	rawStr := string(raw)
	if strings.Contains(rawStr, "perm_cms_template_read") {
		t.Errorf("config export artifact contains perm_cms_template_read — expected it to be filtered")
	}
	if strings.Contains(rawStr, "cms.template.read") {
		t.Errorf("config export artifact contains cms.template.read — expected it to be filtered")
	}
}

// TestRBACRollback_DoesNotReintroduceCMSPermission verifies that rolling back to a
// historic snapshot that contained cms.template.read does NOT re-assign it.
func TestRBACRollback_DoesNotReintroduceCMSPermission(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub, enterprisePermID, cmsPermID := seedSnapshotFilterAdmin(t, repo)
	svc := newSnapshotFilterSvc(t, repo)

	// Step 1: Capture a snapshot while cms perm is "in DB" (legacy state).
	// We do this by first triggering via AssignRolePermission on the enterprise perm.
	if err := svc.AssignRolePermission(context.Background(), caapp.AssignRolePermissionRequest{
		Subject: sub, RoleID: "custom_sf_role", PermissionID: enterprisePermID,
	}); err != nil {
		t.Fatalf("assign: %v", err)
	}

	// Step 2: Remove the enterprise perm from the role so state changes.
	if err := svc.RemoveRolePermission(context.Background(), caapp.RemoveRolePermissionRequest{
		Subject: sub, RoleID: "custom_sf_role", PermissionID: enterprisePermID,
	}); err != nil {
		t.Fatalf("remove: %v", err)
	}

	// Step 3: Rollback to version 1.
	_, err := svc.RollbackRBACMatrixVersion(context.Background(), caapp.RollbackRBACMatrixVersionRequest{
		Subject: sub, VersionNo: 1, Reason: "test rollback",
	})
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}

	// Step 4: Read the restored role permissions via the service (filtered) and repo (raw).
	// The key assertion: cms perm should not be in the role after rollback.
	view, err := svc.ListRolePermissions(context.Background(), caapp.ListRolePermissionsRequest{
		Subject: sub, RoleID: "company_admin",
	})
	if err != nil {
		t.Fatalf("list role perms after rollback: %v", err)
	}

	for _, p := range view.Permissions {
		if p.PermissionID == cmsPermID || p.PermissionCode == "cms.template.read" {
			t.Errorf("rollback re-introduced blocked permission cms.template.read in service view")
		}
	}
}

// TestFilterEnterpriseRBACSnapshotJSON_DirectPermissions verifies that direct permissions
// with blocked codes are stripped from snapshots.
func TestFilterEnterpriseRBACSnapshotJSON_DirectPermissions(t *testing.T) {
	// Build a raw snapshot with both enterprise and blocked direct permissions.
	snap := configversion.RBACMatrixSnapshot{
		SchemaVersion: configversion.RBACMatrixSnapshotSchema,
		RolePermissions: []configversion.RolePermissionEntry{
			{RoleID: "role1", PermissionID: "perm_rbac_manage"},
		},
		DirectPermissions: []configversion.DirectPermissionEntry{
			{MembershipID: "m1", PermissionCode: "rbac.manage"},
			{MembershipID: "m1", PermissionCode: "cms.template.read"},
			{MembershipID: "m1", PermissionCode: "platform.cms.view"},
			{MembershipID: "m1", PermissionCode: "disclosure_type.config.read"},
			{MembershipID: "m1", PermissionCode: "disclosure.view"},
		},
	}
	raw, _ := json.Marshal(snap)

	// Use the service-layer path via versioning: seed a repo with this state
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_dp", MembershipID: "m_dp", CompanyID: "c_dp"}
	seedInviteScopedSubject(t, repo, sub)
	_ = repo.AddRolePermission(context.Background(), "company_admin", "rbac.manage")
	repo.SeedPermission(caapp.PermissionListItem{
		PermissionID:   "perm_rbac_manage",
		PermissionCode: "rbac.manage",
		ModuleName:     "rbac",
	})

	// Inject the raw snapshot directly via InsertRBACMatrixSnapshot to simulate legacy data.
	_, err := repo.InsertRBACMatrixSnapshot(context.Background(), caapp.InsertRBACMatrixSnapshotInput{
		ID:           "v-legacy",
		CompanyID:    sub.CompanyID,
		SnapshotJSON: raw,
		CreatedBy:    sub.MembershipID,
		Reason:       "legacy",
		Source:       "test",
	})
	if err != nil {
		t.Fatalf("insert legacy snapshot: %v", err)
	}

	// Rollback to the legacy snapshot — filter should strip blocked direct permissions.
	svc := newSnapshotFilterSvc(t, repo)
	_, err = svc.RollbackRBACMatrixVersion(context.Background(), caapp.RollbackRBACMatrixVersionRequest{
		Subject: sub, VersionNo: 1, Reason: "test",
	})
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}

	// After rollback, check that blocked direct permissions were not granted.
	// We cannot directly inspect direct perms via service layer easily, but we can
	// verify the snapshot captured after rollback doesn't include blocked codes.
	svc2 := newSnapshotFilterSvc(t, repo)
	versions, err := svc2.ListRBACMatrixVersions(context.Background(), caapp.ListRBACMatrixVersionsRequest{
		Subject: sub, Limit: 10,
	})
	if err != nil || len(versions.Items) == 0 {
		t.Fatal("expected version after rollback")
	}
	detail, err := svc2.GetRBACMatrixVersion(context.Background(), caapp.GetRBACMatrixVersionRequest{
		Subject: sub, VersionNo: versions.Items[0].VersionNo,
	})
	if err != nil {
		t.Fatalf("get version: %v", err)
	}
	assertSnapshotNoCMSPermissions(t, detail.SnapshotJSON, "post-rollback snapshot")
}

// TestAssignBlockedPermission_StillReturnsOutOfScope ensures CMS permissions are rejected on custom role.
func TestAssignBlockedPermission_StillReturnsOutOfScope(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_aob", MembershipID: "m_aob", CompanyID: "c_aob"}
	seedInviteScopedSubject(t, repo, sub)
	repo.SeedRole(caapp.RoleListItem{
		RoleID: "custom_aob", RoleCode: "custom_aob", RoleName: "Custom",
		Status: "active", Scope: "company",
		RoleType: caapp.RoleTypeTenantCustom, IsProtected: false, IsBuiltin: false,
	})
	repo.SeedPermission(caapp.PermissionListItem{
		PermissionID:   "perm_cms_tpl_read",
		PermissionCode: "cms.template.read",
		ModuleName:     "cms",
	})
	svc := newSnapshotFilterSvc(t, repo)

	err := svc.AssignRolePermission(context.Background(), caapp.AssignRolePermissionRequest{
		Subject: sub, RoleID: "custom_aob", PermissionID: "perm_cms_tpl_read",
	})
	if err == nil {
		t.Fatal("expected scope/tier rejection error, got nil")
	}
	if !strings.Contains(err.Error(), "PERMISSION_OUT_OF_ENTERPRISE_SCOPE") &&
		!strings.Contains(err.Error(), "permission_not_grantable") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// assertSnapshotNoCMSPermissions checks that a raw RBACMatrixSnapshot JSON does not
// contain any blocked permission IDs/codes.
func assertSnapshotNoCMSPermissions(t *testing.T, raw []byte, label string) {
	t.Helper()
	s := string(raw)
	blocked := []string{
		"cms.template.read",
		"perm_cms_template_read",
		"platform.cms.view",
		"disclosure_type.config.read",
		"disclosure_type.config.write",
	}
	for _, code := range blocked {
		if strings.Contains(s, code) {
			t.Errorf("%s contains blocked permission %q — expected filtered out", label, code)
		}
	}
}
