package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// AvatarRepository persists user avatar assets in MySQL.
type AvatarRepository struct {
	db *sql.DB
}

// NewAvatarRepository creates a MySQL avatar repository.
func NewAvatarRepository(db *sql.DB) *AvatarRepository {
	return &AvatarRepository{db: db}
}

func (r *AvatarRepository) CreatePendingAsset(ctx context.Context, asset iamapp.UserMediaAsset) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO user_media_assets (
			asset_id, user_id, object_key, content_type, size_bytes, state, expires_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, asset.AssetID, asset.UserID, asset.ObjectKey, asset.ContentType, asset.SizeBytes, asset.State, asset.ExpiresAt.UTC())
	if err != nil {
		return fmt.Errorf("insert user media asset: %w", err)
	}
	return nil
}

func (r *AvatarRepository) GetAssetByID(ctx context.Context, assetID string) (*iamapp.UserMediaAsset, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT asset_id, user_id, object_key, content_type, size_bytes, state, expires_at, created_at, updated_at
		FROM user_media_assets
		WHERE asset_id = ?
	`, assetID)
	return scanUserMediaAsset(row)
}

func (r *AvatarRepository) GetAssetByUser(ctx context.Context, userID, assetID string) (*iamapp.UserMediaAsset, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT asset_id, user_id, object_key, content_type, size_bytes, state, expires_at, created_at, updated_at
		FROM user_media_assets
		WHERE asset_id = ? AND user_id = ?
	`, assetID, userID)
	item, err := scanUserMediaAsset(row)
	if err != nil {
		if he, ok := perr.AsHTTPError(err); ok && he.HTTPStatus == http.StatusNotFound {
			return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "avatar asset access denied", nil)
		}
		return nil, err
	}
	return item, nil
}

func (r *AvatarRepository) MarkAssetReady(ctx context.Context, userID, assetID string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE user_media_assets
		SET state = 'ready', updated_at = CURRENT_TIMESTAMP
		WHERE asset_id = ? AND user_id = ? AND state = 'pending_upload'
	`, assetID, userID)
	if err != nil {
		return fmt.Errorf("mark avatar asset ready: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return perr.NewHTTPError(http.StatusConflict, perr.CodeInvalidRequest, "avatar asset is not pending upload", nil)
	}
	return nil
}

func (r *AvatarRepository) MarkUserReadyAssetsDeleted(ctx context.Context, userID, exceptAssetID string) error {
	userID = strings.TrimSpace(userID)
	exceptAssetID = strings.TrimSpace(exceptAssetID)
	var err error
	if exceptAssetID != "" {
		_, err = r.db.ExecContext(ctx, `
			UPDATE user_media_assets
			SET state = 'deleted', updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND state = 'ready' AND asset_id <> ?
		`, userID, exceptAssetID)
	} else {
		_, err = r.db.ExecContext(ctx, `
			UPDATE user_media_assets
			SET state = 'deleted', updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND state = 'ready'
		`, userID)
	}
	if err != nil {
		return fmt.Errorf("mark avatar assets deleted: %w", err)
	}
	return nil
}

func (r *AvatarRepository) GetUserAvatar(ctx context.Context, userID string) (*iamapp.UserAvatarMeta, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT avatar_object_key, avatar_content_type, avatar_updated_at
		FROM users
		WHERE user_id = ?
	`, userID)
	var objectKey, contentType sql.NullString
	var updatedAt sql.NullTime
	if err := row.Scan(&objectKey, &contentType, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "user not found", nil)
		}
		return nil, fmt.Errorf("get user avatar: %w", err)
	}
	if !objectKey.Valid || strings.TrimSpace(objectKey.String) == "" {
		return &iamapp.UserAvatarMeta{}, nil
	}
	meta := &iamapp.UserAvatarMeta{
		ObjectKey:   strings.TrimSpace(objectKey.String),
		ContentType: strings.TrimSpace(contentType.String),
	}
	if updatedAt.Valid {
		t := updatedAt.Time.UTC()
		meta.UpdatedAt = &t
	}
	return meta, nil
}

func (r *AvatarRepository) SetUserAvatar(ctx context.Context, userID, objectKey, contentType string) error {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET avatar_object_key = ?, avatar_content_type = ?, avatar_updated_at = ?
		WHERE user_id = ?
	`, objectKey, contentType, now, userID)
	if err != nil {
		return fmt.Errorf("set user avatar: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "user not found", nil)
	}
	return nil
}

func (r *AvatarRepository) ClearUserAvatar(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET avatar_object_key = NULL, avatar_content_type = NULL, avatar_updated_at = NULL
		WHERE user_id = ?
	`, userID)
	if err != nil {
		return fmt.Errorf("clear user avatar: %w", err)
	}
	return nil
}

func scanUserMediaAsset(row *sql.Row) (*iamapp.UserMediaAsset, error) {
	var item iamapp.UserMediaAsset
	if err := row.Scan(
		&item.AssetID, &item.UserID, &item.ObjectKey, &item.ContentType, &item.SizeBytes,
		&item.State, &item.ExpiresAt, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "avatar asset not found", nil)
		}
		return nil, fmt.Errorf("scan user media asset: %w", err)
	}
	item.ExpiresAt = item.ExpiresAt.UTC()
	item.CreatedAt = item.CreatedAt.UTC()
	item.UpdatedAt = item.UpdatedAt.UTC()
	return &item, nil
}
