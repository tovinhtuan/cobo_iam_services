package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	marketapp "github.com/cobo/cobo_iam_services/internal/marketreference/app"
)

type listedFakeInspector struct{}

func (listedFakeInspector) InspectAccessToken(_ context.Context, _ string) (*iamapp.AccessTokenClaims, error) {
	return &iamapp.AccessTokenClaims{Sub: "u_cms", MembershipID: "m1", CompanyID: "c1"}, nil
}

func (listedFakeInspector) InspectPreCompanyToken(context.Context, string) (*iamapp.PreCompanyTokenClaims, error) {
	return nil, nil
}

type listedFakeAuthorizer struct {
	perms []string
}

func (a listedFakeAuthorizer) Authorize(context.Context, authapp.AuthorizeRequest) (*authapp.AuthorizeDecision, error) {
	return nil, nil
}

func (a listedFakeAuthorizer) AuthorizeBatch(context.Context, authapp.AuthorizeBatchRequest) (*authapp.AuthorizeBatchResponse, error) {
	return nil, nil
}

func (a listedFakeAuthorizer) GetEffectiveAccess(context.Context, string, string) (*authapp.EffectiveAccessSummary, error) {
	return &authapp.EffectiveAccessSummary{Permissions: a.perms}, nil
}

type listedFakeRepo struct {
	listFn func(context.Context, marketapp.ListParams) (marketapp.ListResult, error)
	getFn  func(context.Context, string) (marketapp.ListedCompanyDetail, error)
}

func (f *listedFakeRepo) List(ctx context.Context, p marketapp.ListParams) (marketapp.ListResult, error) {
	if f.listFn != nil {
		return f.listFn(ctx, p)
	}
	return marketapp.ListResult{}, nil
}

func (f *listedFakeRepo) GetBySymbol(ctx context.Context, symbol string) (marketapp.ListedCompanyDetail, error) {
	if f.getFn != nil {
		return f.getFn(ctx, symbol)
	}
	return marketapp.ListedCompanyDetail{}, nil
}

func newListedCompaniesTestHandler(t *testing.T, perms []string, svc *marketapp.Service) *Handler {
	t.Helper()
	return &Handler{
		inspector:       listedFakeInspector{},
		authorizer:      listedFakeAuthorizer{perms: perms},
		listedCompanies: svc,
		metrics:         newCMSMetrics(),
	}
}

func TestListListedCompanies_success(t *testing.T) {
	now := time.Now().UTC()
	svc := marketapp.NewService(&listedFakeRepo{
		listFn: func(_ context.Context, p marketapp.ListParams) (marketapp.ListResult, error) {
			if p.Q != "VI" {
				t.Fatalf("q = %q", p.Q)
			}
			return marketapp.ListResult{
				Items: []marketapp.ListedCompanySummary{{
					Symbol: "VIC", CompanyName: "Vingroup", Exchange: "HOSE", HasProfile: true,
					ProfileUpdatedAt: &now, Source: marketapp.ProfileSourceKBS,
				}},
				Total: 1,
			}, nil
		},
	}, nil)
	h := newListedCompaniesTestHandler(t, []string{"platform.cms.view"}, svc)
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/platform/cms/market/listed-companies?q=VI&page=1&limit=20", nil)
	req.Header.Set("Authorization", "Bearer test")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Data struct {
			Items []map[string]any `json:"items"`
		} `json:"data"`
		Meta struct {
			Total int `json:"total"`
			Page  int `json:"page"`
			Limit int `json:"limit"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Meta.Total != 1 || len(body.Data.Items) != 1 {
		t.Fatalf("meta/data: %+v", body)
	}
	if _, ok := body.Data.Items[0]["tax_id"]; ok {
		t.Fatal("list item must not include tax_id")
	}
}

func TestGetListedCompany_fullProfile(t *testing.T) {
	tax := "0100109106"
	svc := marketapp.NewService(&listedFakeRepo{
		getFn: func(_ context.Context, symbol string) (marketapp.ListedCompanyDetail, error) {
			if symbol != "VIC" {
				t.Fatalf("symbol=%q", symbol)
			}
			return marketapp.ListedCompanyDetail{
				Symbol: "VIC", CompanyName: "Vingroup", Source: marketapp.ProfileSourceKBS, HasProfile: true,
				Identity:     &marketapp.IdentityGroup{Exchange: strPtr("HOSE")},
				LegalContact: &marketapp.LegalContactGroup{TaxID: &tax},
			}, nil
		},
	}, nil)
	h := newListedCompaniesTestHandler(t, []string{"platform.cms.view"}, svc)
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/platform/cms/market/listed-companies/vic", nil)
	req.Header.Set("Authorization", "Bearer test")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Data struct {
			HasProfile   bool           `json:"has_profile"`
			LegalContact map[string]any `json:"legal_contact"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Data.HasProfile || body.Data.LegalContact["tax_id"] != tax {
		t.Fatalf("got %+v", body.Data)
	}
}

func TestGetListedCompany_partialProfile(t *testing.T) {
	svc := marketapp.NewService(&listedFakeRepo{
		getFn: func(context.Context, string) (marketapp.ListedCompanyDetail, error) {
			return marketapp.ListedCompanyDetail{
				Symbol: "FPT", CompanyName: "FPT", Source: marketapp.ProfileSourceKBS, HasProfile: false,
			}, nil
		},
	}, nil)
	h := newListedCompaniesTestHandler(t, []string{"platform.cms.view"}, svc)
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/platform/cms/market/listed-companies/FPT", nil)
	req.Header.Set("Authorization", "Bearer test")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Data struct {
			HasProfile bool `json:"has_profile"`
			Identity   any  `json:"identity"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.HasProfile || body.Data.Identity != nil {
		t.Fatalf("got %+v", body.Data)
	}
}

func TestGetListedCompany_notFound(t *testing.T) {
	svc := marketapp.NewService(&listedFakeRepo{
		getFn: func(context.Context, string) (marketapp.ListedCompanyDetail, error) {
			return marketapp.ListedCompanyDetail{}, marketapp.ErrNotFound
		},
	}, nil)
	h := newListedCompaniesTestHandler(t, []string{"platform.cms.view"}, svc)
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/platform/cms/market/listed-companies/ZZZ", nil)
	req.Header.Set("Authorization", "Bearer test")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestListedCompanies_nilServiceStillRegistered503(t *testing.T) {
	h := &Handler{
		inspector:       listedFakeInspector{},
		authorizer:      listedFakeAuthorizer{perms: []string{"platform.cms.view"}},
		listedCompanies: nil,
		metrics:         newCMSMetrics(),
	}
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/platform/cms/market/listed-companies", nil)
	req.Header.Set("Authorization", "Bearer test")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code == http.StatusNotFound {
		t.Fatalf("route must be registered even when listedCompanies is nil; got 404")
	}
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestListedCompanies_disabledService503(t *testing.T) {
	h := newListedCompaniesTestHandler(t, []string{"platform.cms.view"}, marketapp.NewDisabledService())
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/platform/cms/market/listed-companies", nil)
	req.Header.Set("Authorization", "Bearer test")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestListedCompanies_forbiddenWithoutCMSView(t *testing.T) {
	svc := marketapp.NewService(&listedFakeRepo{}, nil)
	h := newListedCompaniesTestHandler(t, []string{"rbac.manage"}, svc)
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/platform/cms/market/listed-companies", nil)
	req.Header.Set("Authorization", "Bearer test")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestListedCompanies_invalidLimit400(t *testing.T) {
	svc := marketapp.NewService(&listedFakeRepo{}, nil)
	h := newListedCompaniesTestHandler(t, []string{"platform.cms.view"}, svc)
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/platform/cms/market/listed-companies?limit=200", nil)
	req.Header.Set("Authorization", "Bearer test")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestListedCompanies_invalidExchange400(t *testing.T) {
	svc := marketapp.NewService(&listedFakeRepo{}, nil)
	h := newListedCompaniesTestHandler(t, []string{"platform.cms.view"}, svc)
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/platform/cms/market/listed-companies?exchange=NYSE", nil)
	req.Header.Set("Authorization", "Bearer test")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func strPtr(s string) *string { return &s }
