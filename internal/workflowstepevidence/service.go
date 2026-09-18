package workflowstepevidence

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	wff "github.com/cobo/cobo_iam_services/internal/workflowfulfillment"
	"github.com/google/uuid"
)

// Service implements Generic Step Evidence CRUD (G2B) + audit/security hardening (G2C).
type Service struct {
	deadline wff.DeadlineContext
	files    Repository
	storage  ObjectStorage
	audit    Auditor
	now      func() time.Time
	log      *slog.Logger
}

// NewService constructs the generic evidence service.
func NewService(deadline wff.DeadlineContext, files Repository, storage ObjectStorage, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{deadline: deadline, files: files, storage: storage, now: time.Now, log: log}
}

// WithNow overrides the clock (tests).
func (s *Service) WithNow(now func() time.Time) *Service {
	s.now = now
	return s
}

// WithAudit wires platform audit (G2C). Nil-safe: without audit, mutations still succeed.
func (s *Service) WithAudit(a Auditor) *Service {
	s.audit = a
	return s
}

// ListEvidenceFiles returns ACTIVE files + BE capabilities. View scope only.
func (s *Service) ListEvidenceFiles(ctx context.Context, sub wff.Subject, recordID, stepCode string) (*ListResponse, error) {
	if err := s.deadline.AuthorizeView(ctx, sub); err != nil {
		return nil, err
	}
	wf, err := s.deadline.LoadWorkflowForRecord(ctx, sub, recordID)
	if err != nil {
		return nil, err
	}
	stepCode = strings.TrimSpace(stepCode)
	states, err := s.deadline.ListStepStates(ctx, wf.WorkflowInstanceID)
	if err != nil {
		return nil, err
	}
	isCurrent, isCompleted, err := wff.EvaluateStepAuthority(wf, states, stepCode, s.now())
	if err != nil {
		return nil, err
	}
	canMutatePerm := s.deadline.AuthorizeMutation(ctx, sub, recordID) == nil
	canMutate := canMutatePerm && isCurrent && !isCompleted

	owner := ownerFrom(wf, stepCode)
	active, err := s.files.ListActiveByStep(ctx, owner)
	if err != nil {
		return nil, err
	}
	fileCaps := FileCapabilities{
		CanDownload: true,
		CanDelete:   canMutate,
		CanReplace:  canMutate,
	}
	out := make([]FileDTO, 0, len(active))
	for _, f := range active {
		out = append(out, toFileDTO(f, fileCaps))
	}
	return &ListResponse{
		RecordID:           wf.RecordID,
		WorkflowInstanceID: wf.WorkflowInstanceID,
		StepCode:           stepCode,
		Completed:          isCompleted,
		Capabilities:       ListCapabilities{CanUpload: canMutate},
		Files:              out,
	}, nil
}

// UploadEvidenceFile creates an ACTIVE generic evidence file.
func (s *Service) UploadEvidenceFile(ctx context.Context, sub wff.Subject, recordID, stepCode, fileName, contentType string, body io.Reader, sizeHint int64) (*UploadResult, error) {
	meta, err := s.prepareMutation(ctx, sub, recordID, stepCode)
	if err != nil {
		return nil, err
	}
	fileName = wff.SanitizeFileName(fileName)
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = wff.GuessContentType(fileName)
	}
	if sizeHint <= 0 {
		sizeHint = MaxSizeBytes
	}
	if err := mapFilePolicyErr(wff.ValidateUploadMeta(fileName, contentType, sizeHint)); err != nil {
		return nil, err
	}

	fileID := "wse_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	storageKey := EvidenceObjectKey(sub.CompanyID, fileID, fileName)
	written, err := s.storage.Write(storageKey, io.LimitReader(body, MaxSizeBytes+1))
	if err != nil {
		return nil, perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "failed to persist file", err)
	}
	if written <= 0 {
		_ = s.storage.Delete(storageKey)
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "empty file is not allowed", nil)
	}
	if written > MaxSizeBytes {
		_ = s.storage.Delete(storageKey)
		return nil, perr.NewHTTPError(http.StatusRequestEntityTooLarge, perr.CodeWorkflowStepEvidenceFileTooLarge, "file too large", nil)
	}
	if err := mapFilePolicyErr(wff.ValidateUploadMeta(fileName, contentType, written)); err != nil {
		_ = s.storage.Delete(storageKey)
		return nil, err
	}

	now := s.now().UTC()
	owner := ownerFrom(meta.wf, meta.stepCode)
	row := EvidenceFile{
		ID:                 fileID,
		CompanyID:          sub.CompanyID,
		DisclosureRecordID: meta.wf.RecordID,
		WorkflowInstanceID: meta.wf.WorkflowInstanceID,
		StepCode:           meta.stepCode,
		StorageKey:         storageKey,
		OriginalFileName:   fileName,
		MimeType:           contentType,
		FileSize:           written,
		UploadedBy:         sub.UserID,
		UploadedAt:         now,
		LifecycleStatus:    LifecycleActive,
	}
	err = s.files.CreateActiveInTx(ctx, CreateTxInput{
		File:                row,
		Owner:               owner,
		RequireNotCompleted: true,
		MaxActive:           MaxActiveFilesPerStep,
	})
	if err != nil {
		if delErr := s.storage.Delete(storageKey); delErr != nil {
			s.log.Error("evidence compensating delete failed",
				slog.String("storage_namespace", StorageNamespace),
				slog.String("file_id", fileID),
				slog.String("disclosure_record_id", meta.wf.RecordID),
				slog.String("workflow_instance_id", meta.wf.WorkflowInstanceID),
				slog.String("step_code", meta.stepCode),
				slog.String("err", delErr.Error()),
			)
		}
		return nil, mapRepoErr(err)
	}
	s.appendEvidenceAudit(ctx, AuditActionUpload, fileID, sub, evidenceAuditContext{
		CompanyID:          sub.CompanyID,
		DisclosureRecordID: meta.wf.RecordID,
		WorkflowInstanceID: meta.wf.WorkflowInstanceID,
		StepCode:           meta.stepCode,
		ActorUserID:        sub.UserID,
		ActorMembershipID:  sub.MembershipID,
	}, map[string]any{
		"operation":          "upload",
		"original_file_name": fileName,
		"mime_type":          contentType,
		"file_size":          written,
	})
	caps := FileCapabilities{CanDownload: true, CanDelete: true, CanReplace: true}
	return &UploadResult{File: toFileDTO(row, caps)}, nil
}

// DownloadEvidenceFile streams ACTIVE file bytes. View scope; completed OK. No audit (G2C).
func (s *Service) DownloadEvidenceFile(ctx context.Context, sub wff.Subject, recordID, stepCode, fileID string) (*EvidenceFile, []byte, error) {
	if err := s.deadline.AuthorizeView(ctx, sub); err != nil {
		return nil, nil, err
	}
	wf, err := s.deadline.LoadWorkflowForRecord(ctx, sub, recordID)
	if err != nil {
		return nil, nil, err
	}
	stepCode = strings.TrimSpace(stepCode)
	states, err := s.deadline.ListStepStates(ctx, wf.WorkflowInstanceID)
	if err != nil {
		return nil, nil, err
	}
	if _, _, err := wff.EvaluateStepAuthority(wf, states, stepCode, s.now()); err != nil {
		return nil, nil, err
	}
	owner := ownerFrom(wf, stepCode)
	f, err := s.files.GetActiveByIDInContext(ctx, owner, fileID)
	if err != nil {
		return nil, nil, err
	}
	if f == nil {
		return nil, nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeWorkflowStepEvidenceFileNotFound, "evidence file not found", nil)
	}
	data, err := s.storage.Read(f.StorageKey)
	if err != nil {
		return nil, nil, perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "failed to read file", err)
	}
	return f, data, nil
}

// DeleteEvidenceFile logically deletes an ACTIVE file.
func (s *Service) DeleteEvidenceFile(ctx context.Context, sub wff.Subject, recordID, stepCode, fileID string) error {
	meta, err := s.prepareMutation(ctx, sub, recordID, stepCode)
	if err != nil {
		return err
	}
	owner := ownerFrom(meta.wf, meta.stepCode)
	f, err := s.files.GetActiveByIDInContext(ctx, owner, fileID)
	if err != nil {
		return err
	}
	if f == nil {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeWorkflowStepEvidenceFileNotFound, "evidence file not found", nil)
	}
	err = s.files.DeleteInTx(ctx, DeleteTxInput{
		FileID:              fileID,
		Owner:               owner,
		DeletedBy:           sub.UserID,
		DeletedAt:           s.now().UTC(),
		RequireNotCompleted: true,
	})
	if err != nil {
		return mapRepoErr(err)
	}
	// Logical delete only — no filesystem unlink on success path.
	s.appendEvidenceAudit(ctx, AuditActionDelete, fileID, sub, evidenceAuditContext{
		CompanyID:          sub.CompanyID,
		DisclosureRecordID: meta.wf.RecordID,
		WorkflowInstanceID: meta.wf.WorkflowInstanceID,
		StepCode:           meta.stepCode,
		ActorUserID:        sub.UserID,
		ActorMembershipID:  sub.MembershipID,
	}, map[string]any{
		"operation":          "delete",
		"original_file_name": f.OriginalFileName,
		"mime_type":          f.MimeType,
		"file_size":          f.FileSize,
		"lifecycle_from":     LifecycleActive,
		"lifecycle_to":       LifecycleDeleted,
	})
	return nil
}

// ReplaceEvidenceFile append-only replaces an ACTIVE file.
func (s *Service) ReplaceEvidenceFile(ctx context.Context, sub wff.Subject, recordID, stepCode, fileID, fileName, contentType string, body io.Reader, sizeHint int64) (*UploadResult, error) {
	meta, err := s.prepareMutation(ctx, sub, recordID, stepCode)
	if err != nil {
		return nil, err
	}
	owner := ownerFrom(meta.wf, meta.stepCode)
	old, err := s.files.GetActiveByIDInContext(ctx, owner, fileID)
	if err != nil {
		return nil, err
	}
	if old == nil {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeWorkflowStepEvidenceFileNotFound, "evidence file not found", nil)
	}
	fileName = wff.SanitizeFileName(fileName)
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = wff.GuessContentType(fileName)
	}
	if sizeHint <= 0 {
		sizeHint = MaxSizeBytes
	}
	if err := mapFilePolicyErr(wff.ValidateUploadMeta(fileName, contentType, sizeHint)); err != nil {
		return nil, err
	}

	newID := "wse_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	storageKey := EvidenceObjectKey(sub.CompanyID, newID, fileName)
	written, err := s.storage.Write(storageKey, io.LimitReader(body, MaxSizeBytes+1))
	if err != nil {
		return nil, perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "failed to persist file", err)
	}
	if written <= 0 {
		_ = s.storage.Delete(storageKey)
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "empty file is not allowed", nil)
	}
	if written > MaxSizeBytes {
		_ = s.storage.Delete(storageKey)
		return nil, perr.NewHTTPError(http.StatusRequestEntityTooLarge, perr.CodeWorkflowStepEvidenceFileTooLarge, "file too large", nil)
	}
	if err := mapFilePolicyErr(wff.ValidateUploadMeta(fileName, contentType, written)); err != nil {
		_ = s.storage.Delete(storageKey)
		return nil, err
	}

	now := s.now().UTC()
	row := EvidenceFile{
		ID:                 newID,
		CompanyID:          sub.CompanyID,
		DisclosureRecordID: meta.wf.RecordID,
		WorkflowInstanceID: meta.wf.WorkflowInstanceID,
		StepCode:           meta.stepCode,
		StorageKey:         storageKey,
		OriginalFileName:   fileName,
		MimeType:           contentType,
		FileSize:           written,
		UploadedBy:         sub.UserID,
		UploadedAt:         now,
		LifecycleStatus:    LifecycleActive,
		SupersedesFileID:   old.ID,
	}
	err = s.files.ReplaceInTx(ctx, ReplaceTxInput{
		OldFileID:           old.ID,
		NewFile:             row,
		Owner:               owner,
		RequireNotCompleted: true,
	})
	if err != nil {
		if delErr := s.storage.Delete(storageKey); delErr != nil {
			s.log.Error("evidence replace compensating delete failed",
				slog.String("storage_namespace", StorageNamespace),
				slog.String("file_id", newID),
				slog.String("old_file_id", old.ID),
				slog.String("err", delErr.Error()),
			)
		}
		return nil, mapRepoErr(err)
	}
	s.appendEvidenceAudit(ctx, AuditActionReplace, newID, sub, evidenceAuditContext{
		CompanyID:          sub.CompanyID,
		DisclosureRecordID: meta.wf.RecordID,
		WorkflowInstanceID: meta.wf.WorkflowInstanceID,
		StepCode:           meta.stepCode,
		ActorUserID:        sub.UserID,
		ActorMembershipID:  sub.MembershipID,
	}, map[string]any{
		"operation":              "replace",
		"old_file_id":            old.ID,
		"new_file_id":            newID,
		"original_file_name":     fileName,
		"mime_type":              contentType,
		"file_size":              written,
		"old_original_file_name": old.OriginalFileName,
		"lifecycle_old_from":     LifecycleActive,
		"lifecycle_old_to":       LifecycleSuperseded,
		"lifecycle_new":          LifecycleActive,
	})
	caps := FileCapabilities{CanDownload: true, CanDelete: true, CanReplace: true}
	return &UploadResult{File: toFileDTO(row, caps)}, nil
}

type mutationMeta struct {
	wf       wff.WorkflowContext
	stepCode string
}

func (s *Service) prepareMutation(ctx context.Context, sub wff.Subject, recordID, stepCode string) (*mutationMeta, error) {
	if err := s.deadline.AuthorizeMutation(ctx, sub, recordID); err != nil {
		return nil, err
	}
	wf, err := s.deadline.LoadWorkflowForRecord(ctx, sub, recordID)
	if err != nil {
		return nil, err
	}
	stepCode = strings.TrimSpace(stepCode)
	states, err := s.deadline.ListStepStates(ctx, wf.WorkflowInstanceID)
	if err != nil {
		return nil, err
	}
	isCurrent, isCompleted, err := wff.EvaluateStepAuthority(wf, states, stepCode, s.now())
	if err != nil {
		return nil, err
	}
	if isCompleted {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeWorkflowStepAlreadyCompleted, "workflow step already completed", nil)
	}
	if !isCurrent {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeWorkflowStepNotCurrent, "workflow step is not current", nil)
	}
	return &mutationMeta{wf: wf, stepCode: stepCode}, nil
}

func ownerFrom(wf wff.WorkflowContext, stepCode string) OwnerContext {
	return OwnerContext{
		CompanyID:          wf.CompanyID,
		DisclosureRecordID: wf.RecordID,
		WorkflowInstanceID: wf.WorkflowInstanceID,
		StepCode:           stepCode,
	}
}

func toFileDTO(f EvidenceFile, caps FileCapabilities) FileDTO {
	return FileDTO{
		FileID:       f.ID,
		FileName:     f.OriginalFileName,
		MimeType:     f.MimeType,
		FileSize:     f.FileSize,
		UploadedAt:   f.UploadedAt.UTC().Format(time.RFC3339Nano),
		UploadedBy:   f.UploadedBy,
		Capabilities: caps,
	}
}

func mapRepoErr(err error) error {
	if err == nil {
		return nil
	}
	var stepDone ErrStepCompleted
	if errors.As(err, &stepDone) {
		return perr.NewHTTPError(http.StatusConflict, perr.CodeWorkflowStepAlreadyCompleted, "workflow step already completed", nil)
	}
	var limit ErrActiveLimitReached
	if errors.As(err, &limit) {
		return perr.NewHTTPError(http.StatusConflict, perr.CodeWorkflowStepEvidenceFileLimitReached, "active evidence file limit reached", nil)
	}
	var notActive ErrNotActive
	if errors.As(err, &notActive) {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeWorkflowStepEvidenceFileNotFound, "evidence file not found", nil)
	}
	var badLife ErrInvalidLifecycleTransition
	if errors.As(err, &badLife) {
		return perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "invalid evidence lifecycle transition", nil)
	}
	return perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "evidence operation failed", err)
}

// mapFilePolicyErr remaps B2 shared ValidateUploadMeta DOCUMENT_FULFILLMENT_* codes
// to Generic Evidence-owned WORKFLOW_STEP_EVIDENCE_* codes (G2C domain leak cleanup).
func mapFilePolicyErr(err error) error {
	if err == nil {
		return nil
	}
	var he *perr.HTTPError
	if !errors.As(err, &he) || he == nil {
		return err
	}
	switch he.Code {
	case perr.CodeDocumentFulfillmentFileTooLarge:
		return perr.NewHTTPError(he.HTTPStatus, perr.CodeWorkflowStepEvidenceFileTooLarge, he.Message, he.Cause)
	case perr.CodeDocumentFulfillmentFileTypeInvalid:
		return perr.NewHTTPError(he.HTTPStatus, perr.CodeWorkflowStepEvidenceFileTypeInvalid, he.Message, he.Cause)
	case perr.CodeDocumentFulfillmentFileNotFound:
		return perr.NewHTTPError(he.HTTPStatus, perr.CodeWorkflowStepEvidenceFileNotFound, he.Message, he.Cause)
	case perr.CodeDocumentFulfillmentFileLimitReached:
		return perr.NewHTTPError(he.HTTPStatus, perr.CodeWorkflowStepEvidenceFileLimitReached, he.Message, he.Cause)
	default:
		return err
	}
}
