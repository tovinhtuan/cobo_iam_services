package workflowstepevidence

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Foundation is the G2A runtime helper (disk + repository). No HTTP. No audit.
// G2B will wrap this with ACL (deadline.view / deadline.confirm + current step).
type Foundation struct {
	files   Repository
	storage ObjectStorage
	now     func() time.Time
	log     *slog.Logger
}

// NewFoundation constructs the G2A foundation.
func NewFoundation(files Repository, storage ObjectStorage, log *slog.Logger) *Foundation {
	if log == nil {
		log = slog.Default()
	}
	return &Foundation{files: files, storage: storage, now: time.Now, log: log}
}

// WithNow overrides the clock (tests).
func (f *Foundation) WithNow(now func() time.Time) *Foundation {
	f.now = now
	return f
}

// ListActive returns ACTIVE files only for the owner context (deterministic order via repo).
func (f *Foundation) ListActive(ctx context.Context, owner OwnerContext) ([]EvidenceFile, error) {
	return f.files.ListActiveByStep(ctx, owner)
}

// CreateActive writes disk then inserts ACTIVE under step lock + quota.
// On DB failure, compensates by deleting the newly written physical object.
func (f *Foundation) CreateActive(
	ctx context.Context,
	owner OwnerContext,
	uploadedBy, fileName, contentType string,
	body io.Reader,
	sizeHint int64,
) (*EvidenceFile, error) {
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		fileName = "file.bin"
	}
	fileID := "wse_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	storageKey := EvidenceObjectKey(owner.CompanyID, fileID, fileName)
	limit := MaxSizeBytes
	if sizeHint > 0 && sizeHint < MaxSizeBytes {
		limit = sizeHint
	}
	written, err := f.storage.Write(storageKey, io.LimitReader(body, limit+1))
	if err != nil {
		return nil, err
	}
	if written <= 0 || written > MaxSizeBytes {
		_ = f.storage.Delete(storageKey)
		return nil, ErrInvalidUpload{Reason: "invalid file size"}
	}
	now := f.now().UTC()
	row := EvidenceFile{
		ID:                 fileID,
		CompanyID:          owner.CompanyID,
		DisclosureRecordID: owner.DisclosureRecordID,
		WorkflowInstanceID: owner.WorkflowInstanceID,
		StepCode:           owner.StepCode,
		StorageKey:         storageKey,
		OriginalFileName:   fileName,
		MimeType:           contentType,
		FileSize:           written,
		UploadedBy:         uploadedBy,
		UploadedAt:         now,
		LifecycleStatus:    LifecycleActive,
	}
	err = f.files.CreateActiveInTx(ctx, CreateTxInput{
		File:                row,
		Owner:               owner,
		RequireNotCompleted: true,
		MaxActive:           MaxActiveFilesPerStep,
	})
	if err != nil {
		if delErr := f.storage.Delete(storageKey); delErr != nil {
			f.log.Error("evidence compensating delete failed",
				slog.String("storage_namespace", StorageNamespace),
				slog.String("file_id", fileID),
				slog.String("err", delErr.Error()),
			)
		}
		return nil, err
	}
	return &row, nil
}

// DeleteLogical marks ACTIVE → DELETED. No physical unlink.
func (f *Foundation) DeleteLogical(ctx context.Context, owner OwnerContext, fileID, deletedBy string) error {
	return f.files.DeleteInTx(ctx, DeleteTxInput{
		FileID:              fileID,
		Owner:               owner,
		DeletedBy:           deletedBy,
		DeletedAt:           f.now().UTC(),
		RequireNotCompleted: true,
	})
}

// ReplaceAppendOnly writes a new physical object, then SUPERSEDES old + inserts new ACTIVE.
// On DB failure, compensates the new physical object; old remains ACTIVE.
func (f *Foundation) ReplaceAppendOnly(
	ctx context.Context,
	owner OwnerContext,
	oldFileID, uploadedBy, fileName, contentType string,
	body io.Reader,
	sizeHint int64,
) (*EvidenceFile, error) {
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		fileName = "file.bin"
	}
	fileID := "wse_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	storageKey := EvidenceObjectKey(owner.CompanyID, fileID, fileName)
	limit := MaxSizeBytes
	if sizeHint > 0 && sizeHint < MaxSizeBytes {
		limit = sizeHint
	}
	written, err := f.storage.Write(storageKey, io.LimitReader(body, limit+1))
	if err != nil {
		return nil, err
	}
	if written <= 0 || written > MaxSizeBytes {
		_ = f.storage.Delete(storageKey)
		return nil, ErrInvalidUpload{Reason: "invalid file size"}
	}
	now := f.now().UTC()
	row := EvidenceFile{
		ID:                 fileID,
		CompanyID:          owner.CompanyID,
		DisclosureRecordID: owner.DisclosureRecordID,
		WorkflowInstanceID: owner.WorkflowInstanceID,
		StepCode:           owner.StepCode,
		StorageKey:         storageKey,
		OriginalFileName:   fileName,
		MimeType:           contentType,
		FileSize:           written,
		UploadedBy:         uploadedBy,
		UploadedAt:         now,
		LifecycleStatus:    LifecycleActive,
		SupersedesFileID:   oldFileID,
	}
	err = f.files.ReplaceInTx(ctx, ReplaceTxInput{
		OldFileID:           oldFileID,
		NewFile:             row,
		Owner:               owner,
		RequireNotCompleted: true,
	})
	if err != nil {
		if delErr := f.storage.Delete(storageKey); delErr != nil {
			f.log.Error("evidence replace compensating delete failed",
				slog.String("storage_namespace", StorageNamespace),
				slog.String("file_id", fileID),
				slog.String("err", delErr.Error()),
			)
		}
		return nil, err
	}
	return &row, nil
}
