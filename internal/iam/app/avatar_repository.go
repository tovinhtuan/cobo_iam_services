package app

import (
	"context"
	"time"
)

// UserMediaAsset is a row in user_media_assets.
type UserMediaAsset struct {
	AssetID     string
	UserID      string
	ObjectKey   string
	ContentType string
	SizeBytes   int64
	State       string
	ExpiresAt   time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// UserAvatarMeta is denormalized avatar state on users.
type UserAvatarMeta struct {
	ObjectKey   string
	ContentType string
	UpdatedAt   *time.Time
}

// AvatarRepository persists avatar assets and user avatar columns.
type AvatarRepository interface {
	CreatePendingAsset(ctx context.Context, asset UserMediaAsset) error
	GetAssetByID(ctx context.Context, assetID string) (*UserMediaAsset, error)
	GetAssetByUser(ctx context.Context, userID, assetID string) (*UserMediaAsset, error)
	MarkAssetReady(ctx context.Context, userID, assetID string) error
	MarkUserReadyAssetsDeleted(ctx context.Context, userID, exceptAssetID string) error
	GetUserAvatar(ctx context.Context, userID string) (*UserAvatarMeta, error)
	SetUserAvatar(ctx context.Context, userID, objectKey, contentType string) error
	ClearUserAvatar(ctx context.Context, userID string) error
}
