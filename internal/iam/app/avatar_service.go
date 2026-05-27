package app

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/mediaupload"
	"github.com/google/uuid"
)

// AvatarUploadIntentRequest is the body for POST /me/avatar/upload-intent.
type AvatarUploadIntentRequest struct {
	FileName    string
	ContentType string
	SizeBytes   int64
}

// AvatarUploadIntentResult is returned after a successful upload intent.
type AvatarUploadIntentResult struct {
	AssetID   string
	UploadURL string
	ExpiresAt time.Time
}

// AvatarCompleteResult is returned after finalize.
type AvatarCompleteResult struct {
	AssetID     string
	ObjectKey   string
	ContentType string
	SizeBytes   int64
	State       string
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

// CreateUploadIntent validates input, creates a pending asset, and returns a signed PUT URL.
// baseURL is the public API origin (no trailing slash), used to build upload_url.
func (s *AvatarService) CreateUploadIntent(ctx context.Context, userID, baseURL string, req AvatarUploadIntentRequest) (*AvatarUploadIntentResult, error) {
	if s == nil || s.repo == nil || s.signer == nil {
		return nil, perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeInternal, "avatar upload not configured", nil)
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "user required", nil)
	}
	contentType := strings.ToLower(strings.TrimSpace(req.ContentType))
	if contentType == "" || req.SizeBytes <= 0 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "content_type and size_bytes are required", nil)
	}
	if err := s.validateContentType(contentType); err != nil {
		return nil, err
	}
	if err := s.validateSize(req.SizeBytes); err != nil {
		return nil, err
	}
	if sanitizeAvatarFileName(req.FileName) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "file_name is required", nil)
	}

	assetID := "asset_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	ext := extFromContentType(contentType)
	objectKey := mediaupload.AvatarObjectKey(userID, assetID, ext)
	if err := validateAvatarObjectKey(objectKey, userID); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	exp := now.Add(s.uploadTTL)
	uploadPath := avatarUploadPath(assetID)
	signIn := mediaupload.SignInput{
		Purpose:     mediaupload.PurposeUserAvatarUpload,
		OwnerID:     userID,
		AssetID:     assetID,
		Method:      http.MethodPut,
		Path:        uploadPath,
		ContentType: contentType,
		SizeBytes:   req.SizeBytes,
		ExpiresAt:   exp,
	}
	sig := s.signer.Sign(signIn)
	uploadURL := s.buildSignedURL(strings.TrimRight(strings.TrimSpace(baseURL), "/"), uploadPath, map[string]string{
		"exp":          fmt.Sprintf("%d", exp.Unix()),
		"sig":          sig,
		"user_id":      userID,
		"method":       http.MethodPut,
		"content_type": contentType,
		"size_bytes":   fmt.Sprintf("%d", req.SizeBytes),
	})

	asset := UserMediaAsset{
		AssetID:     assetID,
		UserID:      userID,
		ObjectKey:   objectKey,
		ContentType: contentType,
		SizeBytes:   req.SizeBytes,
		State:       "pending_upload",
		ExpiresAt:   exp,
		CreatedAt:   now,
	}
	if err := s.repo.CreatePendingAsset(ctx, asset); err != nil {
		return nil, err
	}
	return &AvatarUploadIntentResult{
		AssetID:   assetID,
		UploadURL: uploadURL,
		ExpiresAt: exp,
	}, nil
}

// VerifySignature checks the HMAC for a signed upload or content URL.
func (s *AvatarService) VerifySignature(sig string, in mediaupload.SignInput) bool {
	if s == nil || s.signer == nil {
		return false
	}
	if !mediaupload.VerifyNotExpired(in, time.Now().UTC()) {
		return false
	}
	return s.signer.Verify(sig, in)
}

// GetAssetForSignedUpload loads a pending asset for the authenticated upload path.
func (s *AvatarService) GetAssetForSignedUpload(ctx context.Context, userID, assetID string) (*UserMediaAsset, error) {
	asset, err := s.getOwnedAsset(ctx, userID, assetID)
	if err != nil {
		return nil, err
	}
	if asset.State != "pending_upload" {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeInvalidRequest, "avatar asset is not pending upload", nil)
	}
	if time.Now().UTC().After(asset.ExpiresAt) {
		return nil, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeSessionExpired, "signed upload url expired", nil)
	}
	return asset, nil
}

// WriteSignedUpload persists the binary for a pending asset.
func (s *AvatarService) WriteSignedUpload(ctx context.Context, asset *UserMediaAsset, contentType string, maxSize int64, body io.Reader) error {
	if s == nil || s.storage == nil {
		return perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeInternal, "avatar storage not configured", nil)
	}
	if asset == nil {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "asset required", nil)
	}
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if err := s.validateContentType(contentType); err != nil {
		return err
	}
	if asset.ContentType != contentType {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "content_type mismatch", nil)
	}
	limit := asset.SizeBytes
	if maxSize > 0 && maxSize < limit {
		limit = maxSize
	}
	if limit <= 0 || limit > s.maxBytes {
		return perr.NewHTTPError(http.StatusRequestEntityTooLarge, perr.CodeInvalidRequest, "file too large", nil)
	}
	if err := validateAvatarObjectKey(asset.ObjectKey, asset.UserID); err != nil {
		return err
	}
	written, err := s.storage.Write(asset.ObjectKey, io.LimitReader(body, limit+1))
	if err != nil {
		return perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "failed to persist avatar upload", err)
	}
	if written > limit {
		_ = s.storage.Delete(asset.ObjectKey)
		return perr.NewHTTPError(http.StatusRequestEntityTooLarge, perr.CodeInvalidRequest, "file too large", nil)
	}
	if written != asset.SizeBytes {
		_ = s.storage.Delete(asset.ObjectKey)
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "uploaded payload size mismatch", nil)
	}
	_ = ctx
	return nil
}

// Complete marks the asset ready and updates the user's active avatar.
func (s *AvatarService) Complete(ctx context.Context, userID, assetID string) (*AvatarCompleteResult, error) {
	if s == nil || s.repo == nil || s.storage == nil {
		return nil, perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeInternal, "avatar upload not configured", nil)
	}
	asset, err := s.getOwnedAsset(ctx, userID, assetID)
	if err != nil {
		return nil, err
	}
	if asset.State != "pending_upload" {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeInvalidRequest, "avatar asset is not pending upload", nil)
	}
	if time.Now().UTC().After(asset.ExpiresAt) {
		return nil, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeSessionExpired, "upload intent expired", nil)
	}
	if !s.storage.Exists(asset.ObjectKey) {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeInvalidRequest, "avatar binary not uploaded yet", nil)
	}

	prev, _ := s.repo.GetUserAvatar(ctx, userID)
	if err := s.repo.MarkAssetReady(ctx, userID, assetID); err != nil {
		return nil, err
	}
	if err := s.repo.MarkUserReadyAssetsDeleted(ctx, userID, assetID); err != nil {
		return nil, err
	}
	if err := s.repo.SetUserAvatar(ctx, userID, asset.ObjectKey, asset.ContentType); err != nil {
		return nil, err
	}
	if prev != nil && prev.ObjectKey != "" && prev.ObjectKey != asset.ObjectKey {
		_ = s.storage.Delete(prev.ObjectKey)
	}

	return &AvatarCompleteResult{
		AssetID:     asset.AssetID,
		ObjectKey:   asset.ObjectKey,
		ContentType: asset.ContentType,
		SizeBytes:   asset.SizeBytes,
		State:       "ready",
	}, nil
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

func (s *AvatarService) getOwnedAsset(ctx context.Context, userID, assetID string) (*UserMediaAsset, error) {
	asset, err := s.repo.GetAssetByUser(ctx, userID, assetID)
	if err != nil {
		return nil, err
	}
	if asset.UserID != userID {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "avatar asset access denied", nil)
	}
	return asset, nil
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

func avatarUploadPath(assetID string) string {
	return "/api/v1/me/avatar/upload/" + url.PathEscape(strings.TrimSpace(assetID))
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

func sanitizeAvatarFileName(name string) string {
	base := strings.TrimSpace(filepath.Base(name))
	base = strings.ReplaceAll(base, " ", "_")
	base = strings.ReplaceAll(base, "..", "")
	if base == "." || base == "/" || base == `\` {
		return ""
	}
	return base
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
