package httpserver

import (
	"net/http"
	"strings"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
)

// Request paths that are not company data changes even with a write method: sign-in and session
// handling, the user's own account, and POST routes that only read (export, validate, simulate,
// authorization decisions).
var (
	readOnlyWritePrefixes = []string{"/api/v1/auth/", "/api/v1/me/", "/api/v1/oauth/"}
	readOnlyWritePaths    = map[string]struct{}{
		"/api/v1/admin/config-export":               {},
		"/api/v1/admin/configuration/validate":      {},
		"/api/v1/admin/notification-rules/simulate": {},
		"/api/v1/template-builder/validate":         {},
		"/internal/v1/authorize":                    {},
		"/internal/v1/authorize/batch":              {},
	}
)

// writeRequestMiddleware marks requests that change company data (iamapp.WithWriteRequest), so a
// suspended company ("Tạm ngưng") keeps read and export access only.
func writeRequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isCompanyWrite(r.Method, r.URL.Path) {
			r = r.WithContext(iamapp.WithWriteRequest(r.Context()))
		}
		next.ServeHTTP(w, r)
	})
}

func isCompanyWrite(method, path string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	path = strings.TrimRight(path, "/")
	if _, ok := readOnlyWritePaths[path]; ok {
		return false
	}
	for _, p := range readOnlyWritePrefixes {
		if strings.HasPrefix(path+"/", p) {
			return false
		}
	}
	return true
}
