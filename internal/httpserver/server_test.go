package httpserver_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cobo/cobo_iam_services/internal/httpserver"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	iamtokendual "github.com/cobo/cobo_iam_services/internal/iam/infra/token/dual"
	iamtokenjwt "github.com/cobo/cobo_iam_services/internal/iam/infra/token/jwt"
	iamtokenopaque "github.com/cobo/cobo_iam_services/internal/iam/infra/token/opaque"
	"github.com/cobo/cobo_iam_services/internal/platform/config"
	"github.com/cobo/cobo_iam_services/internal/platform/logger"
)

func testAPIConfig() config.Config {
	return config.Config{
		ServiceName:             "cobo_iam_services",
		Env:                     "test",
		HTTPAddr:                ":0",
		HTTPReadTimeout:         15 * time.Second,
		HTTPWriteTimeout:        15 * time.Second,
		HTTPIdleTimeout:         60 * time.Second,
		WorkerTickInterval:      5 * time.Second,
		EffectiveAccessCacheTTL: 5 * time.Minute,
		LogLevel:                "error",
	}
}

func newTestHandler(t *testing.T, db *sql.DB) http.Handler {
	t.Helper()
	log := logger.New("error")
	h, cleanup, err := httpserver.New(context.Background(), httpserver.Deps{
		Log:    log,
		Config: testAPIConfig(),
		DB:     db,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	return h
}

func newTestHandlerWithDeps(t *testing.T, db *sql.DB, cfg config.Config, tm httpserver.TokenManager) http.Handler {
	t.Helper()
	log := logger.New("error")
	h, cleanup, err := httpserver.New(context.Background(), httpserver.Deps{
		Log:          log,
		Config:       cfg,
		DB:           db,
		TokenManager: tm,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	return h
}

func TestIntegration_healthz(t *testing.T) {
	srv := httptest.NewServer(newTestHandler(t, nil))
	defer srv.Close()

	res, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", res.StatusCode)
	}
}

func TestIntegration_readyz_noDatabase(t *testing.T) {
	srv := httptest.NewServer(newTestHandler(t, nil))
	defer srv.Close()

	res, err := http.Get(srv.URL + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status=%d want 503", res.StatusCode)
	}
}

func TestIntegration_login_singleCompany(t *testing.T) {
	srv := httptest.NewServer(newTestHandler(t, nil))
	defer srv.Close()

	body := bytes.NewBufferString(`{"login_id":"single@example.com","password":"secret"}`)
	res, err := http.Post(srv.URL+"/api/v1/auth/login", "application/json", body)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status=%d body=%s", res.StatusCode, b)
	}
	var out struct {
		NextAction string `json:"next_action"`
		Session    struct {
			AccessToken string `json:"access_token"`
		} `json:"session"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.NextAction != "load_effective_access" {
		t.Fatalf("next_action=%q", out.NextAction)
	}
	if out.Session.AccessToken == "" {
		t.Fatal("missing access_token")
	}
}

type staticID struct{ n int }

func (s *staticID) NewUUID() string {
	s.n++
	return "test-id-" + time.Now().UTC().Format("150405") + "-" + string(rune('a'+s.n))
}

func TestIntegration_dualMode_loginJwt_andProtectedEndpointAcceptsLegacyOpaque(t *testing.T) {
	id := &staticID{}
	opaque := iamtokenopaque.NewManager(id)
	cfg := testAPIConfig()
	cfg.AccessTokenMode = "dual"
	cfg.JWTAlg = "HS256"
	cfg.JWTSigningPrivateKey = "dual-mode-secret"
	cfg.JWTIssuer = "test-issuer"
	cfg.JWTAudience = "test-aud"
	cfg.AccessTokenTTL = 5 * time.Minute

	j := iamtokenjwt.NewManager(cfg, id, opaque)
	dual := iamtokendual.NewManager(j, opaque, j)
	srv := httptest.NewServer(newTestHandlerWithDeps(t, nil, cfg, dual))
	defer srv.Close()

	// 1) login should issue JWT in dual mode
	loginBody := bytes.NewBufferString(`{"login_id":"single@example.com","password":"secret"}`)
	loginRes, err := http.Post(srv.URL+"/api/v1/auth/login", "application/json", loginBody)
	if err != nil {
		t.Fatal(err)
	}
	defer loginRes.Body.Close()
	if loginRes.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(loginRes.Body)
		t.Fatalf("login status=%d body=%s", loginRes.StatusCode, b)
	}
	var loginOut struct {
		Session struct {
			AccessToken string `json:"access_token"`
		} `json:"session"`
	}
	if err := json.NewDecoder(loginRes.Body).Decode(&loginOut); err != nil {
		t.Fatal(err)
	}
	if loginOut.Session.AccessToken == "" {
		t.Fatal("missing access token")
	}
	if bytes.Count([]byte(loginOut.Session.AccessToken), []byte(".")) != 2 {
		t.Fatalf("expected JWT token, got: %q", loginOut.Session.AccessToken)
	}

	// 2) protected endpoint with JWT token
	reqJWT, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/me/effective-access", nil)
	reqJWT.Header.Set("Authorization", "Bearer "+loginOut.Session.AccessToken)
	resJWT, err := http.DefaultClient.Do(reqJWT)
	if err != nil {
		t.Fatal(err)
	}
	defer resJWT.Body.Close()
	if resJWT.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resJWT.Body)
		t.Fatalf("jwt protected call status=%d body=%s", resJWT.StatusCode, b)
	}

	// 3) protected endpoint with legacy opaque token fallback
	legacyOpaque, _, err := opaque.IssueAccessToken(context.Background(), iamapp.AccessTokenClaims{
		Sub:          "u_single",
		SessionID:    "legacy-session",
		MembershipID: "m_010",
		CompanyID:    "c_010",
	})
	if err != nil {
		t.Fatal(err)
	}
	reqOpaque, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/me/effective-access", nil)
	reqOpaque.Header.Set("Authorization", "Bearer "+legacyOpaque)
	resOpaque, err := http.DefaultClient.Do(reqOpaque)
	if err != nil {
		t.Fatal(err)
	}
	defer resOpaque.Body.Close()
	if resOpaque.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resOpaque.Body)
		t.Fatalf("opaque fallback call status=%d body=%s", resOpaque.StatusCode, b)
	}
}
