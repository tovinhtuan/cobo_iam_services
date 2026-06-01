package http

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
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
		Repo:          repo,
		Storage:       store,
		Signer:        signer,
		MaxBytes:      2 * 1024 * 1024,
		AllowedTypes:  map[string]struct{}{"image/png": {}},
		UploadTTL:     15 * time.Minute,
		PublicBaseURL: "http://127.0.0.1",
	})
	base := NewHandler(slog.Default(), nil, avatarFakeInspector{claims: &iamapp.AccessTokenClaims{Sub: "u_me"}}, nil, nil, nil, nil)
	me := NewMeHandler(base, nil, nil, nil, profiles, nil, nil, nil, svc, "http://127.0.0.1")
	return me, svc, store
}

// buildMultipartBody constructs a multipart/form-data body with a single "avatar" file field.
// It sets the Content-Type on the file part so the handler can read it directly.
func buildMultipartBody(t *testing.T, filename, contentType string, data []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="avatar"; filename="%s"`, filename))
	h.Set("Content-Type", contentType)
	fw, err := w.CreatePart(h)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, w.FormDataContentType()
}

func TestMeHandler_avatarUploadMultipart(t *testing.T) {
	me, _, store := newAvatarMeHandler(t, avatarProfileRepo{status: "active"})
	mux := http.NewServeMux()
	me.Register(mux)

	body, ct := buildMultipartBody(t, "photo.png", "image/png", []byte("pngdata"))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/avatar", body)
	req.Header.Set("Authorization", "Bearer test")
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "avatar_url") {
		t.Fatalf("response missing avatar_url: %s", rec.Body.String())
	}
	// verify binary written to storage — object key format: users/{user_id}/avatar/{asset_id}.png
	found := false
	for _, key := range store.Keys() {
		if strings.HasPrefix(key, "users/u_me/avatar/") && strings.HasSuffix(key, ".png") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected avatar file in storage after multipart upload")
	}
}

func TestMeHandler_avatarContent_signedGET(t *testing.T) {
	me, svc, store := newAvatarMeHandler(t, avatarProfileRepo{status: "active"})
	mux := http.NewServeMux()
	me.Register(mux)

	ctx := context.Background()
	// Seed avatar via service directly (simulates a prior upload).
	payload := []byte("abc")
	_, err := svc.UploadMultipart(ctx, "u_me", "http://127.0.0.1", "image/png", bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		t.Fatal(err)
	}
	_ = store

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

func TestMeHandler_avatarUpload_lockedAccount(t *testing.T) {
	me, _, _ := newAvatarMeHandler(t, avatarProfileRepo{status: "locked"})
	mux := http.NewServeMux()
	me.Register(mux)

	body, ct := buildMultipartBody(t, "photo.png", "image/png", []byte("data"))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/avatar", body)
	req.Header.Set("Authorization", "Bearer test")
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("upload status=%d want 403", rec.Code)
	}

	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/me/avatar", nil)
	delReq.Header.Set("Authorization", "Bearer test")
	delRec := httptest.NewRecorder()
	mux.ServeHTTP(delRec, delReq)
	if delRec.Code != http.StatusForbidden {
		t.Fatalf("delete status=%d want 403", delRec.Code)
	}
}
