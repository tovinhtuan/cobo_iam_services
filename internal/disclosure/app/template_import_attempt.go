package app

import (
	"context"
	"time"
)

// TemplateImportAttempt is the redacted history row. It never holds a raw file or a raw token.
type TemplateImportAttempt struct {
	ID                     string
	CompanyID              string
	ActorUserID            string
	ActorMembershipID      string
	Filename               string
	FileSizeBytes          int
	FileSHA256             string
	CanonicalPayloadSHA256 string
	ValidationTokenSHA256  string
	SchemaVersion          string
	Status                 string
	ParseValid             bool
	DomainValid            bool
	ActivationReady        bool
	CanConfirm             bool
	MappingRequired        bool
	RequiredMappingCount   int
	ResolvedMappingCount   int
	UnresolvedMappingCount int
	ErrorCodes             []TemplateImportValidationIssueDTO
	MappingSummary         []TemplateImportRequiredMappingDTO
	TargetTypeID           string
	CreatedTypeID          string
	ConfirmErrorCode       string
	RowVersion             int64
	CreatedAt              time.Time
	ValidatedAt            time.Time
	ConfirmedAt            time.Time
	UpdatedAt              time.Time
	ConfirmingAt           time.Time
	LeaseExpiresAt         time.Time
}

// ImportAttemptStore persists redacted import attempts.
type ImportAttemptStore interface {
	CreateImportAttempt(ctx context.Context, row *TemplateImportAttempt) error
	GetImportAttemptByID(ctx context.Context, id string) (*TemplateImportAttempt, error)
	ClaimImportAttempt(ctx context.Context, id, companyID, actorUserID, targetTypeID string, rowVersion int64, cutoff, now time.Time) (int64, error)
	MarkImportAttemptConfirmed(ctx context.Context, id string, rowVersion int64, createdTypeID, targetTypeID string, at time.Time) error
	ReconcileExpiredImportAttempt(ctx context.Context, id string, rowVersion int64, createdTypeID string, now time.Time) error
	MarkImportAttemptFailed(ctx context.Context, id string, rowVersion int64, errorCode, targetTypeID string, at time.Time) error
	ListImportAttempts(ctx context.Context, q ListTemplateImportHistoryRequest) ([]TemplateImportAttempt, error)
}

func importAttemptStore(repo Repository) (ImportAttemptStore, bool) {
	store, ok := repo.(ImportAttemptStore)
	return store, ok && store != nil
}
