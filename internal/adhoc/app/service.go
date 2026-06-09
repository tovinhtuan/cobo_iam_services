package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

// adhocRecordIDNamespace is the fixed UUID namespace for ADR-1B deterministic
// record-ID derivation. Pre-allocating a stable RecordID per (company, proposal)
// makes AdminApprove's record-creation step idempotent under retry: a repeat
// attempt computes the same ID and is detected as a duplicate-key replay
// instead of creating an orphaned second disclosure_records row (CF-01).
var adhocRecordIDNamespace = uuid.MustParse("6f1e9b2a-6c1d-4f3e-9a8b-2d4c5e6f7081")

type service struct {
	repo                Repository
	recordCreator       RecordCreator
	typeCatalog         TypeCatalog
	idg                 idgen.Generator
	autoApprove         bool // WORKFLOW_ADHOC_AUTOAPPROVE_ENABLED: skip focal step
	auth                authapp.Service
	membershipValidator MembershipValidator
	notifier            ProposalNotifier // nil = notifications disabled
	metrics             Metrics          // metrics emission; supplied as a no-op when ADHOC_EMAIL_METRICS_ENABLED=false
}

func NewService(repo Repository, recordCreator RecordCreator, typeCatalog TypeCatalog, idg idgen.Generator, autoApprove bool, auth authapp.Service, mv MembershipValidator, notifier ProposalNotifier, metrics Metrics) Service {
	return &service{repo: repo, recordCreator: recordCreator, typeCatalog: typeCatalog, idg: idg, autoApprove: autoApprove, auth: auth, membershipValidator: mv, notifier: notifier, metrics: metrics}
}

// safeNotify calls fn with the live request-scoped ctx captured by the closure
// and recovers any panic the notifier raises. Notification failures never
// roll back a committed proposal transition (fire-and-forget semantic) — R-2.
func safeNotify(proposalID string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("adhoc: notification panic recovered",
				slog.String("proposal_id", proposalID),
				slog.Any("panic", r))
		}
	}()
	fn()
}

func (s *service) CreateProposal(ctx context.Context, req CreateProposalRequest) (*ProposalDTO, error) {
	if err := s.authorize(ctx, req.Subject, "ad_hoc_alert.propose", authapp.ResourceRef{Type: "ad_hoc_proposal"}); err != nil {
		return nil, err
	}
	req.TypeID = strings.TrimSpace(req.TypeID)
	if req.TypeID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id is required", nil)
	}
	if s.typeCatalog == nil {
		return nil, perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "template catalog is unavailable", nil)
	}
	templateCategory, err := s.typeCatalog.GetTemplateCategory(ctx, req.Subject.CompanyID, req.TypeID)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	if strings.TrimSpace(strings.ToLower(templateCategory)) != "irregular" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "ad-hoc proposals are only supported for irregular templates", nil)
	}
	// Phase 1: validate processing_days per step (must be > 0 when provided).
	for _, o := range req.StepOverrides {
		if strings.TrimSpace(o.StepID) == "" {
			return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "step_id is required in step_overrides", nil)
		}
		if o.ProcessingDays < 0 {
			return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "processing_days must be >= 0", nil)
		}
	}

	// Validate process controller (ADR-1: required at create time).
	pcID := strings.TrimSpace(req.ProcessControllerMembershipID)
	if pcID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "process_controller_membership_id is required", nil)
	}
	if pcID == req.Subject.MembershipID {
		allowSelf, err := s.creatorMaySelfAssignProcessController(ctx, req.Subject.CompanyID, req.Subject.MembershipID)
		if err != nil {
			return nil, mapRepositoryError(fmt.Errorf("check creator process controller eligibility: %w", err))
		}
		if !allowSelf {
			return nil, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, "process controller cannot be the creator", nil)
		}
	}
	if s.membershipValidator != nil {
		active, err := s.membershipValidator.IsActiveMembership(ctx, req.Subject.CompanyID, pcID)
		if err != nil || !active {
			return nil, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, "process controller membership not found or inactive", nil)
		}
		hasPerm, err := s.membershipValidator.HasPermission(ctx, req.Subject.CompanyID, pcID, "ad_hoc_alert.process_control")
		if err != nil || !hasPerm {
			return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "designated member does not have process control authority (ad_hoc_alert.process_control)", nil)
		}
	}

	t0 := strings.TrimSpace(req.ProposedT0Date)
	deadlineDays, deadlineDate, err := resolveProposedDeadline(t0, req.ProposedDeadline, req.ProposedDeadlineDays)
	if err != nil {
		return nil, err
	}
	p := ProposalDTO{
		ProposalID:          s.idg.NewUUID(),
		CompanyID:           req.Subject.CompanyID,
		TypeID:              req.TypeID,
		Status:              StatusDraft,
		StepOverrides:       req.StepOverrides,
		ChangeNote:          strings.TrimSpace(req.ChangeNote),
		CreatedBy:           req.Subject.MembershipID,
		ProcessControllerID: pcID,
	}
	if t0 != "" {
		p.ProposedT0Date = &t0
	}
	p.ProposedDeadlineDays = deadlineDays
	p.ProposedDeadlineDate = deadlineDate
	out, err := s.repo.Insert(ctx, p)
	return out, mapRepositoryError(err)
}

func (s *service) SubmitProposal(ctx context.Context, req ProposalActionRequest) (*ProposalDTO, error) {
	if err := s.authorize(ctx, req.Subject, "ad_hoc_alert.propose", authapp.ResourceRef{Type: "ad_hoc_proposal", ID: req.ProposalID}); err != nil {
		return nil, err
	}
	cur, err := s.repo.FindByID(ctx, req.Subject.CompanyID, req.ProposalID)
	if err != nil {
		return nil, err
	}
	if cur.Status != StatusDraft {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "proposal is not in draft state", nil)
	}
	if cur.CreatedBy != req.Subject.MembershipID {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "only the creator can submit the proposal", nil)
	}
	nextStatus := StatusPendingFocalApproval
	if s.autoApprove {
		nextStatus = StatusPendingAdminApproval
	}
	updated, applied, err := s.repo.UpdateStatus(ctx, StatusUpdate{
		ProposalID:               req.ProposalID,
		CompanyID:                req.Subject.CompanyID,
		Status:                   nextStatus,
		FromStatus:               cur.Status, // ADR-2: cur.Status == StatusDraft (checked above)
		ActorMembershipID:        req.Subject.MembershipID,
		ActorUserID:              req.Subject.UserID,
		SetFocalApprovalMetadata: false,
	})
	if err != nil {
		return nil, err
	}
	if applied {
		s.metrics.RecordTransition(updated.CompanyID, cur.Status, nextStatus)
	}
	snap := s.enrichProposalForNotification(ctx, *updated)
	if s.notifier != nil && s.membershipValidator != nil {
		safeNotify(snap.ProposalID, func() {
			if s.autoApprove {
				if ctrl, err := s.membershipValidator.ResolveMembership(ctx, snap.CompanyID, snap.ProcessControllerID); err == nil && ctrl != nil {
					s.notifier.NotifyControllerForReview(ctx, snap, *ctrl)
				}
			} else {
				if focals, err := s.membershipValidator.ListMembersWithPermissionFull(ctx, snap.CompanyID, "ad_hoc_alert.focal_review"); err == nil {
					s.notifier.NotifyFocalsForReview(ctx, snap, focals)
				}
			}
		})
	}
	return updated, nil
}

func (s *service) FocalApprove(ctx context.Context, req ProposalActionRequest) (*ProposalDTO, error) {
	if err := s.authorize(ctx, req.Subject, "ad_hoc_alert.focal_review", authapp.ResourceRef{Type: "ad_hoc_proposal", ID: req.ProposalID}); err != nil {
		return nil, err
	}
	if s.autoApprove {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "focal approval step is disabled (WORKFLOW_ADHOC_AUTOAPPROVE_ENABLED=true)", nil)
	}
	cur, err := s.repo.FindByID(ctx, req.Subject.CompanyID, req.ProposalID)
	if err != nil {
		return nil, err
	}
	if cur.Status != StatusPendingFocalApproval {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "proposal is not pending focal approval", nil)
	}
	focalUpdated, applied, err := s.repo.UpdateStatus(ctx, StatusUpdate{
		ProposalID:               req.ProposalID,
		CompanyID:                req.Subject.CompanyID,
		Status:                   StatusPendingAdminApproval,
		FromStatus:               cur.Status, // ADR-2: cur.Status == StatusPendingFocalApproval (checked above)
		ActorMembershipID:        req.Subject.MembershipID,
		ActorUserID:              req.Subject.UserID,
		SetFocalApprovalMetadata: true,
	})
	if err != nil {
		return nil, err
	}
	if applied {
		s.metrics.RecordTransition(focalUpdated.CompanyID, cur.Status, StatusPendingAdminApproval)
	}
	snap := s.enrichProposalForNotification(ctx, *focalUpdated)
	if s.notifier != nil && s.membershipValidator != nil {
		safeNotify(snap.ProposalID, func() {
			if ctrl, err := s.membershipValidator.ResolveMembership(ctx, snap.CompanyID, snap.ProcessControllerID); err == nil && ctrl != nil {
				s.notifier.NotifyControllerForReview(ctx, snap, *ctrl)
			}
		})
	}
	return focalUpdated, nil
}

func (s *service) AdminApprove(ctx context.Context, req AdminApproveRequest) (*AdminApproveResponse, error) {
	// ADR-2: identity check — only the designated process controller may approve.
	// Permission-based check (ad_hoc_alert.admin_review) is deprecated and no longer the gate.
	cur0, err := s.repo.FindByID(ctx, req.Subject.CompanyID, req.ProposalID)
	if err != nil {
		return nil, err
	}
	if cur0.ProcessControllerID != req.Subject.MembershipID {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "only the designated process controller can approve this proposal", nil)
	}
	reservation, err := s.repo.ReserveAdminApproval(ctx, ReserveAdminApprovalInput{
		CompanyID:         req.Subject.CompanyID,
		ProposalID:        req.ProposalID,
		IdempotencyKey:    strings.TrimSpace(req.IdempotencyKey),
		ActorMembershipID: req.Subject.MembershipID,
	})
	if err != nil {
		return nil, err
	}
	cur := reservation.Proposal
	if cur == nil {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "proposal not found", nil)
	}
	if reservation.ReplayApproved {
		return &AdminApproveResponse{
			Proposal:           *cur,
			RecordID:           cur.RecordID,
			WorkflowInstanceID: cur.WorkflowInstanceID,
		}, nil
	}
	if cur.Status != StatusPendingAdminApproval {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "proposal is not pending admin approval", nil)
	}

	finalT0Time, finalT0Date, err := parseOptionalISODate(req.FinalT0Date, "final_t0_date")
	if err != nil {
		return nil, err
	}
	_, finalDeadlineDate, err := parseOptionalISODate(req.FinalDeadlineDate, "final_deadline_date")
	if err != nil {
		return nil, err
	}
	adjustmentNote := strings.TrimSpace(req.AdjustmentNote)

	// Auto-create and submit the disclosure record synchronously.
	typeDisplayName, _ := s.typeCatalog.GetTypeDisplayName(ctx, cur.CompanyID, cur.TypeID)
	title := ResolveAdHocRecordTitle(cur.ChangeNote, typeDisplayName, cur.TypeID)
	recordID := reservation.ProgressRecordID
	workflowInstanceID := reservation.ProgressWorkflowID
	if strings.TrimSpace(recordID) == "" {
		// ADR-1B: derive a deterministic RecordID from (company, proposal) so a
		// retried creation attempt is recognized as a replay (idempotent) instead
		// of inserting a second, orphaned disclosure_records row (CF-01).
		deterministicRecordID := uuid.NewSHA1(
			adhocRecordIDNamespace,
			[]byte(fmt.Sprintf("%s:%s", cur.CompanyID, cur.ProposalID)),
		).String()
		// CF-15: the disclosure record's creator must be the proposal's original
		// creator (cur.CreatedBy), not the approving admin (req.Subject.MembershipID).
		// req.Subject remains the acting/authorizing actor for this request.
		recordID, workflowInstanceID, err = s.recordCreator.CreateAndSubmitRecordWithOpts(ctx, cur.CompanyID, cur.TypeID, cur.CreatedBy, title, finalT0Time, CreateRecordOpts{
			RecordID:      deterministicRecordID,
			StepOverrides: cur.StepOverrides,
		})
		if err != nil {
			if httpErr, ok := perr.AsHTTPError(err); ok {
				return nil, httpErr
			}
			return nil, perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "failed to create disclosure record", err)
		}
		if err := s.repo.SaveAdminApprovalProgress(ctx, req.Subject.CompanyID, req.ProposalID, strings.TrimSpace(req.IdempotencyKey), recordID, workflowInstanceID, ""); err != nil {
			return nil, err
		}
	}

	updated, applied, err := s.repo.CompleteAdminApproval(ctx, StatusUpdate{
		ProposalID: req.ProposalID,
		CompanyID:  req.Subject.CompanyID,
		Status:     StatusApproved,
		// ADR-2: pending_admin_approval -> approved. CompleteAdminApproval already
		// guards with `AND status = ? AND (approval_idempotency_key = ? OR ...)`
		// (a stronger, idempotency-key-aware guard than the generic FromStatus
		// predicate) — FromStatus is set here for contract-completeness/documentation
		// and is intentionally not consumed by CompleteAdminApproval's SQL.
		FromStatus:         StatusPendingAdminApproval,
		ActorMembershipID:  req.Subject.MembershipID,
		ActorUserID:        req.Subject.UserID,
		RecordID:           recordID,
		WorkflowInstanceID: workflowInstanceID,
		FinalT0Date:        finalT0Date,
		FinalDeadlineDate:  finalDeadlineDate,
		AdjustmentNote:     adjustmentNote,
	}, strings.TrimSpace(req.IdempotencyKey))
	if err != nil {
		return nil, err
	}
	if applied {
		s.metrics.RecordTransition(updated.CompanyID, StatusPendingAdminApproval, StatusApproved)
	}
	snap := s.enrichProposalForNotification(ctx, *updated)
	if s.notifier != nil && s.membershipValidator != nil {
		safeNotify(snap.ProposalID, func() {
			if creator, err := s.membershipValidator.ResolveMembership(ctx, snap.CompanyID, snap.CreatedBy); err == nil && creator != nil {
				s.notifier.NotifyCreatorApproved(ctx, snap, *creator)
			}
		})
	}
	return &AdminApproveResponse{
		Proposal:           *updated,
		RecordID:           recordID,
		WorkflowInstanceID: workflowInstanceID,
	}, nil
}

func (s *service) Reject(ctx context.Context, req RejectRequest) (*ProposalDTO, error) {
	cur, err := s.repo.FindByID(ctx, req.Subject.CompanyID, req.ProposalID)
	if err != nil {
		return nil, err
	}
	switch cur.Status {
	case StatusPendingFocalApproval, StatusPendingAdminApproval:
	default:
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "proposal cannot be rejected in current state", nil)
	}
	if cur.Status == StatusPendingFocalApproval {
		if err := s.authorize(ctx, req.Subject, "ad_hoc_alert.focal_review", authapp.ResourceRef{Type: "ad_hoc_proposal", ID: req.ProposalID, Attributes: map[string]any{"workflow_state": cur.Status}}); err != nil {
			return nil, err
		}
	} else {
		// pending_admin_approval: only the designated process controller may reject.
		if cur.ProcessControllerID != req.Subject.MembershipID {
			return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "only the designated process controller can reject this proposal at admin stage", nil)
		}
	}
	reason := strings.TrimSpace(req.RejectReason)
	if reason == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "reject_reason is required", nil)
	}
	rejected, applied, err := s.repo.UpdateStatus(ctx, StatusUpdate{
		ProposalID: req.ProposalID,
		CompanyID:  req.Subject.CompanyID,
		Status:     StatusRejected,
		// ADR-2: cur.Status is one of {StatusPendingFocalApproval, StatusPendingAdminApproval} (checked above).
		FromStatus:        cur.Status,
		ActorMembershipID: req.Subject.MembershipID,
		ActorUserID:       req.Subject.UserID,
		RejectReason:      reason,
	})
	if err != nil {
		return nil, err
	}
	if applied {
		s.metrics.RecordTransition(rejected.CompanyID, cur.Status, StatusRejected)
	}
	snap := s.enrichProposalForNotification(ctx, *rejected)
	if s.notifier != nil && s.membershipValidator != nil {
		safeNotify(snap.ProposalID, func() {
			if creator, err := s.membershipValidator.ResolveMembership(ctx, snap.CompanyID, snap.CreatedBy); err == nil && creator != nil {
				s.notifier.NotifyCreatorRejected(ctx, snap, *creator)
			}
		})
	}
	return rejected, nil
}

func (s *service) Cancel(ctx context.Context, req ProposalActionRequest) (*ProposalDTO, error) {
	if err := s.authorize(ctx, req.Subject, "ad_hoc_alert.propose", authapp.ResourceRef{Type: "ad_hoc_proposal", ID: req.ProposalID}); err != nil {
		return nil, err
	}
	cur, err := s.repo.FindByID(ctx, req.Subject.CompanyID, req.ProposalID)
	if err != nil {
		return nil, err
	}
	switch cur.Status {
	case StatusDraft, StatusPendingFocalApproval, StatusPendingAdminApproval:
	default:
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "proposal cannot be cancelled in current state", nil)
	}
	if cur.CreatedBy != req.Subject.MembershipID {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "only the creator can cancel the proposal", nil)
	}
	updated, applied, err := s.repo.UpdateStatus(ctx, StatusUpdate{
		ProposalID: req.ProposalID,
		CompanyID:  req.Subject.CompanyID,
		Status:     StatusCancelled,
		// ADR-2: cur.Status is one of {StatusDraft, StatusPendingFocalApproval, StatusPendingAdminApproval} (checked above).
		FromStatus:        cur.Status,
		ActorMembershipID: req.Subject.MembershipID,
		ActorUserID:       req.Subject.UserID,
	})
	if err != nil {
		return nil, err
	}
	if applied {
		s.metrics.RecordTransition(updated.CompanyID, cur.Status, StatusCancelled)
	}
	return updated, nil
}

func (s *service) GetProposal(ctx context.Context, req GetProposalRequest) (*ProposalDTO, error) {
	if err := s.authorize(ctx, req.Subject, "ad_hoc_alert.read", authapp.ResourceRef{Type: "ad_hoc_proposal", ID: req.ProposalID}); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.ProposalID) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "proposal_id is required", nil)
	}
	return s.repo.FindByID(ctx, req.Subject.CompanyID, req.ProposalID)
}

func (s *service) ListProposals(ctx context.Context, req ListProposalsRequest) (*ListProposalsResponse, error) {
	if err := s.authorize(ctx, req.Subject, "ad_hoc_alert.read", authapp.ResourceRef{Type: "ad_hoc_proposal"}); err != nil {
		return nil, err
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 || req.PageSize > 100 {
		req.PageSize = 20
	}
	items, total, err := s.repo.List(ctx, req.Subject.CompanyID, req.StatusFilter, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	return &ListProposalsResponse{Items: items, Page: req.Page, PageSize: req.PageSize, Total: total}, nil
}

func (s *service) ListEligibleControllers(ctx context.Context, req ListEligibleControllersRequest) ([]EligibleController, error) {
	if err := s.authorize(ctx, req.Subject, "ad_hoc_alert.propose", authapp.ResourceRef{Type: "ad_hoc_proposal"}); err != nil {
		return nil, err
	}
	if s.membershipValidator == nil {
		return []EligibleController{}, nil
	}
	excludeMembershipID := req.Subject.MembershipID
	allowSelf, err := s.creatorMaySelfAssignProcessController(ctx, req.Subject.CompanyID, req.Subject.MembershipID)
	if err != nil {
		return nil, fmt.Errorf("check creator process controller eligibility: %w", err)
	}
	if allowSelf {
		excludeMembershipID = ""
	}
	return s.membershipValidator.ListMembersWithPermission(ctx, req.Subject.CompanyID, "ad_hoc_alert.process_control", excludeMembershipID)
}

func (s *service) creatorMaySelfAssignProcessController(ctx context.Context, companyID, membershipID string) (bool, error) {
	if s.membershipValidator == nil {
		return false, nil
	}
	return s.membershipValidator.HasActiveRoleCode(ctx, companyID, membershipID, RoleCodeAdminDoanhNghiep)
}

func (s *service) authorize(ctx context.Context, sub Subject, action string, resource authapp.ResourceRef) error {
	decision, err := s.auth.Authorize(ctx, authapp.AuthorizeRequest{
		Subject:  authapp.SubjectRef{UserID: sub.UserID, MembershipID: sub.MembershipID, CompanyID: sub.CompanyID},
		Action:   action,
		Resource: resource,
	})
	if err != nil {
		return mapRepositoryError(fmt.Errorf("authorize adhoc action: %w", err))
	}
	if decision.Decision != authapp.DecisionAllow {
		code := perr.CodePermissionDenied
		if decision.DenyReasonCode != nil {
			code = *decision.DenyReasonCode
		}
		return perr.NewHTTPError(http.StatusForbidden, code, "access denied", nil)
	}
	return nil
}

// enrichProposalForNotification populates CompanyName and CreatorName on a
// ProposalDTO copy so email templates receive human-readable display fields
// instead of UUIDs. Errors are swallowed with safe fallbacks — notification
// enrichment must never block the primary business flow.
func (s *service) enrichProposalForNotification(ctx context.Context, p ProposalDTO) ProposalDTO {
	p.ProposalTitle, p.ProposalContent = splitChangeNote(p.ChangeNote)
	if s.membershipValidator == nil {
		p.CompanyName = "Công ty của bạn"
		return p
	}
	if name, err := s.membershipValidator.ResolveCompanyName(ctx, p.CompanyID); err == nil && name != "" {
		p.CompanyName = name
	} else {
		p.CompanyName = "Công ty của bạn"
	}
	if info, err := s.membershipValidator.ResolveMembership(ctx, p.CompanyID, p.CreatedBy); err == nil && info != nil && info.FullName != "" {
		p.CreatorName = info.FullName
	}
	return p
}

func parseOptionalISODate(raw, field string) (*time.Time, *string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil, nil
	}
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, field+" must be YYYY-MM-DD", nil)
	}
	normalized := parsed.Format("2006-01-02")
	return &parsed, &normalized, nil
}
