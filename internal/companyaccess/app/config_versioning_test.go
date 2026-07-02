package app_test

import (
	"context"
	"net/http"
	"testing"

	auditinmem "github.com/cobo/cobo_iam_services/internal/audit/infra/inmemory"
	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	authprojection "github.com/cobo/cobo_iam_services/internal/authorization/infra/projection"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func newVersioningSvc(t *testing.T, repo *cainmem.AdminRepository) caapp.AdminService {
	t.Helper()
	cache := authprojection.NewInMemoryStore(0)
	return caapp.NewAdminService(repo, fakeAuthService{
		decision:    authapp.DecisionAllow,
		permissions: []string{"rbac.manage", "system.settings"},
	}, fixedIDGen("ver-1"),
		caapp.WithAuditRepository(auditinmem.NewRepository()),
		caapp.WithEffectiveAccessCache(cache),
	)
}

func seedRbacAdmin(t *testing.T, repo *cainmem.AdminRepository) caapp.AdminSubject {
	t.Helper()
	sub := caapp.AdminSubject{UserID: "u_ver", MembershipID: "m_ver", CompanyID: "c_ver"}
	seedInviteScopedSubject(t, repo, sub)
	_ = repo.AddRolePermission(context.Background(), "company_admin", "rbac.manage")
	// Seed enterprise permissions used by versioning tests for AssignRolePermission.
	for _, code := range []string{"perm_disclosure_read", "perm_disclosure_write"} {
		repo.SeedPermission(caapp.PermissionListItem{
			PermissionID:   code,
			PermissionCode: code,
			PermissionName: code,
			ModuleName:     "disclosure",
		})
	}
	return sub
}

func TestNotificationMutation_CreatesVersionSnapshot(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := seedRbacAdmin(t, repo)
	svc := newVersioningSvc(t, repo)

	payload := map[string]any{
		"rule_code": caapp.AlertChannelPrefsRuleCode,
		"status":    "active",
		"channels": map[string]any{
			"email": map[string]any{"enabled": true},
		},
	}
	if err := svc.CreateNotificationRule(context.Background(), caapp.CreateNotificationRuleRequest{Subject: sub, Payload: payload}); err != nil {
		t.Fatalf("create: %v", err)
	}
	rules, _ := svc.ListNotificationRules(context.Background(), caapp.ListNotificationRulesRequest{Subject: sub})
	if len(rules) == 0 {
		t.Fatal("expected rule")
	}
	versions, err := svc.ListNotificationRuleVersions(context.Background(), caapp.ListNotificationRuleVersionsRequest{
		Subject: sub, RuleID: rules[0].NotificationRuleID, Limit: 10,
	})
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions.Items) != 1 {
		t.Fatalf("expected 1 version, got %d", len(versions.Items))
	}
	if versions.Items[0].VersionNo != 1 {
		t.Fatalf("expected version_no=1, got %d", versions.Items[0].VersionNo)
	}
}

func TestNotificationMutation_NoSnapshotOnValidationFailure(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := seedRbacAdmin(t, repo)
	svc := newVersioningSvc(t, repo)
	err := svc.CreateNotificationRule(context.Background(), caapp.CreateNotificationRuleRequest{
		Subject: sub,
		Payload: map[string]any{"rule_code": caapp.AlertChannelPrefsRuleCode, "status": "active"},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	versions, _ := svc.ListNotificationRuleVersions(context.Background(), caapp.ListNotificationRuleVersionsRequest{Subject: sub, Limit: 10})
	if len(versions.Items) != 0 {
		t.Fatalf("expected no versions, got %d", len(versions.Items))
	}
}

func TestNotificationRollback_CreatesNewSnapshot(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := seedRbacAdmin(t, repo)
	svc := newVersioningSvc(t, repo)
	payload := map[string]any{
		"rule_code": caapp.AlertChannelPrefsRuleCode,
		"status":    "active",
		"channels":  map[string]any{"email": map[string]any{"enabled": true}},
	}
	_ = svc.CreateNotificationRule(context.Background(), caapp.CreateNotificationRuleRequest{Subject: sub, Payload: payload})
	rules, _ := svc.ListNotificationRules(context.Background(), caapp.ListNotificationRulesRequest{Subject: sub})
	ruleID := rules[0].NotificationRuleID
	_, _ = svc.RollbackNotificationRuleVersion(context.Background(), caapp.RollbackNotificationRuleVersionRequest{
		Subject: sub, RuleID: ruleID, VersionNo: 1, Reason: "test",
	})
	versions, _ := svc.ListNotificationRuleVersions(context.Background(), caapp.ListNotificationRuleVersionsRequest{Subject: sub, RuleID: ruleID, Limit: 10})
	if len(versions.Items) < 2 {
		t.Fatalf("expected >=2 versions after rollback, got %d", len(versions.Items))
	}
}

func TestRBACMutation_CreatesMatrixSnapshot(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := seedRbacAdmin(t, repo)
	svc := newVersioningSvc(t, repo)
	if err := svc.AssignRolePermission(context.Background(), caapp.AssignRolePermissionRequest{
		Subject: sub, RoleID: "company_admin", PermissionID: "perm_disclosure_read",
	}); err != nil {
		t.Fatalf("assign: %v", err)
	}
	versions, err := svc.ListRBACMatrixVersions(context.Background(), caapp.ListRBACMatrixVersionsRequest{Subject: sub, Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(versions.Items) != 1 {
		t.Fatalf("expected 1 rbac snapshot, got %d", len(versions.Items))
	}
}

func TestVersionAPI_ForbiddenForLimitedUser(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := seedRbacAdmin(t, repo)
	svc := caapp.NewAdminService(repo, fakeAuthService{
		decision:    authapp.DecisionAllow,
		permissions: []string{},
	}, fixedIDGen("x"))
	_, err := svc.ListNotificationRuleVersions(context.Background(), caapp.ListNotificationRuleVersionsRequest{Subject: sub, Limit: 10})
	if err == nil {
		t.Fatal("expected forbidden")
	}
	he, ok := err.(*perr.HTTPError)
	if !ok || he.HTTPStatus != http.StatusForbidden {
		t.Fatalf("expected 403, got %v", err)
	}
}

func TestVersionAPI_NotFound(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := seedRbacAdmin(t, repo)
	svc := newVersioningSvc(t, repo)
	_, err := svc.GetRBACMatrixVersion(context.Background(), caapp.GetRBACMatrixVersionRequest{Subject: sub, VersionNo: 99})
	if err == nil {
		t.Fatal("expected not found")
	}
	he, ok := err.(*perr.HTTPError)
	if !ok || he.HTTPStatus != http.StatusNotFound {
		t.Fatalf("expected 404, got %v", err)
	}
}

func TestCompareVersions_ReadOnly(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := seedRbacAdmin(t, repo)
	svc := newVersioningSvc(t, repo)
	_ = svc.AssignRolePermission(context.Background(), caapp.AssignRolePermissionRequest{
		Subject: sub, RoleID: "company_admin", PermissionID: "perm_disclosure_read",
	})
	_ = svc.AssignRolePermission(context.Background(), caapp.AssignRolePermissionRequest{
		Subject: sub, RoleID: "company_admin", PermissionID: "perm_disclosure_write",
	})
	cmp, err := svc.CompareRBACMatrixVersions(context.Background(), caapp.CompareRBACMatrixVersionsRequest{
		Subject: sub, FromVersionNo: 1, ToVersionNo: 2,
	})
	if err != nil {
		t.Fatalf("compare: %v", err)
	}
	if cmp.Equal {
		t.Fatal("expected diff between versions")
	}
}

func TestAuditMetadata_NoSnapshotJSON(t *testing.T) {
	auditRepo := auditinmem.NewRepository()
	repo := cainmem.NewAdminRepository()
	sub := seedRbacAdmin(t, repo)
	svc := caapp.NewAdminService(repo, fakeAuthService{
		decision: authapp.DecisionAllow, permissions: []string{"rbac.manage"},
	}, fixedIDGen("ver-2"), caapp.WithAuditRepository(auditRepo))
	_ = svc.AssignRolePermission(context.Background(), caapp.AssignRolePermissionRequest{
		Subject: sub, RoleID: "company_admin", PermissionID: "perm_disclosure_read",
	})
	entries, _ := auditRepo.ListByCompany(context.Background(), sub.CompanyID, "", "", "", "", "", "", 50)
	for _, e := range entries {
		if e.Action == "admin.version.rbac.snapshot_created" {
			if e.Metadata != nil {
				if _, ok := e.Metadata["snapshot_json"]; ok {
					t.Fatal("snapshot_json must not appear in audit metadata")
				}
			}
			return
		}
	}
	t.Fatal("expected snapshot_created audit entry")
}
