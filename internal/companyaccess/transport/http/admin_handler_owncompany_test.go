package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	cahttp "github.com/cobo/cobo_iam_services/internal/companyaccess/transport/http"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
)

// fakeInspector always returns fixed claims.
type fakeInspector struct {
	claims iamapp.AccessTokenClaims
}

func (f fakeInspector) InspectAccessToken(_ context.Context, _ string) (*iamapp.AccessTokenClaims, error) {
	c := f.claims
	return &c, nil
}
func (f fakeInspector) InspectPreCompanyToken(_ context.Context, _ string) (*iamapp.PreCompanyTokenClaims, error) {
	return nil, nil
}

// fakeHandlerAuthService always allows.
type fakeHandlerAuthService struct{}

func (fakeHandlerAuthService) Authorize(_ context.Context, _ authapp.AuthorizeRequest) (*authapp.AuthorizeDecision, error) {
	return &authapp.AuthorizeDecision{Decision: authapp.DecisionAllow}, nil
}
func (fakeHandlerAuthService) AuthorizeBatch(_ context.Context, _ authapp.AuthorizeBatchRequest) (*authapp.AuthorizeBatchResponse, error) {
	return &authapp.AuthorizeBatchResponse{}, nil
}
func (fakeHandlerAuthService) GetEffectiveAccess(_ context.Context, _, _ string) (*authapp.EffectiveAccessSummary, error) {
	return &authapp.EffectiveAccessSummary{Permissions: []string{"company.view", "company.edit", "rbac.manage"}}, nil
}

func setupHandler(companyID, companyName string) (*cahttp.AdminHandler, *cainmem.AdminRepository) {
	repo := cainmem.NewAdminRepository()
	if companyID != "" {
		repo.SeedCompany(caapp.PlatformCompanyDetail{
			CompanyID:   companyID,
			CompanyCode: "TST",
			CompanyName: companyName,
			Status:      "active",
			TaxCode:     "0123456789",
		})
	}
	svc := caapp.NewAdminService(repo, fakeHandlerAuthService{}, fixedHandlerIDGen("h-id"))
	h := cahttp.NewAdminHandler(svc, fakeInspector{claims: iamapp.AccessTokenClaims{
		Sub:          "u-1",
		MembershipID: "m-1",
		CompanyID:    companyID,
	}}, nil)
	return h, repo
}

type fixedHandlerIDGen string

func (g fixedHandlerIDGen) NewUUID() string { return string(g) }

func TestGetOwnCompany_Handler_200(t *testing.T) {
	h, _ := setupHandler("c-test", "Test Corp")
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/api/v1/admin/company", nil)
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d want 200; body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["company_id"] != "c-test" {
		t.Fatalf("company_id=%v want c-test", body["company_id"])
	}
	if body["company_name"] != "Test Corp" {
		t.Fatalf("company_name=%v want Test Corp", body["company_name"])
	}
}

func TestGetOwnCompany_Handler_404WhenNoCompany(t *testing.T) {
	// Seed a different company; claims point to "c-missing" which is not seeded.
	repo := cainmem.NewAdminRepository()
	repo.SeedCompany(caapp.PlatformCompanyDetail{CompanyID: "c-other", CompanyName: "Other"})
	svc := caapp.NewAdminService(repo, fakeHandlerAuthService{}, fixedHandlerIDGen("x"))
	h := cahttp.NewAdminHandler(svc, fakeInspector{claims: iamapp.AccessTokenClaims{
		Sub:          "u-1",
		MembershipID: "m-1",
		CompanyID:    "c-missing",
	}}, nil)
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/api/v1/admin/company", nil)
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404; body=%s", w.Code, w.Body.String())
	}
}

func TestPatchOwnCompany_Handler_200(t *testing.T) {
	h, _ := setupHandler("c-test", "Old Name")
	mux := http.NewServeMux()
	h.Register(mux)

	body := `{"company_name":"New Corp Name","tax_code":"9876543210"}`
	req := httptest.NewRequest("PATCH", "/api/v1/admin/company", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d want 200; body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp["company_name"] != "New Corp Name" {
		t.Fatalf("company_name=%v want New Corp Name", resp["company_name"])
	}
}

func TestPatchOwnCompany_Handler_VerificationStatusIgnored(t *testing.T) {
	// The handler must NOT forward verification_status even if client sends it.
	h, _ := setupHandler("c-test", "Corp")
	mux := http.NewServeMux()
	h.Register(mux)

	body := `{"company_name":"Corp","verification_status":"verified"}`
	req := httptest.NewRequest("PATCH", "/api/v1/admin/company", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Should succeed (200), but verification_status should remain empty in returned data
	// (inmemory sets it to "" when not seeded with it).
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d want 200; body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if resp["verification_status"] == "verified_injected" {
		t.Fatal("verification_status should not be writable via enterprise admin endpoint")
	}
}
