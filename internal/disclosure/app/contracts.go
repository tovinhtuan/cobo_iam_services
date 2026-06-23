package app

import (
	"context"
	"errors"
	"time"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
)

// ErrDuplicateRecordID is returned when CreateRecord is called with an explicit
// RecordID that already exists (unique-key conflict on disclosure_records.record_id).
// Callers that pre-allocate deterministic record IDs (e.g. ad-hoc approval) treat
// this as an idempotent-replay signal rather than a hard failure.
var ErrDuplicateRecordID = errors.New("disclosure record id already exists")

type Service interface {
	CreateRecord(ctx context.Context, req CreateRecordRequest) (*RecordDTO, error)
	UpdateRecord(ctx context.Context, req UpdateRecordRequest) (*RecordDTO, error)
	SubmitRecord(ctx context.Context, req SubmitRecordRequest) (*RecordDTO, error)
	ConfirmRecord(ctx context.Context, req ConfirmRecordRequest) (*RecordDTO, error)
	ListRecords(ctx context.Context, req ListRecordsRequest) (*ListRecordsResponse, error)
	GetRecord(ctx context.Context, req GetRecordRequest) (*RecordDTO, error)
	ListTypeGroups(ctx context.Context, req ListTypeGroupsRequest) (*ListTypeGroupsResponse, error)
	ListDisplayGroups(ctx context.Context, req ListDisplayGroupsRequest) (*ListDisplayGroupsResponse, error)
	ListTypes(ctx context.Context, req ListTypesRequest) (*ListTypesResponse, error)
	GetTypeDetail(ctx context.Context, req GetTypeDetailRequest) (*DisclosureTypeDTO, error)
	GetTypeVersionDetail(ctx context.Context, req GetTypeVersionDetailRequest) (*DisclosureTypeDTO, error)
	GetTemplateReferenceData(ctx context.Context, req GetTemplateReferenceDataRequest) (*GetTemplateReferenceDataResponse, error)
	UpsertTypeVersion(ctx context.Context, req UpsertTypeVersionRequest) (*UpsertTypeVersionResponse, error)
	ListTypeVersions(ctx context.Context, req ListTypeVersionsRequest) (*ListTypeVersionsResponse, error)
	ActivateTypeVersion(ctx context.Context, req ActivateTypeVersionRequest) (*ActivateTypeVersionResponse, error)
	GetCompanyWorkflowOverride(ctx context.Context, req GetCompanyWorkflowOverrideRequest) (*GetCompanyWorkflowOverrideResponse, error)
	UpsertCompanyWorkflowOverrideDraft(ctx context.Context, req UpsertCompanyWorkflowOverrideDraftRequest) (*UpsertCompanyWorkflowOverrideDraftResponse, error)
	ApproveCompanyWorkflowOverride(ctx context.Context, req ApproveCompanyWorkflowOverrideRequest) (*ApproveCompanyWorkflowOverrideResponse, error)
	DeleteCompanyWorkflowOverrideDraft(ctx context.Context, req DeleteCompanyWorkflowOverrideDraftRequest) (*DeleteCompanyWorkflowOverrideDraftResponse, error)
	ResetCompanyWorkflowOverrideActive(ctx context.Context, req ResetCompanyWorkflowOverrideActiveRequest) (*ResetCompanyWorkflowOverrideActiveResponse, error)
	ListCompanyWorkflowOverrideVersions(ctx context.Context, req ListCompanyWorkflowOverrideVersionsRequest) (*ListCompanyWorkflowOverrideVersionsResponse, error)
	GetCompanyWorkflowOverrideDraftReminderPreview(ctx context.Context, req GetCompanyWorkflowOverrideDraftReminderPreviewRequest) (*GetCompanyWorkflowOverrideDraftReminderPreviewResponse, error)
	// Sprint 3 / Batch 2 — Workflow Override Staleness Detection.
	GetWorkflowOverrideStatus(ctx context.Context, req GetWorkflowOverrideStatusRequest) (*GetWorkflowOverrideStatusResponse, error)
	RebaseCheckWorkflowOverride(ctx context.Context, req RebaseCheckWorkflowOverrideRequest) (*RebaseCheckWorkflowOverrideResponse, error)
	GetWorkflowOverrideRebasePreview(ctx context.Context, req GetWorkflowOverrideRebasePreviewRequest) (*GetWorkflowOverrideRebasePreviewResponse, error)
	ResolveWorkflowOverrideConflict(ctx context.Context, req ResolveWorkflowOverrideConflictRequest) (*ResolveWorkflowOverrideConflictResponse, error)
	// Sprint 3 / Batch 5 — Workflow Override Rebase Apply. The only Sprint 3 service method
	// allowed to change what GetEffectiveWorkflow subsequently returns.
	ApplyWorkflowOverrideRebase(ctx context.Context, req ApplyWorkflowOverrideRebaseRequest) (*ApplyWorkflowOverrideRebaseResponse, error)
	GetEffectiveWorkflow(ctx context.Context, req GetEffectiveWorkflowRequest) (*GetEffectiveWorkflowResponse, error)
	GetTemplateDeadlineConfig(ctx context.Context, req GetTemplateDeadlineConfigRequest) (*GetTemplateDeadlineConfigResponse, error)
	UpdateTemplateDeadlineConfig(ctx context.Context, req UpdateTemplateDeadlineConfigRequest) (*UpdateTemplateDeadlineConfigResponse, error)
	ListCompanyGroups(ctx context.Context, req ListCompanyGroupsRequest) (*ListCompanyGroupsResponse, error)
	UpdateWorkflowOverrideStepGroups(ctx context.Context, req UpdateWorkflowOverrideStepGroupsRequest) (*UpdateWorkflowOverrideStepGroupsResponse, error)

	// Company-defined template lifecycle (BE-004A / BE-004B).
	CreateCompanyTemplate(ctx context.Context, req CreateCompanyTemplateRequest) (*CompanyTemplateWriteResponse, error)
	UpdateCompanyTemplate(ctx context.Context, req UpdateCompanyTemplateRequest) (*CompanyTemplateWriteResponse, error)
	TransitionCompanyTemplateLifecycle(ctx context.Context, req TransitionCompanyTemplateLifecycleRequest) (*CompanyTemplateWriteResponse, error)

	// Periodic auto-creation.
	SeedPeriodicCycles(ctx context.Context, now time.Time) (int, error)
	MaterializePeriodicDisclosures(ctx context.Context, now time.Time, creator PeriodicRecordCreator) (int, error)

	// Company preferences (auto_create toggle).
	GetCompanyTypePreference(ctx context.Context, req GetCompanyTypePreferenceRequest) (*CompanyTypePreferenceDTO, error)
	UpsertCompanyTypePreference(ctx context.Context, req UpsertCompanyTypePreferenceRequest) (*CompanyTypePreferenceDTO, error)

	// CMS system template management (Sprint 2).
	CmsArchiveTemplate(ctx context.Context, req CmsArchiveTemplateRequest) (*CmsArchiveTemplateResponse, error)
	CmsGetGlobalWorkflow(ctx context.Context, req CmsGetGlobalWorkflowRequest) (*CmsGetGlobalWorkflowResponse, error)
	CmsUpsertGlobalWorkflow(ctx context.Context, req CmsUpsertGlobalWorkflowRequest) (*GlobalWorkflowDTO, error)
	CmsDeleteGlobalWorkflow(ctx context.Context, req CmsDeleteGlobalWorkflowRequest) error
	CmsListDisplayGroupsCatalog(ctx context.Context, req ListDisplayGroupsRequest) (*ListDisplayGroupsResponse, error)
	CmsCreateDisplayGroup(ctx context.Context, req CmsDisplayGroupCreateRequest) (*DisplayGroupDTO, error)
	CmsUpdateDisplayGroup(ctx context.Context, req CmsDisplayGroupUpdateRequest) (*DisplayGroupDTO, error)
	CmsDeleteDisplayGroup(ctx context.Context, req CmsDisplayGroupDeleteRequest) error
	CmsListDeadlineRules(ctx context.Context, req GetTemplateReferenceDataRequest) ([]CmsDeadlineRuleDTO, error)
	CmsCreateDeadlineRule(ctx context.Context, req CmsDeadlineRuleCreateRequest) (*CmsDeadlineRuleDTO, error)
	CmsUpdateDeadlineRule(ctx context.Context, req CmsDeadlineRuleUpdateRequest) (*CmsDeadlineRuleDTO, error)
	CmsDeleteDeadlineRule(ctx context.Context, req CmsDeadlineRuleDeleteRequest) error
}

type Repository interface {
	Create(ctx context.Context, rec RecordDTO) (*RecordDTO, error)
	Update(ctx context.Context, rec RecordDTO) (*RecordDTO, error)
	FindByID(ctx context.Context, companyID, recordID string) (*RecordDTO, error)
	List(ctx context.Context, companyID string) ([]RecordDTO, error)
	ListTypeGroups(ctx context.Context, companyID string) ([]DisclosureGroupDTO, error)
	ListDisplayGroups(ctx context.Context) ([]DisplayGroupDTO, error)
	ListTypes(ctx context.Context, params ListTypesParams) ([]DisclosureTypeSummaryDTO, int, error)
	GetTypeDetail(ctx context.Context, companyID, typeID string) (*DisclosureTypeDTO, error)
	GetTypeVersionDetail(ctx context.Context, companyID, typeID string, versionNo int) (*DisclosureTypeDTO, error)
	HasActiveEnterpriseWorkflow(ctx context.Context, companyID, typeID string) (bool, error)
	UpsertTypeVersion(ctx context.Context, req UpsertTypeVersionRequest) (*UpsertTypeVersionResponse, error)
	ListTypeVersions(ctx context.Context, companyID, typeID string) ([]DisclosureTypeVersionDTO, error)
	ActivateTypeVersion(ctx context.Context, req ActivateTypeVersionRequest) (*ActivateTypeVersionResponse, error)
	GetCompanyWorkflowOverride(ctx context.Context, companyID, typeID string) (*CompanyWorkflowOverrideViewDTO, error)
	UpsertCompanyWorkflowOverrideDraft(ctx context.Context, req UpsertCompanyWorkflowOverrideDraftRequest) (*UpsertCompanyWorkflowOverrideDraftResponse, error)
	ApproveCompanyWorkflowOverride(ctx context.Context, req ApproveCompanyWorkflowOverrideRequest) (*ApproveCompanyWorkflowOverrideResponse, error)
	DeleteCompanyWorkflowOverrideDraft(ctx context.Context, req DeleteCompanyWorkflowOverrideDraftRequest) (*DeleteCompanyWorkflowOverrideDraftResponse, error)
	ResetCompanyWorkflowOverrideActive(ctx context.Context, req ResetCompanyWorkflowOverrideActiveRequest) (*ResetCompanyWorkflowOverrideActiveResponse, error)
	ListCompanyWorkflowOverrideVersions(ctx context.Context, companyID, typeID string, page, pageSize int) ([]CompanyWorkflowOverrideVersionDTO, int, error)
	// Sprint 3 / Batch 2 — Workflow Override Staleness Detection. None of these four methods is
	// called by, or calls, GetEffectiveWorkflow/GetCompanyWorkflowOverride below.
	TypeExists(ctx context.Context, typeID string) (bool, error)
	GetOverrideStalenessMetadata(ctx context.Context, companyID, typeID string) (*OverrideStalenessRow, bool, error)
	GetCurrentGlobalActiveVersionNo(ctx context.Context, typeID string) (*int, error)
	UpdateOverrideStaleness(ctx context.Context, companyID, typeID, staleStatus string, checkedAt time.Time) error
	// Sprint 3 / Batch 3 — Workflow Override Rebase Preview. Read-only (a single SELECT); not
	// called by, and does not call, GetEffectiveWorkflow/GetCompanyWorkflowOverride. ok=false
	// means no such (typeID, versionNo) row exists.
	GetGlobalWorkflowVersionManifest(ctx context.Context, typeID string, versionNo int) ([]GlobalWorkflowStepInput, bool, error)
	// Sprint 3 / Batch 4 — Workflow Override Conflict Detection. UpsertWorkflowOverrideConflicts
	// writes ONLY workflow_override_conflicts (insert-or-update on the unique conflict_key —
	// Option B idempotency, PREFLIGHT_AUDIT.md §8). GetWorkflowOverrideConflict /
	// ResolveWorkflowOverrideConflict are scoped to (companyID, typeID) — never cross-tenant.
	UpsertWorkflowOverrideConflicts(ctx context.Context, inputs []PersistedConflictInput) ([]PersistedConflictDTO, error)
	GetWorkflowOverrideConflict(ctx context.Context, companyID, typeID, conflictID string) (*PersistedConflictDTO, error)
	ResolveWorkflowOverrideConflict(ctx context.Context, companyID, typeID, conflictID, resolution string, resolutionValue any, resolvedBy string, resolvedAt time.Time) (*PersistedConflictDTO, error)
	// Sprint 3 / Batch 5 — Workflow Override Rebase Apply. Writes ONLY
	// company_template_workflow_override_versions (insert) and
	// company_template_workflow_overrides (active_version_no/status/stale_status/base_version_no/
	// base_hash/last_rebase_check_at/approved_by/approved_at/updated_by/updated_at), in one
	// transaction (DB_WRITE_BOUNDARY_REPORT.md). Returns 409 STATE_CONFLICT if
	// ExpectedActiveVersionNo no longer matches at commit time (race guard).
	ApplyWorkflowOverrideRebase(ctx context.Context, params ApplyWorkflowOverrideRebaseParams) (*ApplyWorkflowOverrideRebaseResult, error)
	GetEffectiveWorkflow(ctx context.Context, companyID, typeID string) (*EffectiveWorkflowDTO, error)
	GetCompanyDeadlineContext(ctx context.Context, companyID string) (CompanyDeadlineContext, error)
	GetCompanyApplicabilityProfile(ctx context.Context, companyID string) (applicability.CompanyApplicabilityProfile, error)
	// GetCompanyTypeDeadlineContext returns CompanyDeadlineContext enriched with
	// per-company cycle anchor override from company_type_preferences.
	GetCompanyTypeDeadlineContext(ctx context.Context, companyID, typeID string) (CompanyDeadlineContext, error)
	GetActiveVersionDeadlineConfig(ctx context.Context, typeID string) (versionNo int, cfg *TemplateDeadlineConfig, err error)
	UpdateActiveVersionDeadlineConfig(ctx context.Context, typeID string, cfg TemplateDeadlineConfig, updatedBy string) error
	ListCompanyGroups(ctx context.Context, companyID, departmentID string, isActive *bool) ([]CompanyGroupDTO, error)
	UpdateWorkflowOverrideStepGroups(ctx context.Context, req UpdateWorkflowOverrideStepGroupsRequest) (*UpdateWorkflowOverrideStepGroupsResponse, error)

	// Company-defined template persistence (BE-004A / BE-004B).
	CreateCompanyTemplate(ctx context.Context, req CreateCompanyTemplateRequest) (*CompanyTemplateWriteResponse, error)
	UpdateCompanyTemplate(ctx context.Context, req UpdateCompanyTemplateRequest) (*CompanyTemplateWriteResponse, error)
	GetCompanyTemplateForLifecycle(ctx context.Context, companyID, typeID string) (*CompanyTemplateWriteResponse, error)
	TransitionCompanyTemplateReviewStatus(ctx context.Context, companyID, typeID, newStatus, updatedBy string) error

	// Periodic auto-creation support.
	ListActivePeriodicTypes(ctx context.Context) ([]PeriodicTypeRow, error)
	UpsertPeriodicCycle(ctx context.Context, in PeriodicCycleRow) error
	ListPendingCycles(ctx context.Context, asOf time.Time, bufferDays int) ([]PeriodicCycleRow, error)
	TryClaimPeriodicCycle(ctx context.Context, cycleID string) (bool, error)
	ReleasePeriodicCycleClaim(ctx context.Context, cycleID string) error
	UpdateCycleRecord(ctx context.Context, cycleID, recordID string) error
	ListAllActiveCompanyIDs(ctx context.Context) ([]string, error)
	GetCompanyTypePreference(ctx context.Context, companyID, typeID string) (*CompanyTypePreference, error)
	UpsertCompanyTypePreference(ctx context.Context, in CompanyTypePreference) error

	// Subscription quota.
	CountCompanyTemplatesByCompanyID(ctx context.Context, companyID string) (int, error)

	// CMS catalog management (Sprint 2).
	ListActiveDeadlineRuleCatalog(ctx context.Context) ([]DeadlineRuleCatalogDTO, error)
	ListCmsDeadlineRules(ctx context.Context) ([]CmsDeadlineRuleDTO, error)
	ArchiveGlobalTemplate(ctx context.Context, typeID, updatedBy string) error
	CountGlobalWorkflowsByTypeId(ctx context.Context, typeID string) (int, error)
	GetGlobalWorkflow(ctx context.Context, typeID string) (*GlobalWorkflowDTO, error)
	UpsertGlobalWorkflow(ctx context.Context, req CmsUpsertGlobalWorkflowRequest, workflowID string) (*GlobalWorkflowDTO, error)
	DeleteGlobalWorkflow(ctx context.Context, typeID string) error
	CreateDisplayGroup(ctx context.Context, req CmsDisplayGroupCreateRequest) (*DisplayGroupDTO, error)
	UpdateDisplayGroup(ctx context.Context, req CmsDisplayGroupUpdateRequest) (*DisplayGroupDTO, error)
	DeleteDisplayGroup(ctx context.Context, code string) error
	CreateDeadlineRule(ctx context.Context, req CmsDeadlineRuleCreateRequest, ruleID string) (*CmsDeadlineRuleDTO, error)
	UpdateDeadlineRule(ctx context.Context, req CmsDeadlineRuleUpdateRequest) (*CmsDeadlineRuleDTO, error)
	DeleteDeadlineRule(ctx context.Context, ruleID string) error
}

type CreateRecordRequest struct {
	Subject Subject
	Payload RecordPayload
	// RecordID, when non-empty, is used as the explicit primary key instead of
	// generating a new one. Used by callers that pre-allocate deterministic IDs
	// (e.g. ad-hoc admin approval, ADR-1B) so retries are idempotent rather than
	// creating orphaned/duplicate records.
	RecordID string
}

type UpdateRecordRequest struct {
	Subject  Subject
	RecordID string
	Payload  RecordPayload
}

type SubmitRecordRequest struct {
	Subject  Subject
	RecordID string
}

type ConfirmRecordRequest struct {
	Subject  Subject
	RecordID string
}

type GetRecordRequest struct {
	Subject  Subject
	RecordID string
}

type ListRecordsRequest struct {
	Subject Subject
}

type ListRecordsResponse struct {
	Items []RecordDTO `json:"items"`
}

type ListTypeGroupsRequest struct {
	Subject Subject
}

type ListTypeGroupsResponse struct {
	Items []DisclosureGroupDTO `json:"items"`
}

type ListDisplayGroupsRequest struct {
	Subject Subject
}

type ListDisplayGroupsResponse struct {
	Items []DisplayGroupDTO `json:"items"`
}

// ListTypesParams carries all filter/pagination/sort inputs to the repository.
// Allowed SortBy values: "name", "created_at". Allowed SortDir: "asc", "desc".
// Defaults (applied by service): SortBy="created_at", SortDir="desc".
type ListTypesParams struct {
	CompanyID        string
	GroupID          string // legacy: filter by disclosure_types.group_id
	DisplayGroupCode string // new model: filter via template_display_groups junction table
	Query            string
	Page             int    // 1-based; 0 or negative → no pagination (return all)
	PageSize         int    // effective only when Page > 0; clamped 1–100, default 20
	SortBy           string // "name" | "created_at"
	SortDir          string // "asc" | "desc"
}

type ListTypesRequest struct {
	Subject          Subject
	GroupID          string
	DisplayGroupCode string
	Query            string
	Page             int
	PageSize         int
	SortBy           string // "name" | "created_at"; empty → default "created_at"
	SortDir          string // "asc" | "desc"; empty → default "desc"
}

type ListTypesResponse struct {
	Items    []DisclosureTypeSummaryDTO `json:"items"`
	Total    int                        `json:"total"`
	Page     int                        `json:"page"`
	PageSize int                        `json:"page_size"`
}

type GetTypeDetailRequest struct {
	Subject Subject
	TypeID  string
}

type GetTypeVersionDetailRequest struct {
	Subject   Subject
	TypeID    string
	VersionNo int
}

type GetTemplateReferenceDataRequest struct {
	Subject Subject
}

type TemplateReferenceDataDTO struct {
	TemplateCategories  []string                 `json:"template_categories"`
	Periodicities       []string                 `json:"periodicities"`
	DeadlineStrategies  []string                 `json:"deadline_strategies"`
	DeadlineRuleCatalog []DeadlineRuleCatalogDTO `json:"deadline_rule_catalog,omitempty"`
	MatrixRules         map[string][]string      `json:"matrix_rules"`
}

type GetTemplateReferenceDataResponse struct {
	Data TemplateReferenceDataDTO `json:"data"`
}

type UpsertTypeVersionRequest struct {
	Subject               Subject
	TypeID                string                                    `json:"type_id"`
	Scope                 string                                    `json:"scope"`
	GroupID               string                                    `json:"group_id"`
	Name                  string                                    `json:"name"`
	Category              string                                    `json:"category"`
	TemplateCategory      string                                    `json:"template_category"`
	DeadlineStrategy      string                                    `json:"deadline_strategy"`
	Description           string                                    `json:"description"`
	LegalBasis            string                                    `json:"legal_basis"`
	Applicability         string                                    `json:"applicability"`
	ImplementationContent string                                    `json:"implementation_content"`
	ImplementationNotes   string                                    `json:"implementation_notes"`
	SpecialCases          string                                    `json:"special_cases"`
	ReportContent         string                                    `json:"report_content"`
	RequiredDocs          string                                    `json:"required_docs"`
	DeadlineRule          string                                    `json:"deadline_rule"`
	Periodicity           string                                    `json:"periodicity"`
	ChannelsText          string                                    `json:"channels_text"`
	Beneficiaries         string                                    `json:"beneficiaries"`
	ReceivingAuthorities  string                                    `json:"receiving_authorities"`
	Format                string                                    `json:"format"`
	LegalRisksText        string                                    `json:"legal_risks_text"`
	GeneralInfo           string                                    `json:"general_info"`
	LegalBases            []LegalBasisDTO                           `json:"legal_bases"`
	Checklist             []ChecklistItemDTO                        `json:"checklist"`
	Tags                  []string                                  `json:"tags"`
	DeadlineConfig        *TemplateDeadlineConfig                   `json:"deadline_config,omitempty"`
	Blocks                []TemplateBlockDTO                        `json:"blocks"`
	DisplayGroupCodes     []string                                  `json:"display_group_codes"`
	ChangeNote            string                                    `json:"change_note"`
	ApplicabilityRules    *applicability.TemplateApplicabilityRules `json:"applicability_rules,omitempty"`
}

type UpsertTypeVersionResponse struct {
	TypeID      string    `json:"type_id"`
	VersionNo   int       `json:"version_no"`
	IsActive    bool      `json:"is_active"`
	UpdatedBy   string    `json:"updated_by"`
	ActivatedAt time.Time `json:"activated_at"`
}

type ListTypeVersionsRequest struct {
	Subject Subject
	TypeID  string
}

type ListTypeVersionsResponse struct {
	Items []DisclosureTypeVersionDTO `json:"items"`
}

type ActivateTypeVersionRequest struct {
	Subject   Subject
	TypeID    string `json:"type_id"`
	VersionNo int    `json:"version_no"`
	Reason    string `json:"reason"`
}

type ActivateTypeVersionResponse struct {
	TypeID      string    `json:"type_id"`
	VersionNo   int       `json:"version_no"`
	IsActive    bool      `json:"is_active"`
	UpdatedBy   string    `json:"updated_by"`
	ActivatedAt time.Time `json:"activated_at"`
}

// BE-004A: Company-defined template create/update (portal path).
type CreateCompanyTemplateRequest struct {
	Subject          Subject
	Name             string   `json:"name"`
	TemplateCategory string   `json:"template_category"` // "periodic" | "irregular"
	Description      string   `json:"description"`
	DeadlineRule     string   `json:"deadline_rule"`
	Periodicity      string   `json:"periodicity"`
	Tags             []string `json:"tags"`
	LegalBasis       string   `json:"legal_basis"`
	Applicability    string   `json:"applicability"`
	ChangeNote       string   `json:"change_note"`
}

type UpdateCompanyTemplateRequest struct {
	Subject          Subject
	TypeID           string   `json:"type_id"`
	Name             string   `json:"name"`
	TemplateCategory string   `json:"template_category"`
	Description      string   `json:"description"`
	DeadlineRule     string   `json:"deadline_rule"`
	Periodicity      string   `json:"periodicity"`
	Tags             []string `json:"tags"`
	LegalBasis       string   `json:"legal_basis"`
	Applicability    string   `json:"applicability"`
	ChangeNote       string   `json:"change_note"`
}

// BE-004B: Lifecycle transition.
type TransitionCompanyTemplateLifecycleRequest struct {
	Subject Subject
	TypeID  string `json:"type_id"`
	Action  string `json:"action"` // "submit-review" | "publish" | "reject" | "archive"
	Reason  string `json:"reason"`
}

type CompanyTemplateWriteResponse struct {
	TypeID           string   `json:"type_id"`
	CompanyID        string   `json:"company_id"`
	Name             string   `json:"name"`
	TemplateCategory string   `json:"template_category"`
	Description      string   `json:"description"`
	ReviewStatus     string   `json:"review_status"`
	DeadlineRule     string   `json:"deadline_rule,omitempty"`
	Periodicity      string   `json:"periodicity,omitempty"`
	Tags             []string `json:"tags,omitempty"`
	CreatedAt        string   `json:"created_at"`
	UpdatedAt        string   `json:"updated_at"`
}

type WorkflowDocumentDTO struct {
	DocID    string `json:"doc_id"`
	Name     string `json:"name"`
	Required bool   `json:"required"`
}

// WorkflowStepGroupDTO is one tổ/nhóm assignment for a workflow step.
// Present in response only when WORKFLOW_GROUPS_ENABLED=true.
type WorkflowStepGroupDTO struct {
	GroupID        string `json:"group_id"`
	GroupName      string `json:"group_name"`
	DepartmentID   string `json:"department_id"`
	DepartmentName string `json:"department_name,omitempty"`
	Source         string `json:"source"`        // "auto_fill" | "manual"
	DurationMode   string `json:"duration_mode"` // "inherit" | "custom"
	ProcessingDays *int   `json:"processing_days,omitempty"`
	DisplayOrder   int    `json:"display_order"`
	IsActive       bool   `json:"is_active"`
}

// WorkflowStepReminderConfig lưu cấu hình nhắc nhở riêng theo từng bước.
// Được lưu trong workflow_json blob — không cần migration SQL.
type WorkflowStepReminderConfig struct {
	Enabled    bool   `json:"enabled"`
	Mode       string `json:"mode,omitempty"`        // "days_before" | "specific_date"
	DaysBefore []int  `json:"days_before,omitempty"` // VD: [1, 3, 5, 7]
}

type WorkflowStepDTO struct {
	StepID          string                      `json:"step_id"`
	Stage           string                      `json:"stage"`
	DepartmentID    string                      `json:"department_id"`
	AssigneeRoleIds []string                    `json:"assignee_role_ids"`
	DueRule         string                      `json:"due_rule"`
	ProcessingDays  int                         `json:"processing_days,omitempty"`
	Documents       []WorkflowDocumentDTO       `json:"documents"`
	DisplayOrder    int                         `json:"display_order"`
	Groups          []WorkflowStepGroupDTO      `json:"groups,omitempty"`
	ReminderConfig  *WorkflowStepReminderConfig `json:"reminder_config,omitempty"`
}

type CompanyWorkflowOverrideHeaderDTO struct {
	OverrideID      string    `json:"override_id"`
	TypeID          string    `json:"type_id"`
	CompanyID       string    `json:"company_id"`
	Status          string    `json:"status"`
	ActiveVersionNo int       `json:"active_version_no"`
	UpdatedAt       time.Time `json:"updated_at"`

	// Sprint 3 / Batch 1 (Workflow Override Metadata Foundation) — additive fields backed by
	// migration 0103. Not yet populated or read by any repository method or runtime path in
	// Batch 1 (GetEffectiveWorkflow/GetCompanyWorkflowOverride are deliberately NOT touched this
	// batch — see docs/ai-cache/workflow-override-foundation-batch1/PREFLIGHT_AUDIT.md). Reading
	// these into the repository's SELECT and acting on them is Batch 2's (Staleness Detection)
	// responsibility. BaseSource is one of "global_workflow" | "global_template" | "unknown".
	// StaleStatus is one of "unknown" | "current" | "stale" (always "unknown" as of Batch 1).
	BaseSource        string     `json:"base_source,omitempty"`
	BaseWorkflowID    string     `json:"base_workflow_id,omitempty"`
	BaseVersionNo     *int       `json:"base_version_no,omitempty"`
	BaseHash          string     `json:"base_hash,omitempty"`
	StaleStatus       string     `json:"stale_status,omitempty"`
	LastRebaseCheckAt *time.Time `json:"last_rebase_check_at,omitempty"`
}

type CompanyWorkflowOverrideVersionDTO struct {
	VersionNo  int               `json:"version_no"`
	DraftEtag  string            `json:"draft_etag,omitempty"`
	State      string            `json:"state"`
	ChangeNote string            `json:"change_note"`
	Workflow   []WorkflowStepDTO `json:"workflow"`
	CreatedBy  string            `json:"created_by"`
	ApprovedBy string            `json:"approved_by,omitempty"`
	ApprovedAt *time.Time        `json:"approved_at,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
}

type CompanyWorkflowOverrideViewDTO struct {
	TypeID          string                             `json:"type_id"`
	CompanyID       string                             `json:"company_id"`
	Override        *CompanyWorkflowOverrideHeaderDTO  `json:"override,omitempty"`
	DraftVersion    *CompanyWorkflowOverrideVersionDTO `json:"draft_version,omitempty"`
	ActiveVersion   *CompanyWorkflowOverrideVersionDTO `json:"active_version,omitempty"`
	EffectiveSource string                             `json:"effective_source"`
}

type GetCompanyWorkflowOverrideRequest struct {
	Subject Subject
	TypeID  string
}

type GetCompanyWorkflowOverrideResponse struct {
	Data CompanyWorkflowOverrideViewDTO `json:"data"`
}

type UpsertCompanyWorkflowOverrideDraftRequest struct {
	Subject       Subject
	TypeID        string            `json:"type_id"`
	BaseVersionNo int               `json:"base_version_no"`
	BaseEtag      string            `json:"base_etag,omitempty"`
	ChangeNote    string            `json:"change_note"`
	Workflow      []WorkflowStepDTO `json:"workflow"`
	// Publish=true atomically approves the draft immediately after saving.
	// Caller must hold template.workflow.override.approve permission.
	Publish bool `json:"publish,omitempty"`
}

type UpsertCompanyWorkflowOverrideDraftResponse struct {
	OverrideID     string            `json:"override_id"`
	TypeID         string            `json:"type_id"`
	CompanyID      string            `json:"company_id"`
	DraftVersionNo int               `json:"draft_version_no"`
	DraftEtag      string            `json:"draft_etag"`
	VersionNo      int               `json:"version_no"`
	State          string            `json:"state"`
	UpdatedAt      time.Time         `json:"updated_at"`
	Workflow       []WorkflowStepDTO `json:"workflow"`
}

type ApproveCompanyWorkflowOverrideRequest struct {
	Subject   Subject
	TypeID    string `json:"type_id"`
	BaseEtag  string `json:"base_etag,omitempty"`
	VersionNo int    `json:"version_no"`
	Reason    string `json:"reason"`
	// SkipSelfApprovalCheck bypasses the maker-checker guard.
	// Used internally by the save+apply (publish) path.
	SkipSelfApprovalCheck bool `json:"-"`
}

type ApproveCompanyWorkflowOverrideResponse struct {
	OverrideID      string    `json:"override_id"`
	TypeID          string    `json:"type_id"`
	CompanyID       string    `json:"company_id"`
	ActiveVersionNo int       `json:"active_version_no"`
	State           string    `json:"state"`
	ApprovedBy      string    `json:"approved_by"`
	ApprovedAt      time.Time `json:"approved_at"`
	EffectiveSource string    `json:"effective_source"`
}

type DeleteCompanyWorkflowOverrideDraftRequest struct {
	Subject   Subject
	TypeID    string
	VersionNo int
}

type DeleteCompanyWorkflowOverrideDraftResponse struct {
	Deleted   bool `json:"deleted"`
	VersionNo int  `json:"version_no"`
}

type ResetCompanyWorkflowOverrideActiveRequest struct {
	Subject Subject
	TypeID  string
	Reason  string `json:"reason"`
}

type ResetCompanyWorkflowOverrideActiveResponse struct {
	OverrideID      string `json:"override_id"`
	TypeID          string `json:"type_id"`
	CompanyID       string `json:"company_id"`
	ActiveVersionNo int    `json:"active_version_no"`
	State           string `json:"state"`
	EffectiveSource string `json:"effective_source"`
}

type ListCompanyWorkflowOverrideVersionsRequest struct {
	Subject  Subject
	TypeID   string
	Page     int
	PageSize int
}

type ListCompanyWorkflowOverrideVersionsResponse struct {
	Items []CompanyWorkflowOverrideVersionDTO `json:"items"`
	Meta  struct {
		Page     int `json:"page"`
		PageSize int `json:"page_size"`
		Total    int `json:"total"`
	} `json:"meta"`
}

type GetCompanyWorkflowOverrideDraftReminderPreviewRequest struct {
	Subject Subject
	TypeID  string
	T0Date  string // optional YYYY-MM-DD; defaults to today in company TZ
}

type GetCompanyWorkflowOverrideDraftReminderPreviewResponse struct {
	Data WorkflowOverrideReminderPreviewDTO `json:"data"`
}

type GetEffectiveWorkflowRequest struct {
	Subject Subject
	TypeID  string
}

// EffectiveWorkflowDTO.Source is one of: "company_override" (tenant override active version),
// "global_workflow" (Sprint 0-2 governed workflow's active global_workflow_versions row), or
// "global_template" (legacy enterprise_workflow content block — fallback when neither of the
// above exists for the type). Precedence: company_override > global_workflow > global_template.
type EffectiveWorkflowDTO struct {
	TypeID    string            `json:"type_id"`
	CompanyID string            `json:"company_id"`
	Source    string            `json:"source"`
	VersionNo int               `json:"version_no"`
	Workflow  []WorkflowStepDTO `json:"workflow"`
}

type GetEffectiveWorkflowResponse struct {
	Data EffectiveWorkflowDTO `json:"data"`
}

// ─── Groups / tổ nhóm (WORKFLOW_GROUPS_ENABLED) ───────────────────────────────

type ListCompanyGroupsRequest struct {
	Subject      Subject
	DepartmentID string // optional filter; empty = all active groups in company
	IsActive     *bool  // nil = no filter
}

type ListCompanyGroupsResponse struct {
	Items []CompanyGroupDTO `json:"items"`
}

// CompanyGroupDTO is a tổ/nhóm (team-level org unit) belonging to a department.
type CompanyGroupDTO struct {
	GroupID        string `json:"group_id"`
	GroupName      string `json:"group_name"`
	DepartmentID   string `json:"department_id"`
	DepartmentName string `json:"department_name,omitempty"`
	IsActive       bool   `json:"is_active"`
}

// WorkflowStepGroupWriteInput is the write shape for one group in a step.
type WorkflowStepGroupWriteInput struct {
	GroupID        string `json:"group_id"`
	DurationMode   string `json:"duration_mode"` // "inherit" | "custom"
	ProcessingDays *int   `json:"processing_days,omitempty"`
	DisplayOrder   int    `json:"display_order"`
}

type UpdateWorkflowOverrideStepGroupsRequest struct {
	Subject  Subject
	TypeID   string                        `json:"type_id"`
	StepID   string                        `json:"step_id"`
	BaseEtag string                        `json:"base_etag"`
	Groups   []WorkflowStepGroupWriteInput `json:"groups"`
	ClearAll bool                          `json:"clear_all_groups"`
}

type UpdateWorkflowOverrideStepGroupsResponse struct {
	DraftEtag string                 `json:"draft_etag"`
	StepID    string                 `json:"step_id"`
	Groups    []WorkflowStepGroupDTO `json:"groups"`
}

// ─── End Groups ────────────────────────────────────────────────────────────────

type GetTemplateDeadlineConfigRequest struct {
	Subject Subject
	TypeID  string
}

type GetTemplateDeadlineConfigResponse struct {
	TypeID         string                 `json:"type_id"`
	VersionNo      int                    `json:"version_no"`
	DeadlineConfig TemplateDeadlineConfig `json:"deadline_config"`
}

type UpdateTemplateDeadlineConfigRequest struct {
	Subject        Subject
	TypeID         string                 `json:"type_id"`
	DeadlineConfig TemplateDeadlineConfig `json:"deadline_config"`
}

type UpdateTemplateDeadlineConfigResponse struct {
	TypeID         string                 `json:"type_id"`
	VersionNo      int                    `json:"version_no"`
	DeadlineConfig TemplateDeadlineConfig `json:"deadline_config"`
	UpdatedBy      string                 `json:"updated_by"`
}

type Subject struct {
	UserID       string
	MembershipID string
	CompanyID    string
}

type RecordPayload struct {
	TypeID       string          `json:"type_id"`
	DepartmentID string          `json:"department_id"`
	Title        string          `json:"title"`
	Summary      string          `json:"summary"`
	Content      string          `json:"content"`
	PlannedDate  string          `json:"planned_date"`
	Attachments  []AttachmentDTO `json:"attachments"`
	EvidenceLink string          `json:"evidence_link"`
}

type AttachmentDTO struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	UploadedAt string `json:"uploaded_at"`
}

type DisclosureGroupDTO struct {
	GroupID      string `json:"group_id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Icon         string `json:"icon"`
	DisplayOrder int    `json:"display_order"`
}

type DisplayGroupDTO struct {
	DisplayGroupCode string `json:"display_group_code"`
	NameVI           string `json:"name_vi"`
	NameEN           string `json:"name_en"`
	Description      string `json:"description"`
	Icon             string `json:"icon"`
	DisplayOrder     int    `json:"display_order"`
	IsActive         bool   `json:"is_active"`
	IsSystem         bool   `json:"is_system"`
}

type DisclosureTypeSummaryDTO struct {
	TypeID  string `json:"type_id"`
	GroupID string `json:"group_id"`
	// Deprecated: use DisplayGroupCodes. Kept for compatibility window (BE-008).
	DisplayGroupCode              string                                    `json:"display_group_code,omitempty"`
	DisplayGroupCodes             []string                                  `json:"display_group_codes"`
	Scope                         string                                    `json:"scope"`
	OwnerCompanyID                string                                    `json:"owner_company_id"`
	Name                          string                                    `json:"name"`
	Category                      string                                    `json:"category"`
	TemplateCategory              string                                    `json:"template_category"`
	Periodicity                   string                                    `json:"periodicity"`
	Description                   string                                    `json:"description"`
	DeadlineRule                  string                                    `json:"deadline_rule"`
	DeadlineRuleDisplay           string                                    `json:"deadline_rule_display,omitempty"`
	IsMandatory                   bool                                      `json:"is_mandatory"`
	HasWorkflow                   bool                                      `json:"has_workflow"`
	ReviewStatus                  string                                    `json:"review_status,omitempty"`
	Tags                          []string                                  `json:"tags"`
	ApplicabilityRules            *applicability.TemplateApplicabilityRules `json:"-"`
	ResolvedStructureDeadlineDays *int                                      `json:"resolved_structure_deadline_days,omitempty"`
}

type DisclosureTypeDTO struct {
	VersionNo             int                     `json:"version_no"`
	TypeID                string                  `json:"type_id"`
	GroupID               string                  `json:"group_id"`
	Scope                 string                  `json:"scope"`
	OwnerCompanyID        string                  `json:"owner_company_id"`
	Name                  string                  `json:"name"`
	Category              string                  `json:"category"`
	TemplateCategory      string                  `json:"template_category"`
	DeadlineStrategy      string                  `json:"deadline_strategy"`
	Description           string                  `json:"description"`
	LegalBasis            string                  `json:"legal_basis"`
	Applicability         string                  `json:"applicability"`
	ImplementationContent string                  `json:"implementation_content"`
	ImplementationNotes   string                  `json:"implementation_notes"`
	SpecialCases          string                  `json:"special_cases"`
	ReportContent         string                  `json:"report_content"`
	RequiredDocs          string                  `json:"required_docs"`
	DeadlineRule          string                  `json:"deadline_rule"`
	DeadlineRuleDisplay   string                  `json:"deadline_rule_display,omitempty"`
	Periodicity           string                  `json:"periodicity"`
	ChannelsText          string                  `json:"channels_text"`
	Beneficiaries         string                  `json:"beneficiaries"`
	ReceivingAuthorities  string                  `json:"receiving_authorities"`
	Format                string                  `json:"format"`
	LegalRisksText        string                  `json:"legal_risks_text"`
	GeneralInfo           string                  `json:"general_info"`
	DeadlineConfig        *TemplateDeadlineConfig `json:"deadline_config,omitempty"`
	DeadlineSummary       *DeadlineSummaryDTO     `json:"deadline_summary,omitempty"`
	LegalBases            []LegalBasisDTO         `json:"legal_bases"`
	Checklist             []ChecklistItemDTO      `json:"checklist"`
	Tags                  []string                `json:"tags"`
	Blocks                []TemplateBlockDTO      `json:"blocks"`
	// Deprecated: use DisplayGroupCodes. Kept for compatibility window (BE-008).
	DisplayGroupCode   string                                    `json:"display_group_code,omitempty"`
	DisplayGroupCodes  []string                                  `json:"display_group_codes"`
	IsMandatory        bool                                      `json:"is_mandatory"`
	HasWorkflow        bool                                      `json:"has_workflow"`
	ReviewStatus       string                                    `json:"review_status,omitempty"`
	ApplicabilityRules *applicability.TemplateApplicabilityRules `json:"applicability_rules,omitempty"`
}

type DeadlineSummaryDTO struct {
	DeadlineMode                 string  `json:"deadline_mode,omitempty"`
	FixedDeadlineDate            *string `json:"fixed_deadline_date,omitempty"`
	StartDate                    *string `json:"start_date,omitempty"`
	BaseDateSource               *string `json:"base_date_source,omitempty"`
	TentativeDeadline            *string `json:"tentative_deadline,omitempty"`
	ActualDeadline               *string `json:"actual_deadline,omitempty"`
	Duration                     *int    `json:"duration,omitempty"`
	DurationType                 *string `json:"duration_type,omitempty"`
	InclusiveStart               *bool   `json:"inclusive_start,omitempty"`
	AdjustedBecauseNonTradingDay *bool   `json:"adjusted_because_non_trading_day,omitempty"`
	NonTradingDayReason          *string `json:"non_trading_day_reason,omitempty"`
	SourceDate                   *string `json:"source_date,omitempty"`
	DeadlineDate                 *string `json:"deadline_date,omitempty"`
	RemainingDays                *int    `json:"remaining_days,omitempty"`
	Status                       string  `json:"status,omitempty"`
	RuleCode                     *string `json:"rule_code,omitempty"`
	RuleDescription              *string `json:"rule_description,omitempty"`
	Timezone                     *string `json:"timezone,omitempty"`
}

type TemplateDeadlineConfig struct {
	DeadlineMode  string               `json:"deadline_mode"`
	FixedDeadline *FixedDeadlineConfig `json:"fixed_deadline,omitempty"`
	DynamicRule   *DynamicDeadlineRule `json:"dynamic_rule,omitempty"`
	// T0Policy defines how the T0 reference date is resolved for timeline computation.
	// Values: "system_date" | "event_date" | "user_defined". Empty = legacy (no timeline).
	T0Policy string `json:"t0_policy,omitempty"`
	// DeadlineDays is total calendar days from T0 to the outer deadline.
	DeadlineDays int `json:"deadline_days,omitempty"`
	// ProcessingDays is the default per-step duration in calendar days.
	ProcessingDays int `json:"processing_days,omitempty"`
	// Portal/CMS template config extensions (stored in deadline_config_json).
	TemplateCategory  string `json:"template_category,omitempty"`
	FrequencyInterval int    `json:"frequency_interval,omitempty"`
	FrequencyUnit     string `json:"frequency_unit,omitempty"`
	AllowT0Override   *bool  `json:"allow_t0_override,omitempty"`
	ReportInfoLocked  *bool  `json:"report_info_locked,omitempty"`
	// StepDefaultSlaDays is the per-step workflow SLA fallback (calendar days).
	// Intentionally separate from DeadlineDays (total SLA) to prevent timeline blow-up.
	StepDefaultSlaDays int `json:"step_default_sla_days,omitempty"`
	// CycleAnchorDay/Month define the fiscal year start for yearly periodic templates.
	// 0 = unset (defaults to 01/01). Not needed for monthly/quarterly.
	CycleAnchorDay   int `json:"cycle_anchor_day,omitempty"`
	CycleAnchorMonth int `json:"cycle_anchor_month,omitempty"`
	// DeadlineDurationType is a runtime-only override (WORKING_DAYS | CALENDAR_DAYS).
	DeadlineDurationType string `json:"deadline_duration_type,omitempty"`
}

// WorkflowOverrideReminderPreviewMilestoneDTO is one projected reminder row for draft preview.
type WorkflowOverrideReminderPreviewMilestoneDTO struct {
	StepID        string `json:"step_id"`
	StepOrder     int    `json:"step_order"`
	MilestoneType string `json:"milestone_type"`
	ScheduledDate string `json:"scheduled_date"`
}

// WorkflowOverrideReminderPreviewDTO is the read-only reminder schedule for a draft workflow.
type WorkflowOverrideReminderPreviewDTO struct {
	TypeID     string                                        `json:"type_id"`
	CompanyID  string                                        `json:"company_id"`
	T0Date     string                                        `json:"t0_date"`
	Timezone   string                                        `json:"timezone"`
	Source     string                                        `json:"source"`
	Milestones []WorkflowOverrideReminderPreviewMilestoneDTO `json:"milestones"`
}

type FixedDeadlineConfig struct {
	Date             string `json:"date"`
	NonTradingPolicy string `json:"non_trading_policy,omitempty"`
}

type DynamicDeadlineRule struct {
	RuleType              string `json:"rule_type"`
	BaseDateSource        string `json:"base_date_source"`
	Duration              int    `json:"duration"`
	DurationType          string `json:"duration_type"`
	InclusiveStart        bool   `json:"inclusive_start"`
	AdjustIfNonTradingDay bool   `json:"adjust_if_non_trading_day"`
	HolidayCalendarSource string `json:"holiday_calendar_source"`
	Description           string `json:"description,omitempty"`
}

type LegalBasisDTO struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Code      string `json:"code"`
	Authority string `json:"authority"`
	IssueDate string `json:"issue_date"`
	Summary   string `json:"summary"`
	Link      string `json:"link"`
}

type ChecklistItemDTO struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Owner   string `json:"owner"`
	DueDate string `json:"due_date"`
	Status  string `json:"status"`
}

type TemplateBlockDTO struct {
	BlockID   string `json:"block_id"`
	BlockKey  string `json:"block_key"`
	BlockType string `json:"block_type"`
	Title     string `json:"title"`
	// Display labels for bilingual CMS UI; persisted title remains the canonical row label (historically VI-oriented).
	NameEN       string         `json:"name_en,omitempty"`
	NameVI       string         `json:"name_vi,omitempty"`
	Description  string         `json:"description"`
	Config       map[string]any `json:"config"`
	Validation   map[string]any `json:"validation"`
	DisplayOrder int            `json:"display_order"`
	Enabled      bool           `json:"enabled"`
}

type DisclosureTypeVersionDTO struct {
	TypeID      string    `json:"type_id"`
	VersionNo   int       `json:"version_no"`
	IsActive    bool      `json:"is_active"`
	ChangeNote  string    `json:"change_note"`
	UpdatedBy   string    `json:"updated_by"`
	ActivatedAt time.Time `json:"activated_at"`
}

type RecordDTO struct {
	RecordID           string          `json:"record_id"`
	CompanyID          string          `json:"company_id"`
	TypeID             string          `json:"type_id"`
	DepartmentID       string          `json:"department_id"`
	Title              string          `json:"title"`
	Summary            string          `json:"summary"`
	Content            string          `json:"content"`
	PlannedDate        string          `json:"planned_date,omitempty"`
	PublishedDate      string          `json:"published_date,omitempty"`
	Status             string          `json:"status"`
	Attachments        []AttachmentDTO `json:"attachments"`
	EvidenceLink       string          `json:"evidence_link,omitempty"`
	WorkflowInstanceID string          `json:"workflow_instance_id,omitempty"`
	CreatedBy          string          `json:"created_by"`
	UpdatedBy          string          `json:"updated_by"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// WorkflowBootstrapper creates a workflow instance when a disclosure record is submitted.
type WorkflowBootstrapper interface {
	EnsureOnSubmit(ctx context.Context, sub Subject, rec RecordDTO) (workflowInstanceID string, err error)
}

// PeriodicMaterializeRepository is the repository surface used during materialize.
type PeriodicMaterializeRepository interface {
	ListPendingCycles(ctx context.Context, asOf time.Time, bufferDays int) ([]PeriodicCycleRow, error)
	TryClaimPeriodicCycle(ctx context.Context, cycleID string) (bool, error)
	ReleasePeriodicCycleClaim(ctx context.Context, cycleID string) error
	UpdateCycleRecord(ctx context.Context, cycleID, recordID string) error
}

// PeriodicRecordCreator is the cross-module interface used by periodic auto-creation.
// Implemented by disclosureapp.service itself; injected into MaterializePeriodicDisclosures
// to allow worker to pass a system-actor creator without circular imports.
type PeriodicRecordCreator interface {
	CreateAndSubmitRecord(ctx context.Context, companyID, typeID, createdByMembershipID, title string, t0Date *time.Time) (recordID, workflowInstanceID string, err error)
	// CreateAndSubmitRecordWithPlannedDate is the periodic materialize path that also sets
	// disclosure_records.planned_date from the cycle's due_date.
	// plannedDate must be YYYY-MM-DD (from periodic_cycles.due_date) or empty string (no-op).
	CreateAndSubmitRecordWithPlannedDate(ctx context.Context, companyID, typeID, createdByMembershipID, title string, t0Date *time.Time, plannedDate string) (recordID, workflowInstanceID string, err error)
}

// PeriodicTypeRow is returned by ListActivePeriodicTypes.
type PeriodicTypeRow struct {
	TypeID             string
	FrequencyUnit      string // "monthly" | "quarterly" | "yearly"
	FrequencyInterval  int
	DeadlineDays       int
	CycleAnchorDay     int // 0 = unset → defaults to 1
	CycleAnchorMonth   int // 0 = unset → defaults to 1
	IsGlobal           bool
	ApplicabilityRules *applicability.TemplateApplicabilityRules
}

// PeriodicCycleRow represents one (type, company, cycle) idempotency slot.
type PeriodicCycleRow struct {
	CycleID    string
	TypeID     string
	TypeName   string
	CompanyID  string
	CycleLabel string
	CycleStart time.Time // DATE at seed; materialize T0 (AC-9)
	DueDate    time.Time
	RecordID   string // empty = pending
}

// CompanyTypePreference is used by the repository layer.
type CompanyTypePreference struct {
	CompanyID         string
	TypeID            string
	AutoCreateEnabled bool
	UpdatedBy         string
	// Per-company fiscal year start override for PERIODIC deadline mode.
	// 0 = use template default (CycleAnchorMonth/CycleAnchorDay in TemplateDeadlineConfig).
	CycleAnchorMonth int
	CycleAnchorDay   int
}

// CompanyTypePreferenceDTO is the API-facing representation.
type CompanyTypePreferenceDTO struct {
	TypeID            string    `json:"type_id"`
	CompanyID         string    `json:"company_id"`
	AutoCreateEnabled bool      `json:"auto_create_enabled"`
	UpdatedAt         time.Time `json:"updated_at"`
	// Per-company fiscal year start override for PERIODIC deadline mode. 0 = use template default.
	CycleAnchorMonth int `json:"cycle_anchor_month,omitempty"`
	CycleAnchorDay   int `json:"cycle_anchor_day,omitempty"`
}

type GetCompanyTypePreferenceRequest struct {
	Subject Subject
	TypeID  string
}

type UpsertCompanyTypePreferenceRequest struct {
	Subject           Subject
	TypeID            string
	AutoCreateEnabled bool
	// Per-company cycle anchor override. 0 = keep/use template default.
	CycleAnchorMonth int `json:"cycle_anchor_month,omitempty"`
	CycleAnchorDay   int `json:"cycle_anchor_day,omitempty"`
}

// ─── CMS System Template Management ──────────────────────────────────────────

type CmsArchiveTemplateRequest struct {
	Subject Subject
	TypeID  string `json:"type_id"`
	Reason  string `json:"reason"`
}

type CmsArchiveTemplateResponse struct {
	TypeID string `json:"type_id"`
	Status string `json:"status"`
}

// ─── Global Workflow ─────────────────────────────────────────────────────────

type GlobalWorkflowStepInput struct {
	StepID string `json:"step_id"`
	// StepKey is the immutable, server-minted stable identity of the step (mig-S1).
	// Additive + backward compatible: legacy clients may omit it. On upsert the server
	// preserves it (match by step_key, then by step_id) and mints a new one for genuinely
	// new steps. Never reused once retired. See Phase 13 STEP_KEY_SPECIFICATION.
	StepKey         string   `json:"step_key,omitempty"`
	Stage           string   `json:"stage"`
	Instructions    string   `json:"instructions,omitempty"`
	DepartmentID    string   `json:"department_id"`
	AssigneeRoleIds []string `json:"assignee_role_ids"`
	DueRule         string   `json:"due_rule"`
	ProcessingDays  int      `json:"processing_days"`
	DisplayOrder    int      `json:"display_order"`
}

type GlobalWorkflowDTO struct {
	WorkflowID string `json:"workflow_id"`
	TypeID     string `json:"type_id"`
	Status     string `json:"status"`
	ChangeNote string `json:"change_note,omitempty"`
	// Version pointers (Batch 2 versioning). Preserved across save-draft (Batch 3). Nil when versioning
	// has not been used for this type. Source of truth for active/published is global_workflow_versions.state.
	PublishedVersionNo *int                      `json:"published_version_no,omitempty"`
	ActiveVersionNo    *int                      `json:"active_version_no,omitempty"`
	Steps              []GlobalWorkflowStepInput `json:"steps"`
	CreatedBy          string                    `json:"created_by"`
	UpdatedBy          string                    `json:"updated_by"`
	CreatedAt          time.Time                 `json:"created_at"`
	UpdatedAt          time.Time                 `json:"updated_at"`
}

type CmsGetGlobalWorkflowRequest struct {
	Subject Subject
	TypeID  string
}

type CmsGetGlobalWorkflowResponse struct {
	Data *GlobalWorkflowDTO `json:"data"`
}

type CmsUpsertGlobalWorkflowRequest struct {
	Subject    Subject
	TypeID     string                    `json:"type_id"`
	ChangeNote string                    `json:"change_note"`
	Steps      []GlobalWorkflowStepInput `json:"steps"`
}

type CmsDeleteGlobalWorkflowRequest struct {
	Subject Subject
	TypeID  string
}

// ─── Display Group CRUD ──────────────────────────────────────────────────────

type CmsDisplayGroupCreateRequest struct {
	Subject      Subject
	Code         string `json:"code"`
	NameVI       string `json:"name_vi"`
	NameEN       string `json:"name_en"`
	Description  string `json:"description"`
	Icon         string `json:"icon"`
	DisplayOrder int    `json:"display_order"`
}

type CmsDisplayGroupUpdateRequest struct {
	Subject      Subject
	Code         string `json:"code"`
	NameVI       string `json:"name_vi"`
	NameEN       string `json:"name_en"`
	Description  string `json:"description"`
	Icon         string `json:"icon"`
	DisplayOrder int    `json:"display_order"`
	IsActive     *bool  `json:"is_active"`
}

type CmsDisplayGroupDeleteRequest struct {
	Subject Subject
	Code    string
}

// ─── Deadline Rule Catalog CRUD ───────────────────────────────────────────────

type CmsDeadlineRuleCreateRequest struct {
	Subject      Subject
	Code         string `json:"code"`
	LabelVI      string `json:"label_vi"`
	Pattern      string `json:"pattern"`
	InputType    string `json:"input_type"`
	DisplayOrder int    `json:"display_order"`
}

type CmsDeadlineRuleUpdateRequest struct {
	Subject      Subject
	RuleID       string `json:"rule_id"`
	LabelVI      string `json:"label_vi"`
	Pattern      string `json:"pattern"`
	InputType    string `json:"input_type"`
	DisplayOrder int    `json:"display_order"`
	IsActive     *bool  `json:"is_active"`
}

type CmsDeadlineRuleDeleteRequest struct {
	Subject Subject
	RuleID  string
}

type CmsDeadlineRuleDTO struct {
	RuleID       string    `json:"rule_id"`
	Code         string    `json:"code"`
	LabelVI      string    `json:"label_vi"`
	Pattern      string    `json:"pattern"`
	InputType    string    `json:"input_type"`
	IsActive     bool      `json:"is_active"`
	DisplayOrder int       `json:"display_order"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
