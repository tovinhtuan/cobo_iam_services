package app_test

import (
	"context"
	"net/http"
	"testing"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	"github.com/cobo/cobo_iam_services/internal/companyaccess/recommendation"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func TestGetRuleRecommendationsFromHealthChecks(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	svc := newHealthTestService(repo, []string{"rbac.manage"})
	sub := caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "co-1"}
	out, err := svc.GetRuleRecommendations(context.Background(), caapp.GetRuleRecommendationsRequest{Subject: sub})
	if err != nil {
		t.Fatalf("GetRuleRecommendations: %v", err)
	}
	if out.Source != recommendation.SourceConfigurationHealth {
		t.Fatalf("source=%q", out.Source)
	}
	if out.Items == nil {
		t.Fatal("items required")
	}
	if len(out.Items) == 0 {
		t.Fatal("expected at least one recommendation from default health checks")
	}
	if out.Score == nil {
		t.Fatal("score passthrough expected")
	}
	for _, item := range out.Items {
		if item.Source != recommendation.SourceConfigurationHealth {
			t.Fatalf("item source=%q", item.Source)
		}
		if item.ActionLink == "" {
			t.Fatal("action_link required")
		}
	}
}

func TestGetRuleRecommendationsDeniedWithoutPermission(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	svc := newHealthTestService(repo, []string{"disclosure.view"})
	sub := caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "co-1"}
	_, err := svc.GetRuleRecommendations(context.Background(), caapp.GetRuleRecommendationsRequest{Subject: sub})
	if err == nil {
		t.Fatal("expected error")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusForbidden {
		t.Fatalf("expected 403, got %v", err)
	}
}

func TestGetRuleRecommendationsRequiresCompanyScope(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	svc := newHealthTestService(repo, []string{"system.settings"})
	sub := caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: ""}
	_, err := svc.GetRuleRecommendations(context.Background(), caapp.GetRuleRecommendationsRequest{Subject: sub})
	if err == nil {
		t.Fatal("expected error for missing company")
	}
}
