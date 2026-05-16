package app

import (
	"context"
	"time"
)

// Status values for ad_hoc_proposals.
const (
	StatusDraft                 = "ad_hoc_draft"
	StatusPendingFocalApproval  = "pending_focal_approval"
	StatusPendingAdminApproval  = "pending_admin_approval"
	StatusApproved              = "approved"
	StatusRejected              = "rejected"
	StatusCancelled             = "cancelled"
)

type Service interface {
	CreateProposal(ctx context.Context, req CreateProposalRequest) (*ProposalDTO, error)
	SubmitProposal(ctx context.Context, req ProposalActionRequest) (*ProposalDTO, error)
	FocalApprove(ctx context.Context, req ProposalActionRequest) (*ProposalDTO, error)
	AdminApprove(ctx context.Context, req AdminApproveRequest) (*AdminApproveResponse, error)
	Reject(ctx context.Context, req RejectRequest) (*ProposalDTO, error)
	Cancel(ctx context.Context, req ProposalActionRequest) (*ProposalDTO, error)
	GetProposal(ctx context.Context, req GetProposalRequest) (*ProposalDTO, error)
	ListProposals(ctx context.Context, req ListProposalsRequest) (*ListProposalsResponse, error)
}

type Repository interface {
	Insert(ctx context.Context, p ProposalDTO) (*ProposalDTO, error)
	FindByID(ctx context.Context, companyID, proposalID string) (*ProposalDTO, error)
	UpdateStatus(ctx context.Context, upd StatusUpdate) (*ProposalDTO, error)
	List(ctx context.Context, companyID string, statusFilter []string, page, pageSize int) ([]ProposalDTO, int, error)
}

// RecordCreator is the cross-module interface the ad-hoc service uses to submit a disclosure record.
// Implemented by disclosureapp.Service.SubmitRecord — injected to avoid circular imports.
type RecordCreator interface {
	// CreateAndSubmitRecord creates a Draft record for the proposal and immediately submits it.
	// When workflow is enabled, also creates a workflow instance for the record.
	// Returns record_id and workflow_instance_id (may be empty when workflow is disabled).
	CreateAndSubmitRecord(ctx context.Context, companyID, typeID, createdByMembershipID, title string) (recordID, workflowInstanceID string, err error)
}

type Subject struct {
	UserID       string
	MembershipID string
	CompanyID    string
}

// WorkflowStepOverride carries the per-step overrides in a proposal.
// Phase 1: only processing_days override is allowed; steps cannot be added/removed.
type WorkflowStepOverride struct {
	StepID         string `json:"step_id"`
	ProcessingDays int    `json:"processing_days,omitempty"`
}

type CreateProposalRequest struct {
	Subject          Subject
	TypeID           string                 `json:"type_id"`
	StepOverrides    []WorkflowStepOverride `json:"step_overrides"`
	ProposedT0Date   string                 `json:"proposed_t0_date,omitempty"` // YYYY-MM-DD
	ProposedDeadline string                 `json:"proposed_deadline_date,omitempty"`
	ChangeNote       string                 `json:"change_note,omitempty"`
}

type ProposalActionRequest struct {
	Subject    Subject
	ProposalID string
	Comment    string `json:"comment,omitempty"`
}

type AdminApproveRequest struct {
	Subject            Subject
	ProposalID         string
	Comment            string `json:"comment,omitempty"`
	FinalT0Date        string `json:"final_t0_date,omitempty"`        // YYYY-MM-DD
	FinalDeadlineDate  string `json:"final_deadline_date,omitempty"`  // YYYY-MM-DD or days string from FE
	AdjustmentNote     string `json:"adjustment_note,omitempty"`
}

type AdminApproveResponse struct {
	Proposal           ProposalDTO `json:"proposal"`
	RecordID           string      `json:"record_id"`
	WorkflowInstanceID string      `json:"workflow_instance_id"`
}

type RejectRequest struct {
	Subject      Subject
	ProposalID   string
	RejectReason string `json:"reject_reason"`
}

type GetProposalRequest struct {
	Subject    Subject
	ProposalID string
}

type ListProposalsRequest struct {
	Subject      Subject
	StatusFilter []string
	Page         int
	PageSize     int
}

type ListProposalsResponse struct {
	Items    []ProposalDTO `json:"items"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
	Total    int           `json:"total"`
}

type ProposalDTO struct {
	ProposalID           string                 `json:"proposal_id"`
	CompanyID            string                 `json:"company_id"`
	TypeID               string                 `json:"type_id"`
	Status               string                 `json:"status"`
	StepOverrides        []WorkflowStepOverride `json:"step_overrides"`
	ProposedT0Date       *string                `json:"proposed_t0_date,omitempty"`
	ProposedDeadlineDate *string                `json:"proposed_deadline_date,omitempty"`
	ChangeNote           string                 `json:"change_note,omitempty"`
	FocalApprovedBy      string                 `json:"focal_approved_by,omitempty"`
	FocalApprovedAt      *time.Time             `json:"focal_approved_at,omitempty"`
	AdminApprovedBy      string                 `json:"admin_approved_by,omitempty"`
	AdminApprovedAt      *time.Time             `json:"admin_approved_at,omitempty"`
	RejectedBy           string                 `json:"rejected_by,omitempty"`
	RejectedAt           *time.Time             `json:"rejected_at,omitempty"`
	RejectReason         string                 `json:"reject_reason,omitempty"`
	RecordID             string                 `json:"record_id,omitempty"`
	WorkflowInstanceID   string                 `json:"workflow_instance_id,omitempty"`
	CreatedBy            string                 `json:"created_by"`
	CreatedAt            time.Time              `json:"created_at"`
	UpdatedAt            time.Time              `json:"updated_at"`
}

// StatusUpdate carries fields for a state transition update.
type StatusUpdate struct {
	ProposalID         string
	CompanyID          string
	Status             string
	ActorMembershipID  string
	ActorUserID        string
	RejectReason       string
	RecordID           string
	WorkflowInstanceID string
}
