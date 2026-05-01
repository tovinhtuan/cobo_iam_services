package http

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type cmsMediaAsset struct {
	AssetID      string
	CompanyID    string
	FileName     string
	ContentType  string
	SizeBytes    int64
	Context      string
	ObjectKey    string
	UploadURL    string
	UploadMethod string
	State        string
	CreatedAt    time.Time
	CompletedAt  *time.Time
	DeletedAt    *time.Time
	UpdatedBy    string
	ETag         string
	Checksum     string
	ExpiresAt    time.Time
}

type cmsMediaRepository interface {
	CreateIntent(ctx context.Context, item cmsMediaAsset) error
	GetByAssetID(ctx context.Context, assetID string) (*cmsMediaAsset, error)
	GetByCompany(ctx context.Context, companyID, assetID string) (*cmsMediaAsset, error)
	List(ctx context.Context, companyID, typ, q, cursor string, limit int) ([]cmsMediaAsset, error)
	MarkComplete(ctx context.Context, companyID, assetID, actor, etag, checksum string, sizeBytes int64) (*cmsMediaAsset, error)
	MarkDeleted(ctx context.Context, companyID, assetID, actor string) (*cmsMediaAsset, error)
}

type cmsMediaStore struct {
	mu    sync.Mutex
	items map[string]cmsMediaAsset
}

func newCMSMediaRepository(db *sql.DB) cmsMediaRepository {
	if db != nil {
		return &cmsMediaMySQLRepository{db: db}
	}
	return newCMSMediaStore()
}

func newCMSMediaStore() *cmsMediaStore {
	return &cmsMediaStore{
		items: map[string]cmsMediaAsset{},
	}
}

func (s *cmsMediaStore) CreateIntent(_ context.Context, item cmsMediaAsset) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[item.AssetID] = item
	return nil
}

func (s *cmsMediaStore) GetByAssetID(_ context.Context, assetID string) (*cmsMediaAsset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[assetID]
	if !ok {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "media asset not found", nil)
	}
	out := item
	return &out, nil
}

func (s *cmsMediaStore) GetByCompany(_ context.Context, companyID, assetID string) (*cmsMediaAsset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[assetID]
	if !ok || item.CompanyID != companyID || item.DeletedAt != nil {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "media asset not found", nil)
	}
	out := item
	return &out, nil
}

func (s *cmsMediaStore) List(_ context.Context, companyID, typ, q, cursor string, limit int) ([]cmsMediaAsset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	filtered := make([]cmsMediaAsset, 0, len(s.items))
	query := strings.ToLower(strings.TrimSpace(q))
	typeFilter := strings.ToLower(strings.TrimSpace(typ))
	cursor = strings.TrimSpace(cursor)

	for _, item := range s.items {
		if item.CompanyID != companyID {
			continue
		}
		if item.DeletedAt != nil {
			continue
		}
		if typeFilter != "" && strings.ToLower(strings.TrimSpace(item.ContentType)) != typeFilter {
			continue
		}
		if query != "" {
			haystack := strings.ToLower(item.FileName + " " + item.ObjectKey + " " + item.ContentType)
			if !strings.Contains(haystack, query) {
				continue
			}
		}
		if cursor != "" && !item.CreatedAt.UTC().Before(parseCursorFallback(cursor)) {
			continue
		}
		filtered = append(filtered, item)
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered, nil
}

func (s *cmsMediaStore) MarkComplete(_ context.Context, companyID, assetID, actor, etag, checksum string, sizeBytes int64) (*cmsMediaAsset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[assetID]
	if !ok || item.CompanyID != companyID || item.DeletedAt != nil {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "media asset not found", nil)
	}
	if item.State == "ready" {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeInvalidRequest, "media asset already completed", nil)
	}
	if sizeBytes > 0 && item.SizeBytes != sizeBytes {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "size_bytes mismatch", nil)
	}
	now := time.Now().UTC()
	item.State = "ready"
	item.CompletedAt = &now
	item.UpdatedBy = actor
	item.ETag = strings.TrimSpace(etag)
	item.Checksum = strings.TrimSpace(checksum)
	s.items[assetID] = item
	out := item
	return &out, nil
}

func (s *cmsMediaStore) MarkDeleted(_ context.Context, companyID, assetID, actor string) (*cmsMediaAsset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[assetID]
	if !ok || item.CompanyID != companyID || item.DeletedAt != nil {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "media asset not found", nil)
	}
	now := time.Now().UTC()
	item.State = "deleted"
	item.DeletedAt = &now
	item.UpdatedBy = actor
	s.items[assetID] = item
	out := item
	return &out, nil
}

func parseCursorFallback(raw string) time.Time {
	v := strings.TrimSpace(raw)
	if v == "" {
		return time.Time{}
	}
	ts, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return time.Now().UTC().Add(24 * time.Hour)
	}
	return ts.UTC()
}

type cmsMediaMySQLRepository struct {
	db *sql.DB
}

func (r *cmsMediaMySQLRepository) CreateIntent(ctx context.Context, item cmsMediaAsset) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO cms_media_assets (
			asset_id, company_id, file_name, content_type, size_bytes, context, object_key, upload_method, state, upload_expires_at, created_by, updated_by
		) VALUES (?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?)
	`, item.AssetID, item.CompanyID, item.FileName, item.ContentType, item.SizeBytes, item.Context, item.ObjectKey, item.UploadMethod, item.State, item.ExpiresAt, item.UpdatedBy, item.UpdatedBy)
	if err != nil {
		return fmt.Errorf("insert media asset: %w", err)
	}
	return nil
}

func (r *cmsMediaMySQLRepository) GetByAssetID(ctx context.Context, assetID string) (*cmsMediaAsset, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT asset_id, company_id, file_name, content_type, size_bytes, COALESCE(context, ''), object_key, upload_method, state,
		       upload_expires_at, created_at, completed_at, deleted_at, updated_by, COALESCE(etag, ''), COALESCE(checksum, '')
		FROM cms_media_assets
		WHERE asset_id = ?
	`, assetID)
	return scanMediaAssetRow(row)
}

func (r *cmsMediaMySQLRepository) GetByCompany(ctx context.Context, companyID, assetID string) (*cmsMediaAsset, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT asset_id, company_id, file_name, content_type, size_bytes, COALESCE(context, ''), object_key, upload_method, state,
		       upload_expires_at, created_at, completed_at, deleted_at, updated_by, COALESCE(etag, ''), COALESCE(checksum, '')
		FROM cms_media_assets
		WHERE asset_id = ? AND company_id = ? AND deleted_at IS NULL
	`, assetID, companyID)
	return scanMediaAssetRow(row)
}

func (r *cmsMediaMySQLRepository) List(ctx context.Context, companyID, typ, q, cursor string, limit int) ([]cmsMediaAsset, error) {
	args := []any{companyID}
	conds := []string{"company_id = ?", "deleted_at IS NULL"}
	if strings.TrimSpace(typ) != "" {
		conds = append(conds, "content_type = ?")
		args = append(args, strings.ToLower(strings.TrimSpace(typ)))
	}
	if strings.TrimSpace(q) != "" {
		conds = append(conds, "(LOWER(file_name) LIKE ? OR LOWER(object_key) LIKE ?)")
		like := "%" + strings.ToLower(strings.TrimSpace(q)) + "%"
		args = append(args, like, like)
	}
	if strings.TrimSpace(cursor) != "" {
		if ts, err := time.Parse(time.RFC3339, strings.TrimSpace(cursor)); err == nil {
			conds = append(conds, "created_at < ?")
			args = append(args, ts.UTC())
		}
	}
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, `
		SELECT asset_id, company_id, file_name, content_type, size_bytes, COALESCE(context, ''), object_key, upload_method, state,
		       upload_expires_at, created_at, completed_at, deleted_at, updated_by, COALESCE(etag, ''), COALESCE(checksum, '')
		FROM cms_media_assets
		WHERE `+strings.Join(conds, " AND ")+`
		ORDER BY created_at DESC
		LIMIT ?
	`, args...)
	if err != nil {
		return nil, fmt.Errorf("list media assets: %w", err)
	}
	defer rows.Close()
	out := make([]cmsMediaAsset, 0)
	for rows.Next() {
		item, err := scanMediaAssetRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *cmsMediaMySQLRepository) MarkComplete(ctx context.Context, companyID, assetID, actor, etag, checksum string, sizeBytes int64) (*cmsMediaAsset, error) {
	item, err := r.GetByCompany(ctx, companyID, assetID)
	if err != nil {
		return nil, err
	}
	if item.State == "ready" {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeInvalidRequest, "media asset already completed", nil)
	}
	if sizeBytes > 0 && item.SizeBytes != sizeBytes {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "size_bytes mismatch", nil)
	}
	now := time.Now().UTC()
	_, err = r.db.ExecContext(ctx, `
		UPDATE cms_media_assets
		SET state = 'ready', completed_at = ?, updated_by = ?, etag = NULLIF(?, ''), checksum = NULLIF(?, ''), updated_at = CURRENT_TIMESTAMP
		WHERE asset_id = ? AND company_id = ? AND deleted_at IS NULL
	`, now, actor, strings.TrimSpace(etag), strings.TrimSpace(checksum), assetID, companyID)
	if err != nil {
		return nil, fmt.Errorf("complete media asset: %w", err)
	}
	item.State = "ready"
	item.CompletedAt = &now
	item.UpdatedBy = actor
	item.ETag = strings.TrimSpace(etag)
	item.Checksum = strings.TrimSpace(checksum)
	return item, nil
}

func (r *cmsMediaMySQLRepository) MarkDeleted(ctx context.Context, companyID, assetID, actor string) (*cmsMediaAsset, error) {
	item, err := r.GetByCompany(ctx, companyID, assetID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	_, err = r.db.ExecContext(ctx, `
		UPDATE cms_media_assets
		SET state = 'deleted', deleted_at = ?, updated_by = ?, updated_at = CURRENT_TIMESTAMP
		WHERE asset_id = ? AND company_id = ? AND deleted_at IS NULL
	`, now, actor, assetID, companyID)
	if err != nil {
		return nil, fmt.Errorf("delete media asset: %w", err)
	}
	item.State = "deleted"
	item.DeletedAt = &now
	item.UpdatedBy = actor
	return item, nil
}

type singleRowScanner interface {
	Scan(dest ...any) error
}

func scanMediaAssetRow(row singleRowScanner) (*cmsMediaAsset, error) {
	item, err := scanMediaAsset(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "media asset not found", nil)
		}
		return nil, err
	}
	return &item, nil
}

func scanMediaAssetRows(rows *sql.Rows) (cmsMediaAsset, error) {
	return scanMediaAsset(rows)
}

func scanMediaAsset(scanner interface{ Scan(dest ...any) error }) (cmsMediaAsset, error) {
	var item cmsMediaAsset
	var completedAt sql.NullTime
	var deletedAt sql.NullTime
	if err := scanner.Scan(
		&item.AssetID, &item.CompanyID, &item.FileName, &item.ContentType, &item.SizeBytes,
		&item.Context, &item.ObjectKey, &item.UploadMethod, &item.State, &item.ExpiresAt,
		&item.CreatedAt, &completedAt, &deletedAt, &item.UpdatedBy, &item.ETag, &item.Checksum,
	); err != nil {
		return cmsMediaAsset{}, err
	}
	if completedAt.Valid {
		v := completedAt.Time.UTC()
		item.CompletedAt = &v
	}
	if deletedAt.Valid {
		v := deletedAt.Time.UTC()
		item.DeletedAt = &v
	}
	return item, nil
}
