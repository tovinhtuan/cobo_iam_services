package app_test

import (
	"context"
	"net/http"
	"testing"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	"github.com/cobo/cobo_iam_services/internal/companyaccess/dashboard"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func TestGetOperationalDashboardReturnsWidgets(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	svc := newDependencyTestService(repo, []string{"rbac.manage"})
	sub := caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "co-1"}
	out, err := svc.GetOperationalDashboard(context.Background(), caapp.GetOperationalDashboardRequest{Subject: sub})
	if err != nil {
		t.Fatalf("GetOperationalDashboard: %v", err)
	}
	if out.CompanyID != "co-1" {
		t.Fatalf("company_id %q", out.CompanyID)
	}
	if out.OverallStatus == "" {
		t.Fatal("overall_status required")
	}
	if len(out.Widgets) < 4 {
		t.Fatalf("widgets: %d", len(out.Widgets))
	}
	foundHealth := false
	for _, w := range out.Widgets {
		if w.Key == dashboard.WidgetConfigurationHealth {
			foundHealth = true
		}
	}
	if !foundHealth {
		t.Fatal("configuration_health widget missing")
	}
}

func TestGetOperationalDashboardForbidden(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	svc := newDependencyTestService(repo, []string{"disclosure.view"})
	sub := caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "co-1"}
	_, err := svc.GetOperationalDashboard(context.Background(), caapp.GetOperationalDashboardRequest{Subject: sub})
	if err == nil {
		t.Fatal("expected error")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusForbidden {
		t.Fatalf("expected 403, got %v", err)
	}
}
