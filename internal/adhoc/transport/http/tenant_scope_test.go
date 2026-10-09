package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
)

// C3 (risk review 2026-10-09): the adhoc module must not expose the cross-tenant
// legacy migration route that auto-approved every tenant's pending_admin_approval
// proposals while impersonating each tenant's process controller.
func TestLegacyMigrationRoute_NotExposed(t *testing.T) {
	svc := &fakeService{}
	handler := NewHandler(discardLogger(), svc, fakeInspector{}, nil)
	mux := http.NewServeMux()
	handler.Register(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/platform/cms/admin/ops/adhoc-migrate-legacy-approvals", nil)
	req.Header.Set("Authorization", "Bearer atk_tenant_company_001")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 (route removed), got %d: %s", rr.Code, rr.Body.String())
	}
}

// Legacy proposals are now resolved only inside their own tenant via admin-approve.
// The tenant scope must come from the access token: adhocapp.Subject has no json
// tags, so a crafted body can decode into it and must be overwritten.
func TestAdminApprove_UsesTokenCompanyIgnoringBodySubject(t *testing.T) {
	svc := &fakeService{}
	handler := NewHandler(discardLogger(), svc, fakeInspector{}, nil)
	mux := http.NewServeMux()
	handler.Register(mux)

	body := `{"Subject":{"UserID":"user-B","MembershipID":"member-B","CompanyID":"company-B"},"final_t0_date":"2026-06-01"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/company/ad-hoc-proposals/proposal-B/admin-approve", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer atk_tenant_company_001")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	want := adhocapp.Subject{UserID: "user-001", MembershipID: "member-001", CompanyID: "company-001"}
	if got := svc.lastAdminApprove.Subject; got != want {
		t.Fatalf("expected token subject %+v, got %+v", want, got)
	}
}

// The adhoc module serves tenant routes only; platform/cross-tenant operations
// must not be registered here again.
func TestRegister_NoPlatformRoutes(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(src), "/api/v1/platform/") {
			t.Errorf("%s registers or references a /api/v1/platform/ route; adhoc routes must stay tenant-scoped", f)
		}
	}
}

// Every mutating adhoc route must take Subject from the access token, never from a
// crafted body (adhocapp.Subject has no json tags, so it is decodable).
func TestMutatingRoutes_UseTokenSubjectIgnoringBodySubject(t *testing.T) {
	spoof := `{"Subject":{"UserID":"user-B","MembershipID":"member-B","CompanyID":"company-B"},"reject_reason":"x","title":"t"}`
	want := adhocapp.Subject{UserID: "user-001", MembershipID: "member-001", CompanyID: "company-001"}
	routes := []struct{ method, path string }{
		{http.MethodPost, "/api/v1/company/ad-hoc-proposals"},
		{http.MethodPatch, "/api/v1/company/ad-hoc-proposals/proposal-B"},
		{http.MethodPost, "/api/v1/company/ad-hoc-proposals/proposal-B/approve"},
		{http.MethodPost, "/api/v1/company/ad-hoc-proposals/proposal-B/focal-approve"},
		{http.MethodPost, "/api/v1/company/ad-hoc-proposals/proposal-B/reject"},
		{http.MethodPost, "/api/v1/company/ad-hoc-proposals/proposal-B/cancel"},
	}
	for _, rt := range routes {
		svc := &fakeService{}
		handler := NewHandler(discardLogger(), svc, fakeInspector{}, nil)
		mux := http.NewServeMux()
		handler.Register(mux)
		req := httptest.NewRequest(rt.method, rt.path, strings.NewReader(spoof))
		req.Header.Set("Authorization", "Bearer atk_tenant_company_001")
		mux.ServeHTTP(httptest.NewRecorder(), req)
		if svc.lastSubject != want {
			t.Errorf("%s %s: expected token subject %+v, got %+v", rt.method, rt.path, want, svc.lastSubject)
		}
	}
}

// Defense in depth: request structs never decode Subject from JSON.
func TestRequestStructs_DoNotDecodeSubjectFromJSON(t *testing.T) {
	spoof := []byte(`{"Subject":{"UserID":"user-B","MembershipID":"member-B","CompanyID":"company-B"}}`)
	targets := map[string]interface{ subject() adhocapp.Subject }{
		"AdminApproveRequest":       &adminApproveProbe{},
		"ApproveRequest":            &approveProbe{},
		"RejectRequest":             &rejectProbe{},
		"CreateProposalRequest":     &createProbe{},
		"PatchDraftProposalRequest": &patchProbe{},
	}
	for name, target := range targets {
		if err := json.Unmarshal(spoof, target); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := target.subject(); got != (adhocapp.Subject{}) {
			t.Errorf("%s decoded Subject from JSON: %+v", name, got)
		}
	}
}

type adminApproveProbe struct{ adhocapp.AdminApproveRequest }
type approveProbe struct{ adhocapp.ApproveRequest }
type rejectProbe struct{ adhocapp.RejectRequest }
type createProbe struct{ adhocapp.CreateProposalRequest }
type patchProbe struct {
	adhocapp.PatchDraftProposalRequest
}

func (p *adminApproveProbe) subject() adhocapp.Subject { return p.Subject }
func (p *approveProbe) subject() adhocapp.Subject      { return p.Subject }
func (p *rejectProbe) subject() adhocapp.Subject       { return p.Subject }
func (p *createProbe) subject() adhocapp.Subject       { return p.Subject }
func (p *patchProbe) subject() adhocapp.Subject        { return p.Subject }
