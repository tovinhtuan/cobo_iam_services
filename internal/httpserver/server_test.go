package httpserver_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cobo/cobo_iam_services/internal/httpserver"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	iamtokendual "github.com/cobo/cobo_iam_services/internal/iam/infra/token/dual"
	iamtokenjwt "github.com/cobo/cobo_iam_services/internal/iam/infra/token/jwt"
	iamtokenopaque "github.com/cobo/cobo_iam_services/internal/iam/infra/token/opaque"
	"github.com/cobo/cobo_iam_services/internal/platform/config"
	"github.com/cobo/cobo_iam_services/internal/platform/logger"
	"github.com/google/uuid"
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

func TestIntegration_loginPasswordKey_notConfigured(t *testing.T) {
	srv := httptest.NewServer(newTestHandler(t, nil))
	defer srv.Close()

	res, err := http.Get(srv.URL + "/api/v1/auth/login-password-key")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("status=%d want 204", res.StatusCode)
	}
}

func TestIntegration_login_encryptedPassword_RSAOAEP(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pemStr := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}))
	cfg := testAPIConfig()
	cfg.LoginPasswordRSAPrivateKeyPEM = pemStr
	cfg.LoginPasswordRSAKeyID = "test-kid"

	log := logger.New("error")
	h, cleanup, err := httpserver.New(context.Background(), httpserver.Deps{Log: log, Config: cfg, DB: nil})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	srv := httptest.NewServer(h)
	defer srv.Close()

	keyRes, err := http.Get(srv.URL + "/api/v1/auth/login-password-key")
	if err != nil {
		t.Fatal(err)
	}
	defer keyRes.Body.Close()
	if keyRes.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(keyRes.Body)
		t.Fatalf("key status=%d body=%s", keyRes.StatusCode, b)
	}
	var keyOut struct {
		KID              string `json:"kid"`
		Alg              string `json:"alg"`
		PublicKeySPKIB64 string `json:"public_key_spki_b64"`
	}
	if err := json.NewDecoder(keyRes.Body).Decode(&keyOut); err != nil {
		t.Fatal(err)
	}
	if keyOut.KID != "test-kid" || keyOut.Alg != "RSA-OAEP-256" || keyOut.PublicKeySPKIB64 == "" {
		t.Fatalf("unexpected key payload: %+v", keyOut)
	}
	spki, err := base64.StdEncoding.DecodeString(keyOut.PublicKeySPKIB64)
	if err != nil {
		t.Fatal(err)
	}
	pubAny, err := x509.ParsePKIXPublicKey(spki)
	if err != nil {
		t.Fatal(err)
	}
	pub, ok := pubAny.(*rsa.PublicKey)
	if !ok {
		t.Fatal("not RSA public key")
	}
	ct, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pub, []byte("secret"), nil)
	if err != nil {
		t.Fatal(err)
	}
	ctB64 := base64.StdEncoding.EncodeToString(ct)

	loginBody, err := json.Marshal(map[string]any{
		"login_id": "single@example.com",
		"password_cipher": map[string]string{
			"alg":            "RSA-OAEP-256",
			"kid":            keyOut.KID,
			"ciphertext_b64": ctB64,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.Post(srv.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(loginBody))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("login status=%d body=%s", res.StatusCode, b)
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

func TestIntegration_meCapabilities_platformCmsMatrix(t *testing.T) {
	srv := httptest.NewServer(newTestHandler(t, nil))
	defer srv.Close()

	cmsToken := loginAndGetAccessToken(t, srv.URL, "cms.operator@example.com", "secret", "")
	cmsRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/me/capabilities", cmsToken, nil, "")
	if cmsRes.StatusCode != http.StatusOK {
		t.Fatalf("cms capabilities status=%d body=%s", cmsRes.StatusCode, readBody(t, cmsRes.Body))
	}
	var cmsOut struct {
		Modules map[string]bool `json:"modules"`
	}
	mustDecodeJSON(t, cmsRes.Body, &cmsOut)
	if !cmsOut.Modules["platform_cms"] {
		t.Fatalf("expected platform_cms=true for cms operator, got modules=%+v", cmsOut.Modules)
	}

	adminRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/me/capabilities", loginAndGetAccessToken(t, srv.URL, "admin.dn@example.com", "secret", ""), nil, "")
	if adminRes.StatusCode != http.StatusOK {
		t.Fatalf("admin capabilities status=%d body=%s", adminRes.StatusCode, readBody(t, adminRes.Body))
	}
	var adminOut struct {
		Modules map[string]bool `json:"modules"`
	}
	mustDecodeJSON(t, adminRes.Body, &adminOut)
	if adminOut.Modules["platform_cms"] {
		t.Fatalf("expected platform_cms=false for admin.dn without explicit permission, got modules=%+v", adminOut.Modules)
	}

	userToken := loginAndGetAccessToken(t, srv.URL, "user@example.com", "secret", "c_001")
	userRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/me/capabilities", userToken, nil, "")
	if userRes.StatusCode != http.StatusOK {
		t.Fatalf("user capabilities status=%d body=%s", userRes.StatusCode, readBody(t, userRes.Body))
	}
	var userOut struct {
		Modules map[string]bool `json:"modules"`
	}
	mustDecodeJSON(t, userRes.Body, &userOut)
	if userOut.Modules["platform_cms"] {
		t.Fatalf("expected platform_cms=false for user, got modules=%+v", userOut.Modules)
	}
}

func TestIntegration_platformCMSPrefix_dashboardCollectionsEntries(t *testing.T) {
	srv := httptest.NewServer(newTestHandler(t, nil))
	defer srv.Close()

	cmsToken := loginAndGetAccessToken(t, srv.URL, "cms.operator@example.com", "secret", "")
	dashboardRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/platform/cms/dashboard/summary", cmsToken, nil, "")
	if dashboardRes.StatusCode != http.StatusOK {
		t.Fatalf("dashboard summary status=%d body=%s", dashboardRes.StatusCode, readBody(t, dashboardRes.Body))
	}
	var dashboardOut map[string]any
	mustDecodeJSON(t, dashboardRes.Body, &dashboardOut)
	data, _ := dashboardOut["data"].(map[string]any)
	if _, ok := data["total"]; !ok {
		t.Fatalf("dashboard payload missing total: %+v", dashboardOut)
	}
	if data["platform_cms"] != true {
		t.Fatalf("dashboard payload should contain platform_cms=true: %+v", dashboardOut)
	}

	collectionsRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/platform/cms/collections", cmsToken, nil, "")
	if collectionsRes.StatusCode != http.StatusOK {
		t.Fatalf("collections status=%d body=%s", collectionsRes.StatusCode, readBody(t, collectionsRes.Body))
	}
	var collectionsOut struct {
		Data struct {
			Items []map[string]any `json:"items"`
		} `json:"data"`
		Meta map[string]any `json:"meta"`
	}
	mustDecodeJSON(t, collectionsRes.Body, &collectionsOut)
	if collectionsOut.Data.Items == nil {
		t.Fatalf("collections payload missing items: %+v", collectionsOut)
	}

	entriesRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/platform/cms/entries", cmsToken, nil, "")
	if entriesRes.StatusCode != http.StatusOK {
		t.Fatalf("entries status=%d body=%s", entriesRes.StatusCode, readBody(t, entriesRes.Body))
	}
	var entriesOut struct {
		Data struct {
			Items []map[string]any `json:"items"`
		} `json:"data"`
		Meta map[string]any `json:"meta"`
	}
	mustDecodeJSON(t, entriesRes.Body, &entriesOut)
	if entriesOut.Data.Items == nil {
		t.Fatalf("entries payload missing items: %+v", entriesOut)
	}

	releasesRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/platform/cms/releases", cmsToken, nil, "")
	if releasesRes.StatusCode != http.StatusOK {
		t.Fatalf("releases status=%d body=%s", releasesRes.StatusCode, readBody(t, releasesRes.Body))
	}
	var releasesOut struct {
		Data struct {
			Items []map[string]any `json:"items"`
		} `json:"data"`
		Meta map[string]any `json:"meta"`
	}
	mustDecodeJSON(t, releasesRes.Body, &releasesOut)
	if releasesOut.Data.Items == nil {
		t.Fatalf("releases payload missing items: %+v", releasesOut)
	}

	rolesRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/platform/cms/admin/roles", cmsToken, nil, "")
	if rolesRes.StatusCode != http.StatusOK {
		t.Fatalf("roles status=%d body=%s", rolesRes.StatusCode, readBody(t, rolesRes.Body))
	}
	rulesRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/platform/cms/admin/rules", cmsToken, nil, "")
	if rulesRes.StatusCode != http.StatusOK {
		t.Fatalf("rules status=%d body=%s", rulesRes.StatusCode, readBody(t, rulesRes.Body))
	}
	validateRuleRes := doJSONRequest(t, http.MethodPost, srv.URL+"/api/v1/platform/cms/admin/rules/validate", cmsToken, map[string]any{
		"name":        "strict gate",
		"permissions": []string{"platform.cms.view"},
	}, "")
	if validateRuleRes.StatusCode != http.StatusOK {
		t.Fatalf("validate rule status=%d body=%s", validateRuleRes.StatusCode, readBody(t, validateRuleRes.Body))
	}
	auditRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/platform/cms/ops/audit", cmsToken, nil, "")
	if auditRes.StatusCode != http.StatusOK {
		t.Fatalf("audit status=%d body=%s", auditRes.StatusCode, readBody(t, auditRes.Body))
	}
	sessionsRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/platform/cms/ops/sessions", cmsToken, nil, "")
	if sessionsRes.StatusCode != http.StatusOK {
		t.Fatalf("sessions status=%d body=%s", sessionsRes.StatusCode, readBody(t, sessionsRes.Body))
	}
	var sessionsOut struct {
		Data struct {
			Items []map[string]any `json:"items"`
		} `json:"data"`
	}
	mustDecodeJSON(t, sessionsRes.Body, &sessionsOut)
	if len(sessionsOut.Data.Items) == 0 {
		t.Fatalf("sessions payload should not be empty: %+v", sessionsOut)
	}
	sessionID, _ := sessionsOut.Data.Items[0]["session_id"].(string)
	if sessionID == "" {
		t.Fatalf("session_id missing in sessions payload: %+v", sessionsOut)
	}
	revokeRes := doJSONRequest(t, http.MethodPost, srv.URL+"/api/v1/platform/cms/ops/sessions/"+sessionID+"/revoke", cmsToken, nil, "")
	if revokeRes.StatusCode != http.StatusOK {
		t.Fatalf("revoke session status=%d body=%s", revokeRes.StatusCode, readBody(t, revokeRes.Body))
	}
	healthRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/platform/cms/ops/health", cmsToken, nil, "")
	if healthRes.StatusCode != http.StatusOK {
		t.Fatalf("health status=%d body=%s", healthRes.StatusCode, readBody(t, healthRes.Body))
	}
	metricsRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/platform/cms/ops/metrics", cmsToken, nil, "")
	if metricsRes.StatusCode != http.StatusOK {
		t.Fatalf("metrics status=%d body=%s", metricsRes.StatusCode, readBody(t, metricsRes.Body))
	}
	var metricsOut struct {
		Data struct {
			Routes map[string]map[string]any `json:"routes"`
		} `json:"data"`
	}
	mustDecodeJSON(t, metricsRes.Body, &metricsOut)
	if len(metricsOut.Data.Routes) == 0 {
		t.Fatalf("expected cms metrics routes to be populated")
	}
	auditAfterOpsRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/platform/cms/ops/audit", cmsToken, nil, "")
	if auditAfterOpsRes.StatusCode != http.StatusOK {
		t.Fatalf("audit after ops status=%d body=%s", auditAfterOpsRes.StatusCode, readBody(t, auditAfterOpsRes.Body))
	}
	var auditAfterOps struct {
		Meta map[string]any `json:"meta"`
	}
	mustDecodeJSON(t, auditAfterOpsRes.Body, &auditAfterOps)
	totalAny := auditAfterOps.Meta["total"]
	switch total := totalAny.(type) {
	case float64:
		if total < 1 {
			t.Fatalf("expected audit total >= 1 after validate/revoke operations, got %v", total)
		}
	case int:
		if total < 1 {
			t.Fatalf("expected audit total >= 1 after validate/revoke operations, got %v", total)
		}
	default:
		t.Fatalf("expected audit meta.total present after operations, got %T (%v)", totalAny, totalAny)
	}

	userToken := loginAndGetAccessToken(t, srv.URL, "user@example.com", "secret", "c_001")
	forbiddenRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/platform/cms/dashboard/summary", userToken, nil, "")
	if forbiddenRes.StatusCode != http.StatusForbidden {
		t.Fatalf("dashboard forbidden status=%d body=%s", forbiddenRes.StatusCode, readBody(t, forbiddenRes.Body))
	}

	adminDnToken := loginAndGetAccessToken(t, srv.URL, "admin.dn@example.com", "secret", "")
	adminDnRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/platform/cms/dashboard/summary", adminDnToken, nil, "")
	if adminDnRes.StatusCode != http.StatusForbidden {
		t.Fatalf("dashboard admin.dn strict-forbidden status=%d body=%s", adminDnRes.StatusCode, readBody(t, adminDnRes.Body))
	}
	adminDnReleasesRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/platform/cms/releases", adminDnToken, nil, "")
	if adminDnReleasesRes.StatusCode != http.StatusForbidden {
		t.Fatalf("releases admin.dn strict-forbidden status=%d body=%s", adminDnReleasesRes.StatusCode, readBody(t, adminDnReleasesRes.Body))
	}
	adminDnRolesRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/platform/cms/admin/roles", adminDnToken, nil, "")
	if adminDnRolesRes.StatusCode != http.StatusForbidden {
		t.Fatalf("roles admin.dn strict-forbidden status=%d body=%s", adminDnRolesRes.StatusCode, readBody(t, adminDnRolesRes.Body))
	}
}

func TestIntegration_meProfileIncludesSubscriptionTier(t *testing.T) {
	srv := httptest.NewServer(newTestHandler(t, nil))
	defer srv.Close()

	freeUserToken := loginAndGetAccessToken(t, srv.URL, "user@example.com", "secret", "c_001")
	freeRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/me", freeUserToken, nil, "")
	if freeRes.StatusCode != http.StatusOK {
		t.Fatalf("me status=%d body=%s", freeRes.StatusCode, readBody(t, freeRes.Body))
	}

	var freeOut struct {
		User map[string]any `json:"user"`
	}
	mustDecodeJSON(t, freeRes.Body, &freeOut)
	if tier, _ := freeOut.User["subscription_tier"].(string); tier != "Free" {
		t.Fatalf("expected Free subscription tier for user@example.com, got=%v payload=%+v", tier, freeOut.User)
	}

	cmsToken := loginAndGetAccessToken(t, srv.URL, "cms.operator@example.com", "secret", "")
	cmsRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/me", cmsToken, nil, "")
	if cmsRes.StatusCode != http.StatusOK {
		t.Fatalf("cms me status=%d body=%s", cmsRes.StatusCode, readBody(t, cmsRes.Body))
	}
	var cmsOut struct {
		User map[string]any `json:"user"`
	}
	mustDecodeJSON(t, cmsRes.Body, &cmsOut)
	if tier, _ := cmsOut.User["subscription_tier"].(string); tier != "Enterprise" {
		t.Fatalf("expected Enterprise subscription tier for cms.operator@example.com, got=%v payload=%+v", tier, cmsOut.User)
	}
}

func TestIntegration_platformCMSPrefix_entriesReviewsSchedulesContract(t *testing.T) {
	srv := httptest.NewServer(newTestHandler(t, nil))
	defer srv.Close()

	cmsToken := loginAndGetAccessToken(t, srv.URL, "cms.operator@example.com", "secret", "")
	createRes := doJSONRequest(t, http.MethodPost, srv.URL+"/api/v1/platform/cms/entries", cmsToken, map[string]any{
		"type_id":      "dt-periodic-financial",
		"title":        "CMS entry",
		"summary":      "CMS summary",
		"content":      "CMS content",
		"planned_date": "2026-05-12",
	}, "")
	if createRes.StatusCode != http.StatusCreated {
		t.Fatalf("create entry status=%d body=%s", createRes.StatusCode, readBody(t, createRes.Body))
	}
	var createOut struct {
		Data map[string]any `json:"data"`
	}
	mustDecodeJSON(t, createRes.Body, &createOut)
	entryID, _ := createOut.Data["entry_id"].(string)
	if entryID == "" {
		t.Fatalf("missing entry_id in create response: %+v", createOut)
	}

	detailRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/platform/cms/entries/"+entryID, cmsToken, nil, "")
	if detailRes.StatusCode != http.StatusOK {
		t.Fatalf("entry detail status=%d body=%s", detailRes.StatusCode, readBody(t, detailRes.Body))
	}

	updateRes := doJSONRequest(t, http.MethodPut, srv.URL+"/api/v1/platform/cms/entries/"+entryID, cmsToken, map[string]any{
		"type_id":      "dt-periodic-financial",
		"title":        "CMS entry updated",
		"summary":      "CMS summary updated",
		"content":      "CMS content updated",
		"planned_date": "2026-05-15",
	}, "")
	if updateRes.StatusCode != http.StatusOK {
		t.Fatalf("update entry status=%d body=%s", updateRes.StatusCode, readBody(t, updateRes.Body))
	}

	scheduleCreateRes := doJSONRequest(t, http.MethodPost, srv.URL+"/api/v1/platform/cms/schedules", cmsToken, map[string]any{
		"entry_id":   entryID,
		"publish_at": "2026-05-20",
	}, "")
	if scheduleCreateRes.StatusCode != http.StatusCreated {
		t.Fatalf("create schedule status=%d body=%s", scheduleCreateRes.StatusCode, readBody(t, scheduleCreateRes.Body))
	}

	scheduleListRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/platform/cms/schedules", cmsToken, nil, "")
	if scheduleListRes.StatusCode != http.StatusOK {
		t.Fatalf("list schedules status=%d body=%s", scheduleListRes.StatusCode, readBody(t, scheduleListRes.Body))
	}

	submitRes := doJSONRequest(t, http.MethodPost, srv.URL+"/api/v1/disclosures/"+entryID+"/submit", cmsToken, nil, "idem-platform-submit")
	if submitRes.StatusCode != http.StatusOK {
		t.Fatalf("submit status=%d body=%s", submitRes.StatusCode, readBody(t, submitRes.Body))
	}

	reviewsRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/platform/cms/reviews", cmsToken, nil, "")
	if reviewsRes.StatusCode != http.StatusOK {
		t.Fatalf("list reviews status=%d body=%s", reviewsRes.StatusCode, readBody(t, reviewsRes.Body))
	}

	approveRes := doJSONRequest(t, http.MethodPost, srv.URL+"/api/v1/platform/cms/reviews/"+entryID, cmsToken, map[string]any{
		"decision": "approve",
	}, "")
	if approveRes.StatusCode != http.StatusOK {
		t.Fatalf("approve review status=%d body=%s", approveRes.StatusCode, readBody(t, approveRes.Body))
	}

	deleteScheduleRes := doJSONRequest(t, http.MethodDelete, srv.URL+"/api/v1/platform/cms/schedules/"+entryID, cmsToken, nil, "")
	if deleteScheduleRes.StatusCode != http.StatusOK {
		t.Fatalf("delete schedule status=%d body=%s", deleteScheduleRes.StatusCode, readBody(t, deleteScheduleRes.Body))
	}

	auditRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/platform/cms/ops/audit", cmsToken, nil, "")
	if auditRes.StatusCode != http.StatusOK {
		t.Fatalf("audit status=%d body=%s", auditRes.StatusCode, readBody(t, auditRes.Body))
	}
	var auditOut struct {
		Data struct {
			Items []map[string]any `json:"items"`
		} `json:"data"`
	}
	mustDecodeJSON(t, auditRes.Body, &auditOut)
	expected := map[string]bool{
		"cms.entry.create":    false,
		"cms.entry.update":    false,
		"cms.schedule.create": false,
		"cms.review.approve":  false,
		"cms.schedule.delete": false,
	}
	for _, item := range auditOut.Data.Items {
		action, _ := item["action"].(string)
		if _, ok := expected[action]; ok {
			expected[action] = true
		}
	}
	for action, seen := range expected {
		if !seen {
			t.Fatalf("expected audit action %q in audit feed, got items=%+v", action, auditOut.Data.Items)
		}
	}

	filteredAuditRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/platform/cms/ops/audit?action=cms.entry.create", cmsToken, nil, "")
	if filteredAuditRes.StatusCode != http.StatusOK {
		t.Fatalf("filtered audit status=%d body=%s", filteredAuditRes.StatusCode, readBody(t, filteredAuditRes.Body))
	}
	var filteredAuditOut struct {
		Data struct {
			Items []map[string]any `json:"items"`
		} `json:"data"`
		Meta map[string]any `json:"meta"`
	}
	mustDecodeJSON(t, filteredAuditRes.Body, &filteredAuditOut)
	if len(filteredAuditOut.Data.Items) == 0 {
		t.Fatalf("expected filtered audit items for cms.entry.create")
	}
	for _, item := range filteredAuditOut.Data.Items {
		if item["action"] != "cms.entry.create" {
			t.Fatalf("unexpected action in filtered audit feed: %+v", item)
		}
	}
	invalidFilterRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/platform/cms/ops/audit?action=unknown.cms.action", cmsToken, nil, "")
	if invalidFilterRes.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid action filter status=%d body=%s", invalidFilterRes.StatusCode, readBody(t, invalidFilterRes.Body))
	}
}

func TestIntegration_platformCMSPrefix_mediaUploadContract(t *testing.T) {
	srv := httptest.NewServer(newTestHandler(t, nil))
	defer srv.Close()

	adminToken := loginAndGetAccessToken(t, srv.URL, "cms.operator@example.com", "secret", "")

	createIntentRes := doJSONRequest(t, http.MethodPost, srv.URL+"/api/v1/platform/cms/media/upload", adminToken, map[string]any{
		"file_name":    "banner-home.png",
		"content_type": "image/png",
		"size_bytes":   2048,
		"context":      "template",
	}, "")
	if createIntentRes.StatusCode != http.StatusCreated {
		t.Fatalf("create media upload intent status=%d body=%s", createIntentRes.StatusCode, readBody(t, createIntentRes.Body))
	}
	var createIntentOut struct {
		Data map[string]any `json:"data"`
	}
	mustDecodeJSON(t, createIntentRes.Body, &createIntentOut)
	assetID, _ := createIntentOut.Data["asset_id"].(string)
	if assetID == "" {
		t.Fatalf("missing asset_id in intent response: %+v", createIntentOut)
	}
	uploadInfo, _ := createIntentOut.Data["upload"].(map[string]any)
	if uploadInfo == nil || uploadInfo["url"] == "" {
		t.Fatalf("missing upload payload in intent response: %+v", createIntentOut)
	}
	uploadURL, _ := uploadInfo["url"].(string)
	if uploadURL == "" {
		t.Fatalf("missing upload url in intent response: %+v", createIntentOut)
	}
	req, err := http.NewRequest(http.MethodPut, uploadURL, strings.NewReader(strings.Repeat("x", 2048)))
	if err != nil {
		t.Fatalf("build upload request: %v", err)
	}
	req.Header.Set("Content-Type", "image/png")
	uploadRes, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("execute upload request: %v", err)
	}
	if uploadRes.StatusCode != http.StatusOK {
		t.Fatalf("signed binary upload status=%d body=%s", uploadRes.StatusCode, readBody(t, uploadRes.Body))
	}

	completeRes := doJSONRequest(t, http.MethodPost, srv.URL+"/api/v1/platform/cms/media/"+assetID+"/complete", adminToken, map[string]any{
		"etag":       "etag-value",
		"checksum":   "sha256-value",
		"size_bytes": 2048,
	}, "")
	if completeRes.StatusCode != http.StatusOK {
		t.Fatalf("complete media upload status=%d body=%s", completeRes.StatusCode, readBody(t, completeRes.Body))
	}

	listRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/platform/cms/media?type=image/png&q=banner", adminToken, nil, "")
	if listRes.StatusCode != http.StatusOK {
		t.Fatalf("list media status=%d body=%s", listRes.StatusCode, readBody(t, listRes.Body))
	}
	var listOut struct {
		Data struct {
			Items []map[string]any `json:"items"`
		} `json:"data"`
	}
	mustDecodeJSON(t, listRes.Body, &listOut)
	if len(listOut.Data.Items) == 0 {
		t.Fatalf("expected listed media assets, got empty list")
	}
	if state, _ := listOut.Data.Items[0]["state"].(string); state != "ready" {
		t.Fatalf("expected ready state after complete, got=%v items=%+v", state, listOut.Data.Items)
	}

	invalidTypeRes := doJSONRequest(t, http.MethodPost, srv.URL+"/api/v1/platform/cms/media/upload", adminToken, map[string]any{
		"file_name":    "malware.exe",
		"content_type": "application/octet-stream",
		"size_bytes":   1024,
	}, "")
	if invalidTypeRes.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid content_type status=%d body=%s", invalidTypeRes.StatusCode, readBody(t, invalidTypeRes.Body))
	}

	oversizeRes := doJSONRequest(t, http.MethodPost, srv.URL+"/api/v1/platform/cms/media/upload", adminToken, map[string]any{
		"file_name":    "big.pdf",
		"content_type": "application/pdf",
		"size_bytes":   50 * 1024 * 1024,
	}, "")
	if oversizeRes.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversize media status=%d body=%s", oversizeRes.StatusCode, readBody(t, oversizeRes.Body))
	}

	forbiddenToken := loginAndGetAccessToken(t, srv.URL, "user@example.com", "secret", "c_001")
	forbiddenRes := doJSONRequest(t, http.MethodPost, srv.URL+"/api/v1/platform/cms/media/upload", forbiddenToken, map[string]any{
		"file_name":    "forbidden.png",
		"content_type": "image/png",
		"size_bytes":   1024,
	}, "")
	if forbiddenRes.StatusCode != http.StatusForbidden {
		t.Fatalf("forbidden media upload status=%d body=%s", forbiddenRes.StatusCode, readBody(t, forbiddenRes.Body))
	}

	deleteRes := doJSONRequest(t, http.MethodDelete, srv.URL+"/api/v1/platform/cms/media/"+assetID, adminToken, nil, "")
	if deleteRes.StatusCode != http.StatusOK {
		t.Fatalf("delete media status=%d body=%s", deleteRes.StatusCode, readBody(t, deleteRes.Body))
	}

	auditRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/platform/cms/ops/audit", adminToken, nil, "")
	if auditRes.StatusCode != http.StatusOK {
		t.Fatalf("media audit status=%d body=%s", auditRes.StatusCode, readBody(t, auditRes.Body))
	}
	var auditOut struct {
		Data struct {
			Items []map[string]any `json:"items"`
		} `json:"data"`
	}
	mustDecodeJSON(t, auditRes.Body, &auditOut)
	expected := map[string]bool{
		"cms.media.upload.intent":   false,
		"cms.media.upload.complete": false,
		"cms.media.delete":          false,
	}
	for _, item := range auditOut.Data.Items {
		action, _ := item["action"].(string)
		if _, ok := expected[action]; ok {
			expected[action] = true
		}
	}
	for action, seen := range expected {
		if !seen {
			t.Fatalf("expected media audit action %q in audit feed, got items=%+v", action, auditOut.Data.Items)
		}
	}
}

func TestIntegration_platformCMSPrefix_adminUsersCreateAndList(t *testing.T) {
	srv := httptest.NewServer(newTestHandler(t, nil))
	defer srv.Close()

	adminToken := loginAndGetAccessToken(t, srv.URL, "cms.operator@example.com", "secret", "")
	createdLogin := "cms-admin-" + strings.ToLower(strings.ReplaceAll(uuid.NewString(), "-", ""))[0:12] + "@example.com"

	createRes := doJSONRequest(t, http.MethodPost, srv.URL+"/api/v1/platform/cms/admin/users", adminToken, map[string]any{
		"login_id":          createdLogin,
		"password":          "secret1234567!",
		"full_name":         "CMS Company User",
		"account_status":    "active",
		"company_id":        "c_001",
		"membership_status": "active",
	}, "")
	if createRes.StatusCode != http.StatusCreated {
		t.Fatalf("create admin user status=%d body=%s", createRes.StatusCode, readBody(t, createRes.Body))
	}
	var createOut struct {
		Data map[string]any `json:"data"`
	}
	mustDecodeJSON(t, createRes.Body, &createOut)
	createdUserID, _ := createOut.Data["user_id"].(string)
	if createdUserID == "" {
		t.Fatalf("missing user_id in create response: %+v", createOut)
	}
	if createOut.Data["membership_id"] == "" {
		t.Fatalf("expected membership_id in create response: %+v", createOut)
	}

	listRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/platform/cms/admin/users?company_id=c_001", adminToken, nil, "")
	if listRes.StatusCode != http.StatusOK {
		t.Fatalf("list admin users status=%d body=%s", listRes.StatusCode, readBody(t, listRes.Body))
	}
	var listOut struct {
		Data struct {
			Items []map[string]any `json:"items"`
		} `json:"data"`
		Meta map[string]any `json:"meta"`
	}
	mustDecodeJSON(t, listRes.Body, &listOut)
	found := false
	for _, item := range listOut.Data.Items {
		if item["user_id"] == createdUserID || item["UserID"] == createdUserID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("created user not found in list response, user_id=%s items=%+v", createdUserID, listOut.Data.Items)
	}
}

// Guards against accidentally dropping the CMS admin create-company route (would surface as FE 404 via Vite proxy).
func TestIntegration_platformCMSPrefix_adminCompaniesRouteRegistered(t *testing.T) {
	srv := httptest.NewServer(newTestHandler(t, nil))
	defer srv.Close()

	createRes := doJSONRequest(t, http.MethodPost, srv.URL+"/api/v1/platform/cms/admin/companies", "", map[string]any{
		"company_name": "Smoke Co",
	}, "")
	if createRes.StatusCode == http.StatusNotFound {
		t.Fatalf("POST /api/v1/platform/cms/admin/companies returned 404; route missing from mux")
	}
	if createRes.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without bearer, got status=%d body=%s", createRes.StatusCode, readBody(t, createRes.Body))
	}
}

func TestIntegration_disclosureC1_contractMatrix_happyPathAndErrors(t *testing.T) {
	srv := httptest.NewServer(newTestHandler(t, nil))
	defer srv.Close()

	userToken := loginAndGetAccessToken(t, srv.URL, "user@example.com", "secret", "c_001")

	createPayload := map[string]any{
		"type_id":       "dt-periodic-financial",
		"department_id": "ou_legal",
		"title":         "Quarterly Disclosure",
		"summary":       "Q1 summary",
		"content":       "Detailed disclosure content",
		"planned_date":  "2026-05-01",
		"attachments": []map[string]string{
			{"id": "att-1", "name": "evidence.pdf", "type": "application/pdf", "uploaded_at": "2026-04-27T00:00:00Z"},
		},
		"evidence_link": "https://example.com/evidence",
	}
	createRes := doJSONRequest(t, http.MethodPost, srv.URL+"/api/v1/disclosures", userToken, createPayload, "")
	if createRes.StatusCode != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", createRes.StatusCode, readBody(t, createRes.Body))
	}
	var created map[string]any
	mustDecodeJSON(t, createRes.Body, &created)
	recordID, _ := created["record_id"].(string)
	if recordID == "" {
		t.Fatal("missing record_id in create response")
	}
	if created["type_id"] != "dt-periodic-financial" || created["summary"] != "Q1 summary" || created["planned_date"] != "2026-05-01" {
		t.Fatalf("unexpected create contract fields: %+v", created)
	}

	getRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/disclosures/"+recordID, userToken, nil, "")
	if getRes.StatusCode != http.StatusOK {
		t.Fatalf("get status=%d body=%s", getRes.StatusCode, readBody(t, getRes.Body))
	}
	var got map[string]any
	mustDecodeJSON(t, getRes.Body, &got)
	if got["record_id"] != recordID {
		t.Fatalf("unexpected get record_id=%v want %s", got["record_id"], recordID)
	}

	updatePayload := map[string]any{
		"type_id":       "dt-periodic-financial",
		"department_id": "ou_legal",
		"title":         "Quarterly Disclosure Updated",
		"summary":       "Updated summary",
		"content":       "Updated content",
		"planned_date":  "2026-05-02",
		"attachments": []map[string]string{
			{"id": "att-2", "name": "updated.pdf", "type": "application/pdf", "uploaded_at": "2026-04-27T01:00:00Z"},
		},
		"evidence_link": "https://example.com/evidence-updated",
	}
	updateRes := doJSONRequest(t, http.MethodPatch, srv.URL+"/api/v1/disclosures/"+recordID, userToken, updatePayload, "")
	if updateRes.StatusCode != http.StatusOK {
		t.Fatalf("update status=%d body=%s", updateRes.StatusCode, readBody(t, updateRes.Body))
	}
	var updated map[string]any
	mustDecodeJSON(t, updateRes.Body, &updated)
	if updated["title"] != "Quarterly Disclosure Updated" || updated["status"] != "Draft" {
		t.Fatalf("unexpected update response: %+v", updated)
	}

	listResDenied := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/disclosures", userToken, nil, "")
	if listResDenied.StatusCode != http.StatusForbidden {
		t.Fatalf("list by non-admin status=%d body=%s", listResDenied.StatusCode, readBody(t, listResDenied.Body))
	}

	submitRes := doJSONRequest(t, http.MethodPost, srv.URL+"/api/v1/disclosures/"+recordID+"/submit", userToken, nil, "idem-submit-c1")
	if submitRes.StatusCode != http.StatusOK {
		t.Fatalf("submit status=%d body=%s", submitRes.StatusCode, readBody(t, submitRes.Body))
	}
	var submitted map[string]any
	mustDecodeJSON(t, submitRes.Body, &submitted)
	if submitted["status"] != "PendingReview" {
		t.Fatalf("unexpected submit status: %+v", submitted)
	}

	confirmByNonAdminRes := doJSONRequest(t, http.MethodPost, srv.URL+"/api/v1/disclosures/"+recordID+"/confirm", userToken, nil, "idem-confirm-user")
	if confirmByNonAdminRes.StatusCode != http.StatusForbidden {
		t.Fatalf("confirm by non-admin status=%d body=%s", confirmByNonAdminRes.StatusCode, readBody(t, confirmByNonAdminRes.Body))
	}

	adminToken := loginAndGetAccessToken(t, srv.URL, "admin.dn@example.com", "secret", "")
	listRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/disclosures", adminToken, nil, "")
	if listRes.StatusCode != http.StatusForbidden {
		t.Fatalf("list by admin status=%d body=%s", listRes.StatusCode, readBody(t, listRes.Body))
	}
	_ = readBody(t, listRes.Body)

	confirmByAdminRes := doJSONRequest(t, http.MethodPost, srv.URL+"/api/v1/disclosures/"+recordID+"/confirm", adminToken, nil, "idem-confirm-admin")
	if confirmByAdminRes.StatusCode != http.StatusConflict {
		t.Fatalf("confirm before workflow approval should conflict, got status=%d body=%s", confirmByAdminRes.StatusCode, readBody(t, confirmByAdminRes.Body))
	}
}

func TestIntegration_disclosureC1_contractMatrix_validationAndNotFound(t *testing.T) {
	srv := httptest.NewServer(newTestHandler(t, nil))
	defer srv.Close()

	userToken := loginAndGetAccessToken(t, srv.URL, "user@example.com", "secret", "c_001")

	missingTitleRes := doJSONRequest(t, http.MethodPost, srv.URL+"/api/v1/disclosures", userToken, map[string]any{
		"department_id": "ou_legal",
		"content":       "has content",
	}, "")
	if missingTitleRes.StatusCode != http.StatusBadRequest {
		t.Fatalf("missing title status=%d body=%s", missingTitleRes.StatusCode, readBody(t, missingTitleRes.Body))
	}

	invalidDateRes := doJSONRequest(t, http.MethodPost, srv.URL+"/api/v1/disclosures", userToken, map[string]any{
		"title":        "Invalid Date",
		"content":      "Body",
		"planned_date": "2026/05/01",
	}, "")
	if invalidDateRes.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid planned_date status=%d body=%s", invalidDateRes.StatusCode, readBody(t, invalidDateRes.Body))
	}

	notFoundUpdateRes := doJSONRequest(t, http.MethodPatch, srv.URL+"/api/v1/disclosures/not-found-id", userToken, map[string]any{
		"title":   "Any",
		"content": "Any",
	}, "")
	if notFoundUpdateRes.StatusCode != http.StatusNotFound {
		t.Fatalf("update not found status=%d body=%s", notFoundUpdateRes.StatusCode, readBody(t, notFoundUpdateRes.Body))
	}

	unauthenticatedRes := doJSONRequest(t, http.MethodPost, srv.URL+"/api/v1/disclosures", "", map[string]any{
		"title":   "No Auth",
		"content": "No Auth",
	}, "")
	if unauthenticatedRes.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status=%d body=%s", unauthenticatedRes.StatusCode, readBody(t, unauthenticatedRes.Body))
	}
}

func loginAndGetAccessToken(t *testing.T, baseURL, loginID, password, preferredCompanyID string) string {
	t.Helper()
	loginRes := doJSONRequest(t, http.MethodPost, baseURL+"/api/v1/auth/login", "", map[string]any{
		"login_id": loginID,
		"password": password,
	}, "")
	if loginRes.StatusCode != http.StatusOK {
		t.Fatalf("login status=%d body=%s", loginRes.StatusCode, readBody(t, loginRes.Body))
	}
	var loginOut struct {
		NextAction  string `json:"next_action"`
		Memberships []struct {
			CompanyID string `json:"company_id"`
		} `json:"memberships"`
		Session struct {
			AccessToken     string `json:"access_token"`
			PreCompanyToken string `json:"pre_company_token"`
		} `json:"session"`
	}
	mustDecodeJSON(t, loginRes.Body, &loginOut)
	if loginOut.Session.AccessToken != "" {
		return loginOut.Session.AccessToken
	}
	if loginOut.Session.PreCompanyToken == "" {
		t.Fatal("missing both access_token and pre_company_token")
	}

	companyID := preferredCompanyID
	if companyID == "" {
		if len(loginOut.Memberships) == 0 {
			t.Fatal("cannot select company: memberships is empty")
		}
		companyID = loginOut.Memberships[0].CompanyID
	}

	selectRes := doJSONRequest(t, http.MethodPost, baseURL+"/api/v1/auth/select-company", loginOut.Session.PreCompanyToken, map[string]any{
		"company_id": companyID,
	}, "")
	if selectRes.StatusCode != http.StatusOK {
		t.Fatalf("select-company status=%d body=%s", selectRes.StatusCode, readBody(t, selectRes.Body))
	}
	var selectOut struct {
		AccessToken string `json:"access_token"`
	}
	mustDecodeJSON(t, selectRes.Body, &selectOut)
	if selectOut.AccessToken == "" {
		t.Fatal("missing access_token after company selection")
	}
	return selectOut.AccessToken
}

func doJSONRequest(t *testing.T, method, url, accessToken string, body any, idempotencyKey string) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	return res
}

func mustDecodeJSON(t *testing.T, body io.ReadCloser, out any) {
	t.Helper()
	defer body.Close()
	if err := json.NewDecoder(body).Decode(out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func readBody(t *testing.T, body io.ReadCloser) string {
	t.Helper()
	defer body.Close()
	b, _ := io.ReadAll(body)
	return string(b)
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

func TestIntegration_disclosureTypeCatalog_contractAndAuth(t *testing.T) {
	srv := httptest.NewServer(newTestHandler(t, nil))
	defer srv.Close()

	token := loginAndGetAccessToken(t, srv.URL, "cms.operator@example.com", "secret", "c_001")

	reqGroups, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/disclosure-groups", nil)
	reqGroups.Header.Set("Authorization", "Bearer "+token)
	resGroups, err := http.DefaultClient.Do(reqGroups)
	if err != nil {
		t.Fatal(err)
	}
	defer resGroups.Body.Close()
	if resGroups.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resGroups.Body)
		t.Fatalf("groups status=%d body=%s", resGroups.StatusCode, b)
	}
	var groupsOut struct {
		Items []struct {
			GroupID string `json:"group_id"`
			Name    string `json:"name"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resGroups.Body).Decode(&groupsOut); err != nil {
		t.Fatal(err)
	}
	if len(groupsOut.Items) == 0 {
		t.Fatal("expected at least one disclosure group")
	}

	reqTypes, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/disclosure-types?group_id=group-002", nil)
	reqTypes.Header.Set("Authorization", "Bearer "+token)
	resTypes, err := http.DefaultClient.Do(reqTypes)
	if err != nil {
		t.Fatal(err)
	}
	defer resTypes.Body.Close()
	if resTypes.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resTypes.Body)
		t.Fatalf("types status=%d body=%s", resTypes.StatusCode, b)
	}
	var typesOut struct {
		Items []struct {
			TypeID string `json:"type_id"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resTypes.Body).Decode(&typesOut); err != nil {
		t.Fatal(err)
	}
	if len(typesOut.Items) == 0 {
		t.Fatal("expected filtered disclosure types")
	}

	reqDetail, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/disclosure-types/dt-event-major-change", nil)
	reqDetail.Header.Set("Authorization", "Bearer "+token)
	resDetail, err := http.DefaultClient.Do(reqDetail)
	if err != nil {
		t.Fatal(err)
	}
	defer resDetail.Body.Close()
	if resDetail.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resDetail.Body)
		t.Fatalf("detail status=%d body=%s", resDetail.StatusCode, b)
	}
	var detailOut struct {
		TypeID string `json:"type_id"`
	}
	if err := json.NewDecoder(resDetail.Body).Decode(&detailOut); err != nil {
		t.Fatal(err)
	}
	if detailOut.TypeID != "dt-event-major-change" {
		t.Fatalf("unexpected type detail id=%s", detailOut.TypeID)
	}

	reqEffectiveWorkflow, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/disclosure-types/dt-event-major-change/effective-workflow", nil)
	reqEffectiveWorkflow.Header.Set("Authorization", "Bearer "+token)
	resEffectiveWorkflow, err := http.DefaultClient.Do(reqEffectiveWorkflow)
	if err != nil {
		t.Fatal(err)
	}
	defer resEffectiveWorkflow.Body.Close()
	if resEffectiveWorkflow.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resEffectiveWorkflow.Body)
		t.Fatalf("effective workflow status=%d body=%s", resEffectiveWorkflow.StatusCode, b)
	}

	reqMissing, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/disclosure-types/type-not-found", nil)
	reqMissing.Header.Set("Authorization", "Bearer "+token)
	resMissing, err := http.DefaultClient.Do(reqMissing)
	if err != nil {
		t.Fatal(err)
	}
	defer resMissing.Body.Close()
	if resMissing.StatusCode != http.StatusNotFound {
		b, _ := io.ReadAll(resMissing.Body)
		t.Fatalf("missing detail status=%d body=%s", resMissing.StatusCode, b)
	}

	forbiddenToken := loginAndGetAccessToken(t, srv.URL, "single@example.com", "secret", "c_010")
	reqForbidden, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/disclosure-groups", nil)
	reqForbidden.Header.Set("Authorization", "Bearer "+forbiddenToken)
	resForbidden, err := http.DefaultClient.Do(reqForbidden)
	if err != nil {
		t.Fatal(err)
	}
	defer resForbidden.Body.Close()
	if resForbidden.StatusCode != http.StatusForbidden {
		b, _ := io.ReadAll(resForbidden.Body)
		t.Fatalf("forbidden groups status=%d body=%s", resForbidden.StatusCode, b)
	}

	reqForbiddenWorkflow, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/disclosure-types/dt-event-major-change/effective-workflow", nil)
	reqForbiddenWorkflow.Header.Set("Authorization", "Bearer "+forbiddenToken)
	resForbiddenWorkflow, err := http.DefaultClient.Do(reqForbiddenWorkflow)
	if err != nil {
		t.Fatal(err)
	}
	defer resForbiddenWorkflow.Body.Close()
	if resForbiddenWorkflow.StatusCode != http.StatusForbidden {
		b, _ := io.ReadAll(resForbiddenWorkflow.Body)
		t.Fatalf("forbidden effective workflow status=%d body=%s", resForbiddenWorkflow.StatusCode, b)
	}
}

func integrationMandatoryDisclosureBlocksMaps() []map[string]any {
	return []map[string]any{
		{
			"block_id": "int-m1", "block_key": "legal_basis", "block_type": "rich_text",
			"title": "LB", "description": "",
			"config": map[string]any{"max_length": 8000, "allow_html": false}, "validation": map[string]any{},
			"display_order": 1, "enabled": true,
		},
		{
			"block_id": "int-m2", "block_key": "disclosure_content", "block_type": "rich_text",
			"title": "DC", "description": "",
			"config": map[string]any{"max_length": 10000, "allow_html": true}, "validation": map[string]any{},
			"display_order": 2, "enabled": true,
		},
		{
			"block_id": "int-m3", "block_key": "deadline", "block_type": "text",
			"title": "DL", "description": "",
			"config": map[string]any{"max_length": 4000}, "validation": map[string]any{},
			"display_order": 3, "enabled": true,
		},
		{
			"block_id": "int-m4", "block_key": "channels_and_format", "block_type": "rich_text",
			"title": "CF", "description": "",
			"config": map[string]any{
				"max_length": 12000,
				"allow_html": false,
				"channels": []any{
					map[string]any{
						"id":         "ch-001",
						"name":       "Website công ty",
						"file_types": []any{"PDF"},
					},
				},
				"file_types": []any{"PDF"},
			}, "validation": map[string]any{},
			"display_order": 4, "enabled": true,
		},
		{
			"block_id": "int-m5", "block_key": "legal_risks", "block_type": "rich_text",
			"title": "LR", "description": "",
			"config": map[string]any{"max_length": 8000, "allow_html": false}, "validation": map[string]any{},
			"display_order": 5, "enabled": true,
		},
		{
			"block_id": "int-m6", "block_key": "enterprise_workflow", "block_type": "rich_text",
			"title": "EW", "description": "",
			"config": map[string]any{
				"max_length": 12000,
				"allow_html": true,
				"steps": []any{
					map[string]any{
						"step_id":           "review-step",
						"stage":             "Review",
						"department_id":     "dept-finance",
						"assignee_role_ids": []any{"role-reviewer"},
						"processing_days":   2,
						"display_order":     1,
						"documents":         []any{},
					},
				},
			}, "validation": map[string]any{},
			"display_order": 6, "enabled": true,
		},
	}
}

func TestIntegration_disclosureTypeCatalog_adminUpsertAndVersioning(t *testing.T) {
	srv := httptest.NewServer(newTestHandler(t, nil))
	defer srv.Close()

	adminToken := loginAndGetAccessToken(t, srv.URL, "cms.operator@example.com", "secret", "c_001")
	userToken := loginAndGetAccessToken(t, srv.URL, "single@example.com", "secret", "c_010")

	upsertRes := doJSONRequest(t, http.MethodPut, srv.URL+"/api/v1/admin/disclosure-types/dt-custom-obligation", adminToken, map[string]any{
		"group_id":            "group-006",
		"name":                "Template nghĩa vụ tùy chỉnh V2",
		"category":            "Tùy chỉnh",
		"template_category":   "custom",
		"deadline_strategy":   "configurable",
		"description":         "Updated template description",
		"deadline_rule":       "Theo cấu hình admin phiên bản 2",
		"periodicity":         "monthly",
		"legal_bases": []map[string]any{
			{
				"id":         "lb-001",
				"title":      "Thông tư hướng dẫn công bố thông tin",
				"code":       "96/2020/TT-BTC",
				"authority":  "Bộ Tài chính",
				"issue_date": "2020-11-16",
				"summary":    "Quy định về nghĩa vụ công bố thông tin định kỳ",
				"link":       "#",
			},
		},
		"checklist": []map[string]any{
			{"id": "ck-001", "title": "Chốt số liệu kế toán quý", "owner": "Kế toán trưởng", "due_date": "T+5", "status": "Completed"},
			{"id": "ck-002", "title": "Soát xét nội bộ", "owner": "Trưởng phòng Tài chính", "due_date": "T+15", "status": "Pending"},
		},
		"channels_text":          "Nội bộ",
		"format":                 "PDF",
		"tags":                   []string{"Tùy chỉnh", "V2"},
		"change_note":            "update deadline semantics",
		"implementation_content": "Updated implementation content",
		"implementation_notes":   "Updated implementation notes",
		"required_docs":          "Updated required docs",
		"legal_risks_text":       "Updated legal risks",
		"general_info":           "Updated general info",
		"receiving_authorities":  "Nội bộ",
		"beneficiaries":          "Ban điều hành",
		"special_cases":          "Updated special cases",
		"report_content":         "Updated report content",
		"applicability":          "Theo phân quyền nội bộ",
		"legal_basis":            "Quy chế nội bộ doanh nghiệp",
		"blocks": append(integrationMandatoryDisclosureBlocksMaps(),
			map[string]any{
				"block_id":      "block-general-info",
				"block_key":     "general_info",
				"block_type":    "text",
				"title":         "Thông tin chung",
				"description":   "Mô tả khối thông tin chung",
				"config":        map[string]any{"max_length": 5000},
				"validation":    map[string]any{"required": true},
				"display_order": 7,
				"enabled":       true,
			},
			map[string]any{
				"block_id":    "block-required-docs",
				"block_key":   "required_docs",
				"block_type":  "checklist",
				"title":       "Hồ sơ bắt buộc",
				"description": "Checklist hồ sơ",
				"config": map[string]any{
					"allow_custom_items": false,
					"options": []map[string]any{
						{"id": "opt-audit", "label": "Audit report"},
						{"id": "opt-board", "label": "Board minutes"},
					},
				},
				"validation":    map[string]any{"required": true},
				"display_order": 8,
				"enabled":       true,
			},
		),
	}, "")
	if upsertRes.StatusCode != http.StatusOK {
		t.Fatalf("admin upsert status=%d body=%s", upsertRes.StatusCode, readBody(t, upsertRes.Body))
	}
	var upsertOut struct {
		TypeID    string `json:"type_id"`
		VersionNo int    `json:"version_no"`
	}
	mustDecodeJSON(t, upsertRes.Body, &upsertOut)
	if upsertOut.TypeID != "dt-custom-obligation" || upsertOut.VersionNo < 2 {
		t.Fatalf("unexpected upsert response: %+v", upsertOut)
	}

	detailRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/disclosure-types/dt-custom-obligation", adminToken, nil, "")
	if detailRes.StatusCode != http.StatusOK {
		t.Fatalf("detail status=%d body=%s", detailRes.StatusCode, readBody(t, detailRes.Body))
	}
	var detailOut struct {
		VersionNo  int      `json:"version_no"`
		Name       string   `json:"name"`
		Tags       []string `json:"tags"`
		LegalBases []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"legal_bases"`
		Checklist []struct {
			ID     string `json:"id"`
			Title  string `json:"title"`
			Status string `json:"status"`
		} `json:"checklist"`
		Blocks []struct {
			BlockID      string `json:"block_id"`
			BlockKey     string `json:"block_key"`
			DisplayOrder int    `json:"display_order"`
		} `json:"blocks"`
	}
	mustDecodeJSON(t, detailRes.Body, &detailOut)
	activateRes := doJSONRequest(t, http.MethodPost, srv.URL+"/api/v1/admin/disclosure-types/dt-custom-obligation/activate", adminToken, map[string]any{
		"version_no": upsertOut.VersionNo,
		"reason":     "activate draft after review",
	}, "")
	if activateRes.StatusCode != http.StatusOK {
		t.Fatalf("activate status=%d body=%s", activateRes.StatusCode, readBody(t, activateRes.Body))
	}
	detailRes = doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/disclosure-types/dt-custom-obligation", adminToken, nil, "")
	if detailRes.StatusCode != http.StatusOK {
		t.Fatalf("detail after activate status=%d body=%s", detailRes.StatusCode, readBody(t, detailRes.Body))
	}
	mustDecodeJSON(t, detailRes.Body, &detailOut)
	if detailOut.VersionNo != upsertOut.VersionNo || detailOut.Name != "Template nghĩa vụ tùy chỉnh V2" {
		t.Fatalf("unexpected detail after upsert: %+v", detailOut)
	}
	if len(detailOut.LegalBases) != 1 || detailOut.LegalBases[0].ID != "lb-001" {
		t.Fatalf("unexpected legal_bases after upsert: %+v", detailOut.LegalBases)
	}
	if len(detailOut.Checklist) != 2 || detailOut.Checklist[0].Status != "Completed" {
		t.Fatalf("unexpected checklist after upsert: %+v", detailOut.Checklist)
	}
	if len(detailOut.Blocks) != 8 {
		t.Fatalf("unexpected blocks length after upsert: %+v", detailOut.Blocks)
	}
	if detailOut.Blocks[0].BlockKey != "legal_basis" || detailOut.Blocks[7].DisplayOrder != 8 {
		t.Fatalf("unexpected blocks order after upsert: %+v", detailOut.Blocks)
	}

	versionsRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/admin/disclosure-types/dt-custom-obligation/versions", adminToken, nil, "")
	if versionsRes.StatusCode != http.StatusOK {
		t.Fatalf("admin versions status=%d body=%s", versionsRes.StatusCode, readBody(t, versionsRes.Body))
	}
	var versionsOut struct {
		Items []struct {
			VersionNo int  `json:"version_no"`
			IsActive  bool `json:"is_active"`
		} `json:"items"`
	}
	mustDecodeJSON(t, versionsRes.Body, &versionsOut)
	if len(versionsOut.Items) < 2 {
		t.Fatalf("expected at least two versions, got=%d", len(versionsOut.Items))
	}
	if versionsOut.Items[0].VersionNo != upsertOut.VersionNo || !versionsOut.Items[0].IsActive {
		t.Fatalf("unexpected latest version metadata: %+v", versionsOut.Items[0])
	}
	versionDetailRes := doJSONRequest(
		t,
		http.MethodGet,
		srv.URL+"/api/v1/admin/disclosure-types/dt-custom-obligation/versions/"+strconv.Itoa(upsertOut.VersionNo),
		adminToken,
		nil,
		"",
	)
	if versionDetailRes.StatusCode != http.StatusOK {
		t.Fatalf("admin version detail status=%d body=%s", versionDetailRes.StatusCode, readBody(t, versionDetailRes.Body))
	}
	var versionDetailOut struct {
		VersionNo          int      `json:"version_no"`
		Name               string   `json:"name"`
		LegalBases []struct {
			ID string `json:"id"`
		} `json:"legal_bases"`
		Checklist []struct {
			ID string `json:"id"`
		} `json:"checklist"`
		Blocks []struct {
			BlockKey string `json:"block_key"`
		} `json:"blocks"`
	}
	mustDecodeJSON(t, versionDetailRes.Body, &versionDetailOut)
	if versionDetailOut.VersionNo != upsertOut.VersionNo || len(versionDetailOut.Blocks) != 8 {
		t.Fatalf("unexpected admin version detail: %+v", versionDetailOut)
	}
	if len(versionDetailOut.LegalBases) != 1 || versionDetailOut.LegalBases[0].ID != "lb-001" {
		t.Fatalf("unexpected version legal_bases: %+v", versionDetailOut.LegalBases)
	}
	if len(versionDetailOut.Checklist) != 2 || versionDetailOut.Checklist[1].ID != "ck-002" {
		t.Fatalf("unexpected version checklist: %+v", versionDetailOut.Checklist)
	}
	forbiddenVersionDetailRes := doJSONRequest(
		t,
		http.MethodGet,
		srv.URL+"/api/v1/admin/disclosure-types/dt-custom-obligation/versions/1",
		userToken,
		nil,
		"",
	)
	if forbiddenVersionDetailRes.StatusCode != http.StatusForbidden {
		t.Fatalf("non-admin version detail status=%d body=%s", forbiddenVersionDetailRes.StatusCode, readBody(t, forbiddenVersionDetailRes.Body))
	}

	refDataRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/admin/disclosure-types/reference-data", adminToken, nil, "")
	if refDataRes.StatusCode != http.StatusOK {
		t.Fatalf("reference-data status=%d body=%s", refDataRes.StatusCode, readBody(t, refDataRes.Body))
	}
	var refDataOut struct {
		Data struct {
			TemplateCategories []string            `json:"template_categories"`
			Periodicities      []string            `json:"periodicities"`
			DeadlineStrategies []string            `json:"deadline_strategies"`
			MatrixRules        map[string][]string `json:"matrix_rules"`
		} `json:"data"`
	}
	mustDecodeJSON(t, refDataRes.Body, &refDataOut)
	if len(refDataOut.Data.TemplateCategories) == 0 || len(refDataOut.Data.Periodicities) == 0 || len(refDataOut.Data.DeadlineStrategies) == 0 {
		t.Fatalf("unexpected reference-data payload: %+v", refDataOut)
	}
	if len(refDataOut.Data.MatrixRules["periodic"]) == 0 {
		t.Fatalf("expected periodic matrix rules in reference-data: %+v", refDataOut.Data.MatrixRules)
	}

	forbiddenRes := doJSONRequest(t, http.MethodPut, srv.URL+"/api/v1/admin/disclosure-types/dt-custom-obligation", userToken, map[string]any{
		"group_id": "group-006",
		"name":     "Should Fail",
	}, "")
	if forbiddenRes.StatusCode != http.StatusForbidden {
		t.Fatalf("non-admin upsert status=%d body=%s", forbiddenRes.StatusCode, readBody(t, forbiddenRes.Body))
	}
	forbiddenRefDataRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/admin/disclosure-types/reference-data", userToken, nil, "")
	if forbiddenRefDataRes.StatusCode != http.StatusForbidden {
		t.Fatalf("non-admin reference-data status=%d body=%s", forbiddenRefDataRes.StatusCode, readBody(t, forbiddenRefDataRes.Body))
	}

	invalidPeriodicRes := doJSONRequest(t, http.MethodPut, srv.URL+"/api/v1/admin/disclosure-types/dt-periodic-financial", adminToken, map[string]any{
		"group_id":          "group-001",
		"name":              "Invalid periodic",
		"template_category": "periodic",
		"deadline_strategy": "event_relative_hours",
		"deadline_rule":     "T+24h",
		"periodicity":       "event_based",
	}, "")
	if invalidPeriodicRes.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid periodic matrix status=%d body=%s", invalidPeriodicRes.StatusCode, readBody(t, invalidPeriodicRes.Body))
	}
	if !strings.Contains(readBody(t, invalidPeriodicRes.Body), "deadline_strategy") {
		t.Fatalf("expected field_errors.deadline_strategy in periodic invalid response")
	}

	invalidIrregularRes := doJSONRequest(t, http.MethodPut, srv.URL+"/api/v1/admin/disclosure-types/dt-event-major-change", adminToken, map[string]any{
		"group_id":          "group-002",
		"name":              "Invalid irregular",
		"template_category": "irregular",
		"deadline_strategy": "fixed_cycle_days",
		"deadline_rule":     "T+2 ngày",
		"periodicity":       "monthly",
	}, "")
	if invalidIrregularRes.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid irregular matrix status=%d body=%s", invalidIrregularRes.StatusCode, readBody(t, invalidIrregularRes.Body))
	}
	if !strings.Contains(readBody(t, invalidIrregularRes.Body), "periodicity") {
		t.Fatalf("expected field_errors.periodicity in irregular invalid response")
	}

	invalidCustomRes := doJSONRequest(t, http.MethodPut, srv.URL+"/api/v1/admin/disclosure-types/dt-custom-obligation", adminToken, map[string]any{
		"group_id":          "group-006",
		"name":              "Invalid custom",
		"template_category": "custom",
		"deadline_strategy": "configurable",
		"deadline_rule":     "theo nhu cầu",
		"periodicity":       "event_based",
	}, "")
	if invalidCustomRes.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid custom matrix status=%d body=%s", invalidCustomRes.StatusCode, readBody(t, invalidCustomRes.Body))
	}
	if !strings.Contains(readBody(t, invalidCustomRes.Body), "field_errors") {
		t.Fatalf("expected field_errors in custom invalid response")
	}
	invalidBlocksRes := doJSONRequest(t, http.MethodPut, srv.URL+"/api/v1/admin/disclosure-types/dt-custom-obligation", adminToken, map[string]any{
		"group_id":          "group-006",
		"name":              "Invalid blocks",
		"template_category": "custom",
		"deadline_strategy": "configurable",
		"deadline_rule":     "theo nhu cầu",
		"periodicity":       "monthly",
		"blocks": []map[string]any{
			{
				"block_id":      "a",
				"block_key":     "dup_key",
				"block_type":    "text",
				"title":         "Block A",
				"display_order": 1,
				"enabled":       true,
			},
			{
				"block_id":      "b",
				"block_key":     "dup_key",
				"block_type":    "text",
				"title":         "Block B",
				"display_order": 1,
				"enabled":       true,
			},
		},
	}, "")
	if invalidBlocksRes.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid blocks status=%d body=%s", invalidBlocksRes.StatusCode, readBody(t, invalidBlocksRes.Body))
	}
	invalidBlocksBody := readBody(t, invalidBlocksRes.Body)
	if !strings.Contains(invalidBlocksBody, "blocks.0.block_key") {
		t.Fatalf("expected duplicate block_key field error in invalid blocks response")
	}
	if !strings.Contains(invalidBlocksBody, "blocks.missing_") {
		t.Fatalf("expected blocks.missing_* field_errors when mandatory keys are absent (duplicate-key path skips flat/block sync rebuild): %s", invalidBlocksBody)
	}

	invalidChecklistSchemaRes := doJSONRequest(t, http.MethodPut, srv.URL+"/api/v1/admin/disclosure-types/dt-custom-obligation", adminToken, map[string]any{
		"group_id":          "group-006",
		"name":              "Invalid checklist schema",
		"template_category": "custom",
		"deadline_strategy": "configurable",
		"deadline_rule":     "theo nhu cầu",
		"periodicity":       "monthly",
		"blocks": append(integrationMandatoryDisclosureBlocksMaps(), map[string]any{
			"block_id":      "chk1",
			"block_key":     "solo_checklist",
			"block_type":    "checklist",
			"title":         "Extra checklist",
			"config":        map[string]any{"allow_custom_items": false},
			"validation":    map[string]any{},
			"display_order": 7,
			"enabled":       true,
		}),
	}, "")
	if invalidChecklistSchemaRes.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid checklist schema status=%d body=%s", invalidChecklistSchemaRes.StatusCode, readBody(t, invalidChecklistSchemaRes.Body))
	}
	if !strings.Contains(readBody(t, invalidChecklistSchemaRes.Body), "config.options") {
		t.Fatalf("expected checklist config.options field error in invalid checklist schema response")
	}

	rollbackActivateRes := doJSONRequest(t, http.MethodPost, srv.URL+"/api/v1/admin/disclosure-types/dt-custom-obligation/activate", adminToken, map[string]any{
		"version_no": 1,
		"reason":     "rollback to baseline",
	}, "")
	if rollbackActivateRes.StatusCode != http.StatusOK {
		t.Fatalf("activate old version status=%d body=%s", rollbackActivateRes.StatusCode, readBody(t, rollbackActivateRes.Body))
	}
	var activateOut struct {
		VersionNo int `json:"version_no"`
	}
	mustDecodeJSON(t, rollbackActivateRes.Body, &activateOut)
	if activateOut.VersionNo != 1 {
		t.Fatalf("unexpected activated version: %+v", activateOut)
	}

	detailAfterActivateRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/disclosure-types/dt-custom-obligation", adminToken, nil, "")
	if detailAfterActivateRes.StatusCode != http.StatusOK {
		t.Fatalf("detail after activate status=%d body=%s", detailAfterActivateRes.StatusCode, readBody(t, detailAfterActivateRes.Body))
	}
	var detailAfterActivate struct {
		VersionNo int    `json:"version_no"`
		Name      string `json:"name"`
	}
	mustDecodeJSON(t, detailAfterActivateRes.Body, &detailAfterActivate)
	if detailAfterActivate.VersionNo != 1 {
		t.Fatalf("expected active version 1 after rollback, got=%+v", detailAfterActivate)
	}

	badActivateRes := doJSONRequest(t, http.MethodPost, srv.URL+"/api/v1/admin/disclosure-types/dt-custom-obligation/activate", adminToken, map[string]any{
		"version_no": 0,
	}, "")
	if badActivateRes.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid activate payload status=%d body=%s", badActivateRes.StatusCode, readBody(t, badActivateRes.Body))
	}

	notFoundActivateRes := doJSONRequest(t, http.MethodPost, srv.URL+"/api/v1/admin/disclosure-types/dt-custom-obligation/activate", adminToken, map[string]any{
		"version_no": 999,
	}, "")
	if notFoundActivateRes.StatusCode != http.StatusNotFound {
		t.Fatalf("activate unknown version status=%d body=%s", notFoundActivateRes.StatusCode, readBody(t, notFoundActivateRes.Body))
	}

	forbiddenActivateRes := doJSONRequest(t, http.MethodPost, srv.URL+"/api/v1/admin/disclosure-types/dt-custom-obligation/activate", userToken, map[string]any{
		"version_no": 1,
	}, "")
	if forbiddenActivateRes.StatusCode != http.StatusForbidden {
		t.Fatalf("non-admin activate status=%d body=%s", forbiddenActivateRes.StatusCode, readBody(t, forbiddenActivateRes.Body))
	}

	auditRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/platform/cms/ops/audit", adminToken, nil, "")
	if auditRes.StatusCode != http.StatusOK {
		t.Fatalf("audit status=%d body=%s", auditRes.StatusCode, readBody(t, auditRes.Body))
	}
	var auditOut struct {
		Data struct {
			Items []map[string]any `json:"items"`
		} `json:"data"`
	}
	mustDecodeJSON(t, auditRes.Body, &auditOut)
	expected := map[string]bool{
		"disclosure.type.version.upsert":   false,
		"disclosure.type.version.activate": false,
	}
	for _, item := range auditOut.Data.Items {
		action, _ := item["action"].(string)
		if _, ok := expected[action]; ok {
			expected[action] = true
		}
	}
	for action, seen := range expected {
		if !seen {
			t.Fatalf("expected audit action %q in audit feed, got items=%+v", action, auditOut.Data.Items)
		}
	}
	var upsertMeta map[string]any
	var activateMeta map[string]any
	for _, item := range auditOut.Data.Items {
		action, _ := item["action"].(string)
		if action == "disclosure.type.version.upsert" {
			meta, _ := item["metadata"].(map[string]any)
			upsertMeta = meta
		}
		if action == "disclosure.type.version.activate" {
			meta, _ := item["metadata"].(map[string]any)
			activateMeta = meta
		}
	}
	if upsertMeta == nil {
		t.Fatalf("expected upsert metadata in audit response, got items=%+v", auditOut.Data.Items)
	}
	if _, ok := upsertMeta["old_version_no"]; !ok {
		t.Fatalf("expected old_version_no in upsert metadata, got=%+v", upsertMeta)
	}
	if _, ok := upsertMeta["new_version_no"]; !ok {
		t.Fatalf("expected new_version_no in upsert metadata, got=%+v", upsertMeta)
	}
	if activateMeta == nil {
		t.Fatalf("expected activate metadata in audit response, got items=%+v", auditOut.Data.Items)
	}
	if strings.TrimSpace(fmt.Sprint(activateMeta["reason"])) == "" {
		t.Fatalf("expected activate reason metadata, got=%+v", activateMeta)
	}

	filteredByResourceRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/platform/cms/ops/audit?action=disclosure.type.version.activate&resource_type=disclosure_type&resource_id=dt-custom-obligation", adminToken, nil, "")
	if filteredByResourceRes.StatusCode != http.StatusOK {
		t.Fatalf("resource filtered audit status=%d body=%s", filteredByResourceRes.StatusCode, readBody(t, filteredByResourceRes.Body))
	}
	var filteredByResourceOut struct {
		Data struct {
			Items []map[string]any `json:"items"`
		} `json:"data"`
	}
	mustDecodeJSON(t, filteredByResourceRes.Body, &filteredByResourceOut)
	if len(filteredByResourceOut.Data.Items) == 0 {
		t.Fatalf("expected resource filtered audit items")
	}
	for _, item := range filteredByResourceOut.Data.Items {
		if item["action"] != "disclosure.type.version.activate" {
			t.Fatalf("unexpected action in resource filtered result: %+v", item)
		}
		if item["resource_type"] != "disclosure_type" || item["resource_id"] != "dt-custom-obligation" {
			t.Fatalf("unexpected resource filter match: %+v", item)
		}
	}

	timeFilteredRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/platform/cms/ops/audit?action=disclosure.type.version.activate&from=2000-01-01T00:00:00Z&to=2100-01-01T00:00:00Z&limit=1", adminToken, nil, "")
	if timeFilteredRes.StatusCode != http.StatusOK {
		t.Fatalf("time filtered audit status=%d body=%s", timeFilteredRes.StatusCode, readBody(t, timeFilteredRes.Body))
	}
	var timeFilteredOut struct {
		Data struct {
			Items []map[string]any `json:"items"`
		} `json:"data"`
		Meta map[string]any `json:"meta"`
	}
	mustDecodeJSON(t, timeFilteredRes.Body, &timeFilteredOut)
	if len(timeFilteredOut.Data.Items) == 0 {
		t.Fatalf("expected time filtered items")
	}
	if _, ok := timeFilteredOut.Meta["next_cursor"]; !ok {
		t.Fatalf("expected next_cursor in meta, got=%+v", timeFilteredOut.Meta)
	}
}

func TestIntegration_disclosureTypeDetail_deadlineSummaryFixedDateWarnOnlyTimezone(t *testing.T) {
	srv := httptest.NewServer(newTestHandler(t, nil))
	defer srv.Close()

	adminToken := loginAndGetAccessToken(t, srv.URL, "cms.operator@example.com", "secret", "c_001")

	upsertRes := doJSONRequest(t, http.MethodPut, srv.URL+"/api/v1/admin/disclosure-types/dt-deadline-summary-integration", adminToken, map[string]any{
		"group_id":          "group-006",
		"name":              "Template deadline summary integration",
		"category":          "Tùy chỉnh",
		"template_category": "custom",
		"deadline_strategy": "configurable",
		"description":       "deadline summary integration test",
		"deadline_rule":     "Theo cấu hình",
		"periodicity":       "ad_hoc",
		"channels_text":     "Website công ty",
		"format":            "PDF",
		"deadline_config": map[string]any{
			"deadline_mode": "FIXED_DATE",
			"fixed_deadline": map[string]any{
				"date":               "2026-01-03",
				"non_trading_policy": "WARN_ONLY_KEEP_DATE",
			},
		},
		"blocks": integrationMandatoryDisclosureBlocksMaps(),
	}, "")
	if upsertRes.StatusCode != http.StatusOK {
		t.Fatalf("upsert status=%d body=%s", upsertRes.StatusCode, readBody(t, upsertRes.Body))
	}

	detailRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/disclosure-types/dt-deadline-summary-integration", adminToken, nil, "")
	if detailRes.StatusCode != http.StatusOK {
		t.Fatalf("detail status=%d body=%s", detailRes.StatusCode, readBody(t, detailRes.Body))
	}
	var detailOut struct {
		DeadlineSummary struct {
			Timezone                     string `json:"timezone"`
			ActualDeadline               string `json:"actual_deadline"`
			NonTradingDayReason          string `json:"non_trading_day_reason"`
			AdjustedBecauseNonTradingDay *bool  `json:"adjusted_because_non_trading_day"`
		} `json:"deadline_summary"`
	}
	mustDecodeJSON(t, detailRes.Body, &detailOut)

	if detailOut.DeadlineSummary.Timezone != "Asia/Ho_Chi_Minh" {
		t.Fatalf("expected timezone Asia/Ho_Chi_Minh, got=%q", detailOut.DeadlineSummary.Timezone)
	}
	if detailOut.DeadlineSummary.ActualDeadline != "2026-01-03" {
		t.Fatalf("expected warn-only policy keeps fixed date 2026-01-03, got=%q", detailOut.DeadlineSummary.ActualDeadline)
	}
	if detailOut.DeadlineSummary.AdjustedBecauseNonTradingDay == nil {
		t.Fatalf("expected adjusted_because_non_trading_day to be present")
	}
	if *detailOut.DeadlineSummary.AdjustedBecauseNonTradingDay {
		t.Fatalf("expected adjusted_because_non_trading_day=false for warn-only policy")
	}
	if strings.TrimSpace(detailOut.DeadlineSummary.NonTradingDayReason) == "" {
		t.Fatalf("expected non_trading_day_reason for holiday fixed date")
	}
}

func TestIntegration_disclosureTypeDetail_deadlineSummaryFixedDateMoveNextWorkingDay(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	repoRoot := filepath.Clean(filepath.Join(wd, "..", ".."))
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("chdir repo root failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	srv := httptest.NewServer(newTestHandler(t, nil))
	defer srv.Close()

	adminToken := loginAndGetAccessToken(t, srv.URL, "cms.operator@example.com", "secret", "c_001")

	upsertRes := doJSONRequest(t, http.MethodPut, srv.URL+"/api/v1/admin/disclosure-types/dt-deadline-summary-move-next-working", adminToken, map[string]any{
		"group_id":          "group-006",
		"name":              "Template deadline summary move next working",
		"category":          "Tùy chỉnh",
		"template_category": "custom",
		"deadline_strategy": "configurable",
		"description":       "deadline summary integration test - move next working day",
		"deadline_rule":     "Theo cấu hình",
		"periodicity":       "ad_hoc",
		"channels_text":     "Website công ty",
		"format":            "PDF",
		"deadline_config": map[string]any{
			"deadline_mode": "FIXED_DATE",
			"fixed_deadline": map[string]any{
				"date":               "2026-01-03", // Saturday
				"non_trading_policy": "MOVE_TO_NEXT_WORKING_DAY",
			},
		},
		"blocks": integrationMandatoryDisclosureBlocksMaps(),
	}, "")
	if upsertRes.StatusCode != http.StatusOK {
		t.Fatalf("upsert status=%d body=%s", upsertRes.StatusCode, readBody(t, upsertRes.Body))
	}

	detailRes := doJSONRequest(t, http.MethodGet, srv.URL+"/api/v1/disclosure-types/dt-deadline-summary-move-next-working", adminToken, nil, "")
	if detailRes.StatusCode != http.StatusOK {
		t.Fatalf("detail status=%d body=%s", detailRes.StatusCode, readBody(t, detailRes.Body))
	}
	var detailOut struct {
		DeadlineSummary struct {
			Timezone                     string `json:"timezone"`
			ActualDeadline               string `json:"actual_deadline"`
			NonTradingDayReason          string `json:"non_trading_day_reason"`
			AdjustedBecauseNonTradingDay *bool  `json:"adjusted_because_non_trading_day"`
		} `json:"deadline_summary"`
	}
	mustDecodeJSON(t, detailRes.Body, &detailOut)

	if detailOut.DeadlineSummary.Timezone != "Asia/Ho_Chi_Minh" {
		t.Fatalf("expected timezone Asia/Ho_Chi_Minh, got=%q", detailOut.DeadlineSummary.Timezone)
	}
	if detailOut.DeadlineSummary.ActualDeadline != "2026-01-05" {
		t.Fatalf("expected MOVE_TO_NEXT_WORKING_DAY adjusts to Monday 2026-01-05, got=%q", detailOut.DeadlineSummary.ActualDeadline)
	}
	if detailOut.DeadlineSummary.AdjustedBecauseNonTradingDay == nil {
		t.Fatalf("expected adjusted_because_non_trading_day to be present")
	}
	if !*detailOut.DeadlineSummary.AdjustedBecauseNonTradingDay {
		t.Fatalf("expected adjusted_because_non_trading_day=true for move-next-working policy")
	}
	if strings.TrimSpace(detailOut.DeadlineSummary.NonTradingDayReason) == "" {
		t.Fatalf("expected non_trading_day_reason for weekend fixed date")
	}
}
