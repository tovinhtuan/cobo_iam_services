package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
)

// Requests that change company data are marked so that token inspection can refuse them in a
// suspended ("Tạm ngưng") company; reads, sign-in/out, the user's own account and read-only POSTs
// (export, validate, simulate) are not.
func TestWriteRequestMarker(t *testing.T) {
	cases := []struct {
		method, path string
		write        bool
	}{
		{"GET", "/api/v1/admin/memberships/m1/permissions", false},
		{"HEAD", "/api/v1/company/deadlines", false},
		{"OPTIONS", "/api/v1/admin/roles", false},
		{"POST", "/api/v1/admin/memberships", true},
		{"PATCH", "/api/v1/admin/memberships/m1", true},
		{"DELETE", "/api/v1/admin/memberships/m1", true},
		{"PUT", "/api/v1/company/deadlines/r1/steps/s1", true},
		{"POST", "/api/v1/auth/login", false},
		{"POST", "/api/v1/auth/refresh", false},
		{"POST", "/api/v1/auth/switch-company", false},
		{"PATCH", "/api/v1/me/profile", false},
		{"POST", "/api/v1/me/change-password", false},
		{"POST", "/api/v1/me/in-app-notifications/read-all", false},
		{"POST", "/api/v1/admin/config-export", false},
		{"POST", "/api/v1/admin/configuration/validate", false},
		{"POST", "/api/v1/admin/notification-rules/simulate", false},
		{"POST", "/api/v1/template-builder/validate", false},
		{"POST", "/internal/v1/authorize", false},
		{"POST", "/internal/v1/authorize/batch", false},
		{"POST", "/api/v1/admin/config-export-evil", true},
		{"POST", "/api/v1/authx/login", true},
	}
	for _, tc := range cases {
		var got bool
		h := writeRequestMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			got = iamapp.IsWriteRequest(r.Context())
		}))
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(tc.method, tc.path, nil))
		if got != tc.write {
			t.Errorf("%s %s marked write=%v, want %v", tc.method, tc.path, got, tc.write)
		}
	}
}
