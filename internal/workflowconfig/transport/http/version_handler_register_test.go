package http_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	wfchttp "github.com/cobo/cobo_iam_services/internal/workflowconfig/transport/http"
)

// Documents the route set registered when WORKFLOW_VERSIONING_ENABLED wires Handler.Register.
func TestHandlerRegister_ConfigurationRoutePatterns(t *testing.T) {
	mux := http.NewServeMux()
	h := wfchttp.NewHandler(nil, nil, nil, nil)
	h.Register(mux)

	paths := []string{
		"/api/v1/platform/cms/templates/bao-cao-quy-2/workflow/configuration",
		"/api/v1/platform/cms/templates/bao-cao-quy-2/workflow/readiness",
		"/api/v1/platform/cms/templates/bao-cao-quy-2/workflow/lifecycle",
		"/api/v1/platform/cms/templates/bao-cao-quy-2/workflow/versions",
	}
	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		_, pattern := mux.Handler(req)
		if pattern == "" || strings.Contains(pattern, "Not Found") {
			t.Fatalf("expected registered pattern for %s, got %q", path, pattern)
		}
	}
}
