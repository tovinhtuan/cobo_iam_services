package workflowfulfillment

import (
	"context"
	"io"
	"time"

	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
)

// ObjectStorage is the DiskStorage-compatible write/read/delete surface.
type ObjectStorage interface {
	Write(objectKey string, body io.Reader) (int64, error)
	Read(objectKey string) ([]byte, error)
	Delete(objectKey string) error
	Exists(objectKey string) bool
}

// SnapshotReader reads B1 requirement snapshots (runtime authority).
type SnapshotReader interface {
	ListDocumentRequirementSnapshotsByInstanceStep(ctx context.Context, companyID, workflowInstanceID, stepCode string) ([]workflowapp.DocumentRequirementSnapshot, error)
	GetDocumentRequirementSnapshotByID(ctx context.Context, companyID, snapshotID string) (*workflowapp.DocumentRequirementSnapshot, error)
}

// DeadlineContext provides company-scoped deadline record + workflow + step runtime.
type DeadlineContext interface {
	LoadWorkflowForRecord(ctx context.Context, sub Subject, recordID string) (WorkflowContext, error)
	ListStepStates(ctx context.Context, workflowInstanceID string) (map[string]StepState, error)
	AuthorizeView(ctx context.Context, sub Subject) error
	AuthorizeMutation(ctx context.Context, sub Subject, recordID string) error
}

// WorkflowContext is the resolved runtime workflow for a disclosure record.
type WorkflowContext struct {
	WorkflowInstanceID string
	CompanyID          string
	RecordID           string
	T0Date             time.Time
	Timezone           string
	SnapshotJSONSteps  []workflowapp.StepSnapshot
}

// StepState is completion / incomplete runtime for a step.
type StepState struct {
	StepCode                       string
	CompletedAt                    *time.Time
	CompletedByMembershipID        string
	MarkedIncompleteAt             *time.Time
	MarkedIncompleteByMembershipID string
	IncompleteReason               string
	DelayDaysApplied               int
}

// Repository persists fulfillment files with transactional concurrency controls.
type Repository interface {
	ListActiveByRequirement(ctx context.Context, companyID, requirementSnapshotID string) ([]FulfillmentFile, error)
	ListActiveByInstanceStep(ctx context.Context, companyID, workflowInstanceID, stepCode string) ([]FulfillmentFile, error)
	GetByID(ctx context.Context, companyID, fileID string) (*FulfillmentFile, error)
	GetActiveByID(ctx context.Context, companyID, fileID string) (*FulfillmentFile, error)
	CountActiveByRequirement(ctx context.Context, requirementSnapshotID string) (int, error)

	// UploadInTx locks snapshot + step state, enforces active limit, inserts ACTIVE row.
	UploadInTx(ctx context.Context, in UploadTxInput) error
	// ReplaceInTx locks old ACTIVE file + snapshot + step, inserts new ACTIVE, marks old SUPERSEDED.
	ReplaceInTx(ctx context.Context, in ReplaceTxInput) error
	// DeleteInTx locks ACTIVE file + step, marks DELETED. Second delete returns not-found semantics via ErrNotActive.
	DeleteInTx(ctx context.Context, in DeleteTxInput) error
}

// UploadTxInput is the DB half of an upload after disk write.
type UploadTxInput struct {
	File                  FulfillmentFile
	RequirementSnapshotID string
	WorkflowInstanceID    string
	StepCode              string
	RequireNotCompleted   bool
	MaxActive             int
}

// ReplaceTxInput replaces an ACTIVE file with a new ACTIVE row (append-only).
type ReplaceTxInput struct {
	OldFileID             string
	NewFile               FulfillmentFile
	RequirementSnapshotID string
	WorkflowInstanceID    string
	StepCode              string
	CompanyID             string
	RequireNotCompleted   bool
}

// DeleteTxInput logically deletes an ACTIVE file.
type DeleteTxInput struct {
	FileID              string
	CompanyID           string
	WorkflowInstanceID  string
	StepCode            string
	DeletedBy           string
	DeletedAt           time.Time
	RequireNotCompleted bool
}

// ErrNotActive indicates the target file is not ACTIVE (deleted/superseded/missing for mutation).
type ErrNotActive struct {
	Reason string
}

func (e ErrNotActive) Error() string {
	if e.Reason == "" {
		return "fulfillment file not active"
	}
	return e.Reason
}

// ErrStepCompleted indicates step completion was observed under lock.
type ErrStepCompleted struct{}

func (ErrStepCompleted) Error() string { return "workflow step already completed" }

// ErrActiveLimitReached indicates active file count would exceed MaxActive.
type ErrActiveLimitReached struct{}

func (ErrActiveLimitReached) Error() string { return "active fulfillment file limit reached" }
