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

func TestIntegration_loginSwitchCompany_effectiveAccess_andAdminGuard(t *testing.T) {
	srv := httptest.NewServer(newTestHandler(t, nil))
	defer srv.Close()

	// 1) Login multi-company account -> requires company selection.
	loginBody := bytes.NewBufferString(`{"login_id":"user@example.com","password":"secret"}`)
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
		NextAction  string `json:"next_action"`
		Memberships []struct {
			CompanyID string `json:"company_id"`
		} `json:"memberships"`
		Session struct {
			PreCompanyToken string `json:"pre_company_token"`
		} `json:"session"`
	}
	if err := json.NewDecoder(loginRes.Body).Decode(&loginOut); err != nil {
		t.Fatal(err)
	}
	if loginOut.NextAction != "select_company" {
		t.Fatalf("next_action=%q want select_company", loginOut.NextAction)
	}
	if loginOut.Session.PreCompanyToken == "" {
		t.Fatal("missing pre_company_token")
	}

	// 2) Select company c_001 -> receives full access token.
	selectReqBody := bytes.NewBufferString(`{"company_id":"c_001"}`)
	selectReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/auth/select-company", selectReqBody)
	selectReq.Header.Set("Content-Type", "application/json")
	selectReq.Header.Set("Authorization", "Bearer "+loginOut.Session.PreCompanyToken)
	selectRes, err := http.DefaultClient.Do(selectReq)
	if err != nil {
		t.Fatal(err)
	}
	defer selectRes.Body.Close()
	if selectRes.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(selectRes.Body)
		t.Fatalf("select company status=%d body=%s", selectRes.StatusCode, b)
	}
	var selectOut struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(selectRes.Body).Decode(&selectOut); err != nil {
		t.Fatal(err)
	}
	if selectOut.AccessToken == "" {
		t.Fatal("missing selected company access_token")
	}

	// 3) Effective-access for c_001 should work.
	effReqC1, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/me/effective-access", nil)
	effReqC1.Header.Set("Authorization", "Bearer "+selectOut.AccessToken)
	effResC1, err := http.DefaultClient.Do(effReqC1)
	if err != nil {
		t.Fatal(err)
	}
	defer effResC1.Body.Close()
	if effResC1.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(effResC1.Body)
		t.Fatalf("effective-access c_001 status=%d body=%s", effResC1.StatusCode, b)
	}

	// 4) Admin endpoint must remain authz-guarded (no auth bypass).
	adminReqC1, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/admin/permissions", nil)
	adminReqC1.Header.Set("Authorization", "Bearer "+selectOut.AccessToken)
	adminResC1, err := http.DefaultClient.Do(adminReqC1)
	if err != nil {
		t.Fatal(err)
	}
	defer adminResC1.Body.Close()
	if adminResC1.StatusCode != http.StatusForbidden {
		b, _ := io.ReadAll(adminResC1.Body)
		t.Fatalf("admin permissions c_001 status=%d body=%s", adminResC1.StatusCode, b)
	}

	// 5) Switch to c_002 then re-check effective-access.
	switchReqBody := bytes.NewBufferString(`{"company_id":"c_002"}`)
	switchReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/auth/switch-company", switchReqBody)
	switchReq.Header.Set("Content-Type", "application/json")
	switchReq.Header.Set("Authorization", "Bearer "+selectOut.AccessToken)
	switchRes, err := http.DefaultClient.Do(switchReq)
	if err != nil {
		t.Fatal(err)
	}
	defer switchRes.Body.Close()
	if switchRes.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(switchRes.Body)
		t.Fatalf("switch company status=%d body=%s", switchRes.StatusCode, b)
	}
	var switchOut struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(switchRes.Body).Decode(&switchOut); err != nil {
		t.Fatal(err)
	}
	if switchOut.AccessToken == "" {
		t.Fatal("missing switched access_token")
	}

	effReqC2, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/me/effective-access", nil)
	effReqC2.Header.Set("Authorization", "Bearer "+switchOut.AccessToken)
	effResC2, err := http.DefaultClient.Do(effReqC2)
	if err != nil {
		t.Fatal(err)
	}
	defer effResC2.Body.Close()
	if effResC2.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(effResC2.Body)
		t.Fatalf("effective-access c_002 status=%d body=%s", effResC2.StatusCode, b)
	}

	// 6) Admin guard should deny in c_002 (viewer role).
	adminReqC2, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/admin/permissions", nil)
	adminReqC2.Header.Set("Authorization", "Bearer "+switchOut.AccessToken)
	adminResC2, err := http.DefaultClient.Do(adminReqC2)
	if err != nil {
		t.Fatal(err)
	}
	defer adminResC2.Body.Close()
	if adminResC2.StatusCode != http.StatusForbidden {
		b, _ := io.ReadAll(adminResC2.Body)
		t.Fatalf("admin permissions c_002 status=%d body=%s", adminResC2.StatusCode, b)
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

func TestIntegration_dualMode_rejectsInvalidJWTAndInvalidOpaque(t *testing.T) {
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

	// Mint JWT with wrong audience (same key, different audience) -> must be rejected.
	wrongAudCfg := cfg
	wrongAudCfg.JWTAudience = "wrong-aud"
	wrongAudIssuer := iamtokenjwt.NewManager(wrongAudCfg, id, opaque)
	badJWT, _, err := wrongAudIssuer.IssueAccessToken(context.Background(), iamapp.AccessTokenClaims{
		Sub:          "u_single",
		SessionID:    "bad-jwt-session",
		MembershipID: "m_010",
		CompanyID:    "c_010",
	})
	if err != nil {
		t.Fatal(err)
	}
	reqBadJWT, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/me/effective-access", nil)
	reqBadJWT.Header.Set("Authorization", "Bearer "+badJWT)
	resBadJWT, err := http.DefaultClient.Do(reqBadJWT)
	if err != nil {
		t.Fatal(err)
	}
	defer resBadJWT.Body.Close()
	if resBadJWT.StatusCode != http.StatusUnauthorized {
		b, _ := io.ReadAll(resBadJWT.Body)
		t.Fatalf("bad jwt status=%d body=%s", resBadJWT.StatusCode, b)
	}

	// Opaque token not found in fallback store -> must be rejected.
	reqBadOpaque, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/me/effective-access", nil)
	reqBadOpaque.Header.Set("Authorization", "Bearer atk_invalid_opaque")
	resBadOpaque, err := http.DefaultClient.Do(reqBadOpaque)
	if err != nil {
		t.Fatal(err)
	}
	defer resBadOpaque.Body.Close()
	if resBadOpaque.StatusCode != http.StatusUnauthorized {
		b, _ := io.ReadAll(resBadOpaque.Body)
		t.Fatalf("bad opaque status=%d body=%s", resBadOpaque.StatusCode, b)
	}
}

func TestIntegration_jwtMode_acceptsJWT_andRejectsOpaque(t *testing.T) {
	id := &staticID{}
	opaque := iamtokenopaque.NewManager(id)
	cfg := testAPIConfig()
	cfg.AccessTokenMode = "jwt"
	cfg.JWTAlg = "HS256"
	cfg.JWTSigningPrivateKey = "jwt-only-secret"
	cfg.JWTIssuer = "test-issuer"
	cfg.JWTAudience = "test-aud"
	cfg.AccessTokenTTL = 5 * time.Minute

	j := iamtokenjwt.NewManager(cfg, id, opaque)
	srv := httptest.NewServer(newTestHandlerWithDeps(t, nil, cfg, j))
	defer srv.Close()

	// 1) login returns JWT access token in jwt-only mode
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

	// 2) protected endpoint with JWT succeeds
	reqJWT, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/me/effective-access", nil)
	reqJWT.Header.Set("Authorization", "Bearer "+loginOut.Session.AccessToken)
	resJWT, err := http.DefaultClient.Do(reqJWT)
	if err != nil {
		t.Fatal(err)
	}
	defer resJWT.Body.Close()
	if resJWT.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resJWT.Body)
		t.Fatalf("jwt call status=%d body=%s", resJWT.StatusCode, b)
	}

	// 3) opaque legacy token must be rejected in jwt-only mode
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
	if resOpaque.StatusCode != http.StatusUnauthorized {
		b, _ := io.ReadAll(resOpaque.Body)
		t.Fatalf("opaque in jwt-only status=%d body=%s", resOpaque.StatusCode, b)
	}
}

func TestIntegration_jwtMode_expiredTokenRejectedAtHTTPLayer(t *testing.T) {
	id := &staticID{}
	opaque := iamtokenopaque.NewManager(id)
	cfg := testAPIConfig()
	cfg.AccessTokenMode = "jwt"
	cfg.JWTAlg = "HS256"
	cfg.JWTSigningPrivateKey = "jwt-expired-secret"
	cfg.JWTIssuer = "test-issuer"
	cfg.JWTAudience = "test-aud"
	cfg.AccessTokenTTL = 1 * time.Second
	cfg.JWTClockSkewSec = 0

	j := iamtokenjwt.NewManager(cfg, id, opaque)
	srv := httptest.NewServer(newTestHandlerWithDeps(t, nil, cfg, j))
	defer srv.Close()

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

	// Wait token expiry and assert HTTP layer rejects it.
	time.Sleep(2 * time.Second)
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/me/effective-access", nil)
	req.Header.Set("Authorization", "Bearer "+loginOut.Session.AccessToken)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("expired token status=%d body=%s", res.StatusCode, b)
	}
}
