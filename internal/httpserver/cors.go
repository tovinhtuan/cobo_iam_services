package httpserver

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/cobo/cobo_iam_services/internal/platform/config"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
)

// corsMiddleware sets CORS response headers when the request Origin is allowed.
// Browsers treat http://localhost:3000 and http://127.0.0.1:3000 as different
// origins, so a SPA at localhost calling an API at 127.0.0.1 requires this.
func corsMiddleware(cfg config.Config, next http.Handler) http.Handler {
	allow := buildAllowedOrigins(cfg)
	allowHeader := "Authorization, Content-Type, " + httpx.RequestIDHeader + ", Idempotency-Key, X-Idempotency-Key"

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && allow.isAllowed(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", allowHeader)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}
		if r.Method == http.MethodOptions && origin != "" && allow.isAllowed(origin) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func buildAllowedOrigins(cfg config.Config) *originAllowlist {
	raw := strings.TrimSpace(cfg.CORSAllowedOrigins)
	if raw != "" {
		parts := strings.Split(raw, ",")
		set := make(map[string]struct{}, len(parts))
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				set[normalizeOrigin(s)] = struct{}{}
			}
		}
		return &originAllowlist{mode: "explicit", explicit: set}
	}
	// In development, allow Vite/SPA on any loopback port plus PUBLIC_WEB Base URL.
	if !strings.EqualFold(cfg.Env, "development") {
		return &originAllowlist{mode: "deny"}
	}
	return &originAllowlist{mode: "dev", publicBase: normalizeOrigin(cfg.PublicWebBaseURL)}
}

type originAllowlist struct {
	mode       string // "explicit" | "dev" | "deny"
	explicit   map[string]struct{}
	publicBase string
}

func (a *originAllowlist) isAllowed(origin string) bool {
	if a == nil {
		return false
	}
	n := normalizeOrigin(origin)
	if n == "" {
		return false
	}
	switch a.mode {
	case "deny":
		return false
	case "explicit":
		_, ok := a.explicit[n]
		return ok
	case "dev":
		if a.publicBase != "" && n == a.publicBase {
			return true
		}
		return isLoopbackOrigin(n)
	default:
		return false
	}
}

func normalizeOrigin(s string) string {
	return strings.TrimSuffix(strings.TrimSpace(s), "/")
}

func isLoopbackOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}
