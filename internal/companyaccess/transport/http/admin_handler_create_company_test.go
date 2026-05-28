package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	"github.com/cobo/cobo_iam_services/internal/platform/idempotency"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type recordingAudit struct {
	last auditapp.AppendAuditLogRequest
}

func (r *recordingAudit) AppendAuditLog(_ context.Context, req auditapp.AppendAuditLogRequest) error {
	r.last = req
	return nil
}

func newCreateHandler(t *testing.T, selfCreate bool) (*AdminHandler, *cainmem.AdminRepository) {
	t.Helper()
	repo := cainmem.NewAdminRepository()
	_, err := repo.CreateUser(context.Background(), caapp.UserView{
		UserID: "u1", LoginID: "u1@example.com", FullName: "U1", AccountStatus: "active",
	}, "hash", caapp.CreateUserOptions{})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	initSvc := caapp.NewAdminService(repo, provisionAllowAuth{}, fixedProvisionIDGen("m_init"))
	if _, err := initSvc.InitializeCompany(context.Background(), caapp.InitializeCompanyRequest{
		UserID: "u1", CompanyName: "First Co",
	}); err != nil {
		t.Fatalf("InitializeCompany: %v", err)
	}

	svc := caapp.NewAdminService(repo, provisionAllowAuth{}, fixedProvisionIDGen("m_new"),
		caapp.WithSubscriptionTierLookup(func(context.Context, string) string { return "Premium" }))
	h := NewAdminHandler(svc, provisionFakeInspector{}, &recordingAudit{})
	h.WithTokenIssuer(okTokenIssuer{}, fakeSessionRepo{})
	h.WithSelfCreateEnabled(selfCreate)
	return h, repo
}

func TestCreateSelfServiceCompany_FeatureFlagOff(t *testing.T) {
	h, _ := newCreateHandler(t, false)
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/company/create", strings.NewReader(`{"company_name":"New Co"}`))
	req.Header.Set("Authorization", "Bearer t")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != string(perr.CodeFeatureDisabled) {
		t.Fatalf("code=%v", errObj["code"])
	}
}

func TestCreateSelfServiceCompany_SuccessAndAuditContext(t *testing.T) {
	audit := &recordingAudit{}
	h, _ := newCreateHandler(t, true)
	h.audit = audit
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/company/create", strings.NewReader(`{"company_name":"New Co"}`))
	req.Header.Set("Authorization", "Bearer t")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if audit.last.Action != "company.create_self_service" {
		t.Fatalf("audit action=%q", audit.last.Action)
	}
	if audit.last.ActorMembershipID == "" || audit.last.CompanyID == "" {
		t.Fatalf("audit must use new membership/company context: %+v", audit.last)
	}
	if audit.last.ActorMembershipID != "m_new" {
		t.Fatalf("audit membership=%q want m_new", audit.last.ActorMembershipID)
	}
}

func TestCreateSelfServiceCompany_IdempotencyReplay(t *testing.T) {
	h, _ := newCreateHandler(t, true)
	idem := &fakeIdemStore{
		tryResult: idempotency.Result{
			Replay: true, ReplayHTTPStatus: http.StatusCreated, ReplayBody: []byte(`{"company_id":"cached"}`),
		},
	}
	h.WithIdempotency(idem, false)
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/company/create", strings.NewReader(`{"company_name":"New Co"}`))
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Idempotency-Key", "create-key-1")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestCreateSelfServiceCompany_TokenFailureRollback(t *testing.T) {
	h, repo := newCreateHandler(t, true)
	h.tokenIssuer = failingTokenIssuer{}
	mux := http.NewServeMux()
	h.Register(mux)

	before, err := repo.CountEligibleMembershipsForUser(context.Background(), "u1")
	if err != nil {
		t.Fatalf("CountEligibleMembershipsForUser: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/company/create", strings.NewReader(`{"company_name":"Rollback Co"}`))
	req.Header.Set("Authorization", "Bearer t")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rr.Code)
	}
	after, err := repo.CountEligibleMembershipsForUser(context.Background(), "u1")
	if err != nil {
		t.Fatalf("CountEligibleMembershipsForUser: %v", err)
	}
	if after != before {
		t.Fatalf("expected rollback, eligible memberships before=%d after=%d", before, after)
	}
}

func TestCreateSelfServiceCompany_TokenFailureAbandonsIdempotency(t *testing.T) {
	h, _ := newCreateHandler(t, true)
	h.tokenIssuer = failingTokenIssuer{}
	idem := &fakeIdemStore{tryResult: idempotency.Result{ReservationID: "res-create-rollback"}}
	h.WithIdempotency(idem, false)
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/company/create", strings.NewReader(`{"company_name":"Rollback Co"}`))
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Idempotency-Key", "create-rollback-key")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rr.Code)
	}
	if !idem.abandon {
		t.Fatal("expected idempotency abandon on session/token failure")
	}
}

func TestCompanyProvisionRequestHash_CreateDifferentBodyConflicts(t *testing.T) {
	h1 := companyProvisionRequestHash("u1", companyProvisionBody{CompanyName: "A"}, "create")
	h2 := companyProvisionRequestHash("u1", companyProvisionBody{CompanyName: "B"}, "create")
	if h1 == h2 {
		t.Fatal("expected different hashes")
	}
}
