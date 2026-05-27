package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	iaminmem "github.com/cobo/cobo_iam_services/internal/iam/infra/inmemory"
	"github.com/cobo/cobo_iam_services/internal/platform/mediaupload"
)

type avatarFakeInspector struct {
	claims *iamapp.AccessTokenClaims
}

func (a avatarFakeInspector) InspectAccessToken(context.Context, string) (*iamapp.AccessTokenClaims, error) {
	return a.claims, nil
}

func (avatarFakeInspector) InspectPreCompanyToken(context.Context, string) (*iamapp.PreCompanyTokenClaims, error) {
	return nil, nil
}

type avatarProfileRepo struct {
	status string
}

func (r avatarProfileRepo) GetAdminAccountSettings(context.Context, string) (*caapp.AdminAccountSettingsView, error) {
	return &caapp.AdminAccountSettingsView{AccountStatus: r.status}, nil
}

func (avatarProfileRepo) PatchAdminAccountSettings(context.Context, string, *string, *string, *string) error {
	return nil
}

func newAvatarMeHandler(t *testing.T, profiles avatarProfileRepo) (*MeHandler, *iamapp.AvatarService, *iaminmem.AvatarStorage) {
	t.Helper()
	repo := iaminmem.NewAvatarRepository()
	store := iaminmem.NewAvatarStorage()
	signer := mediaupload.NewSigner("handler-test-secret", 15*time.Minute)
	svc := iamapp.NewAvatarService(iamapp.AvatarServiceConfig{
		Repo:         repo,
		Storage:      store,
		Signer:       signer,
		MaxBytes:     2 * 1024 * 1024,
		AllowedTypes: map[string]struct{}{"image/png": {}},
		UploadTTL:    15 * time.Minute,
		PublicBaseURL: "http://127.0.0.1",
	})
	base := NewHandler(slog.Default(), nil, avatarFakeInspector{claims: &iamapp.AccessTokenClaims{Sub: "u_me"}}, nil, nil, nil, nil)
	me := NewMeHandler(base, nil, nil, nil, profiles, nil, nil, nil, svc, "http://127.0.0.1")
	return me, svc, store
}

func TestMeHandler_avatarSignedPUT(t *testing.T) {
	me, _, store := newAvatarMeHandler(t, avatarProfileRepo{status: "active"})
	mux := http.NewServeMux()
	me.Register(mux)

	intentBody := `{"file_name":"a.png","content_type":"image/png","size_bytes":4}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/avatar/upload-intent", strings.NewReader(intentBody))
	req.Header.Set("Authorization", "Bearer test")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("intent status=%d body=%s", rec.Code, rec.Body.String())
	}
	var intentOut struct {
		AssetID   string `json:"asset_id"`
		UploadURL string `json:"upload_url"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&intentOut); err != nil {
		t.Fatal(err)
	}
	putReq, err := http.NewRequest(http.MethodPut, intentOut.UploadURL, bytes.NewReader([]byte("data")))
	if err != nil {
		t.Fatal(err)
	}
	putReq.Header.Set("Content-Type", "image/png")
	putRec := httptest.NewRecorder()
	mux.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("put status=%d body=%s", putRec.Code, putRec.Body.String())
	}
	asset, _ := url.Parse(intentOut.UploadURL)
	_ = asset
	// verify stored via complete path in follow-up
	completeReq := httptest.NewRequest(http.MethodPost, "/api/v1/me/avatar/"+intentOut.AssetID+"/complete", nil)
	completeReq.Header.Set("Authorization", "Bearer test")
	completeRec := httptest.NewRecorder()
	mux.ServeHTTP(completeRec, completeReq)
	if completeRec.Code != http.StatusOK {
		t.Fatalf("complete status=%d body=%s", completeRec.Code, completeRec.Body.String())
	}
	if !store.Exists("users/u_me/avatar/" + intentOut.AssetID + ".png") {
		t.Fatal("expected avatar file on disk store")
	}
}

func TestMeHandler_avatarSignedPUT_invalidSignature(t *testing.T) {
	me, _, _ := newAvatarMeHandler(t, avatarProfileRepo{status: "active"})
	mux := http.NewServeMux()
	me.Register(mux)

	intentBody := `{"file_name":"a.png","content_type":"image/png","size_bytes":4}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/avatar/upload-intent", strings.NewReader(intentBody))
	req.Header.Set("Authorization", "Bearer test")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var intentOut struct {
		UploadURL string `json:"upload_url"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&intentOut)

	u, _ := url.Parse(intentOut.UploadURL)
	q := u.Query()
	q.Set("sig", "bad-signature")
	u.RawQuery = q.Encode()
	putReq := httptest.NewRequest(http.MethodPut, u.String(), bytes.NewReader([]byte("data")))
	putReq.Header.Set("Content-Type", "image/png")
	putRec := httptest.NewRecorder()
	mux.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", putRec.Code)
	}
}

func TestMeHandler_avatarContent_signedGET(t *testing.T) {
	me, svc, store := newAvatarMeHandler(t, avatarProfileRepo{status: "active"})
	mux := http.NewServeMux()
	me.Register(mux)

	ctx := context.Background()
	out, _ := svc.CreateUploadIntent(ctx, "u_me", "http://127.0.0.1", iamapp.AvatarUploadIntentRequest{
		FileName: "a.png", ContentType: "image/png", SizeBytes: 3,
	})
	asset, _ := svc.GetAssetForSignedUpload(ctx, "u_me", out.AssetID)
	store.WriteBytes(asset.ObjectKey, []byte("abc"))
	_, _ = svc.Complete(ctx, "u_me", out.AssetID)

	exp := time.Now().UTC().Add(5 * time.Minute)
	contentURL := svc.BuildSignedContentURL("u_me", "http://127.0.0.1", exp)
	getReq := httptest.NewRequest(http.MethodGet, contentURL, nil)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("content status=%d body=%s", getRec.Code, getRec.Body.String())
	}
	if ct := getRec.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("content-type=%q", ct)
	}
	body, _ := io.ReadAll(getRec.Body)
	if string(body) != "abc" {
		t.Fatalf("body=%q", body)
	}
}

func TestMeHandler_avatarIntent_lockedAccount(t *testing.T) {
	me, _, _ := newAvatarMeHandler(t, avatarProfileRepo{status: "locked"})
	mux := http.NewServeMux()
	me.Register(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/avatar/upload-intent", strings.NewReader(`{"file_name":"a.png","content_type":"image/png","size_bytes":4}`))
	req.Header.Set("Authorization", "Bearer test")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("intent status=%d want 403", rec.Code)
	}

	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/me/avatar", nil)
	delReq.Header.Set("Authorization", "Bearer test")
	delRec := httptest.NewRecorder()
	mux.ServeHTTP(delRec, delReq)
	if delRec.Code != http.StatusForbidden {
		t.Fatalf("delete status=%d want 403", delRec.Code)
	}
}
