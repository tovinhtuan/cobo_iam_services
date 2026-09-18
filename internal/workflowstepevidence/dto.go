package workflowstepevidence

// FileDTO is an ACTIVE generic evidence file in the runtime list (no storage_key).
type FileDTO struct {
	FileID       string           `json:"file_id"`
	FileName     string           `json:"file_name"`
	MimeType     string           `json:"mime_type"`
	FileSize     int64            `json:"file_size"`
	UploadedAt   string           `json:"uploaded_at"`
	UploadedBy   string           `json:"uploaded_by"`
	Capabilities FileCapabilities `json:"capabilities"`
}

// FileCapabilities is BE-owned per-file mutation/download authority.
type FileCapabilities struct {
	CanDownload bool `json:"can_download"`
	CanDelete   bool `json:"can_delete"`
	CanReplace  bool `json:"can_replace"`
}

// ListCapabilities is BE-owned collection-level authority.
type ListCapabilities struct {
	CanUpload bool `json:"can_upload"`
}

// ListResponse is the GET evidence-files payload.
type ListResponse struct {
	RecordID           string           `json:"record_id"`
	WorkflowInstanceID string           `json:"workflow_instance_id"`
	StepCode           string           `json:"step_code"`
	Completed          bool             `json:"completed"`
	Capabilities       ListCapabilities `json:"capabilities"`
	Files              []FileDTO        `json:"files"`
}

// UploadResult is returned after upload or replace.
type UploadResult struct {
	File FileDTO `json:"file"`
}
