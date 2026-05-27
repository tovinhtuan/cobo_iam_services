package app_test

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"strconv"
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
		Repo: repo,
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

func TestAvatarService_CreateUploadIntent_valid(t *testing.T) {
	ctx := context.Background()
	svc, repo, _ := newTestAvatarService(t)

	out, err := svc.CreateUploadIntent(ctx, "u_1", "http://api.test", iamapp.AvatarUploadIntentRequest{
		FileName:    "avatar.png",
		ContentType: "image/png",
		SizeBytes:   1024,
	})
	if err != nil {
		t.Fatalf("CreateUploadIntent: %v", err)
	}
	if out.AssetID == "" || out.UploadURL == "" {
		t.Fatalf("missing asset_id or upload_url: %+v", out)
	}
	asset, err := repo.GetAssetByUser(ctx, "u_1", out.AssetID)
	if err != nil {
		t.Fatalf("GetAssetByUser: %v", err)
	}
	if asset.State != "pending_upload" {
		t.Fatalf("state=%q", asset.State)
	}
	if !strings.Contains(out.UploadURL, out.AssetID) {
		t.Fatalf("upload url missing asset id: %s", out.UploadURL)
	}
}

func TestAvatarService_CreateUploadIntent_invalidType(t *testing.T) {
	svc, _, _ := newTestAvatarService(t)
	_, err := svc.CreateUploadIntent(context.Background(), "u_1", "http://api.test", iamapp.AvatarUploadIntentRequest{
		FileName:    "x.pdf",
		ContentType: "application/pdf",
		SizeBytes:   100,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("err=%v", err)
	}
}

func TestAvatarService_CreateUploadIntent_tooLarge(t *testing.T) {
	svc, _, _ := newTestAvatarService(t)
	_, err := svc.CreateUploadIntent(context.Background(), "u_1", "http://api.test", iamapp.AvatarUploadIntentRequest{
		FileName:    "big.png",
		ContentType: "image/png",
		SizeBytes:   3 * 1024 * 1024,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusRequestEntityTooLarge {
		t.Fatalf("err=%v", err)
	}
}

func TestAvatarService_Complete_updatesUserAvatar(t *testing.T) {
	ctx := context.Background()
	svc, repo, store := newTestAvatarService(t)

	out, err := svc.CreateUploadIntent(ctx, "u_1", "http://api.test", iamapp.AvatarUploadIntentRequest{
		FileName:    "avatar.png",
		ContentType: "image/png",
		SizeBytes:   5,
	})
	if err != nil {
		t.Fatalf("intent: %v", err)
	}
	asset, _ := repo.GetAssetByUser(ctx, "u_1", out.AssetID)
	store.WriteBytes(asset.ObjectKey, []byte("12345"))

	done, err := svc.Complete(ctx, "u_1", out.AssetID)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if done.State != "ready" {
		t.Fatalf("state=%q", done.State)
	}
	meta, err := repo.GetUserAvatar(ctx, "u_1")
	if err != nil {
		t.Fatalf("GetUserAvatar: %v", err)
	}
	if meta.ObjectKey != asset.ObjectKey || meta.ContentType != "image/png" {
		t.Fatalf("meta=%+v", meta)
	}
}

func TestAvatarService_Delete_clearsAvatar(t *testing.T) {
	ctx := context.Background()
	svc, repo, store := newTestAvatarService(t)

	out, _ := svc.CreateUploadIntent(ctx, "u_1", "http://api.test", iamapp.AvatarUploadIntentRequest{
		FileName: "a.png", ContentType: "image/png", SizeBytes: 3,
	})
	asset, _ := repo.GetAssetByUser(ctx, "u_1", out.AssetID)
	store.WriteBytes(asset.ObjectKey, []byte("png"))
	_, _ = svc.Complete(ctx, "u_1", out.AssetID)

	if err := svc.Delete(ctx, "u_1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	meta, _ := repo.GetUserAvatar(ctx, "u_1")
	if meta.ObjectKey != "" {
		t.Fatalf("expected cleared avatar, got %+v", meta)
	}
}

func TestAvatarService_wrongUserRejected(t *testing.T) {
	ctx := context.Background()
	svc, repo, store := newTestAvatarService(t)

	out, _ := svc.CreateUploadIntent(ctx, "u_1", "http://api.test", iamapp.AvatarUploadIntentRequest{
		FileName: "a.png", ContentType: "image/png", SizeBytes: 3,
	})
	asset, _ := repo.GetAssetByUser(ctx, "u_1", out.AssetID)
	store.WriteBytes(asset.ObjectKey, []byte("png"))

	_, err := svc.Complete(ctx, "u_2", out.AssetID)
	if err == nil {
		t.Fatal("expected forbidden")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusForbidden {
		t.Fatalf("err=%v", err)
	}
}

func TestAvatarService_expiredAssetRejected(t *testing.T) {
	ctx := context.Background()
	svc, repo, store := newTestAvatarService(t)

	out, _ := svc.CreateUploadIntent(ctx, "u_1", "http://api.test", iamapp.AvatarUploadIntentRequest{
		FileName: "a.png", ContentType: "image/png", SizeBytes: 3,
	})
	asset, _ := repo.GetAssetByUser(ctx, "u_1", out.AssetID)
	asset.ExpiresAt = time.Now().UTC().Add(-time.Hour)
	repo.CreatePendingAsset(ctx, *asset)
	store.WriteBytes(asset.ObjectKey, []byte("png"))

	_, err := svc.Complete(ctx, "u_1", out.AssetID)
	if err == nil {
		t.Fatal("expected expired error")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusUnauthorized {
		t.Fatalf("err=%v", err)
	}
}

func TestAvatarService_signedUploadRoundTrip(t *testing.T) {
	ctx := context.Background()
	svc, repo, store := newTestAvatarService(t)

	out, err := svc.CreateUploadIntent(ctx, "u_1", "http://api.test", iamapp.AvatarUploadIntentRequest{
		FileName: "a.png", ContentType: "image/png", SizeBytes: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(out.UploadURL)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	expUnix := q.Get("exp")
	sig := q.Get("sig")
	userID := q.Get("user_id")
	contentType := q.Get("content_type")
	sizeBytes := int64(4)

	signIn := mediaupload.SignInput{
		Purpose:     mediaupload.PurposeUserAvatarUpload,
		OwnerID:     userID,
		AssetID:     out.AssetID,
		Method:      http.MethodPut,
		Path:        "/api/v1/me/avatar/upload/" + url.PathEscape(out.AssetID),
		ContentType: contentType,
		SizeBytes:   sizeBytes,
	}
	var expTime time.Time
	if ts, err := parseUnix(expUnix); err == nil {
		expTime = ts
		signIn.ExpiresAt = expTime
	}
	if !svc.VerifySignature(sig, signIn) {
		t.Fatal("signature should verify")
	}
	bad := signIn
	bad.Method = http.MethodGet
	if svc.VerifySignature(sig, bad) {
		t.Fatal("wrong method should fail")
	}

	asset, err := svc.GetAssetForSignedUpload(ctx, userID, out.AssetID)
	if err != nil {
		t.Fatalf("GetAssetForSignedUpload: %v", err)
	}
	body := bytes.NewReader([]byte("data"))
	if err := svc.WriteSignedUpload(ctx, asset, contentType, sizeBytes, body); err != nil {
		t.Fatalf("WriteSignedUpload: %v", err)
	}
	if !store.Exists(asset.ObjectKey) {
		t.Fatal("file not stored")
	}
	_ = repo
}

func TestAvatarService_MeAvatarFields(t *testing.T) {
	ctx := context.Background()
	svc, _, store := newTestAvatarService(t)

	url, updated, err := svc.MeAvatarFields(ctx, "u_1", "http://api.test")
	if err != nil {
		t.Fatalf("MeAvatarFields: %v", err)
	}
	if url != nil || updated != nil {
		t.Fatalf("expected null avatar fields, got url=%v updated=%v", url, updated)
	}

	out, _ := svc.CreateUploadIntent(ctx, "u_1", "http://api.test", iamapp.AvatarUploadIntentRequest{
		FileName: "a.png", ContentType: "image/png", SizeBytes: 3,
	})
	asset, _ := svc.GetAssetForSignedUpload(ctx, "u_1", out.AssetID)
	store.WriteBytes(asset.ObjectKey, []byte("abc"))
	_, _ = svc.Complete(ctx, "u_1", out.AssetID)

	url, updated, err = svc.MeAvatarFields(ctx, "u_1", "http://api.test")
	if err != nil {
		t.Fatalf("MeAvatarFields after complete: %v", err)
	}
	urlStr, ok := url.(string)
	if !ok || urlStr == "" {
		t.Fatalf("avatar_url=%v", url)
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
	svc, _, store := newTestAvatarService(t)

	out, _ := svc.CreateUploadIntent(ctx, "u_1", "http://api.test", iamapp.AvatarUploadIntentRequest{
		FileName: "a.png", ContentType: "image/png", SizeBytes: 3,
	})
	asset, _ := svc.GetAssetForSignedUpload(ctx, "u_1", out.AssetID)
	store.WriteBytes(asset.ObjectKey, []byte("abc"))
	_, _ = svc.Complete(ctx, "u_1", out.AssetID)

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

func parseUnix(s string) (time.Time, error) {
	sec, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(sec, 0).UTC(), nil
}
