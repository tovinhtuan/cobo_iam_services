package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cobo/cobo_iam_services/internal/platform/config"
)

func TestCORS_devAllowsLoopback(t *testing.T) {
	t.Parallel()
	m := corsMiddleware(config.Config{Env: "development", PublicWebBaseURL: "http://example.com:9999"},
		http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))

	for _, origin := range []string{"http://127.0.0.1:3000", "http://localhost:3000", "https://127.0.0.1:3000"} {
		origin := origin
		t.Run(origin, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "http://localhost:8080/api", nil)
			req.Header.Set("Origin", origin)
			rr := httptest.NewRecorder()
			m.ServeHTTP(rr, req)
			if got := rr.Header().Get("Access-Control-Allow-Origin"); got != origin {
				t.Fatalf("Allow-Origin: got %q want %q", got, origin)
			}
		})
	}
}

func TestCORS_devPublicWebBaseURL(t *testing.T) {
	t.Parallel()
	m := corsMiddleware(config.Config{Env: "development", PublicWebBaseURL: "https://app.example/"},
		http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://app.example")
	rr := httptest.NewRecorder()
	m.ServeHTTP(rr, req)
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example" {
		t.Fatalf("Allow-Origin: got %q", got)
	}
}

func TestCORS_productionDeniesByDefault(t *testing.T) {
	t.Parallel()
	m := corsMiddleware(config.Config{Env: "production", PublicWebBaseURL: "http://localhost:3000"},
		http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()
	m.ServeHTTP(rr, req)
	if rr.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("expected no CORS in production without CORS_ALLOWED_ORIGINS")
	}
}

func TestCORS_explicitList(t *testing.T) {
	t.Parallel()
	m := corsMiddleware(
		config.Config{Env: "production", CORSAllowedOrigins: "https://a.example, https://b.example "},
		http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}),
	)
	for _, c := range []struct {
		origin string
		want   string
	}{
		{origin: "https://a.example", want: "https://a.example"},
		{origin: "https://b.example", want: "https://b.example"},
		{origin: "http://localhost:3000", want: ""},
	} {
		c := c
		t.Run(c.origin, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Origin", c.origin)
			rr := httptest.NewRecorder()
			m.ServeHTTP(rr, req)
			if got := rr.Header().Get("Access-Control-Allow-Origin"); got != c.want {
				t.Fatalf("Allow-Origin: got %q want %q", got, c.want)
			}
		})
	}
}
