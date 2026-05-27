package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	auditappimpl "github.com/cobo/cobo_iam_services/internal/audit/appimpl"
	auditinmem "github.com/cobo/cobo_iam_services/internal/audit/infra/inmemory"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	iaminmem "github.com/cobo/cobo_iam_services/internal/iam/infra/inmemory"
	platformclock "github.com/cobo/cobo_iam_services/internal/platform/clock"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
	"github.com/cobo/cobo_iam_services/internal/platform/mediaupload"
)

type meFakeIdentity struct {
	userID string
}

func (meFakeIdentity) GetByUserID(_ context.Context, userID string) (*iamapp.AuthenticatedUser, error) {
	return &iamapp.AuthenticatedUser{
		UserID:           userID,
		LoginID:          "me@example.com",
		FullName:         "Me User",
		SubscriptionTier: "Free",
	}, nil
}

type meFakeMembers struct{}

func (meFakeMembers) GetMembershipsByUser(context.Context, string) ([]caapp.MembershipView, error) {
	return nil, nil
}
func (meFakeMembers) GetActiveMembership(context.Context, string, string) (*caapp.MembershipView, error) {
	return nil, nil
}
func (meFakeMembers) GetMembershipRoles(context.Context, string) ([]string, error) { return nil, nil }
func (meFakeMembers) GetMembershipDepartments(context.Context, string) ([]caapp.DepartmentView, error) {
	return nil, nil
}
func (meFakeMembers) GetMembershipTitles(context.Context, string) ([]string, error) { return nil, nil }

func newMeHandlerForGETMe(t *testing.T, auditRepo *auditinmem.Repository) (*MeHandler, *iamapp.AvatarService, *iaminmem.AvatarStorage) {
	t.Helper()
	avatarRepo := iaminmem.NewAvatarRepository()
	store := iaminmem.NewAvatarStorage()
	signer := mediaupload.NewSigner("me-test-secret", 15*time.Minute)
	svc := iamapp.NewAvatarService(iamapp.AvatarServiceConfig{
		Repo:         avatarRepo,
		Storage:      store,
		Signer:       signer,
		MaxBytes:     2 * 1024 * 1024,
		AllowedTypes: map[string]struct{}{"image/png": {}},
		UploadTTL:    15 * time.Minute,
		PublicBaseURL: "http://api.test",
	})
	auditSvc := auditappimpl.NewService(auditRepo, platformclock.System{}, idgen.UUIDv7Generator{})
	base := NewHandler(slog.Default(), nil, avatarFakeInspector{claims: &iamapp.AccessTokenClaims{Sub: "u_me"}}, auditSvc, nil, nil, nil)
	me := NewMeHandler(base, meFakeIdentity{}, meFakeMembers{}, nil, avatarProfileRepo{status: "active"}, nil, nil, nil, svc, "http://api.test")
	return me, svc, store
}

func decodeMeUser(t *testing.T, body string) map[string]any {
	t.Helper()
	var out struct {
		User map[string]any `json:"user"`
	}
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("decode: %v body=%s", err, body)
	}
	return out.User
}

func TestMeHandler_GET_me_noAvatar(t *testing.T) {
	me, _, _ := newMeHandlerForGETMe(t, auditinmem.NewRepository())
	mux := http.NewServeMux()
	me.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer test")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	user := decodeMeUser(t, rec.Body.String())
	if user["avatar_url"] != nil {
		t.Fatalf("avatar_url=%v want null", user["avatar_url"])
	}
	if user["avatar_updated_at"] != nil {
		t.Fatalf("avatar_updated_at=%v want null", user["avatar_updated_at"])
	}
	if _, ok := user["avatar_object_key"]; ok {
		t.Fatal("avatar_object_key must not be exposed")
	}
}

func TestMeHandler_GET_me_withAvatar_signedURL(t *testing.T) {
	me, svc, store := newMeHandlerForGETMe(t, auditinmem.NewRepository())
	mux := http.NewServeMux()
	me.Register(mux)
	ctx := context.Background()

	out, _ := svc.CreateUploadIntent(ctx, "u_me", "http://api.test", iamapp.AvatarUploadIntentRequest{
		FileName: "a.png", ContentType: "image/png", SizeBytes: 3,
	})
	asset, _ := svc.GetAssetForSignedUpload(ctx, "u_me", out.AssetID)
	store.WriteBytes(asset.ObjectKey, []byte("xyz"))
	_, _ = svc.Complete(ctx, "u_me", out.AssetID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer test")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	user := decodeMeUser(t, rec.Body.String())
	avatarURL, _ := user["avatar_url"].(string)
	if avatarURL == "" || !strings.HasPrefix(avatarURL, "http://api.test/api/v1/me/avatar/content") {
		t.Fatalf("avatar_url=%q", avatarURL)
	}
	if user["avatar_updated_at"] == nil {
		t.Fatal("expected avatar_updated_at")
	}
	contentReq := httptest.NewRequest(http.MethodGet, avatarURL, nil)
	contentRec := httptest.NewRecorder()
	mux.ServeHTTP(contentRec, contentReq)
	if contentRec.Code != http.StatusOK {
		t.Fatalf("content fetch status=%d", contentRec.Code)
	}
	if got := contentRec.Body.String(); got != "xyz" {
		t.Fatalf("content body=%q", got)
	}
}

func TestMeHandler_GET_me_afterDelete_clearsAvatar(t *testing.T) {
	me, svc, store := newMeHandlerForGETMe(t, auditinmem.NewRepository())
	mux := http.NewServeMux()
	me.Register(mux)
	ctx := context.Background()

	out, _ := svc.CreateUploadIntent(ctx, "u_me", "http://api.test", iamapp.AvatarUploadIntentRequest{
		FileName: "a.png", ContentType: "image/png", SizeBytes: 3,
	})
	asset, _ := svc.GetAssetForSignedUpload(ctx, "u_me", out.AssetID)
	store.WriteBytes(asset.ObjectKey, []byte("xyz"))
	_, _ = svc.Complete(ctx, "u_me", out.AssetID)
	_ = svc.Delete(ctx, "u_me")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer test")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	user := decodeMeUser(t, rec.Body.String())
	if user["avatar_url"] != nil || user["avatar_updated_at"] != nil {
		t.Fatalf("expected cleared avatar: %+v", user)
	}
}

func TestMeHandler_avatarAuditEvents(t *testing.T) {
	auditRepo := auditinmem.NewRepository()
	me, _, _ := newMeHandlerForGETMe(t, auditRepo)
	mux := http.NewServeMux()
	me.Register(mux)

	intentReq := httptest.NewRequest(http.MethodPost, "/api/v1/me/avatar/upload-intent", strings.NewReader(`{"file_name":"a.png","content_type":"image/png","size_bytes":3}`))
	intentReq.Header.Set("Authorization", "Bearer test")
	intentReq.Header.Set("Content-Type", "application/json")
	intentRec := httptest.NewRecorder()
	mux.ServeHTTP(intentRec, intentReq)
	var intentOut struct {
		AssetID   string `json:"asset_id"`
		UploadURL string `json:"upload_url"`
	}
	_ = json.NewDecoder(intentRec.Body).Decode(&intentOut)

	putReq, _ := http.NewRequest(http.MethodPut, intentOut.UploadURL, strings.NewReader("abc"))
	putReq.Header.Set("Content-Type", "image/png")
	putRec := httptest.NewRecorder()
	mux.ServeHTTP(putRec, putReq)

	completeReq := httptest.NewRequest(http.MethodPost, "/api/v1/me/avatar/"+intentOut.AssetID+"/complete", nil)
	completeReq.Header.Set("Authorization", "Bearer test")
	completeRec := httptest.NewRecorder()
	mux.ServeHTTP(completeRec, completeReq)

	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/me/avatar", nil)
	delReq.Header.Set("Authorization", "Bearer test")
	delRec := httptest.NewRecorder()
	mux.ServeHTTP(delRec, delReq)

	entries, err := auditRepo.ListByCompany(context.Background(), "", "", "", "", "", "", "", 100)
	if err != nil {
		t.Fatalf("list audit: %v", err)
	}
	actions := make(map[string]bool)
	for _, e := range entries {
		actions[e.Action] = true
		if e.Action == "user.avatar.upload.intent" {
			if e.Metadata["asset_id"] != intentOut.AssetID {
				t.Fatalf("intent metadata=%v", e.Metadata)
			}
			if _, ok := e.Metadata["upload_url"]; ok {
				t.Fatal("audit must not log signed upload_url")
			}
			if _, ok := e.Metadata["sig"]; ok {
				t.Fatal("audit must not log sig")
			}
		}
	}
	for _, want := range []string{"user.avatar.upload.intent", "user.avatar.upload.complete", "user.avatar.delete"} {
		if !actions[want] {
			t.Fatalf("missing audit action %q; got %v", want, actions)
		}
	}
}
