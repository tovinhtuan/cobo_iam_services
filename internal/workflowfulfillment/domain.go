package workflowfulfillment

import "time"

// Lifecycle statuses for fulfillment file rows.
const (
	LifecycleActive     = "ACTIVE"
	LifecycleSuperseded = "SUPERSEDED"
	LifecycleDeleted    = "DELETED"
)

// MaxActiveFilesPerRequirement is the V1 hard operational limit.
const MaxActiveFilesPerRequirement = 10

// MaxSizeBytes matches workflow document template policy (20 MiB).
const MaxSizeBytes = int64(20 * 1024 * 1024)

// FulfillmentFile is one immutable uploaded file version attached to a requirement snapshot.
type FulfillmentFile struct {
	ID                    string
	CompanyID             string
	DisclosureRecordID    string
	WorkflowInstanceID    string
	StepCode              string
	RequirementSnapshotID string
	StorageKey            string
	OriginalFileName      string
	MimeType              string
	FileSize              int64
	UploadedBy            string
	UploadedAt            time.Time
	LifecycleStatus       string
	SupersedesFileID      string
	SupersededByFileID    string
	DeletedAt             *time.Time
	DeletedBy             string
}

// IsActive reports whether the file counts toward list/limit/future B3 satisfaction.
func (f FulfillmentFile) IsActive() bool {
	return f.LifecycleStatus == LifecycleActive
}

// Subject is the authenticated tenant actor.
type Subject struct {
	UserID       string
	MembershipID string
	CompanyID    string
}

// TemplateFileDTO is an optional authoring template reference from the snapshot.
type TemplateFileDTO struct {
	FileID   string `json:"file_id"`
	FileName string `json:"file_name"`
}

// FileDTO is an active fulfillment file in the runtime list (no storage_key).
type FileDTO struct {
	FileID     string `json:"file_id"`
	FileName   string `json:"file_name"`
	MimeType   string `json:"mime_type"`
	FileSize   int64  `json:"file_size"`
	UploadedAt string `json:"uploaded_at"`
	UploadedBy string `json:"uploaded_by"`
}

// CapabilitiesDTO is BE-owned mutation/download authority for a requirement.
type CapabilitiesDTO struct {
	CanUpload   bool `json:"can_upload"`
	CanDownload bool `json:"can_download"`
	CanDelete   bool `json:"can_delete"`
	CanReplace  bool `json:"can_replace"`
}

// RequirementDTO is one runtime snapshot requirement with active files.
type RequirementDTO struct {
	RequirementSnapshotID string           `json:"requirement_snapshot_id"`
	SourceDocID           string           `json:"source_doc_id"`
	Name                  string           `json:"name"`
	Required              bool             `json:"required"`
	Ordinal               int              `json:"ordinal"`
	TemplateFile          *TemplateFileDTO `json:"template_file"`
	Files                 []FileDTO        `json:"files"`
	Capabilities          CapabilitiesDTO  `json:"capabilities"`
}

// RequirementsResponse is the GET document-requirements payload.
type RequirementsResponse struct {
	RecordID           string           `json:"record_id"`
	WorkflowInstanceID string           `json:"workflow_instance_id"`
	StepCode           string           `json:"step_code"`
	Completed          bool             `json:"completed"`
	Requirements       []RequirementDTO `json:"requirements"`
}

// UploadResult is returned after upload or replace.
type UploadResult struct {
	File FileDTO `json:"file"`
}
