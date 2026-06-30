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

type healthAuthService struct {
	permissions []string
}

func (h healthAuthService) Authorize(_ context.Context, _ authapp.AuthorizeRequest) (*authapp.AuthorizeDecision, error) {
	return &authapp.AuthorizeDecision{Decision: authapp.DecisionAllow}, nil
}

func (h healthAuthService) AuthorizeBatch(_ context.Context, _ authapp.AuthorizeBatchRequest) (*authapp.AuthorizeBatchResponse, error) {
	return &authapp.AuthorizeBatchResponse{}, nil
}

func (h healthAuthService) GetEffectiveAccess(_ context.Context, _, _ string) (*authapp.EffectiveAccessSummary, error) {
	return &authapp.EffectiveAccessSummary{Permissions: h.permissions}, nil
}

func newHealthTestService(repo *cainmem.AdminRepository, perms []string) caapp.AdminService {
	return caapp.NewAdminService(
		repo,
		healthAuthService{permissions: perms},
		fixedIDGen("id-1"),
		caapp.WithConflictSnapshotReader(cainmem.NewConflictSnapshotReader(repo)),
	)
}

func TestGetConfigurationHealthReturnsChecks(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	svc := newHealthTestService(repo, []string{"rbac.manage"})
	sub := caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "co-1"}
	out, err := svc.GetConfigurationHealth(context.Background(), caapp.GetConfigurationHealthRequest{Subject: sub})
	if err != nil {
		t.Fatalf("GetConfigurationHealth: %v", err)
	}
	if out.OverallStatus == "" {
		t.Fatal("overall_status required")
	}
	if out.Checks == nil {
		t.Fatal("checks required")
	}
	foundInfo := false
	for _, c := range out.Checks {
		if c.Code == "notification.storage_not_configured" {
			foundInfo = true
		}
	}
	if !foundInfo {
		t.Fatal("expected notification.storage_not_configured for empty tenant")
	}
	if out.Score == nil {
		t.Fatal("score required")
	}
	if out.Score.Algorithm != "weighted_severity_v1" {
		t.Fatalf("algorithm=%q", out.Score.Algorithm)
	}
	if out.Score.Value < 0 || out.Score.Value > 100 {
		t.Fatalf("value out of range: %d", out.Score.Value)
	}
}

func TestGetConfigurationHealthDeniedWithoutPermission(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	svc := newHealthTestService(repo, []string{"disclosure.view"})
	sub := caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "co-1"}
	_, err := svc.GetConfigurationHealth(context.Background(), caapp.GetConfigurationHealthRequest{Subject: sub})
	if err == nil {
		t.Fatal("expected error")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusForbidden {
		t.Fatalf("expected 403 forbidden, got %v", err)
	}
}

func TestGetConfigurationHealthRequiresCompanyScope(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	svc := newHealthTestService(repo, []string{"system.settings"})
	sub := caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: ""}
	_, err := svc.GetConfigurationHealth(context.Background(), caapp.GetConfigurationHealthRequest{Subject: sub})
	if err == nil {
		t.Fatal("expected error for missing company")
	}
}
