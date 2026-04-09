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
