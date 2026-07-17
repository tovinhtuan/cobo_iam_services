package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProtectMetricsHandler_blocksPublicRemoteAddr(t *testing.T) {
	h := protectMetricsHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), "internal-token")

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.RemoteAddr = "8.8.8.8:44321"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestProtectMetricsHandler_allowsPrivateRemoteAddr(t *testing.T) {
	h := protectMetricsHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), "internal-token")

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.RemoteAddr = "172.18.0.5:44211"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d want %d", rr.Code, http.StatusOK)
	}
}

func TestProtectMetricsHandler_allowsPublicWhenTokenMatches(t *testing.T) {
	h := protectMetricsHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), "internal-token")

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.RemoteAddr = "8.8.8.8:44321"
	req.Header.Set("X-Internal-Token", "internal-token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d want %d", rr.Code, http.StatusOK)
	}
}
