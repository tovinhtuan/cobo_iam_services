package app_test

import (
	"bytes"
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	iaminmem "github.com/cobo/cobo_iam_services/internal/iam/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/mediaupload"
)

func newTestAvatarService(t *testing.T) (*iamapp.AvatarService, *iaminmem.AvatarRepository, *iaminmem.AvatarStorage) {
	t.Helper()
	repo := iaminmem.NewAvatarRepository()
	store := iaminmem.NewAvatarStorage()
	signer := mediaupload.NewSigner("test-avatar-secret", 15*time.Minute)
	svc := iamapp.NewAvatarService(iamapp.AvatarServiceConfig{
		Repo:    repo,
		Storage: store,
		Signer:  signer,
		MaxBytes: 2 * 1024 * 1024,
		AllowedTypes: map[string]struct{}{
			"image/png":  {},
			"image/jpeg": {},
			"image/webp": {},
		},
		UploadTTL:     15 * time.Minute,
		PublicBaseURL: "http://api.test",
	})
	return svc, repo, store
}

func TestAvatarService_UploadMultipart_valid(t *testing.T) {
	ctx := context.Background()
	svc, repo, store := newTestAvatarService(t)

	payload := []byte("pngdata")
	out, err := svc.UploadMultipart(ctx, "u_1", "http://api.test", "image/png", bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		t.Fatalf("UploadMultipart: %v", err)
	}
	if out.AvatarURL == "" {
		t.Fatal("empty AvatarURL")
	}
	if !strings.Contains(out.AvatarURL, "/api/v1/me/avatar/content") {
		t.Fatalf("avatar_url path: %s", out.AvatarURL)
	}
	if out.AvatarUpdatedAt.IsZero() {
		t.Fatal("zero AvatarUpdatedAt")
	}
	// Verify users table updated.
	meta, err := repo.GetUserAvatar(ctx, "u_1")
	if err != nil {
		t.Fatalf("GetUserAvatar: %v", err)
	}
	if meta.ObjectKey == "" || meta.ContentType != "image/png" {
		t.Fatalf("meta=%+v", meta)
	}
	// Verify binary stored.
	if !store.Exists(meta.ObjectKey) {
		t.Fatal("binary not in storage")
	}
}

func TestAvatarService_UploadMultipart_invalidType(t *testing.T) {
	svc, _, _ := newTestAvatarService(t)
	_, err := svc.UploadMultipart(context.Background(), "u_1", "http://api.test", "application/pdf", bytes.NewReader([]byte("data")), 4)
	if err == nil {
		t.Fatal("expected error for invalid content type")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != 400 {
		t.Fatalf("err=%v", err)
	}
}

func TestAvatarService_UploadMultipart_tooLarge(t *testing.T) {
	svc, _, _ := newTestAvatarService(t)
	_, err := svc.UploadMultipart(context.Background(), "u_1", "http://api.test", "image/png", bytes.NewReader([]byte("data")), 3*1024*1024)
	if err == nil {
		t.Fatal("expected error for oversized file")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != 413 {
		t.Fatalf("err=%v", err)
	}
}

func TestAvatarService_UploadMultipart_replacesOldAvatar(t *testing.T) {
	ctx := context.Background()
	svc, repo, store := newTestAvatarService(t)

	p1 := []byte("first")
	_, err := svc.UploadMultipart(ctx, "u_1", "http://api.test", "image/png", bytes.NewReader(p1), int64(len(p1)))
	if err != nil {
		t.Fatalf("first upload: %v", err)
	}
	meta1, _ := repo.GetUserAvatar(ctx, "u_1")
	oldKey := meta1.ObjectKey

	p2 := []byte("second")
	_, err = svc.UploadMultipart(ctx, "u_1", "http://api.test", "image/jpeg", bytes.NewReader(p2), int64(len(p2)))
	if err != nil {
		t.Fatalf("second upload: %v", err)
	}
	meta2, _ := repo.GetUserAvatar(ctx, "u_1")
	if meta2.ObjectKey == oldKey {
		t.Fatal("object key should change after replace")
	}
	if meta2.ContentType != "image/jpeg" {
		t.Fatalf("content type=%q", meta2.ContentType)
	}
	// Old binary deleted.
	if store.Exists(oldKey) {
		t.Fatal("old avatar binary should be deleted after replace")
	}
}

func TestAvatarService_Delete_clearsAvatar(t *testing.T) {
	ctx := context.Background()
	svc, repo, store := newTestAvatarService(t)

	payload := []byte("png")
	_, err := svc.UploadMultipart(ctx, "u_1", "http://api.test", "image/png", bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	meta, _ := repo.GetUserAvatar(ctx, "u_1")
	key := meta.ObjectKey

	if err := svc.Delete(ctx, "u_1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	cleared, _ := repo.GetUserAvatar(ctx, "u_1")
	if cleared.ObjectKey != "" {
		t.Fatalf("expected cleared avatar, got %+v", cleared)
	}
	if store.Exists(key) {
		t.Fatal("binary should be deleted")
	}
}

func TestAvatarService_MeAvatarFields(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newTestAvatarService(t)

	avatarURL, updated, err := svc.MeAvatarFields(ctx, "u_1", "http://api.test")
	if err != nil {
		t.Fatalf("MeAvatarFields: %v", err)
	}
	if avatarURL != nil || updated != nil {
		t.Fatalf("expected null before upload, got url=%v updated=%v", avatarURL, updated)
	}

	payload := []byte("abc")
	_, err = svc.UploadMultipart(ctx, "u_1", "http://api.test", "image/png", bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		t.Fatalf("upload: %v", err)
	}

	avatarURL, updated, err = svc.MeAvatarFields(ctx, "u_1", "http://api.test")
	if err != nil {
		t.Fatalf("MeAvatarFields after upload: %v", err)
	}
	urlStr, ok := avatarURL.(string)
	if !ok || urlStr == "" {
		t.Fatalf("avatar_url=%v", avatarURL)
	}
	if !strings.Contains(urlStr, "/api/v1/me/avatar/content") {
		t.Fatalf("avatar_url path: %s", urlStr)
	}
	if !strings.Contains(urlStr, "sig=") {
		t.Fatalf("avatar_url missing sig: %s", urlStr)
	}
	if strings.Contains(urlStr, "users/") {
		t.Fatalf("avatar_url must not expose object_key: %s", urlStr)
	}
	updatedStr, ok := updated.(string)
	if !ok || updatedStr == "" {
		t.Fatalf("avatar_updated_at=%v", updated)
	}
}

func TestAvatarService_GetContentBytes(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newTestAvatarService(t)

	payload := []byte("abc")
	_, err := svc.UploadMultipart(ctx, "u_1", "http://api.test", "image/png", bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		t.Fatalf("upload: %v", err)
	}

	data, ct, err := svc.GetContentBytes(ctx, "u_1")
	if err != nil {
		t.Fatalf("GetContentBytes: %v", err)
	}
	if string(data) != "abc" || ct != "image/png" {
		t.Fatalf("data=%q ct=%q", data, ct)
	}

	exp := time.Now().UTC().Add(5 * time.Minute)
	contentURL := svc.BuildSignedContentURL("u_1", "http://api.test", exp)
	if !strings.Contains(contentURL, "sig=") {
		t.Fatalf("content url: %s", contentURL)
	}
	u, _ := url.Parse(contentURL)
	signIn := svc.BuildContentSignInput("u_1", exp)
	if !svc.VerifySignature(u.Query().Get("sig"), signIn) {
		t.Fatal("content signature should verify")
	}
}

func TestAvatarService_VerifySignature_contentURL(t *testing.T) {
	svc, _, _ := newTestAvatarService(t)
	exp := time.Now().UTC().Add(5 * time.Minute)
	signIn := svc.BuildContentSignInput("u_1", exp)

	// Sign it.
	contentURL := svc.BuildSignedContentURL("u_1", "http://api.test", exp)
	u, _ := url.Parse(contentURL)
	sig := u.Query().Get("sig")

	if !svc.VerifySignature(sig, signIn) {
		t.Fatal("valid signature should verify")
	}
	// Wrong userID should fail.
	wrongIn := svc.BuildContentSignInput("u_2", exp)
	if svc.VerifySignature(sig, wrongIn) {
		t.Fatal("wrong user should not verify")
	}
	// Expired should fail.
	expiredIn := svc.BuildContentSignInput("u_1", time.Now().UTC().Add(-time.Hour))
	if svc.VerifySignature(sig, expiredIn) {
		t.Fatal("expired signature should not verify")
	}
}
