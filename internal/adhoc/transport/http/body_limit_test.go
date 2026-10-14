package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProposalWriteRoutes_BoundAndValidateBody(t *testing.T) {
	oversized := `{"title":"` + strings.Repeat("a", int(maxProposalWriteBodyBytes)) + `"}`
	routes := []struct{ method, path string }{
		{http.MethodPost, "/api/v1/company/ad-hoc-proposals"},
		{http.MethodPatch, "/api/v1/company/ad-hoc-proposals/proposal-1"},
	}
	cases := []struct {
		name string
		body string
		want int
	}{
		{"oversized", oversized, http.StatusRequestEntityTooLarge},
		{"malformed", `{"title":`, http.StatusBadRequest},
	}
	for _, rt := range routes {
		for _, tc := range cases {
			svc := &fakeService{}
			mux := http.NewServeMux()
			NewHandler(discardLogger(), svc, fakeInspector{}, nil).Register(mux)
			req := httptest.NewRequest(rt.method, rt.path, strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer atk_tenant_company_001")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != tc.want || !strings.Contains(rec.Body.String(), `"INVALID_REQUEST"`) {
				t.Errorf("%s %s %s: status=%d body=%s", rt.method, rt.path, tc.name, rec.Code, rec.Body.String())
			}
			if svc.lastSubject.CompanyID != "" {
				t.Errorf("%s %s %s: service must not be called", rt.method, rt.path, tc.name)
			}
		}
	}
}
