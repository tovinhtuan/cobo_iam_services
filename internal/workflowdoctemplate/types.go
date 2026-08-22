package workflowdoctemplate

import (
	"context"
	"io"
	"time"
)

const (
	OwnerScopeCMS     = "cms"
	OwnerScopeCompany = "company"
	StateReady        = "ready"
	StateDeleted      = "deleted"
	MaxSizeBytes      = int64(20 * 1024 * 1024)
)

// Asset is an immutable workflow document template / sample file.
type Asset struct {
	FileID      string
	OwnerScope  string
	CompanyID   string
	FileName    string
	ContentType string
	SizeBytes   int64
	ObjectKey   string
	State       string
	CreatedBy   string
	CreatedAt   time.Time
	DeletedAt   *time.Time
}

// UploadResult is returned after a successful multipart upload.
type UploadResult struct {
	FileID      string `json:"file_id"`
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	OwnerScope  string `json:"owner_scope"`
	CompanyID   string `json:"company_id"`
}

// Repository persists asset metadata (binary lives in mediaupload.DiskStorage).
type Repository interface {
	Create(ctx context.Context, asset Asset) error
	GetByFileID(ctx context.Context, fileID string) (*Asset, error)
	MarkDeleted(ctx context.Context, fileID string, deletedAt time.Time) error
}

// ObjectStorage writes/reads binary objects.
type ObjectStorage interface {
	Write(objectKey string, body io.Reader) (int64, error)
	Read(objectKey string) ([]byte, error)
	Delete(objectKey string) error
}
