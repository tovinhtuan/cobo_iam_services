package app

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/mediaupload"
	"github.com/google/uuid"
)

// AvatarUploadMultipartResult is returned after a successful multipart upload.
type AvatarUploadMultipartResult struct {
	AvatarURL       string
	AvatarUpdatedAt time.Time
}

// AvatarServiceConfig wires avatar upload dependencies.
type AvatarServiceConfig struct {
	Repo          AvatarRepository
	Storage       AvatarObjectStorage
	Signer        *mediaupload.Signer
	MaxBytes      int64
	AllowedTypes  map[string]struct{}
	UploadTTL     time.Duration
	PublicBaseURL string
}

// AvatarService implements self-service user avatar upload flows.
type AvatarService struct {
	repo         AvatarRepository
	storage      AvatarObjectStorage
	signer       *mediaupload.Signer
	maxBytes     int64
	allowedTypes map[string]struct{}
	uploadTTL    time.Duration
	publicBase   string
}

// NewAvatarService creates an avatar service.
func NewAvatarService(cfg AvatarServiceConfig) *AvatarService {
	ttl := cfg.UploadTTL
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	return &AvatarService{
		repo:         cfg.Repo,
		storage:      cfg.Storage,
		signer:       cfg.Signer,
		maxBytes:     cfg.MaxBytes,
		allowedTypes: cfg.AllowedTypes,
		uploadTTL:    ttl,
		publicBase:   strings.TrimRight(strings.TrimSpace(cfg.PublicBaseURL), "/"),
	}
}

// UploadMultipart validates, writes binary to storage, and updates the user's active avatar in one step.
// baseURL is the public API origin (no trailing slash), used to build the signed content URL returned in the result.
func (s *AvatarService) UploadMultipart(ctx context.Context, userID, baseURL, contentType string, body io.Reader, sizeBytes int64) (*AvatarUploadMultipartResult, error) {
	if s == nil || s.repo == nil || s.storage == nil || s.signer == nil {
		return nil, perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeInternal, "avatar upload not configured", nil)
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "user required", nil)
	}
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if err := s.validateContentType(contentType); err != nil {
		return nil, err
	}
	if err := s.validateSize(sizeBytes); err != nil {
		return nil, err
	}

	assetID := "asset_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	ext := extFromContentType(contentType)
	objectKey := mediaupload.AvatarObjectKey(userID, assetID, ext)
	if err := validateAvatarObjectKey(objectKey, userID); err != nil {
		return nil, err
	}

	prev, _ := s.repo.GetUserAvatar(ctx, userID)

	written, err := s.storage.Write(objectKey, io.LimitReader(body, sizeBytes+1))
	if err != nil {
		return nil, perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "failed to persist avatar", err)
	}
	if written > sizeBytes {
		_ = s.storage.Delete(objectKey)
		return nil, perr.NewHTTPError(http.StatusRequestEntityTooLarge, perr.CodeInvalidRequest, "file too large", nil)
	}

	if err := s.repo.SetUserAvatar(ctx, userID, objectKey, contentType); err != nil {
		_ = s.storage.Delete(objectKey)
		return nil, err
	}
	if prev != nil && prev.ObjectKey != "" && prev.ObjectKey != objectKey {
		_ = s.storage.Delete(prev.ObjectKey)
	}

	now := time.Now().UTC()
	exp := now.Add(s.uploadTTL)
	avatarURL := s.BuildSignedContentURL(userID, baseURL, exp)
	return &AvatarUploadMultipartResult{
		AvatarURL:       avatarURL,
		AvatarUpdatedAt: now,
	}, nil
}

// VerifySignature checks the HMAC for a signed content URL.
func (s *AvatarService) VerifySignature(sig string, in mediaupload.SignInput) bool {
	if s == nil || s.signer == nil {
		return false
	}
	if !mediaupload.VerifyNotExpired(in, time.Now().UTC()) {
		return false
	}
	return s.signer.Verify(sig, in)
}

// Delete removes the user's avatar (idempotent).
func (s *AvatarService) Delete(ctx context.Context, userID string) error {
	if s == nil || s.repo == nil {
		return perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeInternal, "avatar upload not configured", nil)
	}
	userID = strings.TrimSpace(userID)
	meta, err := s.repo.GetUserAvatar(ctx, userID)
	if err != nil {
		return err
	}
	_ = s.repo.MarkUserReadyAssetsDeleted(ctx, userID, "")
	if err := s.repo.ClearUserAvatar(ctx, userID); err != nil {
		return err
	}
	if meta != nil && meta.ObjectKey != "" && s.storage != nil {
		_ = s.storage.Delete(meta.ObjectKey)
	}
	return nil
}

// GetContentBytes returns the active avatar bytes for a user.
func (s *AvatarService) GetContentBytes(ctx context.Context, userID string) ([]byte, string, error) {
	if s == nil || s.repo == nil || s.storage == nil {
		return nil, "", perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeInternal, "avatar content not configured", nil)
	}
	meta, err := s.repo.GetUserAvatar(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	if meta == nil || strings.TrimSpace(meta.ObjectKey) == "" {
		return nil, "", perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "avatar not found", nil)
	}
	if err := validateAvatarObjectKey(meta.ObjectKey, userID); err != nil {
		return nil, "", err
	}
	data, err := s.storage.Read(meta.ObjectKey)
	if err != nil {
		return nil, "", perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "avatar not found", err)
	}
	ct := strings.TrimSpace(meta.ContentType)
	if ct == "" {
		ct = "application/octet-stream"
	}
	return data, ct, nil
}

// BuildContentSignInput builds signature input for GET /me/avatar/content.
func (s *AvatarService) BuildContentSignInput(userID string, exp time.Time) mediaupload.SignInput {
	return mediaupload.SignInput{
		Purpose:   mediaupload.PurposeUserAvatarContent,
		OwnerID:   strings.TrimSpace(userID),
		Method:    http.MethodGet,
		Path:      "/api/v1/me/avatar/content",
		ExpiresAt: exp.UTC(),
	}
}

// MeAvatarFields returns avatar_url and avatar_updated_at for GET /me (no object_key).
func (s *AvatarService) MeAvatarFields(ctx context.Context, userID, baseURL string) (avatarURL, avatarUpdatedAt any, err error) {
	if s == nil || s.repo == nil || s.signer == nil {
		return nil, nil, nil
	}
	meta, err := s.repo.GetUserAvatar(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	if meta == nil || strings.TrimSpace(meta.ObjectKey) == "" || meta.UpdatedAt == nil {
		return nil, nil, nil
	}
	exp := time.Now().UTC().Add(s.uploadTTL)
	url := s.BuildSignedContentURL(userID, baseURL, exp)
	if url == "" {
		return nil, nil, nil
	}
	return url, meta.UpdatedAt.UTC().Format(time.RFC3339), nil
}

// BuildSignedContentURL returns an absolute signed GET URL for avatar content.
func (s *AvatarService) BuildSignedContentURL(userID, baseURL string, exp time.Time) string {
	if s == nil || s.signer == nil {
		return ""
	}
	in := s.BuildContentSignInput(userID, exp)
	sig := s.signer.Sign(in)
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = s.publicBase
	}
	return s.buildSignedURL(base, in.Path, map[string]string{
		"exp":     fmt.Sprintf("%d", exp.Unix()),
		"sig":     sig,
		"user_id": strings.TrimSpace(userID),
		"method":  http.MethodGet,
	})
}

func (s *AvatarService) validateContentType(contentType string) error {
	if len(s.allowedTypes) == 0 {
		return nil
	}
	if _, ok := s.allowedTypes[contentType]; ok {
		return nil
	}
	return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "unsupported content_type", nil)
}

func (s *AvatarService) validateSize(size int64) error {
	if size <= 0 {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "size_bytes must be positive", nil)
	}
	if size > s.maxBytes {
		return perr.NewHTTPError(http.StatusRequestEntityTooLarge, perr.CodeInvalidRequest, "file too large", nil)
	}
	return nil
}

func (s *AvatarService) buildSignedURL(baseURL, path string, query map[string]string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = "http://localhost:8080"
	}
	u, _ := url.Parse(base + path)
	q := u.Query()
	for k, v := range query {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func extFromContentType(contentType string) string {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/png":
		return "png"
	case "image/jpeg":
		return "jpg"
	case "image/webp":
		return "webp"
	default:
		return "bin"
	}
}

func validateAvatarObjectKey(objectKey, userID string) error {
	key := strings.TrimSpace(objectKey)
	if key == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid avatar object_key", nil)
	}
	if strings.Contains(key, "..") {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid avatar object_key", nil)
	}
	prefix := fmt.Sprintf("users/%s/avatar/", strings.TrimSpace(userID))
	if !strings.HasPrefix(key, prefix) {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid avatar object_key", nil)
	}
	return nil
}
