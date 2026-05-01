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
	TypeID                string             `json:"type_id"`
	GroupID               string             `json:"group_id"`
	Name                  string             `json:"name"`
	Category              string             `json:"category"`
	TemplateCategory      string             `json:"template_category"`
	DeadlineStrategy      string             `json:"deadline_strategy"`
	Description           string             `json:"description"`
	LegalBasis            string             `json:"legal_basis"`
	Applicability         string             `json:"applicability"`
	ImplementationContent string             `json:"implementation_content"`
	ImplementationNotes   string             `json:"implementation_notes"`
	SpecialCases          string             `json:"special_cases"`
	ReportContent         string             `json:"report_content"`
	RequiredDocs          string             `json:"required_docs"`
	DeadlineRule          string             `json:"deadline_rule"`
	Periodicity           string             `json:"periodicity"`
	ChannelsText          string             `json:"channels_text"`
	Beneficiaries         string             `json:"beneficiaries"`
	ReceivingAuthorities  string             `json:"receiving_authorities"`
	Format                string             `json:"format"`
	LegalRisksText        string             `json:"legal_risks_text"`
	GeneralInfo           string             `json:"general_info"`
	Tags                  []string           `json:"tags"`
	Blocks                []TemplateBlockDTO `json:"blocks"`
	ChangeNote            string             `json:"change_note"`
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
	Name             string   `json:"name"`
	Category         string   `json:"category"`
	TemplateCategory string   `json:"template_category"`
	Description      string   `json:"description"`
	DeadlineRule     string   `json:"deadline_rule"`
	Tags             []string `json:"tags"`
}

type DisclosureTypeDTO struct {
	VersionNo             int                `json:"version_no"`
	TypeID                string             `json:"type_id"`
	GroupID               string             `json:"group_id"`
	Name                  string             `json:"name"`
	Category              string             `json:"category"`
	TemplateCategory      string             `json:"template_category"`
	DeadlineStrategy      string             `json:"deadline_strategy"`
	Description           string             `json:"description"`
	LegalBasis            string             `json:"legal_basis"`
	Applicability         string             `json:"applicability"`
	ImplementationContent string             `json:"implementation_content"`
	ImplementationNotes   string             `json:"implementation_notes"`
	SpecialCases          string             `json:"special_cases"`
	ReportContent         string             `json:"report_content"`
	RequiredDocs          string             `json:"required_docs"`
	DeadlineRule          string             `json:"deadline_rule"`
	Periodicity           string             `json:"periodicity"`
	ChannelsText          string             `json:"channels_text"`
	Beneficiaries         string             `json:"beneficiaries"`
	ReceivingAuthorities  string             `json:"receiving_authorities"`
	Format                string             `json:"format"`
	LegalRisksText        string             `json:"legal_risks_text"`
	GeneralInfo           string             `json:"general_info"`
	Tags                  []string           `json:"tags"`
	Blocks                []TemplateBlockDTO `json:"blocks"`
}

type TemplateBlockDTO struct {
	BlockID      string         `json:"block_id"`
	BlockKey     string         `json:"block_key"`
	BlockType    string         `json:"block_type"`
	Title        string         `json:"title"`
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
	EvidenceLink  string          `json:"evidence_link,omitempty"`
	CreatedBy     string          `json:"created_by"`
	UpdatedBy     string          `json:"updated_by"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}
