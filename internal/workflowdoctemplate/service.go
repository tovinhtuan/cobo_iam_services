package workflowdoctemplate

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/mediaupload"
	"github.com/google/uuid"
)

// Service uploads and serves purpose-scoped workflow document template files.
type Service struct {
	repo    Repository
	storage ObjectStorage
}

// NewService constructs a Service.
func NewService(repo Repository, storage ObjectStorage) *Service {
	return &Service{repo: repo, storage: storage}
}

// UploadMultipart stores a new immutable file and returns its reference.
func (s *Service) UploadMultipart(ctx context.Context, ownerScope, companyID, createdBy, fileName, contentType string, body io.Reader, sizeBytes int64) (*UploadResult, error) {
	if s == nil || s.repo == nil || s.storage == nil {
		return nil, perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeInternal, "workflow document template upload not configured", nil)
	}
	ownerScope = strings.TrimSpace(ownerScope)
	companyID = strings.TrimSpace(companyID)
	createdBy = strings.TrimSpace(createdBy)
	if ownerScope != OwnerScopeCMS && ownerScope != OwnerScopeCompany {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid owner_scope", nil)
	}
	if companyID == "" || createdBy == "" {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "company and user required", nil)
	}
	fileName = sanitizeFileName(fileName)
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if err := validateUploadMeta(fileName, contentType, sizeBytes); err != nil {
		return nil, err
	}

	fileID := "wdt_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	objectKey := mediaupload.WorkflowDocTemplateObjectKey(ownerScope, companyID, fileID, fileName)
	written, err := s.storage.Write(objectKey, io.LimitReader(body, sizeBytes+1))
	if err != nil {
		return nil, perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "failed to persist file", err)
	}
	if written > sizeBytes || written <= 0 {
		_ = s.storage.Delete(objectKey)
		return nil, perr.NewHTTPError(http.StatusRequestEntityTooLarge, perr.CodeInvalidRequest, "file too large", nil)
	}

	now := time.Now().UTC()
	asset := Asset{
		FileID:      fileID,
		OwnerScope:  ownerScope,
		CompanyID:   companyID,
		FileName:    fileName,
		ContentType: contentType,
		SizeBytes:   written,
		ObjectKey:   objectKey,
		State:       StateReady,
		CreatedBy:   createdBy,
		CreatedAt:   now,
	}
	if err := s.repo.Create(ctx, asset); err != nil {
		_ = s.storage.Delete(objectKey)
		return nil, err
	}
	return &UploadResult{
		FileID:      fileID,
		FileName:    fileName,
		ContentType: contentType,
		SizeBytes:   written,
		OwnerScope:  ownerScope,
		CompanyID:   companyID,
	}, nil
}

// GetReadyAsset loads a ready asset by id.
func (s *Service) GetReadyAsset(ctx context.Context, fileID string) (*Asset, error) {
	if s == nil || s.repo == nil {
		return nil, perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeInternal, "workflow document template not configured", nil)
	}
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "file_id required", nil)
	}
	asset, err := s.repo.GetByFileID(ctx, fileID)
	if err != nil {
		return nil, err
	}
	if asset == nil || asset.State != StateReady {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "file not found", nil)
	}
	return asset, nil
}

// ReadContent returns bytes for a ready asset after ACL check by caller.
func (s *Service) ReadContent(ctx context.Context, fileID string) (*Asset, []byte, error) {
	asset, err := s.GetReadyAsset(ctx, fileID)
	if err != nil {
		return nil, nil, err
	}
	if s.storage == nil {
		return nil, nil, perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeInternal, "workflow document template storage not configured", nil)
	}
	data, err := s.storage.Read(asset.ObjectKey)
	if err != nil {
		return nil, nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "file not found", err)
	}
	return asset, data, nil
}

// AssertCanBind validates file_id for workflow JSON binding (B2).
func (s *Service) AssertCanBind(ctx context.Context, fileID, bindScope, bindCompanyID string) (*Asset, error) {
	if strings.TrimSpace(fileID) == "" {
		return nil, nil
	}
	asset, err := s.GetReadyAsset(ctx, fileID)
	if err != nil {
		return nil, err
	}
	if !CanBindForWorkflowSave(asset, bindScope, bindCompanyID) {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "cannot reference this file", nil)
	}
	return asset, nil
}

// SoftDelete marks metadata deleted without removing historical binaries (version safety).
func (s *Service) SoftDelete(ctx context.Context, fileID string) error {
	asset, err := s.GetReadyAsset(ctx, fileID)
	if err != nil {
		return err
	}
	_ = asset
	return s.repo.MarkDeleted(ctx, fileID, time.Now().UTC())
}
