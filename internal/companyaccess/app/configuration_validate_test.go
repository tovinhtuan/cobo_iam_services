package app_test

import (
	"context"
	"net/http"
	"testing"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	"github.com/cobo/cobo_iam_services/internal/companyaccess/validation"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func TestValidateConfigurationReturnsSuites(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	svc := newHealthTestService(repo, []string{"rbac.manage"})
	sub := caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "co-1"}
	out, err := svc.ValidateConfiguration(context.Background(), caapp.ValidateConfigurationRequest{Subject: sub})
	if err != nil {
		t.Fatalf("ValidateConfiguration: %v", err)
	}
	if out.CompanyID != "co-1" {
		t.Fatalf("company_id: got %q", out.CompanyID)
	}
	if out.ValidatedAt.IsZero() {
		t.Fatal("validated_at required")
	}
	if len(out.Suites) != len(validation.StageOrder) {
		t.Fatalf("expected %d suites, got %d", len(validation.StageOrder), len(out.Suites))
	}
	foundStorageInfo := false
	foundConflictSuite := false
	for _, suite := range out.Suites {
		if suite.Suite == validation.SuiteConflict {
			foundConflictSuite = true
		}
		for _, c := range suite.Checks {
			if c.Code == "notification.storage_not_configured" {
				foundStorageInfo = true
			}
		}
	}
	if !foundConflictSuite {
		t.Fatal("conflict suite required")
	}
	if !foundStorageInfo {
		t.Fatal("expected notification.storage_not_configured in validate (health subset)")
	}
}

func TestValidateConfigurationDeniedWithoutPermission(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	svc := newHealthTestService(repo, []string{"disclosure.view"})
	sub := caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "co-1"}
	_, err := svc.ValidateConfiguration(context.Background(), caapp.ValidateConfigurationRequest{Subject: sub})
	if err == nil {
		t.Fatal("expected error")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusForbidden {
		t.Fatalf("expected 403 forbidden, got %v", err)
	}
}

func TestValidateConfigurationRequiresCompanyScope(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	svc := newHealthTestService(repo, []string{"system.settings"})
	sub := caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: ""}
	_, err := svc.ValidateConfiguration(context.Background(), caapp.ValidateConfigurationRequest{Subject: sub})
	if err == nil {
		t.Fatal("expected error for missing company")
	}
}

func TestValidateConfigurationSuiteFilter(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	svc := newHealthTestService(repo, []string{"rbac.manage"})
	sub := caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "co-1"}
	out, err := svc.ValidateConfiguration(context.Background(), caapp.ValidateConfigurationRequest{
		Subject: sub,
		Suites:  []string{"schema"},
	})
	if err != nil {
		t.Fatalf("ValidateConfiguration: %v", err)
	}
	if len(out.Suites) != 1 || out.Suites[0].Suite != validation.SuiteSchema {
		t.Fatalf("unexpected suites: %+v", out.Suites)
	}
}
