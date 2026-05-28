package http

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
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	"github.com/cobo/cobo_iam_services/internal/platform/idempotency"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type provisionFakeInspector struct{}

func (provisionFakeInspector) InspectAccessToken(context.Context, string) (*iamapp.AccessTokenClaims, error) {
	return &iamapp.AccessTokenClaims{Sub: "u1", SessionID: "sess-1"}, nil
}

func (provisionFakeInspector) InspectPreCompanyToken(context.Context, string) (*iamapp.PreCompanyTokenClaims, error) {
	return nil, nil
}

type fixedProvisionIDGen string

func (g fixedProvisionIDGen) NewUUID() string { return string(g) }

type provisionAllowAuth struct{}

func (provisionAllowAuth) Authorize(context.Context, authapp.AuthorizeRequest) (*authapp.AuthorizeDecision, error) {
	return &authapp.AuthorizeDecision{Decision: authapp.DecisionAllow}, nil
}

func (provisionAllowAuth) AuthorizeBatch(context.Context, authapp.AuthorizeBatchRequest) (*authapp.AuthorizeBatchResponse, error) {
	return &authapp.AuthorizeBatchResponse{}, nil
}

func (provisionAllowAuth) GetEffectiveAccess(context.Context, string, string) (*authapp.EffectiveAccessSummary, error) {
	return &authapp.EffectiveAccessSummary{}, nil
}

type okTokenIssuer struct{}

func (okTokenIssuer) IssueAccessToken(context.Context, iamapp.AccessTokenClaims) (string, int64, error) {
	return "access_ok", 3600, nil
}

func (okTokenIssuer) IssuePreCompanyToken(context.Context, string, string) (string, int64, error) {
	return "", 0, nil
}

func (okTokenIssuer) IssueRefreshToken(context.Context, string, string) (string, error) {
	return "refresh_ok", nil
}

type failingTokenIssuer struct{}

func (failingTokenIssuer) IssueAccessToken(context.Context, iamapp.AccessTokenClaims) (string, int64, error) {
	return "", 0, nil
}

func (failingTokenIssuer) IssuePreCompanyToken(context.Context, string, string) (string, int64, error) {
	return "", 0, nil
}

func (failingTokenIssuer) IssueRefreshToken(context.Context, string, string) (string, error) {
	return "", nil
}

type fakeSessionRepo struct{}

func (fakeSessionRepo) Create(context.Context, iamapp.CreateSessionParams) error { return nil }
func (fakeSessionRepo) FindByRefreshToken(context.Context, string) (*iamapp.SessionState, error) {
	return nil, nil
}
func (fakeSessionRepo) RevokeByRefreshToken(context.Context, string) error { return nil }
func (fakeSessionRepo) UpdateContext(context.Context, string, string, string) error { return nil }
func (fakeSessionRepo) RotateRefreshToken(context.Context, string, string) error    { return nil }
func (fakeSessionRepo) ListByUser(context.Context, string) ([]iamapp.SessionState, error) {
	return nil, nil
}
func (fakeSessionRepo) RevokeBySessionID(context.Context, string, string) error { return nil }
func (fakeSessionRepo) RevokeAllByUser(context.Context, string, string) error  { return nil }
func (fakeSessionRepo) AssertSessionActive(context.Context, string) error     { return nil }

type fakeIdemStore struct {
	tryResult idempotency.Result
	complete  bool
	abandon   bool
}

func (f *fakeIdemStore) TryReserve(context.Context, idempotency.Params) (idempotency.Result, error) {
	return f.tryResult, nil
}

func (f *fakeIdemStore) Complete(context.Context, string, []byte) error {
	f.complete = true
	return nil
}

func (f *fakeIdemStore) Abandon(context.Context, string) error {
	f.abandon = true
	return nil
}

func newInitializeHandler(t *testing.T) (*AdminHandler, *caapp.AdminService) {
	t.Helper()
	repo := cainmem.NewAdminRepository()
	_, err := repo.CreateUser(context.Background(), caapp.UserView{
		UserID: "u1", LoginID: "u1@example.com", FullName: "U1", AccountStatus: "active",
	}, "hash", caapp.CreateUserOptions{})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	svc := caapp.NewAdminService(repo, provisionAllowAuth{}, fixedProvisionIDGen("m_new"))
	h := NewAdminHandler(svc, provisionFakeInspector{}, nil)
	h.WithTokenIssuer(okTokenIssuer{}, fakeSessionRepo{})
	return h, &svc
}

func TestInitializeCompany_IdempotencyReplay(t *testing.T) {
	h, _ := newInitializeHandler(t)
	idem := &fakeIdemStore{
		tryResult: idempotency.Result{
			Replay: true, ReplayHTTPStatus: http.StatusCreated, ReplayBody: []byte(`{"company_id":"cached"}`),
		},
	}
	h.WithIdempotency(idem, false)
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/company/initialize", strings.NewReader(`{"company_name":"Acme"}`))
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Idempotency-Key", "key-1")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestInitializeCompany_EmptyTokenReturns500(t *testing.T) {
	h, _ := newInitializeHandler(t)
	h.tokenIssuer = failingTokenIssuer{}
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/company/initialize", strings.NewReader(`{"company_name":"Acme"}`))
	req.Header.Set("Authorization", "Bearer t")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d want 500", rr.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != string(perr.CodeSessionContextUpdateFailed) {
		t.Fatalf("code=%v", errObj["code"])
	}
}

func TestInitializeCompany_CompletesIdempotencyOnSuccess(t *testing.T) {
	h, _ := newInitializeHandler(t)
	idem := &fakeIdemStore{tryResult: idempotency.Result{ReservationID: "res-1"}}
	h.WithIdempotency(idem, false)
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/company/initialize", strings.NewReader(`{"company_name":"Acme"}`))
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Idempotency-Key", "key-2")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !idem.complete {
		t.Fatal("expected idempotency complete")
	}
}

func TestCompanyProvisionRequestHash_DifferentBodyConflicts(t *testing.T) {
	h1 := companyProvisionRequestHash("u1", companyProvisionBody{CompanyName: "A"}, "initialize")
	h2 := companyProvisionRequestHash("u1", companyProvisionBody{CompanyName: "B"}, "initialize")
	if h1 == h2 {
		t.Fatal("expected different hashes for different bodies")
	}
}
