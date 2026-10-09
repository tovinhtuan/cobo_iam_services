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

// C4/H2 (risk review 2026-10-09): tenant routes under /api/v1/admin always act on the company of
// the access token. A company_id that differs from the token's company is rejected with
// 403 COMPANY_SCOPE_MISMATCH, whoever the caller is. Cross-company work for platform operators
// goes through /api/v1/platform/cms/admin/*.

type scopeAuth struct{ perms []string }

func (scopeAuth) Authorize(context.Context, authapp.AuthorizeRequest) (*authapp.AuthorizeDecision, error) {
	return &authapp.AuthorizeDecision{Decision: authapp.DecisionAllow}, nil
}

func (scopeAuth) AuthorizeBatch(context.Context, authapp.AuthorizeBatchRequest) (*authapp.AuthorizeBatchResponse, error) {
	return &authapp.AuthorizeBatchResponse{}, nil
}

func (a scopeAuth) GetEffectiveAccess(context.Context, string, string) (*authapp.EffectiveAccessSummary, error) {
	return &authapp.EffectiveAccessSummary{Permissions: a.perms}, nil
}

const (
	scopeHandlerCompany = "c-test"
	scopeHandlerOther   = "c-other"
)

func newScopeHandlerMux(t *testing.T, perms []string) (*http.ServeMux, *cainmem.AdminRepository) {
	t.Helper()
	repo := cainmem.NewAdminRepository()
	if _, err := repo.CreateUser(context.Background(), caapp.UserView{
		UserID: "u-1", LoginID: "admin@example.com", FullName: "Admin", AccountStatus: "active",
	}, "hash", caapp.CreateUserOptions{MembershipID: "m-1", CompanyID: scopeHandlerCompany, MembershipStatus: "active"}); err != nil {
		t.Fatalf("seed admin: %v", err)
	}
	if _, err := repo.CreateUser(context.Background(), caapp.UserView{
		UserID: "u-target", LoginID: "target@example.com", FullName: "Target", AccountStatus: "active",
	}, "hash", caapp.CreateUserOptions{}); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	svc := caapp.NewAdminService(repo, scopeAuth{perms: perms}, fixedHandlerIDGen("h-id"))
	h := cahttp.NewAdminHandler(svc, fakeInspector{claims: iamapp.AccessTokenClaims{
		Sub: "u-1", MembershipID: "m-1", CompanyID: scopeHandlerCompany,
	}}, nil)
	mux := http.NewServeMux()
	h.Register(mux)
	return mux, repo
}

func scopeCall(mux *http.ServeMux, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

func errorCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON %q: %v", w.Body.String(), err)
	}
	return body.Error.Code
}

var (
	scopeTenantPerms   = []string{"rbac.manage", "admin.membership.invite"}
	scopeOperatorPerms = []string{"platform.cms.view", "rbac.manage", "admin.membership.invite"}
)

func TestTenantRoutes_CompanyMustMatchToken(t *testing.T) {
	personas := map[string][]string{"tenantAdmin": scopeTenantPerms, "platformOperator": scopeOperatorPerms}
	for name, perms := range personas {
		t.Run(name, func(t *testing.T) {
			mux, repo := newScopeHandlerMux(t, perms)
			routes := []struct{ method, path, body string }{
				{"POST", "/api/v1/admin/users", `{"login_id":"x@example.com","password":"StrongPass123!","full_name":"X","company_id":"` + scopeHandlerOther + `"}`},
				{"POST", "/api/v1/admin/memberships", `{"user_id":"u-target","company_id":"` + scopeHandlerOther + `"}`},
				{"GET", "/api/v1/admin/companies/" + scopeHandlerOther + "/memberships", ""},
			}
			for _, rt := range routes {
				w := scopeCall(mux, rt.method, rt.path, rt.body)
				if w.Code != http.StatusForbidden || errorCode(t, w) != "COMPANY_SCOPE_MISMATCH" {
					t.Errorf("%s %s: expected 403 COMPANY_SCOPE_MISMATCH, got %d %s", rt.method, rt.path, w.Code, w.Body.String())
				}
			}
			if items, err := repo.ListMembershipsByCompany(context.Background(), scopeHandlerOther); err != nil || len(items) != 0 {
				t.Fatalf("expected no membership in %s, got %d (err=%v)", scopeHandlerOther, len(items), err)
			}
		})
	}
}

func TestTenantRoutes_OwnCompanyStillWorks(t *testing.T) {
	for name, perms := range map[string][]string{"tenantAdmin": scopeTenantPerms, "platformOperator": scopeOperatorPerms} {
		t.Run(name, func(t *testing.T) {
			mux, _ := newScopeHandlerMux(t, perms)

			w := scopeCall(mux, "GET", "/api/v1/admin/companies/"+scopeHandlerCompany+"/memberships", "")
			if w.Code != http.StatusOK {
				t.Fatalf("list own company: %d %s", w.Code, w.Body.String())
			}
			w = scopeCall(mux, "POST", "/api/v1/admin/users",
				`{"login_id":"own@example.com","password":"StrongPass123!","full_name":"Own","company_id":"`+scopeHandlerCompany+`"}`)
			if w.Code != http.StatusCreated {
				t.Fatalf("create user own company: %d %s", w.Code, w.Body.String())
			}
			w = scopeCall(mux, "POST", "/api/v1/admin/memberships", `{"user_id":"u-target","company_id":"`+scopeHandlerCompany+`"}`)
			if w.Code != http.StatusCreated {
				t.Fatalf("create membership own company: %d %s", w.Code, w.Body.String())
			}
		})
	}
}

// An empty company on a tenant route means "the company of the token", also for a platform
// operator: tenant routes never create company-less users or memberships.
func TestTenantRoutes_EmptyCompanyMeansTokenCompany(t *testing.T) {
	for name, perms := range map[string][]string{"tenantAdmin": scopeTenantPerms, "platformOperator": scopeOperatorPerms} {
		t.Run(name, func(t *testing.T) {
			mux, _ := newScopeHandlerMux(t, perms)
			w := scopeCall(mux, "POST", "/api/v1/admin/users",
				`{"login_id":"empty@example.com","password":"StrongPass123!","full_name":"Empty"}`)
			if w.Code != http.StatusCreated {
				t.Fatalf("create user: %d %s", w.Code, w.Body.String())
			}
			var out struct {
				CompanyID    string `json:"company_id"`
				MembershipID string `json:"membership_id"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			if out.CompanyID != scopeHandlerCompany || out.MembershipID == "" {
				t.Fatalf("expected membership in %s, got company=%q membership=%q", scopeHandlerCompany, out.CompanyID, out.MembershipID)
			}
		})
	}
}
