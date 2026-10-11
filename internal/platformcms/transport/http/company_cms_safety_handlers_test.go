package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	auditinmem "github.com/cobo/cobo_iam_services/internal/audit/infra/inmemory"
	companyaccessapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

// Contract: docs/ai-cache/cms-companies-safety-2026-10-11/02-contract.md (handler layer).

func newCompanySafetyHandler(t *testing.T, perms []string) (*Handler, *cainmem.AdminRepository) {
	t.Helper()
	repo := cainmem.NewAdminRepository()
	repo.SeedCompany(companyaccessapp.PlatformCompanyDetail{
		CompanyID: "c_target", CompanyCode: "ABC001", CompanyName: "Công ty ABC", Status: "active",
		VerificationStatus: "unverified", UpdatedAtRFC3339: "2026-10-01T03:00:00Z",
	})
	authz := listedFakeAuthorizer{perms: perms}
	svc := companyaccessapp.NewAdminService(repo, authz, idgen.UUIDv7Generator{},
		companyaccessapp.WithAuditRepository(auditinmem.NewRepository()))
	return &Handler{inspector: listedFakeInspector{}, authorizer: authz, adminSvc: svc, metrics: newCMSMetrics()}, repo
}

func doCompanyRequest(h *Handler, method, path, body string) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body == "" {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader([]byte(body))
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	mux := http.NewServeMux()
	h.Register(mux)
	mux.ServeHTTP(w, req)
	return w
}

func decodeEnvelope(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode %s: %v", w.Body.String(), err)
	}
	return out
}

var operatorPerms = []string{"platform.cms.view", "rbac.manage"}

func TestCompanyStatusHandler_ReasonRequiredAndMetaStatusChange(t *testing.T) {
	h, _ := newCompanySafetyHandler(t, operatorPerms)

	// Old clients (no body) are rejected with field errors (D-01).
	w := doCompanyRequest(h, http.MethodPost, "/api/v1/platform/cms/admin/companies/c_target/suspend", "")
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "reason_code") {
		t.Fatalf("no body: %d %s", w.Code, w.Body.String())
	}

	w = doCompanyRequest(h, http.MethodPost, "/api/v1/platform/cms/admin/companies/c_target/suspend", `{"reason_code":"PAYMENT_OVERDUE"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("suspend: %d %s", w.Code, w.Body.String())
	}
	env := decodeEnvelope(t, w)
	data, _ := env["data"].(map[string]any)
	meta, _ := env["meta"].(map[string]any)
	change, _ := meta["status_change"].(map[string]any)
	if data["status"] != "suspended" || change["changed"] != true || change["from"] != "active" || change["to"] != "suspended" {
		t.Fatalf("data=%v meta=%v", data, meta)
	}
	if rev, _ := data["revision"].(string); !strings.HasPrefix(rev, "co1:") {
		t.Fatalf("revision missing: %v", data["revision"])
	}

	// Deactivate needs the company code.
	w = doCompanyRequest(h, http.MethodPost, "/api/v1/platform/cms/admin/companies/c_target/deactivate", `{"reason_code":"CONTRACT_ENDED","confirm_company_code":"WRONG"}`)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "confirm_company_code") {
		t.Fatalf("wrong code: %d %s", w.Code, w.Body.String())
	}
	w = doCompanyRequest(h, http.MethodPost, "/api/v1/platform/cms/admin/companies/c_target/deactivate", `{"reason_code":"CONTRACT_ENDED","confirm_company_code":"abc001"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("deactivate: %d %s", w.Code, w.Body.String())
	}
	// inactive → suspend: 409 INVALID_STATE_TRANSITION
	w = doCompanyRequest(h, http.MethodPost, "/api/v1/platform/cms/admin/companies/c_target/suspend", `{"reason_code":"SECURITY"}`)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "INVALID_STATE_TRANSITION") {
		t.Fatalf("inactive→suspend: %d %s", w.Code, w.Body.String())
	}
}

func TestCompanyHandlers_BodyLimitsAndStaleRevision(t *testing.T) {
	h, _ := newCompanySafetyHandler(t, operatorPerms)

	big := `{"reason_code":"OTHER","reason_note":"` + strings.Repeat("x", 20<<10) + `"}`
	if w := doCompanyRequest(h, http.MethodPost, "/api/v1/platform/cms/admin/companies/c_target/suspend", big); w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status body > 16 KiB: %d", w.Code)
	}
	bigPatch := `{"address":"` + strings.Repeat("x", 70<<10) + `"}`
	if w := doCompanyRequest(h, http.MethodPatch, "/api/v1/platform/cms/admin/companies/c_target", bigPatch); w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("patch body > 64 KiB: %d", w.Code)
	}
	if w := doCompanyRequest(h, http.MethodPatch, "/api/v1/platform/cms/admin/companies/c_target", `{"tax_code":"123"}`); w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "field_errors") {
		t.Fatalf("invalid tax code: %d %s", w.Code, w.Body.String())
	}

	w := doCompanyRequest(h, http.MethodGet, "/api/v1/platform/cms/admin/companies/c_target", "")
	rev, _ := decodeEnvelope(t, w)["data"].(map[string]any)["revision"].(string)
	if w := doCompanyRequest(h, http.MethodPatch, "/api/v1/platform/cms/admin/companies/c_target", `{"address":"Hà Nội","expected_revision":"`+rev+`"}`); w.Code != http.StatusOK {
		t.Fatalf("fresh patch: %d %s", w.Code, w.Body.String())
	}
	w = doCompanyRequest(h, http.MethodPatch, "/api/v1/platform/cms/admin/companies/c_target", `{"address":"Huế","expected_revision":"`+rev+`"}`)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "STALE_ETAG") || !strings.Contains(w.Body.String(), "current_revision") {
		t.Fatalf("stale patch: %d %s", w.Code, w.Body.String())
	}
}

func TestCompanyHistoryHandler_AndEnterpriseAdminForbidden(t *testing.T) {
	h, _ := newCompanySafetyHandler(t, operatorPerms)
	w := doCompanyRequest(h, http.MethodGet, "/api/v1/platform/cms/admin/companies/c_target/history?limit=5", "")
	if w.Code != http.StatusOK {
		t.Fatalf("history: %d %s", w.Code, w.Body.String())
	}
	env := decodeEnvelope(t, w)
	if _, ok := env["data"].(map[string]any)["items"].([]any); !ok {
		t.Fatalf("history items missing: %v", env)
	}
	if env["meta"].(map[string]any)["limit"] != float64(5) {
		t.Fatalf("meta = %v", env["meta"])
	}
	if w := doCompanyRequest(h, http.MethodGet, "/api/v1/platform/cms/admin/companies/c_target/history?cursor=not-a-cursor", ""); w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), `"cursor"`) {
		t.Fatalf("malformed cursor: %d %s", w.Code, w.Body.String())
	}

	ent, _ := newCompanySafetyHandler(t, []string{"rbac.manage", "admin.membership.invite"})
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/platform/cms/admin/companies/c_target/history", ""},
		{http.MethodPost, "/api/v1/platform/cms/admin/companies/c_target/suspend", `{"reason_code":"SECURITY"}`},
		{http.MethodPatch, "/api/v1/platform/cms/admin/companies/c_target", `{"address":"x"}`},
	} {
		if w := doCompanyRequest(ent, tc.method, tc.path, tc.body); w.Code != http.StatusForbidden {
			t.Fatalf("%s %s as enterprise admin: %d", tc.method, tc.path, w.Code)
		}
	}
}
