package app_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	auditinmem "github.com/cobo/cobo_iam_services/internal/audit/infra/inmemory"
	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	authprojection "github.com/cobo/cobo_iam_services/internal/authorization/infra/projection"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	"github.com/cobo/cobo_iam_services/internal/companyaccess/configexport"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func newConfigExportSvc(t *testing.T, repo *cainmem.AdminRepository, perms []string) caapp.AdminService {
	t.Helper()
	cache := authprojection.NewInMemoryStore(0)
	return caapp.NewAdminService(repo, fakeAuthService{
		decision:    authapp.DecisionAllow,
		permissions: perms,
	}, fixedIDGen("cex-1"),
		caapp.WithAuditRepository(auditinmem.NewRepository()),
		caapp.WithEffectiveAccessCache(cache),
	)
}

func seedConfigExportAdmin(t *testing.T, repo *cainmem.AdminRepository) caapp.AdminSubject {
	t.Helper()
	sub := caapp.AdminSubject{UserID: "u_cex", MembershipID: "m_cex", CompanyID: "c_cex"}
	seedInviteScopedSubject(t, repo, sub)
	_ = repo.AddRolePermission(context.Background(), "company_admin", "rbac.manage")
	return sub
}

func seedAlertPrefsRule(t *testing.T, repo *cainmem.AdminRepository, companyID string) {
	t.Helper()
	err := repo.AddNotificationRule(context.Background(), map[string]any{
		"company_id": companyID,
		"rule_code": caapp.AlertChannelPrefsRuleCode,
		"status":    "active",
		"channels": map[string]any{
			"email": map[string]any{"enabled": true},
		},
	})
	if err != nil {
		t.Fatalf("seed alert prefs: %v", err)
	}
}

func TestConfigExport_ReturnsV1Schema(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := seedConfigExportAdmin(t, repo)
	seedAlertPrefsRule(t, repo, sub.CompanyID)
	svc := newConfigExportSvc(t, repo, []string{"rbac.manage"})

	job, err := svc.CreateConfigExport(context.Background(), caapp.CreateConfigExportRequest{Subject: sub})
	if err != nil {
		t.Fatalf("create export: %v", err)
	}
	if job.SchemaVersion != configexport.SchemaVersionEnterpriseExport {
		t.Fatalf("schema_version = %q", job.SchemaVersion)
	}

	raw, err := svc.DownloadConfigExport(context.Background(), caapp.DownloadConfigExportRequest{
		Subject: sub, ExportID: job.ExportID,
	})
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	artifact, err := caapp.ParseConfigExportArtifact(raw)
	if err != nil {
		t.Fatalf("parse artifact: %v", err)
	}
	if artifact["schema_version"] != configexport.SchemaVersionEnterpriseExport {
		t.Fatalf("artifact schema_version = %v", artifact["schema_version"])
	}
}

func TestConfigExport_IncludesScopedModules(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := seedConfigExportAdmin(t, repo)
	seedAlertPrefsRule(t, repo, sub.CompanyID)
	svc := newConfigExportSvc(t, repo, []string{"rbac.manage"})

	job, err := svc.CreateConfigExport(context.Background(), caapp.CreateConfigExportRequest{Subject: sub})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(job.Modules) != 2 {
		t.Fatalf("modules = %v", job.Modules)
	}
	raw, _ := svc.DownloadConfigExport(context.Background(), caapp.DownloadConfigExportRequest{Subject: sub, ExportID: job.ExportID})
	artifact, _ := caapp.ParseConfigExportArtifact(raw)
	data, _ := artifact["data"].(map[string]any)
	if data[configexport.ModuleNotificationAlertChannelPrefs] == nil {
		t.Fatal("missing notification module")
	}
	if data[configexport.ModuleRBACMatrix] == nil {
		t.Fatal("missing rbac module")
	}
}

func TestConfigExport_DeterministicChecksum(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := seedConfigExportAdmin(t, repo)
	seedAlertPrefsRule(t, repo, sub.CompanyID)
	svc := newConfigExportSvc(t, repo, []string{"rbac.manage"})

	j1, err := svc.CreateConfigExport(context.Background(), caapp.CreateConfigExportRequest{Subject: sub})
	if err != nil {
		t.Fatalf("export 1: %v", err)
	}
	j2, err := svc.CreateConfigExport(context.Background(), caapp.CreateConfigExportRequest{Subject: sub})
	if err != nil {
		t.Fatalf("export 2: %v", err)
	}
	if j1.Checksum != j2.Checksum {
		t.Fatalf("checksum mismatch: %s vs %s", j1.Checksum, j2.Checksum)
	}
}

func TestConfigExport_SanitizesSecretFields(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := seedConfigExportAdmin(t, repo)
	err := repo.AddNotificationRule(context.Background(), map[string]any{
		"company_id": sub.CompanyID,
		"rule_code":  caapp.AlertChannelPrefsRuleCode,
		"status":     "active",
		"channels": map[string]any{
			"email": map[string]any{"enabled": true, "smtp_password": "secret-value"},
		},
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	svc := newConfigExportSvc(t, repo, []string{"rbac.manage"})
	job, err := svc.CreateConfigExport(context.Background(), caapp.CreateConfigExportRequest{Subject: sub})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	raw, _ := svc.DownloadConfigExport(context.Background(), caapp.DownloadConfigExportRequest{Subject: sub, ExportID: job.ExportID})
	if strings.Contains(strings.ToLower(string(raw)), "secret-value") {
		t.Fatal("artifact leaked secret value")
	}
	if strings.Contains(strings.ToLower(string(raw)), "smtp_password") {
		t.Fatal("artifact contains secret-shaped key")
	}
}

func TestConfigExport_LimitedUser403(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := seedConfigExportAdmin(t, repo)
	svc := newConfigExportSvc(t, repo, []string{"disclosure.read"})
	_, err := svc.CreateConfigExport(context.Background(), caapp.CreateConfigExportRequest{Subject: sub})
	if err == nil {
		t.Fatal("expected error")
	}
	he, ok := err.(*perr.HTTPError)
	if !ok || he.HTTPStatus != http.StatusForbidden {
		t.Fatalf("expected 403, got %v", err)
	}
}

func TestConfigExport_InvalidModule400(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := seedConfigExportAdmin(t, repo)
	svc := newConfigExportSvc(t, repo, []string{"rbac.manage"})
	_, err := svc.CreateConfigExport(context.Background(), caapp.CreateConfigExportRequest{
		Subject: sub,
		Modules: []string{"unknown.module"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	he, ok := err.(*perr.HTTPError)
	if !ok || he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("expected 400, got %v", err)
	}
}

func TestConfigExport_NoCrossTenantLeak(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	subA := seedConfigExportAdmin(t, repo)
	seedAlertPrefsRule(t, repo, subA.CompanyID)
	subB := caapp.AdminSubject{UserID: "u_b", MembershipID: "m_b", CompanyID: "c_other"}
	seedInviteScopedSubject(t, repo, subB)
	_ = repo.AddRolePermission(context.Background(), "company_admin", "rbac.manage")
	svc := newConfigExportSvc(t, repo, []string{"rbac.manage"})

	job, err := svc.CreateConfigExport(context.Background(), caapp.CreateConfigExportRequest{Subject: subA})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	_, err = svc.DownloadConfigExport(context.Background(), caapp.DownloadConfigExportRequest{
		Subject: subB, ExportID: job.ExportID,
	})
	if err == nil {
		t.Fatal("expected cross-tenant denial")
	}
	he, ok := err.(*perr.HTTPError)
	if !ok || he.HTTPStatus != http.StatusNotFound {
		t.Fatalf("expected 404, got %v", err)
	}
}

func TestConfigExport_AuditMetadataSafe(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := seedConfigExportAdmin(t, repo)
	seedAlertPrefsRule(t, repo, sub.CompanyID)
	auditRepo := auditinmem.NewRepository()
	svc := caapp.NewAdminService(repo, fakeAuthService{
		decision: authapp.DecisionAllow, permissions: []string{"rbac.manage"},
	}, fixedIDGen("cex-audit"),
		caapp.WithAuditRepository(auditRepo),
		caapp.WithEffectiveAccessCache(authprojection.NewInMemoryStore(0)),
	)
	job, err := svc.CreateConfigExport(context.Background(), caapp.CreateConfigExportRequest{Subject: sub})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	meta := caapp.ConfigExportAuditMetadata(job)
	raw, _ := json.Marshal(meta)
	if strings.Contains(string(raw), "channels") || strings.Contains(string(raw), "role_permissions") {
		t.Fatal("audit metadata must not include artifact payload")
	}
}

func TestConfigExport_NoMutationSideEffect(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := seedConfigExportAdmin(t, repo)
	seedAlertPrefsRule(t, repo, sub.CompanyID)
	svc := newConfigExportSvc(t, repo, []string{"rbac.manage"})

	beforeRules, _ := repo.ListNotificationRules(context.Background(), sub.CompanyID)
	beforeRoles, _ := repo.ListRoles(context.Background(), sub.CompanyID)
	_, err := svc.CreateConfigExport(context.Background(), caapp.CreateConfigExportRequest{Subject: sub})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	afterRules, _ := repo.ListNotificationRules(context.Background(), sub.CompanyID)
	afterRoles, _ := repo.ListRoles(context.Background(), sub.CompanyID)
	if len(beforeRules) != len(afterRules) || len(beforeRoles) != len(afterRoles) {
		t.Fatal("export mutated configuration")
	}
}
