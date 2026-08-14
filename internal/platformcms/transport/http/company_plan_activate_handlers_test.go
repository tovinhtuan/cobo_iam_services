package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
	"github.com/cobo/cobo_iam_services/internal/subscription/companyplan"
)

type activateFakeInspector struct {
	fail bool
}

func (a activateFakeInspector) InspectAccessToken(context.Context, string) (*iamapp.AccessTokenClaims, error) {
	if a.fail {
		return nil, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeSessionExpired, "invalid token", nil)
	}
	return &iamapp.AccessTokenClaims{Sub: "u_cms", MembershipID: "m_cms", CompanyID: "c_platform"}, nil
}

func (activateFakeInspector) InspectPreCompanyToken(context.Context, string) (*iamapp.PreCompanyTokenClaims, error) {
	return nil, nil
}

type activateFakeAuthorizer struct {
	perms []string
}

func (a activateFakeAuthorizer) Authorize(context.Context, authapp.AuthorizeRequest) (*authapp.AuthorizeDecision, error) {
	return &authapp.AuthorizeDecision{Decision: authapp.DecisionAllow}, nil
}
func (a activateFakeAuthorizer) AuthorizeBatch(context.Context, authapp.AuthorizeBatchRequest) (*authapp.AuthorizeBatchResponse, error) {
	return nil, nil
}
func (a activateFakeAuthorizer) GetEffectiveAccess(context.Context, string, string) (*authapp.EffectiveAccessSummary, error) {
	return &authapp.EffectiveAccessSummary{Permissions: a.perms}, nil
}

type capturingAudit struct {
	mu      sync.Mutex
	entries []auditapp.AppendAuditLogRequest
}

func (c *capturingAudit) AppendAuditLog(_ context.Context, req auditapp.AppendAuditLogRequest) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = append(c.entries, req)
	return nil
}

type stubAdminAuth struct {
	permissions []string
}

func (s stubAdminAuth) Authorize(context.Context, authapp.AuthorizeRequest) (*authapp.AuthorizeDecision, error) {
	return &authapp.AuthorizeDecision{Decision: authapp.DecisionAllow}, nil
}
func (s stubAdminAuth) AuthorizeBatch(context.Context, authapp.AuthorizeBatchRequest) (*authapp.AuthorizeBatchResponse, error) {
	return nil, nil
}
func (s stubAdminAuth) GetEffectiveAccess(context.Context, string, string) (*authapp.EffectiveAccessSummary, error) {
	return &authapp.EffectiveAccessSummary{Permissions: s.permissions}, nil
}

func newActivateTestHandler(t *testing.T, handlerPerms []string, inspectorFail bool) (*Handler, *companyplan.MemoryRepository, *capturingAudit) {
	t.Helper()
	plans := companyplan.NewMemoryRepository()
	adminRepo := cainmem.NewAdminRepository()
	adminRepo.SeedCompany(caapp.PlatformCompanyDetail{
		CompanyID: "c-target", CompanyCode: "ABC001", CompanyName: "Công ty ABC", Status: "active",
	})
	adminSvc := caapp.NewAdminService(
		adminRepo,
		stubAdminAuth{permissions: []string{"rbac.manage"}},
		idgen.UUIDv7Generator{},
		caapp.WithCompanyPlanReader(companyplan.NewService(plans)),
	)
	audit := &capturingAudit{}
	h := &Handler{
		inspector:       activateFakeInspector{fail: inspectorFail},
		authorizer:      activateFakeAuthorizer{perms: handlerPerms},
		adminSvc:        adminSvc,
		auditSvc:        audit,
		companyPlanRepo: plans,
		metrics:         newCMSMetrics(),
	}
	return h, plans, audit
}

func doActivate(h *Handler, companyID, planCode string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]string{
		"plan_code":         planCode,
		"verified_amount":   "5000000",
		"payment_reference": "COBO ABC001 NANGCAPGOI",
		"verification_note": "checked bank statement",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/platform/cms/admin/companies/"+companyID+"/subscription/activate", bytes.NewReader(body))
	req.SetPathValue("company_id", companyID)
	req.Header.Set("Authorization", "Bearer t")
	rec := httptest.NewRecorder()
	h.postCMSCompanySubscriptionActivate(rec, req)
	return rec
}

func TestPostCMSCompanySubscriptionActivate_PlatformOKAndIdempotent(t *testing.T) {
	h, plans, audit := newActivateTestHandler(t, []string{"platform.cms.view", "rbac.manage"}, false)
	rec := doActivate(h, "c-target", "PREMIUM")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec2 := doActivate(h, "c-target", "PREMIUM")
	if rec2.Code != http.StatusOK {
		t.Fatalf("idempotent status=%d body=%s", rec2.Code, rec2.Body.String())
	}
	if !strings.Contains(rec2.Body.String(), `"already_active":true`) {
		t.Fatalf("want already_active true: %s", rec2.Body.String())
	}
	occ, _ := plans.ListOccupyingByCompany(context.Background(), "c-target")
	active := 0
	for _, p := range occ {
		if p.Status == companyplan.PlanStatusActive {
			active++
		}
	}
	if active != 1 {
		t.Fatalf("duplicate occupying ACTIVE=%d %+v", active, occ)
	}
	if len(audit.entries) < 2 || audit.entries[0].Action != cmsActionCompanyPlanActivate {
		t.Fatalf("audit=%+v", audit.entries)
	}
	if audit.entries[0].CompanyID != "c-target" {
		t.Fatalf("audit company=%s", audit.entries[0].CompanyID)
	}
}

func TestPostCMSCompanySubscriptionActivate_ForbiddenAndValidation(t *testing.T) {
	ent, _, _ := newActivateTestHandler(t, []string{"company.view", "company.edit"}, false)
	if rec := doActivate(ent, "c-target", "PREMIUM"); rec.Code != http.StatusForbidden {
		t.Fatalf("enterprise want 403 got %d %s", rec.Code, rec.Body.String())
	}

	unauth, _, _ := newActivateTestHandler(t, []string{"platform.cms.view", "rbac.manage"}, true)
	if rec := doActivate(unauth, "c-target", "PREMIUM"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauth want 401 got %d", rec.Code)
	}

	h, _, _ := newActivateTestHandler(t, []string{"platform.cms.view", "rbac.manage"}, false)
	if rec := doActivate(h, "c-target", "FREE"); rec.Code != http.StatusBadRequest {
		t.Fatalf("FREE want 400 got %d %s", rec.Code, rec.Body.String())
	}
	if rec := doActivate(h, "missing", "PREMIUM"); rec.Code != http.StatusNotFound && rec.Code != http.StatusBadRequest {
		t.Fatalf("missing company want 404/400 got %d %s", rec.Code, rec.Body.String())
	}
}

func TestLookupCompanyCodeSQLUsesCompanyCodeColumn(t *testing.T) {
	src, err := os.ReadFile("subscription_upgrade_handlers.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "SELECT company_code FROM companies") {
		t.Fatal("lookup must select company_code")
	}
	if strings.Contains(string(src), "SELECT code FROM companies") {
		t.Fatal("must not select legacy code column")
	}
	if strings.Contains(string(src), `ReplaceAll(companyID, "-", "")`) {
		t.Fatal("must not fallback payment note to UUID")
	}
}

func TestListCompaniesSQLSearchesCompanyCode(t *testing.T) {
	src, err := os.ReadFile("../../../companyaccess/infra/mysql/admin_repository_companies.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "c.company_code LIKE ?") {
		t.Fatal("platform company list must search company_code")
	}
}
