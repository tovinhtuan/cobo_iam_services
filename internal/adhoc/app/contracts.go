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
	// PatchDraftProposal updates editable draft fields and optionally replaces workflow_steps atomically.
	PatchDraftProposal(ctx context.Context, req PatchDraftProposalRequest) (*ProposalDTO, error)
	SubmitProposal(ctx context.Context, req ProposalActionRequest) (*ProposalDTO, error)
	// Approve replaces FocalApprove for the one-round multi-reviewer flow (v3 D1/D3/D4).
	Approve(ctx context.Context, req ApproveRequest) (*ApproveResponse, error)
	// AdminApprove is kept unchanged for the legacy two-round flow.
	//
	// @deprecated: serves only (1) legacy clients still calling POST .../admin-approve
	// directly, and (2) FinalizeLegacyApproval's internal reuse for the one-time
	// migration endpoint. Do not call from any new code path. Removed only once
	// both conditions in the migration runbook (§12.5) are satisfied.
	AdminApprove(ctx context.Context, req AdminApproveRequest) (*AdminApproveResponse, error)
	Reject(ctx context.Context, req RejectRequest) (*ProposalDTO, error)
	Cancel(ctx context.Context, req ProposalActionRequest) (*ProposalDTO, error)
	GetProposal(ctx context.Context, req GetProposalRequest) (*ProposalDTO, error)
	ListProposals(ctx context.Context, req ListProposalsRequest) (*ListProposalsResponse, error)
	ListEligibleReviewers(ctx context.Context, req ListEligibleReviewersRequest) ([]EligibleController, error)
	// FinalizeLegacyApproval is a thin wrapper around AdminApprove (field-mapped per
	// §5.3) used by the temporary migration endpoint to auto-finalize proposals stuck
	// at pending_admin_approval. Gated internally on rbac.manage.
	FinalizeLegacyApproval(ctx context.Context, sub Subject, companyID, proposalID string) error
	// ListPendingLegacyApprovals is gated on rbac.manage (platform admin only) since
	// it scans across all companies.
	ListPendingLegacyApprovals(ctx context.Context, sub Subject) ([]PendingApprovalRow, error)
}

type ListEligibleReviewersRequest struct {
	Subject Subject
}

// PendingApprovalRow identifies a proposal still stuck at pending_admin_approval,
// for the one-time legacy migration endpoint (§6.7/A1).
type PendingApprovalRow struct {
	ProposalID string
	CompanyID  string
}

type Repository interface {
	Insert(ctx context.Context, p ProposalDTO) (*ProposalDTO, error)
	FindByID(ctx context.Context, companyID, proposalID string) (*ProposalDTO, error)
	// UpdateDraft atomically updates draft proposal fields + optional workflow snapshot.
	// Returns conflict when the proposal is not in ad_hoc_draft.
	UpdateDraft(ctx context.Context, upd DraftUpdate) (*ProposalDTO, error)
	// The bool return ("applied") is true only when this call's own guarded UPDATE
	// matched and applied the transition (ADR-2 EV-1); false on idempotent replay
	// (EV-2) or any non-success outcome — callers must use it to avoid
	// double-emitting transition metrics.
	UpdateStatus(ctx context.Context, upd StatusUpdate) (*ProposalDTO, bool, error)
	ReserveAdminApproval(ctx context.Context, in ReserveAdminApprovalInput) (*AdminApprovalReservation, error)
	SaveAdminApprovalProgress(ctx context.Context, companyID, proposalID, idemKey, recordID, workflowID, lastError string) error
	// The bool return ("applied") follows the same EV-1/EV-2 contract as UpdateStatus.
	CompleteAdminApproval(ctx context.Context, upd StatusUpdate, idemKey string) (*ProposalDTO, bool, error)
	// List returns company-scoped proposals. When createdByMembershipID is non-empty,
	// only rows with created_by = that membership are returned (server-side scope=my).
	List(ctx context.Context, companyID string, statusFilter []string, createdByMembershipID string, page, pageSize int) ([]ProposalDTO, int, error)

	// ReserveVote casts one reviewer's vote inside a single FOR UPDATE transaction
	// (Phase A of §6.5). Returns 403 (not assigned) / 409 (wrong status) as errors.
	ReserveVote(ctx context.Context, in ReserveVoteInput) (*VoteReservation, error)
	// CompleteFinalize is Phase C of §6.5 — the guarded UPDATE that transitions
	// pending_focal_approval -> approved. The bool return follows the EV-1/EV-2
	// contract above.
	CompleteFinalize(ctx context.Context, upd StatusUpdate) (*ProposalDTO, bool, error)
	IsAssignedReviewer(ctx context.Context, companyID, proposalID, membershipID string) (bool, error)
	ListReviewers(ctx context.Context, companyID, proposalID string) ([]ReviewerDTO, error)
	ListApprovals(ctx context.Context, companyID, proposalID string) ([]ApprovalDTO, error)
	// ListPendingAdminApproval scans across all companies (no tenant scoping) for
	// the one-time legacy migration endpoint (§6.7/A1).
	ListPendingAdminApproval(ctx context.Context) ([]PendingApprovalRow, error)
}

type TypeCatalog interface {
	GetTemplateCategory(ctx context.Context, companyID, typeID string) (string, error)
	GetTypeDisplayName(ctx context.Context, companyID, typeID string) (string, error)
}

// WorkflowSeeder clones disclosure-type effective workflow into proposal step inputs.
// Template itself is never mutated. Assignee is always left empty on seed.
type WorkflowSeeder interface {
	SeedFromDisclosureType(ctx context.Context, companyID, typeID string) ([]ProposalWorkflowStepInput, error)
}

// CreateRecordOpts carries optional inputs for record + workflow creation (WF5-B, periodic T0 in Batch 2).
type CreateRecordOpts struct {
	StepOverrides []WorkflowStepOverride // ad-hoc only (legacy schema); must not dual-send with ProposalWorkflow
	CycleStart    *time.Time             // periodic materialize (Batch 2)
	PlannedDate   string                 // periodic: cycle due_date → disclosure_records.planned_date (YYYY-MM-DD); empty = not set
	// RecordID, when non-empty, is the deterministic record ID (ADR-1B) the
	// caller pre-allocated; passed through to disclosureapp.CreateRecordRequest
	// so retries are idempotent instead of creating orphaned/duplicate records.
	RecordID string
	// ProposalWorkflow, when schema_version=2, is the frozen proposal-owned snapshot authority.
	// RecordCreator must materialize from this blob and MUST NOT call GetEffectiveWorkflow.
	ProposalWorkflow *ProposalWorkflowSnapshot
	// SkipCompanySubmit: periodic materialize creates record + workflow without company submit.
	// submitted_at stays NULL until explicit SubmitRecord (MATERIALIZATION_IS_NOT_SUBMISSION).
	SkipCompanySubmit bool
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
	// NotifyFocalsForReview broadcasts to every member with ad_hoc_alert.focal_review.
	// Kept for backward compatibility (plan §6.8/B2); used by the legacy submit path.
	NotifyFocalsForReview(ctx context.Context, proposal ProposalDTO, focals []MemberInfo)
	// NotifyReviewersForReview sends targeted notifications to only the assigned reviewers
	// of a v3 multi-reviewer proposal (plan Phase 5). Supersedes NotifyFocalsForReview for
	// proposals that have an explicit reviewers[] list.
	NotifyReviewersForReview(ctx context.Context, proposal ProposalDTO, reviewers []MemberInfo)
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
	// ResolveCompanyName returns the display name of a company. Returns empty
	// string (not an error) when the company is not found — callers use the
	// fallback label "Công ty của bạn" rather than surfacing UUID.
	ResolveCompanyName(ctx context.Context, companyID string) (string, error)
}

// Metrics records ad-hoc proposal lifecycle observability signals. Implementations
// must be safe for concurrent use.
//
// RecordTransition (Batch 5(a) / AK.3 — cobo_adhoc_proposal_transition_total)
// must be called only for transitions that were actually applied by this request
// (ADR-2 EV-1) — never for idempotent replays (EV-2) — to avoid corrupting
// rate()-based alerting in Batch 6.
//
// RecordEmailShadowOutcome (Batch 2 / §AK.5 L657 — cobo_adhoc_email_shadow_total)
// reproduces the spec's "duplicate idempotency_key" detection: emit "match" when
// the durable dispatch's persisted record carries the content this caller sent
// (fresh insert or a genuine identical-content idempotent replay), or "mismatch"
// when the idempotency key resolved to a record with different content (a real
// collision — the literal condition `COUNT(*) GROUP BY idempotency_key HAVING
// COUNT(*) > 1` exists to catch, observed at the dispatch site since the unique
// constraint on email_notifications.idempotency_key makes it unobservable via a
// post-hoc row count). Called only from the Shadow Mode comparison branch.
type Metrics interface {
	RecordTransition(companyID, fromStatus, toStatus string)
	RecordEmailShadowOutcome(companyID, outcome string)
}

// noopMetrics is the zero-cost default used when ADHOC_EMAIL_METRICS_ENABLED=false.
type noopMetrics struct{}

func (noopMetrics) RecordTransition(string, string, string) {}
func (noopMetrics) RecordEmailShadowOutcome(string, string) {}

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

// WorkflowStepOverride carries the legacy per-step overrides in a proposal (schema v1).
// Legacy path: only processing_days override is allowed; steps cannot be added/removed.
type WorkflowStepOverride struct {
	StepID         string `json:"step_id"`
	ProcessingDays int    `json:"processing_days,omitempty"`
}

type CreateProposalRequest struct {
	Subject       Subject
	TypeID        string                      `json:"type_id"`
	StepOverrides []WorkflowStepOverride      `json:"step_overrides"`
	WorkflowSteps []ProposalWorkflowStepInput `json:"workflow_steps,omitempty"`
	// UseTemplateWorkflow seeds schema v2 from effective workflow when workflow_steps is omitted.
	UseTemplateWorkflow  bool   `json:"use_template_workflow,omitempty"`
	ProposedT0Date       string `json:"proposed_t0_date,omitempty"` // YYYY-MM-DD
	ProposedDeadlineDays int    `json:"proposed_deadline_days,omitempty"`
	ProposedDeadline     string `json:"proposed_deadline_date,omitempty"` // YYYY-MM-DD or legacy day count string
	// ProposedDeadlineDayType is proposal-owned: WORKING_DAYS | CALENDAR_DAYS.
	// Empty/omitted on create is allowed (draft null); submit normalizes null → CALENDAR_DAYS.
	ProposedDeadlineDayType string   `json:"proposed_deadline_day_type,omitempty"`
	ChangeNote              string   `json:"change_note,omitempty"`
	ReviewerMembershipIDs   []string `json:"reviewer_membership_ids,omitempty"`
	// ProcessControllerMembershipID is the deprecated single-reviewer field (A6
	// backward-compat alias). Used only when ReviewerMembershipIDs is empty.
	ProcessControllerMembershipID string `json:"process_controller_membership_id,omitempty"`
}

// PatchDraftProposalRequest updates an editable draft. WorkflowSteps, when non-nil,
// replaces the entire workflow snapshot atomically (even if empty slice — empty is rejected by normalize).
type PatchDraftProposalRequest struct {
	Subject              Subject
	ProposalID           string
	TypeID               *string `json:"type_id,omitempty"`
	ChangeNote           *string `json:"change_note,omitempty"`
	ProposedT0Date       *string `json:"proposed_t0_date,omitempty"`
	ProposedDeadlineDays *int    `json:"proposed_deadline_days,omitempty"`
	ProposedDeadline     *string `json:"proposed_deadline_date,omitempty"`
	// ProposedDeadlineDayType: omit=keep; ""=clear to NULL; WORKING_DAYS|CALENDAR_DAYS=set.
	// JSON null cannot be distinguished from omit with *string — clear via empty string.
	ProposedDeadlineDayType *string                      `json:"proposed_deadline_day_type,omitempty"`
	WorkflowSteps           *[]ProposalWorkflowStepInput `json:"workflow_steps,omitempty"`
	// UseTemplateWorkflow reseeds workflow from the (possibly new) type when workflow_steps is omitted.
	UseTemplateWorkflow bool `json:"use_template_workflow,omitempty"`
}

// DraftUpdate is the repository write for draft PATCH / submit freeze of workflow JSON.
type DraftUpdate struct {
	ProposalID              string
	CompanyID               string
	FromStatus              string // must be StatusDraft
	TypeID                  string
	ChangeNote              string
	ProposedT0Date          *string
	ProposedDeadlineDays    *int
	ProposedDeadlineDate    *string
	ProposedDeadlineDayType *ProposalDeadlineDayType
	// Workflow, when non-nil, is persisted as schema v2 authority in proposed_workflow_json.
	Workflow *ProposalWorkflowSnapshot
	// ClearWorkflowToLegacyOverrides, when Workflow is nil, writes legacy step_overrides JSON.
	LegacyStepOverrides []WorkflowStepOverride
	UseLegacyOverrides  bool
}

// ApproveRequest is the wire body for POST .../approve (replaces focal-approve).
type ApproveRequest struct {
	Subject           Subject
	ProposalID        string
	FinalT0Date       string `json:"final_t0_date,omitempty"`       // YYYY-MM-DD
	FinalDeadlineDate string `json:"final_deadline_date,omitempty"` // YYYY-MM-DD
	AdjustmentNote    string `json:"adjustment_note,omitempty"`
	Comment           string `json:"comment,omitempty"`
}

type ApprovalProgressDTO struct {
	Required  int `json:"required"`
	Completed int `json:"completed"`
}

type ApproveResponse struct {
	Proposal           ProposalDTO         `json:"proposal"`
	ApprovalProgress   ApprovalProgressDTO `json:"approval_progress"`
	Finalized          bool                `json:"finalized"`
	RecordID           string              `json:"record_id,omitempty"`
	WorkflowInstanceID string              `json:"workflow_instance_id,omitempty"`
}

// ReviewerDTO is one assigned reviewer, embedded in ProposalDTO.Reviewers.
type ReviewerDTO struct {
	MembershipID string `json:"membership_id"`
	FullName     string `json:"full_name,omitempty"`
	Email        string `json:"email,omitempty"`
}

// ApprovalDTO is one cast vote, embedded in ProposalDTO.Approvals.
type ApprovalDTO struct {
	MembershipID      string    `json:"membership_id"`
	ApprovedAt        time.Time `json:"approved_at"`
	FinalT0Date       *string   `json:"final_t0_date,omitempty"`
	FinalDeadlineDate *string   `json:"final_deadline_date,omitempty"`
	AdjustmentNote    string    `json:"adjustment_note,omitempty"`
	Comment           string    `json:"comment,omitempty"`
}

// ReserveVoteInput is the input to Repository.ReserveVote (Phase A of §6.5).
type ReserveVoteInput struct {
	CompanyID         string
	ProposalID        string
	ActorMembershipID string
	ActorUserID       string
	FinalT0Date       *string
	FinalDeadlineDate *string
	AdjustmentNote    string
	Comment           string
}

// VoteReservation is the result of Repository.ReserveVote. IsLastVote is true
// when, after this call, completed == required and the proposal is still
// pending_focal_approval — callers must attempt finalizeApprovedProposal,
// including on safe retry after a prior finalize failure (§6.5 test #3).
type VoteReservation struct {
	Proposal              *ProposalDTO
	IsLastVote            bool
	Required              int
	Completed             int
	ActorMembershipID     string
	ActorUserID           string
	LastFinalT0Date       *string
	LastFinalDeadlineDate *string
	LastAdjustmentNote    string
}

// FinalizeResult is the outcome of finalizeApprovedProposal (Phase B+C of §6.5).
type FinalizeResult struct {
	RecordID           string
	WorkflowInstanceID string
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

// ListScopeMy filters list results to the authenticated actor's own proposals.
const ListScopeMy = "my"

type ListProposalsRequest struct {
	Subject      Subject
	StatusFilter []string
	// Scope: empty = company-wide (requires ad_hoc_alert.read);
	// "my" = creator self-list (requires propose OR read; filter from auth membership).
	// Unknown values are rejected by the service.
	Scope    string
	Page     int
	PageSize int
}

type ListProposalsResponse struct {
	Items    []ProposalDTO `json:"items"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
	Total    int           `json:"total"`
}

type ProposalDTO struct {
	ProposalID string `json:"proposal_id"`
	CompanyID  string `json:"company_id"`
	// Display-only fields populated before notification dispatch.
	// Never persisted to ad_hoc_proposals; omitted from JSON when empty.
	CompanyName string `json:"company_name,omitempty"`
	CreatorName string `json:"creator_name,omitempty"`
	// ProposalTitle is the first line of ChangeNote; ProposalContent is the
	// remainder (truncated at 300 chars). Used by email templates to render
	// title and content as separate labeled fields.
	ProposalTitle   string                 `json:"proposal_title,omitempty"`
	ProposalContent string                 `json:"proposal_content,omitempty"`
	TypeID          string                 `json:"type_id"`
	Status          string                 `json:"status"`
	StepOverrides   []WorkflowStepOverride `json:"step_overrides"`
	// Workflow is schema v2 proposal-owned snapshot. Nil for legacy proposals.
	Workflow             *ProposalWorkflowSnapshot `json:"workflow,omitempty"`
	ProposedT0Date       *string                   `json:"proposed_t0_date,omitempty"`
	FinalT0Date          *string                   `json:"final_t0_date,omitempty"`
	FinalDeadlineDate    *string                   `json:"final_deadline_date,omitempty"`
	AdjustmentNote       string                    `json:"adjustment_note,omitempty"`
	ProposedDeadlineDays *int                      `json:"proposed_deadline_days,omitempty"`
	ProposedDeadlineDate *string                   `json:"proposed_deadline_date,omitempty"` // calendar date when set explicitly
	// ProposedDeadlineDayType: WORKING_DAYS | CALENDAR_DAYS. Null in response means
	// legacy/draft unset — EffectiveProposalDeadlineDayType treats null as CALENDAR_DAYS.
	ProposedDeadlineDayType *ProposalDeadlineDayType `json:"proposed_deadline_day_type,omitempty"`
	ChangeNote              string                   `json:"change_note,omitempty"`
	FocalApprovedBy         string                   `json:"focal_approved_by,omitempty"`
	FocalApprovedAt         *time.Time               `json:"focal_approved_at,omitempty"`
	AdminApprovedBy         string                   `json:"admin_approved_by,omitempty"`
	AdminApprovedAt         *time.Time               `json:"admin_approved_at,omitempty"`
	RejectedBy              string                   `json:"rejected_by,omitempty"`
	RejectedAt              *time.Time               `json:"rejected_at,omitempty"`
	RejectReason            string                   `json:"reject_reason,omitempty"`
	RecordID                string                   `json:"record_id,omitempty"`
	WorkflowInstanceID      string                   `json:"workflow_instance_id,omitempty"`
	CreatedBy               string                   `json:"created_by"`
	ProcessControllerID     string                   `json:"process_controller_id,omitempty"`
	CreatedAt               time.Time                `json:"created_at"`
	UpdatedAt               time.Time                `json:"updated_at"`

	// Reviewers, Approvals, ApprovalProgress are embedded by the service layer
	// (GetProposal/ListProposals) — never populated directly by the repository's
	// FindByID/List scan. ApprovalProgress is a pointer so it is omitted entirely
	// when not enriched, instead of rendering as required:0/completed:0.
	Reviewers        []ReviewerDTO        `json:"reviewers,omitempty"`
	Approvals        []ApprovalDTO        `json:"approvals,omitempty"`
	ApprovalProgress *ApprovalProgressDTO `json:"approval_progress,omitempty"`

	// Tracking is additive GetProposal enrichment (T3). Built from frozen workflow +
	// runtime instance/tasks. Omitted on list. Never changes proposal.status.
	Tracking *ProposalTrackingDTO `json:"tracking,omitempty"`

	// ReviewerMembershipIDs is a transient, create-time-only field: the reviewer
	// IDs to persist into ad_hoc_proposal_reviewers when this DTO is passed to
	// Repository.Insert. Never read back from the DB; not part of the wire DTO.
	ReviewerMembershipIDs []string `json:"-"`
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
	// Workflow, when non-nil, rewrites proposed_workflow_json in the same status UPDATE (submit freeze).
	Workflow *ProposalWorkflowSnapshot
	// PersistProposedDeadlineDayType, when true, writes ProposedDeadlineDayType in the same
	// status UPDATE (submit null→CALENDAR_DAYS normalization). Submitted value is then immutable
	// via draft PATCH (status != draft).
	PersistProposedDeadlineDayType bool
	ProposedDeadlineDayType        *ProposalDeadlineDayType
}
