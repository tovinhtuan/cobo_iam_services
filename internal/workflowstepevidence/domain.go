package workflowstepevidence

import "time"

// Lifecycle statuses — same string values as B5 fulfillment (ACTIVE|SUPERSEDED|DELETED).
const (
	LifecycleActive     = "ACTIVE"
	LifecycleSuperseded = "SUPERSEDED"
	LifecycleDeleted    = "DELETED"
)

// MaxActiveFilesPerStep is the V1.1 hard operational limit for generic evidence
// on a single (workflow_instance_id, step_code). Independent of B2 fulfillment quota.
const MaxActiveFilesPerStep = 10

// MaxSizeBytes matches Evidence V1 / workflow document template policy (20 MiB).
const MaxSizeBytes = int64(20 * 1024 * 1024)

// OwnerContext binds a generic evidence row to a disclosure workflow step.
// No requirement_snapshot_id. No task ownership.
type OwnerContext struct {
	CompanyID          string
	DisclosureRecordID string
	WorkflowInstanceID string
	StepCode           string
}

// EvidenceFile is one immutable uploaded generic evidence version for a workflow step.
// StorageKey is internal — never expose in public DTOs (G2B).
type EvidenceFile struct {
	ID                 string
	CompanyID          string
	DisclosureRecordID string
	WorkflowInstanceID string
	StepCode           string
	StorageKey         string
	OriginalFileName   string
	MimeType           string
	FileSize           int64
	UploadedBy         string
	UploadedAt         time.Time
	LifecycleStatus    string
	SupersedesFileID   string
	SupersededByFileID string
	DeletedAt          *time.Time
	DeletedBy          string
}

// IsActive reports whether the file counts toward list/quota.
func (f EvidenceFile) IsActive() bool {
	return f.LifecycleStatus == LifecycleActive
}

// MatchesOwner reports full context binding (company+record+instance+step).
func (f EvidenceFile) MatchesOwner(owner OwnerContext) bool {
	return f.CompanyID == owner.CompanyID &&
		f.DisclosureRecordID == owner.DisclosureRecordID &&
		f.WorkflowInstanceID == owner.WorkflowInstanceID &&
		f.StepCode == owner.StepCode
}
