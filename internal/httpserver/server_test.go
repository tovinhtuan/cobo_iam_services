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
	"io"
	"net/http"
	"net/http/httptest"
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
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d want 404", res.StatusCode)
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
}

func TestIntegration_platformCMSPrefix_entriesReviewsSchedulesContract(t *testing.T) {
	srv := httptest.NewServer(newTestHandler(t, nil))
	defer srv.Close()

	cmsToken := loginAndGetAccessToken(t, srv.URL, "cms.operator@example.com", "secret", "")
	createRes := doJSONRequest(t, http.MethodPost, srv.URL+"/api/v1/platform/cms/entries", cmsToken, map[string]any{
		"type_id":      "DISCLOSURE_FINANCIAL",
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
		"type_id":      "DISCLOSURE_FINANCIAL",
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
}

func TestIntegration_platformCMSPrefix_adminUsersCreateAndList(t *testing.T) {
	srv := httptest.NewServer(newTestHandler(t, nil))
	defer srv.Close()

	adminToken := loginAndGetAccessToken(t, srv.URL, "cms.operator@example.com", "secret", "")
	createdLogin := "cms-admin-" + strings.ToLower(strings.ReplaceAll(uuid.NewString(), "-", ""))[0:12] + "@example.com"

	createRes := doJSONRequest(t, http.MethodPost, srv.URL+"/api/v1/platform/cms/admin/users", adminToken, map[string]any{
		"login_id":          createdLogin,
		"password":          "secret123",
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

func TestIntegration_disclosureC1_contractMatrix_happyPathAndErrors(t *testing.T) {
	srv := httptest.NewServer(newTestHandler(t, nil))
	defer srv.Close()

	userToken := loginAndGetAccessToken(t, srv.URL, "user@example.com", "secret", "c_001")

	createPayload := map[string]any{
		"type_id":       "DISCLOSURE_FINANCIAL",
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
	if created["type_id"] != "DISCLOSURE_FINANCIAL" || created["summary"] != "Q1 summary" || created["planned_date"] != "2026-05-01" {
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
		"type_id":       "DISCLOSURE_FINANCIAL",
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
	if submitted["status"] != "Published" {
		t.Fatalf("unexpected submit status: %+v", submitted)
	}
	if submitted["published_date"] == "" {
		t.Fatalf("published_date should be present after submit: %+v", submitted)
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
	if confirmByAdminRes.StatusCode != http.StatusOK {
		t.Fatalf("confirm by admin status=%d body=%s", confirmByAdminRes.StatusCode, readBody(t, confirmByAdminRes.Body))
	}
	var confirmed map[string]any
	mustDecodeJSON(t, confirmByAdminRes.Body, &confirmed)
	if confirmed["status"] != "Completed" {
		t.Fatalf("unexpected confirm status: %+v", confirmed)
	}

	confirmAgainRes := doJSONRequest(t, http.MethodPost, srv.URL+"/api/v1/disclosures/"+recordID+"/confirm", adminToken, nil, "idem-confirm-admin-2")
	if confirmAgainRes.StatusCode != http.StatusConflict {
		t.Fatalf("confirm again status=%d body=%s", confirmAgainRes.StatusCode, readBody(t, confirmAgainRes.Body))
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
