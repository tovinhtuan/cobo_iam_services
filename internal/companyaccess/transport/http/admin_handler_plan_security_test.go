package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	cahttp "github.com/cobo/cobo_iam_services/internal/companyaccess/transport/http"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/subscription/companyplan"
)

func TestGetOwnCompany_Handler_Unauthenticated(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	repo.SeedCompany(caapp.PlatformCompanyDetail{CompanyID: "c-test", CompanyName: "Test", Status: "active"})
	svc := caapp.NewAdminService(repo, fakeHandlerAuthService{}, fixedHandlerIDGen("x"))
	h := cahttp.NewAdminHandler(svc, failInspector{}, nil)
	mux := http.NewServeMux()
	h.Register(mux)
	req := httptest.NewRequest("GET", "/api/v1/admin/company", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		t.Fatalf("unauthenticated must not be 200, got %d body=%s", w.Code, w.Body.String())
	}
}

type failInspector struct{}

func (failInspector) InspectAccessToken(context.Context, string) (*iamapp.AccessTokenClaims, error) {
	return nil, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeInvalidCredentials, "missing token", nil)
}
func (failInspector) InspectPreCompanyToken(context.Context, string) (*iamapp.PreCompanyTokenClaims, error) {
	return nil, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeInvalidCredentials, "missing token", nil)
}

type denyAuthService struct{}

func (denyAuthService) Authorize(_ context.Context, _ authapp.AuthorizeRequest) (*authapp.AuthorizeDecision, error) {
	return &authapp.AuthorizeDecision{Decision: authapp.DecisionDeny}, nil
}
func (denyAuthService) AuthorizeBatch(_ context.Context, _ authapp.AuthorizeBatchRequest) (*authapp.AuthorizeBatchResponse, error) {
	return &authapp.AuthorizeBatchResponse{}, nil
}
func (denyAuthService) GetEffectiveAccess(_ context.Context, _, _ string) (*authapp.EffectiveAccessSummary, error) {
	return &authapp.EffectiveAccessSummary{Permissions: []string{}}, nil
}

func TestGetOwnCompany_Handler_MissingCompanyView(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	repo.SeedCompany(caapp.PlatformCompanyDetail{CompanyID: "c-test", CompanyName: "Test", Status: "active"})
	svc := caapp.NewAdminService(repo, denyAuthService{}, fixedHandlerIDGen("x"),
		caapp.WithCompanyPlanReader(companyplan.NewService(companyplan.NewMemoryRepository())),
	)
	h := cahttp.NewAdminHandler(svc, fakeInspector{claims: iamapp.AccessTokenClaims{
		Sub: "u-1", MembershipID: "m-1", CompanyID: "c-test",
	}}, nil)
	mux := http.NewServeMux()
	h.Register(mux)
	req := httptest.NewRequest("GET", "/api/v1/admin/company", nil)
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403 body=%s", w.Code, w.Body.String())
	}
}

func TestGetOwnCompany_Handler_PlanDTOShape_NoBillingLeak(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	repo.SeedCompany(caapp.PlatformCompanyDetail{CompanyID: "c-test", CompanyCode: "TST", CompanyName: "Test", Status: "active"})
	plans := companyplan.NewMemoryRepository()
	_ = plans.Create(context.Background(), companyplan.CompanyPlan{
		ID: "p1", CompanyID: "c-test", Code: companyplan.PlanCodePremium, Status: companyplan.PlanStatusActive,
		EffectiveFrom: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Origin: companyplan.RecordOriginDevFixture,
	})
	svc := caapp.NewAdminService(repo, fakeHandlerAuthService{}, fixedHandlerIDGen("x"),
		caapp.WithCompanyPlanReader(companyplan.NewService(plans)),
		caapp.WithCompanyPlanNow(func() time.Time { return time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC) }),
	)
	h := cahttp.NewAdminHandler(svc, fakeInspector{claims: iamapp.AccessTokenClaims{
		Sub: "u-1", MembershipID: "m-1", CompanyID: "c-test",
	}}, nil)
	mux := http.NewServeMux()
	h.Register(mux)
	req := httptest.NewRequest("GET", "/api/v1/admin/company", nil)
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	plan, ok := body["plan"].(map[string]any)
	if !ok {
		t.Fatalf("plan=%v", body["plan"])
	}
	for _, forbidden := range []string{"invoice", "amount", "payment_method", "billing_account", "contract", "entitlement"} {
		if _, hit := plan[forbidden]; hit {
			t.Fatalf("plan must not expose %s", forbidden)
		}
	}
	for _, reqKey := range []string{"code", "display_name", "status", "source"} {
		if _, ok := plan[reqKey]; !ok {
			t.Fatalf("missing %s", reqKey)
		}
	}
	if len(plan) != 4 {
		t.Fatalf("plan keys=%v want exactly 4", plan)
	}
}
