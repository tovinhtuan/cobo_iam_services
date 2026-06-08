package app

import (
	"context"
	"time"
)

// Status values for ad_hoc_proposals.
const (
	StatusDraft                = "ad_hoc_draft"
	StatusPendingFocalApproval = "pending_focal_approval"
	StatusPendingAdminApproval = "pending_admin_approval"
	StatusApproved             = "approved"
	StatusRejected             = "rejected"
	StatusCancelled            = "cancelled"
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
	ListEligibleControllers(ctx context.Context, req ListEligibleControllersRequest) ([]EligibleController, error)
}

type ListEligibleControllersRequest struct {
	Subject Subject
}

type Repository interface {
	Insert(ctx context.Context, p ProposalDTO) (*ProposalDTO, error)
	FindByID(ctx context.Context, companyID, proposalID string) (*ProposalDTO, error)
	// The bool return ("applied") is true only when this call's own guarded UPDATE
	// matched and applied the transition (ADR-2 EV-1); false on idempotent replay
	// (EV-2) or any non-success outcome — callers must use it to avoid
	// double-emitting transition metrics.
	UpdateStatus(ctx context.Context, upd StatusUpdate) (*ProposalDTO, bool, error)
	ReserveAdminApproval(ctx context.Context, in ReserveAdminApprovalInput) (*AdminApprovalReservation, error)
	SaveAdminApprovalProgress(ctx context.Context, companyID, proposalID, idemKey, recordID, workflowID, lastError string) error
	// The bool return ("applied") follows the same EV-1/EV-2 contract as UpdateStatus.
	CompleteAdminApproval(ctx context.Context, upd StatusUpdate, idemKey string) (*ProposalDTO, bool, error)
	List(ctx context.Context, companyID string, statusFilter []string, page, pageSize int) ([]ProposalDTO, int, error)
}

type TypeCatalog interface {
	GetTemplateCategory(ctx context.Context, companyID, typeID string) (string, error)
	GetTypeDisplayName(ctx context.Context, companyID, typeID string) (string, error)
}

// CreateRecordOpts carries optional inputs for record + workflow creation (WF5-B, periodic T0 in Batch 2).
type CreateRecordOpts struct {
	StepOverrides []WorkflowStepOverride // ad-hoc only
	CycleStart    *time.Time             // periodic materialize (Batch 2)
	PlannedDate   string                 // periodic: cycle due_date → disclosure_records.planned_date (YYYY-MM-DD); empty = not set
	// RecordID, when non-empty, is the deterministic record ID (ADR-1B) the
	// caller pre-allocated; passed through to disclosureapp.CreateRecordRequest
	// so retries are idempotent instead of creating orphaned/duplicate records.
	RecordID string
}

// RecordCreator is the cross-module interface the ad-hoc service uses to submit a disclosure record.
// Implemented by disclosureapp.Service.SubmitRecord — injected to avoid circular imports.
type RecordCreator interface {
	// CreateAndSubmitRecord creates a Draft record for the proposal and immediately submits it.
	// When workflow is enabled, also creates a workflow instance for the record.
	// Returns record_id and workflow_instance_id (may be empty when workflow is disabled).
	// Periodic worker uses this entrypoint with empty opts (see disclosure.PeriodicRecordCreator).
	CreateAndSubmitRecord(ctx context.Context, companyID, typeID, createdByMembershipID, title string, t0Date *time.Time) (recordID, workflowInstanceID string, err error)
	// CreateAndSubmitRecordWithOpts is the ad-hoc path with step overrides and future periodic options.
	CreateAndSubmitRecordWithOpts(ctx context.Context, companyID, typeID, createdByMembershipID, title string, t0Date *time.Time, opts CreateRecordOpts) (recordID, workflowInstanceID string, err error)
}

// EligibleController is a membership that has the ad_hoc_alert.process_control permission
// and can be assigned as the process controller when creating a proposal.
type EligibleController struct {
	MembershipID string `json:"membership_id"`
	FullName     string `json:"full_name"`
	Email        string `json:"email"`
}

// MemberInfo carries resolved identity fields needed for notification dispatch.
// Distinct from EligibleController to avoid exposing UserID in the HTTP API response.
type MemberInfo struct {
	UserID       string
	MembershipID string
	Email        string
	FullName     string
}

// ProposalNotifier dispatches in-app and email notifications after proposal state transitions.
// All methods are fire-and-forget: implementations log errors but must never propagate them.
type ProposalNotifier interface {
	NotifyFocalsForReview(ctx context.Context, proposal ProposalDTO, focals []MemberInfo)
	NotifyControllerForReview(ctx context.Context, proposal ProposalDTO, controller MemberInfo)
	NotifyCreatorApproved(ctx context.Context, proposal ProposalDTO, creator MemberInfo)
	NotifyCreatorRejected(ctx context.Context, proposal ProposalDTO, creator MemberInfo)
}

// MembershipValidator lets the adhoc service validate a target membership
// without coupling to the authorization module's internal implementation.
type MembershipValidator interface {
	IsActiveMembership(ctx context.Context, companyID, membershipID string) (bool, error)
	HasPermission(ctx context.Context, companyID, membershipID, permissionCode string) (bool, error)
	HasActiveRoleCode(ctx context.Context, companyID, membershipID, roleCode string) (bool, error)
	ListMembersWithPermission(ctx context.Context, companyID, permissionCode, excludeMembershipID string) ([]EligibleController, error)
	// ResolveMembership returns identity fields for a single active membership.
	// Returns nil, nil when the membership does not exist or is inactive.
	ResolveMembership(ctx context.Context, companyID, membershipID string) (*MemberInfo, error)
	// ListMembersWithPermissionFull returns all active members holding permissionCode,
	// including their UserID for in-app notification dispatch.
	ListMembersWithPermissionFull(ctx context.Context, companyID, permissionCode string) ([]MemberInfo, error)
}

// Metrics records ad-hoc proposal lifecycle observability signals (Batch 5(a) /
// AK.3 — minimum-viable instrumentation: exactly one counter,
// cobo_adhoc_proposal_transition_total). Implementations must be safe for
// concurrent use. RecordTransition must be called only for transitions that
// were actually applied by this request (ADR-2 EV-1) — never for idempotent
// replays (EV-2) — to avoid corrupting rate()-based alerting in Batch 6.
type Metrics interface {
	RecordTransition(companyID, fromStatus, toStatus string)
}

// noopMetrics is the zero-cost default used when ADHOC_EMAIL_METRICS_ENABLED=false.
type noopMetrics struct{}

func (noopMetrics) RecordTransition(string, string, string) {}

// NewNoopMetrics returns a no-op Metrics implementation. Exported because the
// wiring decision (ADHOC_EMAIL_METRICS_ENABLED) lives in httpserver, outside
// this package, and the constructor's metrics argument must never be a literal nil.
func NewNoopMetrics() Metrics { return noopMetrics{} }

// RoleCodeAdminDoanhNghiep is the tenant company-admin role that may self-assign as process controller.
const RoleCodeAdminDoanhNghiep = "admin_doanh_nghiep"

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
	Subject                      Subject
	TypeID                       string                 `json:"type_id"`
	StepOverrides                []WorkflowStepOverride `json:"step_overrides"`
	ProposedT0Date               string                 `json:"proposed_t0_date,omitempty"` // YYYY-MM-DD
	ProposedDeadlineDays         int                    `json:"proposed_deadline_days,omitempty"`
	ProposedDeadline             string                 `json:"proposed_deadline_date,omitempty"` // YYYY-MM-DD or legacy day count string
	ChangeNote                   string                 `json:"change_note,omitempty"`
	ProcessControllerMembershipID string                `json:"process_controller_membership_id,omitempty"`
}

type ProposalActionRequest struct {
	Subject    Subject
	ProposalID string
	Comment    string `json:"comment,omitempty"`
}

type AdminApproveRequest struct {
	Subject           Subject
	ProposalID        string
	IdempotencyKey    string `json:"-"`
	Comment           string `json:"comment,omitempty"`
	FinalT0Date       string `json:"final_t0_date,omitempty"`       // YYYY-MM-DD
	FinalDeadlineDate string `json:"final_deadline_date,omitempty"` // YYYY-MM-DD
	AdjustmentNote    string `json:"adjustment_note,omitempty"`
}

type ReserveAdminApprovalInput struct {
	CompanyID         string
	ProposalID        string
	IdempotencyKey    string
	ActorMembershipID string
}

type AdminApprovalReservation struct {
	Proposal           *ProposalDTO
	ReplayApproved     bool
	ProgressRecordID   string
	ProgressWorkflowID string
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
	ProposalID              string                 `json:"proposal_id"`
	CompanyID               string                 `json:"company_id"`
	TypeID                  string                 `json:"type_id"`
	Status                  string                 `json:"status"`
	StepOverrides           []WorkflowStepOverride `json:"step_overrides"`
	ProposedT0Date          *string                `json:"proposed_t0_date,omitempty"`
	FinalT0Date             *string                `json:"final_t0_date,omitempty"`
	FinalDeadlineDate       *string                `json:"final_deadline_date,omitempty"`
	AdjustmentNote          string                 `json:"adjustment_note,omitempty"`
	ProposedDeadlineDays    *int                   `json:"proposed_deadline_days,omitempty"`
	ProposedDeadlineDate    *string                `json:"proposed_deadline_date,omitempty"` // calendar date when set explicitly
	ChangeNote              string                 `json:"change_note,omitempty"`
	FocalApprovedBy         string                 `json:"focal_approved_by,omitempty"`
	FocalApprovedAt         *time.Time             `json:"focal_approved_at,omitempty"`
	AdminApprovedBy         string                 `json:"admin_approved_by,omitempty"`
	AdminApprovedAt         *time.Time             `json:"admin_approved_at,omitempty"`
	RejectedBy              string                 `json:"rejected_by,omitempty"`
	RejectedAt              *time.Time             `json:"rejected_at,omitempty"`
	RejectReason            string                 `json:"reject_reason,omitempty"`
	RecordID                string                 `json:"record_id,omitempty"`
	WorkflowInstanceID      string                 `json:"workflow_instance_id,omitempty"`
	CreatedBy               string                 `json:"created_by"`
	ProcessControllerID     string                 `json:"process_controller_id,omitempty"`
	CreatedAt               time.Time              `json:"created_at"`
	UpdatedAt               time.Time              `json:"updated_at"`
}

// StatusUpdate carries fields for a state transition update.
type StatusUpdate struct {
	ProposalID string
	CompanyID  string
	Status     string
	// FromStatus is the expected current status (ADR-2 lost-update guard).
	// The repository binds it as `AND status = ?` on the transition UPDATE so a
	// concurrent winner cannot be silently overwritten by a stale transition.
	FromStatus               string
	ActorMembershipID        string
	ActorUserID              string
	SetFocalApprovalMetadata bool
	RejectReason             string
	RecordID                 string
	WorkflowInstanceID       string
	FinalT0Date              *string
	FinalDeadlineDate        *string
	AdjustmentNote           string
}
