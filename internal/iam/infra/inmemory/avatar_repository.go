package inmemory

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// AvatarRepository is an in-memory avatar repository for tests and no-DB mode.
type AvatarRepository struct {
	mu      sync.Mutex
	assets  map[string]iamapp.UserMediaAsset
	avatars map[string]iamapp.UserAvatarMeta
}

// NewAvatarRepository creates an empty in-memory avatar repository.
func NewAvatarRepository() *AvatarRepository {
	return &AvatarRepository{
		assets:  map[string]iamapp.UserMediaAsset{},
		avatars: map[string]iamapp.UserAvatarMeta{},
	}
}

func (r *AvatarRepository) CreatePendingAsset(_ context.Context, asset iamapp.UserMediaAsset) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.assets[asset.AssetID] = asset
	return nil
}

func (r *AvatarRepository) GetAssetByID(_ context.Context, assetID string) (*iamapp.UserMediaAsset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.assets[assetID]
	if !ok {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "avatar asset not found", nil)
	}
	out := item
	return &out, nil
}

func (r *AvatarRepository) GetAssetByUser(_ context.Context, userID, assetID string) (*iamapp.UserMediaAsset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.assets[assetID]
	if !ok || item.UserID != userID {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "avatar asset access denied", nil)
	}
	out := item
	return &out, nil
}

func (r *AvatarRepository) MarkAssetReady(_ context.Context, userID, assetID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.assets[assetID]
	if !ok || item.UserID != userID || item.State != "pending_upload" {
		return perr.NewHTTPError(http.StatusConflict, perr.CodeInvalidRequest, "avatar asset is not pending upload", nil)
	}
	item.State = "ready"
	item.UpdatedAt = time.Now().UTC()
	r.assets[assetID] = item
	return nil
}

func (r *AvatarRepository) MarkUserReadyAssetsDeleted(_ context.Context, userID, exceptAssetID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, item := range r.assets {
		if item.UserID != userID || item.State != "ready" {
			continue
		}
		if exceptAssetID != "" && id == exceptAssetID {
			continue
		}
		item.State = "deleted"
		item.UpdatedAt = time.Now().UTC()
		r.assets[id] = item
	}
	return nil
}

func (r *AvatarRepository) GetUserAvatar(_ context.Context, userID string) (*iamapp.UserAvatarMeta, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	meta, ok := r.avatars[userID]
	if !ok {
		return &iamapp.UserAvatarMeta{}, nil
	}
	out := meta
	return &out, nil
}

func (r *AvatarRepository) SetUserAvatar(_ context.Context, userID, objectKey, contentType string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	r.avatars[userID] = iamapp.UserAvatarMeta{
		ObjectKey:   strings.TrimSpace(objectKey),
		ContentType: strings.TrimSpace(contentType),
		UpdatedAt:   &now,
	}
	return nil
}

func (r *AvatarRepository) ClearUserAvatar(_ context.Context, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.avatars, userID)
	return nil
}
