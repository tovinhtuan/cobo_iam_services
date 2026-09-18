package workflowstepevidence

import (
	"context"
	"io"
	"time"
)

// ObjectStorage is the DiskStorage-compatible write/read/delete surface.
type ObjectStorage interface {
	Write(objectKey string, body io.Reader) (int64, error)
	Read(objectKey string) ([]byte, error)
	Delete(objectKey string) error
	Exists(objectKey string) bool
}

// Repository persists generic evidence files with transactional concurrency controls.
type Repository interface {
	ListActiveByStep(ctx context.Context, owner OwnerContext) ([]EvidenceFile, error)
	CountActiveByStep(ctx context.Context, owner OwnerContext) (int, error)
	GetByIDInContext(ctx context.Context, owner OwnerContext, fileID string) (*EvidenceFile, error)
	GetActiveByIDInContext(ctx context.Context, owner OwnerContext, fileID string) (*EvidenceFile, error)

	// CreateActiveInTx locks step state, enforces active limit, inserts ACTIVE row.
	CreateActiveInTx(ctx context.Context, in CreateTxInput) error
	// ReplaceInTx locks step + old ACTIVE, marks SUPERSEDED, inserts new ACTIVE (net count unchanged).
	ReplaceInTx(ctx context.Context, in ReplaceTxInput) error
	// DeleteInTx locks step + ACTIVE file, marks DELETED (logical only).
	DeleteInTx(ctx context.Context, in DeleteTxInput) error
}

// CreateTxInput is the DB half of an upload after disk write.
type CreateTxInput struct {
	File                EvidenceFile
	Owner               OwnerContext
	RequireNotCompleted bool
	MaxActive           int
}

// ReplaceTxInput replaces an ACTIVE file with a new ACTIVE row (append-only).
type ReplaceTxInput struct {
	OldFileID           string
	NewFile             EvidenceFile
	Owner               OwnerContext
	RequireNotCompleted bool
}

// DeleteTxInput logically deletes an ACTIVE file.
type DeleteTxInput struct {
	FileID              string
	Owner               OwnerContext
	DeletedBy           string
	DeletedAt           time.Time
	RequireNotCompleted bool
}

// ErrNotActive indicates the target file is not ACTIVE for mutation.
type ErrNotActive struct {
	Reason string
}

func (e ErrNotActive) Error() string {
	if e.Reason == "" {
		return "evidence file not active"
	}
	return e.Reason
}

// ErrStepCompleted indicates step completion was observed under lock.
type ErrStepCompleted struct{}

func (ErrStepCompleted) Error() string { return "workflow step already completed" }

// ErrActiveLimitReached indicates active file count would exceed MaxActive.
type ErrActiveLimitReached struct{}

func (ErrActiveLimitReached) Error() string { return "active evidence file limit reached" }

// ErrInvalidUpload indicates empty / oversized payload before DB insert.
type ErrInvalidUpload struct {
	Reason string
}

func (e ErrInvalidUpload) Error() string {
	if e.Reason == "" {
		return "invalid evidence upload"
	}
	return e.Reason
}
