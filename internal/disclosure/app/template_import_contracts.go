package app

import (
	"strings"
	"time"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
)

// Phase A locked constants for CMS Template Import.
const (
	// TemplateImportSchemaVersion is the formal JSON file specification version.
	// Distinct from the disclosure template version_no (which is server-owned and starts at 1).
	TemplateImportSchemaVersion = "1.0"

	// MaxTemplateImportFileSizeBytes is the maximum allowed size of the uploaded JSON file (2 MiB).
	MaxTemplateImportFileSizeBytes = 2 * 1024 * 1024 // 2097152 bytes

	// TemplateImportTokenPurpose is the dedicated HMAC signature domain.
	TemplateImportTokenPurpose = "template_import"

	// TemplateImportTokenTTLMinutes is the lifespan of the stateless validation token.
	TemplateImportTokenTTLMinutes = 15

	// Canonical target taxonomy defaults when omitted in input file.
	DefaultPeriodicGroupID         = "group-001" // "Định kỳ"
	DefaultIrregularGroupID        = "group-002" // "Bất thường"
	DefaultPeriodicDisplayGroupCode = "display_groups_003" // "Tài chính, Kinh doanh"
	DefaultIrregularDisplayGroupCode = "display_groups_001" // "Tuân thủ, Quản trị & Quản lý Rủi ro"

	// Target catalog table and key for department references.
	TargetDepartmentCatalogTable = "workflow_template_departments"
	TargetDepartmentKeyColumn    = "department_code"
)

// FieldPortabilityClassification identifies the import treatment for every field.
const (
	FieldPortabilityImportAsIs           = "IMPORT_AS_IS"
	FieldPortabilityNormalize            = "NORMALIZE"
	FieldPortabilityReselect             = "RESELECT"
	FieldPortabilityServerGenerated      = "SERVER_GENERATED"
	FieldPortabilityClearOnImport        = "CLEAR_ON_IMPORT"
	FieldPortabilityDerivedNotImportable = "DERIVED_NOT_IMPORTABLE"
	FieldPortabilityForbidden            = "FORBIDDEN"
)

// ValidationIssueSeverity defines the severity level of import validation findings.
const (
	ValidationSeverityBlocker = "BLOCKER"
	ValidationSeverityWarning = "WARNING"
)

// TemplateImportEnvelopeV1 represents the root structure of the uploaded .json file.
type TemplateImportEnvelopeV1 struct {
	SchemaVersion string                       `json:"schema_version"`
	Metadata      *TemplateImportMetadataV1    `json:"metadata,omitempty"`
	Template      TemplateImportDefinitionV1   `json:"template"`
}

// TemplateImportMetadataV1 captures optional authoring provenance.
type TemplateImportMetadataV1 struct {
	ExportedAt   string `json:"exported_at,omitempty"`
	ExportedBy   string `json:"exported_by,omitempty"`
	SourceSystem string `json:"source_system,omitempty"`
	Notes        string `json:"notes,omitempty"`
}

// TemplateImportDefinitionV1 represents the core template configuration within the import file.
type TemplateImportDefinitionV1 struct {
	TypeID                string                           `json:"type_id,omitempty"`
	Name                  string                           `json:"name"`
	Description           string                           `json:"description,omitempty"`
	Category              string                           `json:"category,omitempty"`
	TemplateCategory      string                           `json:"template_category"`
	GroupID               string                           `json:"group_id,omitempty"`
	DeadlineStrategy      string                           `json:"deadline_strategy,omitempty"`
	DeadlineRule          string                           `json:"deadline_rule"`
	Periodicity           string                           `json:"periodicity,omitempty"`
	DisplayGroupCodes     []string                         `json:"display_group_codes,omitempty"`
	LegalBasis            string                           `json:"legal_basis,omitempty"`
	Applicability         string                           `json:"applicability,omitempty"`
	ImplementationContent string                           `json:"implementation_content,omitempty"`
	ImplementationNotes   string                           `json:"implementation_notes,omitempty"`
	SpecialCases          string                           `json:"special_cases,omitempty"`
	ReportContent         string                           `json:"report_content,omitempty"`
	RequiredDocs          string                           `json:"required_docs,omitempty"`
	ChannelsText          string                           `json:"channels_text,omitempty"`
	Beneficiaries         string                           `json:"beneficiaries,omitempty"`
	ReceivingAuthorities  string                           `json:"receiving_authorities,omitempty"`
	Format                string                           `json:"format,omitempty"`
	LegalRisksText        string                           `json:"legal_risks_text,omitempty"`
	GeneralInfo           string                           `json:"general_info,omitempty"`
	Tags                  []string                         `json:"tags,omitempty"`
	LegalBases            []LegalBasisDTO                  `json:"legal_bases,omitempty"`
	Checklist             []ChecklistItemDTO               `json:"checklist,omitempty"`
	DeadlineConfig        *TemplateImportDeadlineConfigV1  `json:"deadline_config,omitempty"`
	ApplicabilityRules    *applicability.TemplateApplicabilityRules `json:"applicability_rules,omitempty"`
	Workflow              *TemplateImportWorkflowV1        `json:"workflow,omitempty"`
}

// TemplateImportDeadlineConfigV1 represents Periodicity V2 authoring configuration.
type TemplateImportDeadlineConfigV1 struct {
	FrequencyUnit        string `json:"frequency_unit,omitempty"`
	CycleAnchorDay       *int   `json:"cycle_anchor_day,omitempty"`
	CycleAnchorWeekday   string `json:"cycle_anchor_weekday,omitempty"`
	MonthInQuarter       *int   `json:"month_in_quarter,omitempty"`
	ApplicableFromMode   string `json:"applicable_from_mode,omitempty"`
	ApplicableFromSlot   string `json:"applicable_from_slot,omitempty"`
	ApplicableTo         string `json:"applicable_to,omitempty"`
	DurationType         string `json:"duration_type,omitempty"`
	DeadlineDurationType string `json:"deadline_duration_type,omitempty"`
	DeadlineDays         *int   `json:"deadline_days,omitempty"`
	OpenDaysBefore       *int   `json:"open_days_before,omitempty"`
}

// TemplateImportWorkflowV1 represents the template-pinned Model A workflow structure.
type TemplateImportWorkflowV1 struct {
	Steps []TemplateImportWorkflowStepV1 `json:"steps"`
}

// TemplateImportDepartmentRefV1 is the preferred human-authorable department reference.
// Prefer code/name over opaque DB UUIDs. Validate resolves by exact code, then exact name,
// else mapping_required (admin picks CMS-visible catalog target).
type TemplateImportDepartmentRefV1 struct {
	Code string `json:"code,omitempty"`
	Name string `json:"name,omitempty"`
}

// TemplateImportWorkflowStepV1 represents a single workflow step in the import file.
type TemplateImportWorkflowStepV1 struct {
	StepID          string                               `json:"step_id,omitempty"` // Optional file-local; stripped; Confirm generates server UUID
	Stage           string                               `json:"stage"`
	Description     string                               `json:"description,omitempty"`
	Instructions    string                               `json:"instructions,omitempty"`
	Department      *TemplateImportDepartmentRefV1       `json:"department,omitempty"` // Preferred portable ref
	DepartmentID    string                               `json:"department_id,omitempty"` // Legacy portable source code (not DB UUID)
	DepartmentName  string                               `json:"department_name,omitempty"` // Legacy / display name
	AssigneeRoles   []string                             `json:"assignee_roles,omitempty"` // Preferred static role codes
	AssigneeRoleIDs []string                             `json:"assignee_role_ids,omitempty"` // Legacy alias for assignee_roles
	ProcessingDays  int                                  `json:"processing_days,omitempty"`
	DueRule         string                               `json:"due_rule,omitempty"`
	DisplayOrder    int                                  `json:"display_order,omitempty"`
	ReminderConfig  *TemplateImportStepReminderConfigV1  `json:"reminder_config,omitempty"`
	Documents       []TemplateImportWorkflowDocumentV1   `json:"documents,omitempty"`
}

// DepartmentMappingSourceKey returns a stable non-empty mapping key for Confirm/FE.
// Prefer department source code; else "dept-name:<normalized name>". Never empty when name exists.
func DepartmentMappingSourceKey(departmentID, departmentName string) string {
	if id := strings.TrimSpace(departmentID); id != "" {
		return id
	}
	name := strings.TrimSpace(departmentName)
	if name == "" {
		return ""
	}
	return "dept-name:" + strings.ToLower(name)
}

// TemplateImportStepReminderConfigV1 represents automated reminder settings for a step.
type TemplateImportStepReminderConfigV1 struct {
	Enabled      bool   `json:"enabled,omitempty"`
	OffsetsDays  []int  `json:"offsets_days,omitempty"`
	TemplateKey  string `json:"template_key,omitempty"`
}

// TemplateImportWorkflowDocumentV1 represents a document requirement for a step.
type TemplateImportWorkflowDocumentV1 struct {
	Name             string `json:"name"`
	Required         bool   `json:"required,omitempty"`
	TemplateFileName string `json:"template_file_name,omitempty"`
	TemplateFileID   string `json:"template_file_id,omitempty"` // Non-portable, cleared on import
}

// ValidateTemplateImportRequest represents the input to ValidateTemplateImport service method.
type ValidateTemplateImportRequest struct {
	Subject   Subject
	Filename  string
	FileBytes []byte
}

// ValidateTemplateImportResponse is the complete response DTO for POST .../import/validate.
type ValidateTemplateImportResponse struct {
	ParseValid         bool                               `json:"parse_valid"`
	DomainValid        bool                               `json:"domain_valid"`
	MappingRequired    bool                               `json:"mapping_required"`
	CanConfirm         bool                               `json:"can_confirm"`
	ActivationReady    bool                               `json:"activation_ready"`
	SuggestedTypeID    string                             `json:"suggested_type_id"`
	ValidationToken    string                             `json:"validation_token,omitempty"`
	TokenExpiresAt     string                             `json:"token_expires_at,omitempty"`
	Errors             []TemplateImportValidationIssueDTO `json:"errors"`
	Warnings           []TemplateImportValidationIssueDTO `json:"warnings"`
	RequiredMappings   []TemplateImportRequiredMappingDTO `json:"required_mappings"`
	ActivationBlockers []ActivationBlockerDTO             `json:"activation_blockers"`
	Preview            *TemplateImportPreviewDTO          `json:"preview,omitempty"`
}

// TemplateImportValidationIssueDTO represents a blocker or warning finding.
type TemplateImportValidationIssueDTO struct {
	Code            string `json:"code"`
	FieldPath       string `json:"field_path"`
	Severity        string `json:"severity"` // BLOCKER | WARNING
	Message         string `json:"message"`
	SuggestedAction string `json:"suggested_action,omitempty"`
}

// TemplateImportRequiredMappingDTO represents a department that must be resolved before confirm.
type TemplateImportRequiredMappingDTO struct {
	Type          string `json:"type"` // "department"
	SourceID      string `json:"source_id"`
	SourceName    string `json:"source_name"`
	TargetID      string `json:"target_id,omitempty"`
	IsAutoMatched bool   `json:"is_auto_matched"`
}

// TemplateImportPreviewDTO summarizes the template attributes for the user preview screen.
type TemplateImportPreviewDTO struct {
	Name                     string                      `json:"name"`
	TemplateCategory         string                      `json:"template_category"`
	Periodicity              string                      `json:"periodicity,omitempty"`
	DeadlineRule             string                      `json:"deadline_rule"`
	ResolvedGroupID          string                      `json:"resolved_group_id"`
	ResolvedDisplayGroupCodes []string                    `json:"resolved_display_group_codes"`
	ApplicableFromMode       string                      `json:"applicable_from_mode,omitempty"`
	ApplicableTo             string                      `json:"applicable_to,omitempty"`
	WorkflowStepCount        int                         `json:"workflow_step_count"`
	DocumentRequirementCount int                         `json:"document_requirement_count"`
	NormalizedTemplate       *TemplateImportDefinitionV1 `json:"normalized_template,omitempty"`
}

// ConfirmTemplateImportRequest is the request body for POST .../import/confirm.
type ConfirmTemplateImportRequest struct {
	Subject            Subject                    `json:"-"`
	ValidationToken    string                     `json:"validation_token"`
	TargetTypeID       string                     `json:"target_type_id"`
	TargetName         string                     `json:"target_name"`
	DepartmentMappings map[string]string          `json:"department_mappings,omitempty"` // source_code -> target_code
	NormalizedTemplate TemplateImportDefinitionV1 `json:"normalized_template"`
}

// ConfirmTemplateImportResponse is the response body for POST .../import/confirm.
// Uses source-accurate lifecycle properties (root_status="active", active_version_no=0, is_released=false).
type ConfirmTemplateImportResponse struct {
	TypeID      string    `json:"type_id"`
	VersionNo   int       `json:"version_no"`   // strictly 1
	IsActive    bool      `json:"is_active"`    // strictly false
	IsReleased  bool      `json:"is_released"`  // strictly false
	PortalState string    `json:"portal_state"` // strictly "not_active"
	RootStatus  string    `json:"root_status"`  // strictly "active"
	Name        string    `json:"name"`
	CreatedAt   time.Time `json:"created_at"`
}

// TemplateImportTokenClaims represents the payload bound into the stateless HMAC validation token.
type TemplateImportTokenClaims struct {
	SchemaVersion string `json:"schema_version"`
	PayloadHash   string `json:"payload_hash"` // SHA-256 hex string of canonical normalized template
	ActorID       string `json:"actor_id"`
	IssuedAt      int64  `json:"issued_at"`
	ExpiresAt     int64  `json:"expires_at"`
}
