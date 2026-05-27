package httpserver_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Guards against dropping CMS market listed-companies routes (would surface as 404).
func TestIntegration_marketListedCompaniesRouteRegistered(t *testing.T) {
	srv := httptest.NewServer(newTestHandler(t, nil))
	defer srv.Close()

	res, err := http.Get(srv.URL + "/api/v1/platform/cms/market/listed-companies")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode == http.StatusNotFound {
		t.Fatalf("GET /api/v1/platform/cms/market/listed-companies returned 404; route missing from mux body=%s", body)
	}
	// No bearer: 401 (auth runs before market availability).
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without bearer, got status=%d body=%s", res.StatusCode, body)
	}
}

func TestIntegration_marketListedCompaniesDisabledReturns503WithAuth(t *testing.T) {
	srv := httptest.NewServer(newTestHandler(t, nil))
	defer srv.Close()

	token := loginAndGetAccessToken(t, srv.URL, "cms.operator@example.com", "secret", "c_001")
	res := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/platform/cms/market/listed-companies", token, nil, "")
	if res.StatusCode == http.StatusNotFound {
		t.Fatalf("route missing; got 404 body=%s", readBody(t, res.Body))
	}
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when vnstock disabled in test config, got status=%d body=%s", res.StatusCode, readBody(t, res.Body))
	}
}
