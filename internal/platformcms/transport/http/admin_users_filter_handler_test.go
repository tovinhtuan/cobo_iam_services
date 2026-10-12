package http

import (
	"net/http"
	"strings"
	"testing"
)

// Contract: cobo_web_design/docs/ai-cache/cms-users-redesign-2026-10-12/02-contract.md (handler layer).

func TestAdminUsersHandler_FilterParamsAndStatusCounts(t *testing.T) {
	h, _ := newCompanySafetyHandler(t, operatorPerms)

	w := doCompanyRequest(h, http.MethodGet, "/api/v1/platform/cms/admin/users?company_id=c_target&q=an&status=active", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	meta, _ := decodeEnvelope(t, w)["meta"].(map[string]any)
	counts, ok := meta["status_counts"].(map[string]any)
	if !ok {
		t.Fatalf("meta.status_counts missing: %v", meta)
	}
	for _, key := range []string{"all", "active", "invited", "pending", "inactive"} {
		if _, ok := counts[key]; !ok {
			t.Fatalf("status_counts.%s missing: %v", key, counts)
		}
	}

	w = doCompanyRequest(h, http.MethodGet, "/api/v1/platform/cms/admin/users?company_id=c_target&status=deleted", "")
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "field_errors") || !strings.Contains(w.Body.String(), "status") {
		t.Fatalf("bad status: code=%d body=%s", w.Code, w.Body.String())
	}

	w = doCompanyRequest(h, http.MethodGet, "/api/v1/platform/cms/admin/users?company_id=c_target&q="+strings.Repeat("a", 101), "")
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), `"q"`) {
		t.Fatalf("long q: code=%d body=%s", w.Code, w.Body.String())
	}
}
