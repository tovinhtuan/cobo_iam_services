package app

import (
	"context"
	"time"
)

type Service interface {
	CreateRecord(ctx context.Context, req CreateRecordRequest) (*RecordDTO, error)
	UpdateRecord(ctx context.Context, req UpdateRecordRequest) (*RecordDTO, error)
	SubmitRecord(ctx context.Context, req SubmitRecordRequest) (*RecordDTO, error)
	ConfirmRecord(ctx context.Context, req ConfirmRecordRequest) (*RecordDTO, error)
	ListRecords(ctx context.Context, req ListRecordsRequest) (*ListRecordsResponse, error)
	GetRecord(ctx context.Context, req GetRecordRequest) (*RecordDTO, error)
	ListTypeGroups(ctx context.Context, req ListTypeGroupsRequest) (*ListTypeGroupsResponse, error)
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
	GetEffectiveWorkflow(ctx context.Context, req GetEffectiveWorkflowRequest) (*GetEffectiveWorkflowResponse, error)
	GetTemplateDeadlineConfig(ctx context.Context, req GetTemplateDeadlineConfigRequest) (*GetTemplateDeadlineConfigResponse, error)
	UpdateTemplateDeadlineConfig(ctx context.Context, req UpdateTemplateDeadlineConfigRequest) (*UpdateTemplateDeadlineConfigResponse, error)
	ListCompanyGroups(ctx context.Context, req ListCompanyGroupsRequest) (*ListCompanyGroupsResponse, error)
	UpdateWorkflowOverrideStepGroups(ctx context.Context, req UpdateWorkflowOverrideStepGroupsRequest) (*UpdateWorkflowOverrideStepGroupsResponse, error)
}

type Repository interface {
	Create(ctx context.Context, rec RecordDTO) (*RecordDTO, error)
	Update(ctx context.Context, rec RecordDTO) (*RecordDTO, error)
	FindByID(ctx context.Context, companyID, recordID string) (*RecordDTO, error)
	List(ctx context.Context, companyID string) ([]RecordDTO, error)
	ListTypeGroups(ctx context.Context, companyID string) ([]DisclosureGroupDTO, error)
	ListTypes(ctx context.Context, companyID, groupID, query string) ([]DisclosureTypeSummaryDTO, error)
	GetTypeDetail(ctx context.Context, companyID, typeID string) (*DisclosureTypeDTO, error)
	GetTypeVersionDetail(ctx context.Context, companyID, typeID string, versionNo int) (*DisclosureTypeDTO, error)
	UpsertTypeVersion(ctx context.Context, req UpsertTypeVersionRequest) (*UpsertTypeVersionResponse, error)
	ListTypeVersions(ctx context.Context, companyID, typeID string) ([]DisclosureTypeVersionDTO, error)
	ActivateTypeVersion(ctx context.Context, req ActivateTypeVersionRequest) (*ActivateTypeVersionResponse, error)
	GetCompanyWorkflowOverride(ctx context.Context, companyID, typeID string) (*CompanyWorkflowOverrideViewDTO, error)
	UpsertCompanyWorkflowOverrideDraft(ctx context.Context, req UpsertCompanyWorkflowOverrideDraftRequest) (*UpsertCompanyWorkflowOverrideDraftResponse, error)
	ApproveCompanyWorkflowOverride(ctx context.Context, req ApproveCompanyWorkflowOverrideRequest) (*ApproveCompanyWorkflowOverrideResponse, error)
	DeleteCompanyWorkflowOverrideDraft(ctx context.Context, req DeleteCompanyWorkflowOverrideDraftRequest) (*DeleteCompanyWorkflowOverrideDraftResponse, error)
	ResetCompanyWorkflowOverrideActive(ctx context.Context, req ResetCompanyWorkflowOverrideActiveRequest) (*ResetCompanyWorkflowOverrideActiveResponse, error)
	ListCompanyWorkflowOverrideVersions(ctx context.Context, companyID, typeID string, page, pageSize int) ([]CompanyWorkflowOverrideVersionDTO, int, error)
	GetEffectiveWorkflow(ctx context.Context, companyID, typeID string) (*EffectiveWorkflowDTO, error)
	GetCompanyDeadlineContext(ctx context.Context, companyID string) (CompanyDeadlineContext, error)
	GetActiveVersionDeadlineConfig(ctx context.Context, typeID string) (versionNo int, cfg *TemplateDeadlineConfig, err error)
	UpdateActiveVersionDeadlineConfig(ctx context.Context, typeID string, cfg TemplateDeadlineConfig, updatedBy string) error
	ListCompanyGroups(ctx context.Context, companyID, departmentID string, isActive *bool) ([]CompanyGroupDTO, error)
	UpdateWorkflowOverrideStepGroups(ctx context.Context, req UpdateWorkflowOverrideStepGroupsRequest) (*UpdateWorkflowOverrideStepGroupsResponse, error)
}

type CreateRecordRequest struct {
	Subject Subject
	Payload RecordPayload
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

type ListTypesRequest struct {
	Subject Subject
	GroupID string
	Query   string
}

type ListTypesResponse struct {
	Items []DisclosureTypeSummaryDTO `json:"items"`
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
	TemplateCategories []string            `json:"template_categories"`
	Periodicities      []string            `json:"periodicities"`
	DeadlineStrategies []string            `json:"deadline_strategies"`
	MatrixRules        map[string][]string `json:"matrix_rules"`
}

type GetTemplateReferenceDataResponse struct {
	Data TemplateReferenceDataDTO `json:"data"`
}

type UpsertTypeVersionRequest struct {
	Subject               Subject
	TypeID                string                  `json:"type_id"`
	Scope                 string                  `json:"scope"`
	GroupID               string                  `json:"group_id"`
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
	Periodicity           string                  `json:"periodicity"`
	ChannelsText          string                  `json:"channels_text"`
	Beneficiaries         string                  `json:"beneficiaries"`
	ReceivingAuthorities  string                  `json:"receiving_authorities"`
	Format                string                  `json:"format"`
	LegalRisksText        string                  `json:"legal_risks_text"`
	GeneralInfo           string                  `json:"general_info"`
	ReminderMilestones    []string                `json:"reminder_milestones"`
	LegalBases            []LegalBasisDTO         `json:"legal_bases"`
	Checklist             []ChecklistItemDTO      `json:"checklist"`
	Tags                  []string                `json:"tags"`
	DeadlineConfig        *TemplateDeadlineConfig `json:"deadline_config,omitempty"`
	Blocks                []TemplateBlockDTO      `json:"blocks"`
	ChangeNote            string                  `json:"change_note"`
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
	Source         string `json:"source"`          // "auto_fill" | "manual"
	DurationMode   string `json:"duration_mode"`   // "inherit" | "custom"
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
	DurationMode   string `json:"duration_mode"`   // "inherit" | "custom"
	ProcessingDays *int   `json:"processing_days,omitempty"`
	DisplayOrder   int    `json:"display_order"`
}

type UpdateWorkflowOverrideStepGroupsRequest struct {
	Subject       Subject
	TypeID        string                        `json:"type_id"`
	StepID        string                        `json:"step_id"`
	BaseEtag      string                        `json:"base_etag"`
	Groups        []WorkflowStepGroupWriteInput `json:"groups"`
	ClearAll      bool                          `json:"clear_all_groups"`
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
	TypeID        string                 `json:"type_id"`
	VersionNo     int                    `json:"version_no"`
	DeadlineConfig TemplateDeadlineConfig `json:"deadline_config"`
}

type UpdateTemplateDeadlineConfigRequest struct {
	Subject        Subject
	TypeID         string                 `json:"type_id"`
	DeadlineConfig TemplateDeadlineConfig `json:"deadline_config"`
}

type UpdateTemplateDeadlineConfigResponse struct {
	TypeID        string                 `json:"type_id"`
	VersionNo     int                    `json:"version_no"`
	DeadlineConfig TemplateDeadlineConfig `json:"deadline_config"`
	UpdatedBy     string                 `json:"updated_by"`
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

type DisclosureTypeSummaryDTO struct {
	TypeID           string   `json:"type_id"`
	GroupID          string   `json:"group_id"`
	Scope            string   `json:"scope"`
	OwnerCompanyID   string   `json:"owner_company_id"`
	Name             string   `json:"name"`
	Category         string   `json:"category"`
	TemplateCategory string   `json:"template_category"`
	Description      string   `json:"description"`
	DeadlineRule     string   `json:"deadline_rule"`
	Tags             []string `json:"tags"`
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
	Periodicity           string                  `json:"periodicity"`
	ChannelsText          string                  `json:"channels_text"`
	Beneficiaries         string                  `json:"beneficiaries"`
	ReceivingAuthorities  string                  `json:"receiving_authorities"`
	Format                string                  `json:"format"`
	LegalRisksText        string                  `json:"legal_risks_text"`
	GeneralInfo           string                  `json:"general_info"`
	ReminderMilestones    []string                `json:"reminder_milestones"`
	DeadlineConfig        *TemplateDeadlineConfig `json:"deadline_config,omitempty"`
	DeadlineSummary       *DeadlineSummaryDTO     `json:"deadline_summary,omitempty"`
	LegalBases            []LegalBasisDTO         `json:"legal_bases"`
	Checklist             []ChecklistItemDTO      `json:"checklist"`
	Tags                  []string                `json:"tags"`
	Blocks                []TemplateBlockDTO      `json:"blocks"`
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
	DeadlineMode   string               `json:"deadline_mode"`
	FixedDeadline  *FixedDeadlineConfig `json:"fixed_deadline,omitempty"`
	DynamicRule    *DynamicDeadlineRule `json:"dynamic_rule,omitempty"`
	// T0Policy defines how the T0 reference date is resolved for timeline computation.
	// Values: "system_date" | "event_date" | "user_defined". Empty = legacy (no timeline).
	T0Policy       string `json:"t0_policy,omitempty"`
	// DeadlineDays is total calendar days from T0 to the outer deadline.
	DeadlineDays   int    `json:"deadline_days,omitempty"`
	// ProcessingDays is the default per-step duration in calendar days.
	ProcessingDays int    `json:"processing_days,omitempty"`
	// Portal/CMS template config extensions (stored in deadline_config_json).
	TemplateCategory  string `json:"template_category,omitempty"`
	FrequencyInterval int    `json:"frequency_interval,omitempty"`
	FrequencyUnit     string `json:"frequency_unit,omitempty"`
	AllowT0Override   *bool  `json:"allow_t0_override,omitempty"`
	ReportInfoLocked  *bool  `json:"report_info_locked,omitempty"`
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
	TypeID     string                                      `json:"type_id"`
	CompanyID  string                                      `json:"company_id"`
	T0Date     string                                      `json:"t0_date"`
	Timezone   string                                      `json:"timezone"`
	Source     string                                      `json:"source"`
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
	RecordID      string          `json:"record_id"`
	CompanyID     string          `json:"company_id"`
	TypeID        string          `json:"type_id"`
	DepartmentID  string          `json:"department_id"`
	Title         string          `json:"title"`
	Summary       string          `json:"summary"`
	Content       string          `json:"content"`
	PlannedDate   string          `json:"planned_date,omitempty"`
	PublishedDate string          `json:"published_date,omitempty"`
	Status        string          `json:"status"`
	Attachments   []AttachmentDTO `json:"attachments"`
	EvidenceLink        string          `json:"evidence_link,omitempty"`
	WorkflowInstanceID  string          `json:"workflow_instance_id,omitempty"`
	CreatedBy           string          `json:"created_by"`
	UpdatedBy     string          `json:"updated_by"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}
