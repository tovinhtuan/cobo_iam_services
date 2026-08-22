package mysql

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	wdt "github.com/cobo/cobo_iam_services/internal/workflowdoctemplate"
)

// Repository persists workflow document template asset metadata.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a MySQL-backed repository. db may be nil (no-ops fail closed).
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, asset wdt.Asset) error {
	if r == nil || r.db == nil {
		return perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeInternal, "database unavailable", nil)
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO workflow_document_template_assets
  (file_id, owner_scope, company_id, file_name, content_type, size_bytes, object_key, state, created_by, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		asset.FileID, asset.OwnerScope, asset.CompanyID, asset.FileName, asset.ContentType,
		asset.SizeBytes, asset.ObjectKey, asset.State, asset.CreatedBy, asset.CreatedAt,
	)
	if err != nil {
		return perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "failed to save file metadata", err)
	}
	return nil
}

func (r *Repository) GetByFileID(ctx context.Context, fileID string) (*wdt.Asset, error) {
	if r == nil || r.db == nil {
		return nil, perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeInternal, "database unavailable", nil)
	}
	row := r.db.QueryRowContext(ctx, `
SELECT file_id, owner_scope, company_id, file_name, content_type, size_bytes, object_key, state, created_by, created_at, deleted_at
FROM workflow_document_template_assets WHERE file_id = ?`, fileID)
	var asset wdt.Asset
	var deletedAt sql.NullTime
	err := row.Scan(
		&asset.FileID, &asset.OwnerScope, &asset.CompanyID, &asset.FileName, &asset.ContentType,
		&asset.SizeBytes, &asset.ObjectKey, &asset.State, &asset.CreatedBy, &asset.CreatedAt, &deletedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "failed to load file metadata", err)
	}
	if deletedAt.Valid {
		t := deletedAt.Time.UTC()
		asset.DeletedAt = &t
	}
	return &asset, nil
}

func (r *Repository) MarkDeleted(ctx context.Context, fileID string, deletedAt time.Time) error {
	if r == nil || r.db == nil {
		return perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeInternal, "database unavailable", nil)
	}
	res, err := r.db.ExecContext(ctx, `
UPDATE workflow_document_template_assets
SET state = ?, deleted_at = ?
WHERE file_id = ? AND state = ?`, wdt.StateDeleted, deletedAt, fileID, wdt.StateReady)
	if err != nil {
		return perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "failed to delete file metadata", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "file not found", nil)
	}
	return nil
}
