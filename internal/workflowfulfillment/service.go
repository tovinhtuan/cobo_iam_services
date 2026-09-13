package workflowfulfillment

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
	"github.com/google/uuid"
)

// Service implements B2 runtime document fulfillment.
type Service struct {
	deadline  DeadlineContext
	snapshots SnapshotReader
	files     Repository
	storage   ObjectStorage
	now       func() time.Time
	log       *slog.Logger
}

// NewService constructs the fulfillment service.
func NewService(deadline DeadlineContext, snapshots SnapshotReader, files Repository, storage ObjectStorage, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{
		deadline:  deadline,
		snapshots: snapshots,
		files:     files,
		storage:   storage,
		now:       time.Now,
		log:       log,
	}
}

// WithNow overrides the clock (tests).
func (s *Service) WithNow(now func() time.Time) *Service {
	s.now = now
	return s
}

func (s *Service) ListRequirements(ctx context.Context, sub Subject, recordID, stepCode string) (*RequirementsResponse, error) {
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
	isCurrent, isCompleted, err := evaluateStepAuthority(wf, states, stepCode, s.now())
	if err != nil {
		return nil, err
	}
	canMutatePerm := s.deadline.AuthorizeMutation(ctx, sub, recordID) == nil
	canMutate := canMutatePerm && isCurrent && !isCompleted
	canDownload := true // view already authorized

	snaps, err := s.snapshots.ListDocumentRequirementSnapshotsByInstanceStep(ctx, sub.CompanyID, wf.WorkflowInstanceID, stepCode)
	if err != nil {
		return nil, err
	}
	activeFiles, err := s.files.ListActiveByInstanceStep(ctx, sub.CompanyID, wf.WorkflowInstanceID, stepCode)
	if err != nil {
		return nil, err
	}
	byReq := map[string][]FulfillmentFile{}
	for _, f := range activeFiles {
		byReq[f.RequirementSnapshotID] = append(byReq[f.RequirementSnapshotID], f)
	}

	caps := CapabilitiesDTO{
		CanUpload:   canMutate,
		CanDelete:   canMutate,
		CanReplace:  canMutate,
		CanDownload: canDownload,
	}
	reqs := make([]RequirementDTO, 0, len(snaps))
	for _, snap := range snaps {
		var tpl *TemplateFileDTO
		if strings.TrimSpace(snap.TemplateFileID) != "" {
			tpl = &TemplateFileDTO{
				FileID:   snap.TemplateFileID,
				FileName: snap.TemplateFileName,
			}
		}
		files := byReq[snap.ID]
		fileDTOs := make([]FileDTO, 0, len(files))
		for _, f := range files {
			fileDTOs = append(fileDTOs, toFileDTO(f))
		}
		reqs = append(reqs, RequirementDTO{
			RequirementSnapshotID: snap.ID,
			SourceDocID:           snap.SourceDocID,
			Name:                  snap.Name,
			Required:              snap.Required,
			Ordinal:               snap.Ordinal,
			TemplateFile:          tpl,
			Files:                 fileDTOs,
			Capabilities:          caps,
		})
	}
	_ = isCurrent
	return &RequirementsResponse{
		RecordID:           wf.RecordID,
		WorkflowInstanceID: wf.WorkflowInstanceID,
		StepCode:           stepCode,
		Completed:          isCompleted,
		Requirements:       reqs,
	}, nil
}

func (s *Service) Upload(ctx context.Context, sub Subject, recordID, stepCode, requirementSnapshotID, fileName, contentType string, body io.Reader, sizeHint int64) (*UploadResult, error) {
	meta, err := s.prepareMutation(ctx, sub, recordID, stepCode)
	if err != nil {
		return nil, err
	}
	snap, err := s.resolveRequirement(ctx, sub.CompanyID, requirementSnapshotID, meta)
	if err != nil {
		return nil, err
	}
	fileName = SanitizeFileName(fileName)
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = GuessContentType(fileName)
	}
	if sizeHint <= 0 {
		sizeHint = MaxSizeBytes
	}
	if err := ValidateUploadMeta(fileName, contentType, sizeHint); err != nil {
		return nil, err
	}

	fileID := "wff_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	storageKey := FulfillmentObjectKey(sub.CompanyID, fileID, fileName)
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
		return nil, perr.NewHTTPError(http.StatusRequestEntityTooLarge, perr.CodeDocumentFulfillmentFileTooLarge, "file too large", nil)
	}
	if err := ValidateUploadMeta(fileName, contentType, written); err != nil {
		_ = s.storage.Delete(storageKey)
		return nil, err
	}

	now := s.now().UTC()
	row := FulfillmentFile{
		ID:                    fileID,
		CompanyID:             sub.CompanyID,
		DisclosureRecordID:    meta.wf.RecordID,
		WorkflowInstanceID:    meta.wf.WorkflowInstanceID,
		StepCode:              meta.stepCode,
		RequirementSnapshotID: snap.ID,
		StorageKey:            storageKey,
		OriginalFileName:      fileName,
		MimeType:              contentType,
		FileSize:              written,
		UploadedBy:            sub.UserID,
		UploadedAt:            now,
		LifecycleStatus:       LifecycleActive,
	}
	err = s.files.UploadInTx(ctx, UploadTxInput{
		File:                  row,
		RequirementSnapshotID: snap.ID,
		WorkflowInstanceID:    meta.wf.WorkflowInstanceID,
		StepCode:              meta.stepCode,
		RequireNotCompleted:   true,
		MaxActive:             MaxActiveFilesPerRequirement,
	})
	if err != nil {
		if delErr := s.storage.Delete(storageKey); delErr != nil {
			s.log.Error("fulfillment compensating delete failed",
				slog.String("storage_key_prefix", StorageNamespace),
				slog.String("file_id", fileID),
				slog.String("err", delErr.Error()),
			)
		}
		return nil, mapRepoErr(err)
	}
	return &UploadResult{File: toFileDTO(row)}, nil
}

func (s *Service) Download(ctx context.Context, sub Subject, recordID, stepCode, fileID string) (*FulfillmentFile, []byte, error) {
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
	if _, _, err := evaluateStepAuthority(wf, states, stepCode, s.now()); err != nil {
		return nil, nil, err
	}
	f, err := s.files.GetActiveByID(ctx, sub.CompanyID, fileID)
	if err != nil {
		return nil, nil, err
	}
	if f == nil || !fileMatchesContext(*f, wf, stepCode) {
		return nil, nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeDocumentFulfillmentFileNotFound, "fulfillment file not found", nil)
	}
	data, err := s.storage.Read(f.StorageKey)
	if err != nil {
		return nil, nil, perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "failed to read file", err)
	}
	return f, data, nil
}

func (s *Service) Delete(ctx context.Context, sub Subject, recordID, stepCode, fileID string) error {
	meta, err := s.prepareMutation(ctx, sub, recordID, stepCode)
	if err != nil {
		return err
	}
	f, err := s.files.GetActiveByID(ctx, sub.CompanyID, fileID)
	if err != nil {
		return err
	}
	if f == nil || !fileMatchesContext(*f, meta.wf, meta.stepCode) {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeDocumentFulfillmentFileNotFound, "fulfillment file not found", nil)
	}
	err = s.files.DeleteInTx(ctx, DeleteTxInput{
		FileID:              fileID,
		CompanyID:           sub.CompanyID,
		WorkflowInstanceID:  meta.wf.WorkflowInstanceID,
		StepCode:            meta.stepCode,
		DeletedBy:           sub.UserID,
		DeletedAt:           s.now().UTC(),
		RequireNotCompleted: true,
	})
	return mapRepoErr(err)
}

func (s *Service) Replace(ctx context.Context, sub Subject, recordID, stepCode, fileID, fileName, contentType string, body io.Reader, sizeHint int64) (*UploadResult, error) {
	meta, err := s.prepareMutation(ctx, sub, recordID, stepCode)
	if err != nil {
		return nil, err
	}
	old, err := s.files.GetActiveByID(ctx, sub.CompanyID, fileID)
	if err != nil {
		return nil, err
	}
	if old == nil || !fileMatchesContext(*old, meta.wf, meta.stepCode) {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeDocumentFulfillmentFileNotFound, "fulfillment file not found", nil)
	}
	fileName = SanitizeFileName(fileName)
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = GuessContentType(fileName)
	}
	if sizeHint <= 0 {
		sizeHint = MaxSizeBytes
	}
	if err := ValidateUploadMeta(fileName, contentType, sizeHint); err != nil {
		return nil, err
	}

	newID := "wff_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	storageKey := FulfillmentObjectKey(sub.CompanyID, newID, fileName)
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
		return nil, perr.NewHTTPError(http.StatusRequestEntityTooLarge, perr.CodeDocumentFulfillmentFileTooLarge, "file too large", nil)
	}

	now := s.now().UTC()
	row := FulfillmentFile{
		ID:                    newID,
		CompanyID:             sub.CompanyID,
		DisclosureRecordID:    meta.wf.RecordID,
		WorkflowInstanceID:    meta.wf.WorkflowInstanceID,
		StepCode:              meta.stepCode,
		RequirementSnapshotID: old.RequirementSnapshotID,
		StorageKey:            storageKey,
		OriginalFileName:      fileName,
		MimeType:              contentType,
		FileSize:              written,
		UploadedBy:            sub.UserID,
		UploadedAt:            now,
		LifecycleStatus:       LifecycleActive,
		SupersedesFileID:      old.ID,
	}
	err = s.files.ReplaceInTx(ctx, ReplaceTxInput{
		OldFileID:             old.ID,
		NewFile:               row,
		RequirementSnapshotID: old.RequirementSnapshotID,
		WorkflowInstanceID:    meta.wf.WorkflowInstanceID,
		StepCode:              meta.stepCode,
		CompanyID:             sub.CompanyID,
		RequireNotCompleted:   true,
	})
	if err != nil {
		if delErr := s.storage.Delete(storageKey); delErr != nil {
			s.log.Error("fulfillment replace compensating delete failed",
				slog.String("storage_key_prefix", StorageNamespace),
				slog.String("file_id", newID),
				slog.String("err", delErr.Error()),
			)
		}
		return nil, mapRepoErr(err)
	}
	return &UploadResult{File: toFileDTO(row)}, nil
}

type mutationMeta struct {
	wf       WorkflowContext
	stepCode string
}

func (s *Service) prepareMutation(ctx context.Context, sub Subject, recordID, stepCode string) (*mutationMeta, error) {
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
	isCurrent, isCompleted, err := evaluateStepAuthority(wf, states, stepCode, s.now())
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

func (s *Service) resolveRequirement(ctx context.Context, companyID, requirementSnapshotID string, meta *mutationMeta) (*workflowapp.DocumentRequirementSnapshot, error) {
	requirementSnapshotID = strings.TrimSpace(requirementSnapshotID)
	if requirementSnapshotID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "requirement_snapshot_id is required", nil)
	}
	snap, err := s.snapshots.GetDocumentRequirementSnapshotByID(ctx, companyID, requirementSnapshotID)
	if err != nil {
		return nil, err
	}
	if snap == nil {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeDocumentRequirementSnapshotNotFound, "document requirement snapshot not found", nil)
	}
	if snap.CompanyID != companyID ||
		snap.DisclosureRecordID != meta.wf.RecordID ||
		snap.WorkflowInstanceID != meta.wf.WorkflowInstanceID ||
		snap.StepCode != meta.stepCode {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeDocumentRequirementSnapshotNotFound, "document requirement snapshot not found", nil)
	}
	return snap, nil
}

func fileMatchesContext(f FulfillmentFile, wf WorkflowContext, stepCode string) bool {
	return f.CompanyID == wf.CompanyID &&
		f.DisclosureRecordID == wf.RecordID &&
		f.WorkflowInstanceID == wf.WorkflowInstanceID &&
		f.StepCode == stepCode
}

func toFileDTO(f FulfillmentFile) FileDTO {
	return FileDTO{
		FileID:     f.ID,
		FileName:   f.OriginalFileName,
		MimeType:   f.MimeType,
		FileSize:   f.FileSize,
		UploadedAt: f.UploadedAt.UTC().Format(time.RFC3339Nano),
		UploadedBy: f.UploadedBy,
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
		return perr.NewHTTPError(http.StatusConflict, perr.CodeDocumentFulfillmentFileLimitReached, "active fulfillment file limit reached", nil)
	}
	var notActive ErrNotActive
	if errors.As(err, &notActive) {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeDocumentFulfillmentFileNotFound, "fulfillment file not found", nil)
	}
	if strings.Contains(err.Error(), "requirement snapshot not found") {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeDocumentRequirementSnapshotNotFound, "document requirement snapshot not found", nil)
	}
	return err
}
