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
	org                 OrgDirectory     // nil until AttachWorkflowDeps; required for v2 org refs
	seeder              WorkflowSeeder   // optional template seed
}

func NewService(repo Repository, recordCreator RecordCreator, typeCatalog TypeCatalog, idg idgen.Generator, autoApprove bool, auth authapp.Service, mv MembershipValidator, notifier ProposalNotifier, metrics Metrics) Service {
	return &service{repo: repo, recordCreator: recordCreator, typeCatalog: typeCatalog, idg: idg, autoApprove: autoApprove, auth: auth, membershipValidator: mv, notifier: notifier, metrics: metrics}
}

// AttachWorkflowDeps injects optional schema-v2 org validation and template seeding without
// changing the NewService signature used by existing tests.
func AttachWorkflowDeps(svc Service, org OrgDirectory, seeder WorkflowSeeder) Service {
	if s, ok := svc.(*service); ok {
		s.org = org
		s.seeder = seeder
	}
	return svc
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
		if he, ok := perr.AsHTTPError(err); ok && he.HTTPStatus == http.StatusForbidden {
			return nil, newAdHocPermissionError("ad_hoc_alert.propose", "Missing permission ad_hoc_alert.propose")
		}
		return nil, err
	}
	req.TypeID = strings.TrimSpace(req.TypeID)
	if req.TypeID == "" {
		return nil, newAdHocFieldError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id", "type_id is required")
	}
	if s.typeCatalog == nil {
		return nil, perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "template catalog is unavailable", nil)
	}
	templateCategory, err := s.typeCatalog.GetTemplateCategory(ctx, req.Subject.CompanyID, req.TypeID)
	if err != nil {
		if he, ok := perr.AsHTTPError(err); ok && strings.Contains(strings.ToLower(he.Message), "disclosure type not found") {
			return nil, newAdHocFieldError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id", "Disclosure type is not available for ad-hoc proposal")
		}
		return nil, mapRepositoryError(err)
	}
	if strings.TrimSpace(strings.ToLower(templateCategory)) != "irregular" {
		return nil, newAdHocFieldError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id", "Disclosure type is not available for ad-hoc proposal")
	}

	hasWorkflowSteps := req.WorkflowSteps != nil
	hasLegacyOverrides := len(req.StepOverrides) > 0
	if hasWorkflowSteps && hasLegacyOverrides {
		return nil, newAdHocFieldError(http.StatusBadRequest, perr.CodeInvalidRequest, "workflow_steps", "workflow_contract_conflict: workflow_steps and step_overrides cannot both be set")
	}
	if req.UseTemplateWorkflow && hasLegacyOverrides {
		return nil, newAdHocFieldError(http.StatusBadRequest, perr.CodeInvalidRequest, "use_template_workflow", "workflow_contract_conflict: use_template_workflow and step_overrides cannot both be set")
	}

	var workflowSnap *ProposalWorkflowSnapshot
	var stepOverrides []WorkflowStepOverride
	switch {
	case hasWorkflowSteps:
		snap, nErr := s.normalizeAndValidateWorkflow(ctx, req.Subject.CompanyID, req.TypeID, req.WorkflowSteps, nil, false)
		if nErr != nil {
			return nil, nErr
		}
		workflowSnap = snap
		stepOverrides = DeriveLegacyStepOverrides(snap.Steps)
	case req.UseTemplateWorkflow:
		inputs, sErr := s.seedWorkflowInputs(ctx, req.Subject.CompanyID, req.TypeID)
		if sErr != nil {
			return nil, sErr
		}
		snap, nErr := s.normalizeAndValidateWorkflow(ctx, req.Subject.CompanyID, req.TypeID, inputs, nil, false)
		if nErr != nil {
			return nil, nErr
		}
		workflowSnap = snap
		stepOverrides = DeriveLegacyStepOverrides(snap.Steps)
	default:
		// Legacy path: step_overrides only (or empty).
		for _, o := range req.StepOverrides {
			if strings.TrimSpace(o.StepID) == "" {
				return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "step_id is required in step_overrides", nil)
			}
			if o.ProcessingDays < 0 {
				return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "processing_days must be >= 0", nil)
			}
		}
		stepOverrides = req.StepOverrides
	}

	// Validate reviewers (D3/D4: 1..N, each must hold ad_hoc_alert.focal_review).
	reviewerIDs := normalizeReviewerIDs(req.ReviewerMembershipIDs)
	if len(reviewerIDs) == 0 {
		// A6 backward-compat alias: accept the deprecated single-reviewer field.
		if legacy := strings.TrimSpace(req.ProcessControllerMembershipID); legacy != "" {
			reviewerIDs = []string{legacy}
		}
	}
	if len(reviewerIDs) == 0 {
		return nil, newAdHocFieldError(http.StatusBadRequest, perr.CodeInvalidRequest, "reviewer_membership_ids", "reviewer_membership_ids is required")
	}
	seen := make(map[string]bool, len(reviewerIDs))
	for _, id := range reviewerIDs {
		if seen[id] {
			return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "reviewer_membership_ids must not contain duplicates", nil)
		}
		seen[id] = true
	}
	if seen[req.Subject.MembershipID] {
		allowSelf, err := s.creatorMaySelfAssignReviewer(ctx, req.Subject.CompanyID, req.Subject.MembershipID)
		if err != nil {
			return nil, mapRepositoryError(fmt.Errorf("check creator reviewer eligibility: %w", err))
		}
		if !allowSelf {
			return nil, newAdHocFieldError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, "reviewer_membership_ids", "creator cannot self-assign as reviewer without ad_hoc_alert.focal_review")
		}
	}
	// [B1] Always validate via membershipValidator — no `!= nil` guard. A nil
	// validator is a server misconfiguration and must fail fast, not silently skip.
	if s.membershipValidator == nil {
		return nil, perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "membership validator is unavailable", nil)
	}
	for _, id := range reviewerIDs {
		active, err := s.membershipValidator.IsActiveMembership(ctx, req.Subject.CompanyID, id)
		if err != nil || !active {
			return nil, newAdHocFieldError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, "reviewer_membership_ids", "reviewer_membership_ids contains invalid reviewer")
		}
		hasPerm, err := s.membershipValidator.HasPermission(ctx, req.Subject.CompanyID, id, "ad_hoc_alert.focal_review")
		if err != nil || !hasPerm {
			return nil, newAdHocFieldError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, "reviewer_membership_ids", "reviewer does not have ad_hoc_alert.focal_review")
		}
	}

	t0 := strings.TrimSpace(req.ProposedT0Date)
	deadlineDays, deadlineDate, err := resolveProposedDeadline(t0, req.ProposedDeadline, req.ProposedDeadlineDays)
	if err != nil {
		return nil, err
	}
	p := ProposalDTO{
		ProposalID:            s.idg.NewUUID(),
		CompanyID:             req.Subject.CompanyID,
		TypeID:                req.TypeID,
		Status:                StatusDraft,
		StepOverrides:         stepOverrides,
		Workflow:              workflowSnap,
		ChangeNote:            strings.TrimSpace(req.ChangeNote),
		CreatedBy:             req.Subject.MembershipID,
		ReviewerMembershipIDs: reviewerIDs,
	}
	if t0 != "" {
		p.ProposedT0Date = &t0
	}
	p.ProposedDeadlineDays = deadlineDays
	p.ProposedDeadlineDate = deadlineDate
	out, err := s.repo.Insert(ctx, p)
	return out, mapRepositoryError(err)
}

func (s *service) PatchDraftProposal(ctx context.Context, req PatchDraftProposalRequest) (*ProposalDTO, error) {
	if err := s.authorize(ctx, req.Subject, "ad_hoc_alert.propose", authapp.ResourceRef{Type: "ad_hoc_proposal", ID: req.ProposalID}); err != nil {
		if he, ok := perr.AsHTTPError(err); ok && he.HTTPStatus == http.StatusForbidden {
			return nil, newAdHocPermissionError("ad_hoc_alert.propose", "Missing permission ad_hoc_alert.propose")
		}
		return nil, err
	}
	cur, err := s.repo.FindByID(ctx, req.Subject.CompanyID, req.ProposalID)
	if err != nil {
		return nil, err
	}
	if cur.Status != StatusDraft {
		return nil, newAdHocFieldError(http.StatusConflict, perr.CodeStateConflict, "workflow", "workflow_frozen: proposal is not editable")
	}
	if cur.CreatedBy != req.Subject.MembershipID {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "only the creator can update the draft proposal", nil)
	}
	if cur.Workflow != nil && cur.Workflow.Frozen {
		return nil, newAdHocFieldError(http.StatusConflict, perr.CodeStateConflict, "workflow", "workflow_frozen: proposal workflow is immutable")
	}

	typeID := cur.TypeID
	if req.TypeID != nil {
		typeID = strings.TrimSpace(*req.TypeID)
		if typeID == "" {
			return nil, newAdHocFieldError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id", "type_id is required")
		}
		if s.typeCatalog == nil {
			return nil, perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "template catalog is unavailable", nil)
		}
		templateCategory, catErr := s.typeCatalog.GetTemplateCategory(ctx, req.Subject.CompanyID, typeID)
		if catErr != nil {
			if he, ok := perr.AsHTTPError(catErr); ok && strings.Contains(strings.ToLower(he.Message), "disclosure type not found") {
				return nil, newAdHocFieldError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id", "Disclosure type is not available for ad-hoc proposal")
			}
			return nil, mapRepositoryError(catErr)
		}
		if strings.TrimSpace(strings.ToLower(templateCategory)) != "irregular" {
			return nil, newAdHocFieldError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id", "Disclosure type is not available for ad-hoc proposal")
		}
	}

	changeNote := cur.ChangeNote
	if req.ChangeNote != nil {
		changeNote = strings.TrimSpace(*req.ChangeNote)
	}

	t0 := ""
	if cur.ProposedT0Date != nil {
		t0 = *cur.ProposedT0Date
	}
	if req.ProposedT0Date != nil {
		t0 = strings.TrimSpace(*req.ProposedT0Date)
	}
	legacyDeadline := ""
	daysIn := 0
	if cur.ProposedDeadlineDays != nil {
		daysIn = *cur.ProposedDeadlineDays
	}
	if req.ProposedDeadline != nil {
		legacyDeadline = strings.TrimSpace(*req.ProposedDeadline)
	}
	if req.ProposedDeadlineDays != nil {
		daysIn = *req.ProposedDeadlineDays
	}
	deadlineDays, deadlineDate, err := resolveProposedDeadline(t0, legacyDeadline, daysIn)
	if err != nil {
		return nil, err
	}
	if req.ProposedT0Date == nil && req.ProposedDeadline == nil && req.ProposedDeadlineDays == nil {
		deadlineDays = cur.ProposedDeadlineDays
		deadlineDate = cur.ProposedDeadlineDate
		if cur.ProposedT0Date != nil {
			t0 = *cur.ProposedT0Date
		} else {
			t0 = ""
		}
	}

	typeChanged := typeID != cur.TypeID
	needWorkflowReplace := req.WorkflowSteps != nil || req.UseTemplateWorkflow || typeChanged

	upd := DraftUpdate{
		ProposalID:           req.ProposalID,
		CompanyID:            req.Subject.CompanyID,
		FromStatus:           StatusDraft,
		TypeID:               typeID,
		ChangeNote:           changeNote,
		ProposedDeadlineDays: deadlineDays,
		ProposedDeadlineDate: deadlineDate,
	}
	if t0 != "" {
		upd.ProposedT0Date = &t0
	}

	if needWorkflowReplace {
		var inputs []ProposalWorkflowStepInput
		switch {
		case req.WorkflowSteps != nil:
			if typeChanged {
				// Reject old source refs that disagree with new type provenance when source_step_id set
				// without reseeding — require explicit new workflow_steps matching new type seed lineage
				// or UseTemplateWorkflow. Client-supplied steps are accepted as full replace (no merge).
			}
			inputs = *req.WorkflowSteps
		case req.UseTemplateWorkflow || typeChanged:
			var sErr error
			inputs, sErr = s.seedWorkflowInputs(ctx, req.Subject.CompanyID, typeID)
			if sErr != nil {
				return nil, sErr
			}
		}
		existingIDs := map[string]struct{}{}
		if cur.Workflow != nil && !typeChanged {
			for _, st := range cur.Workflow.Steps {
				existingIDs[st.ID] = struct{}{}
			}
		}
		snap, nErr := s.normalizeAndValidateWorkflow(ctx, req.Subject.CompanyID, typeID, inputs, existingIDs, false)
		if nErr != nil {
			return nil, nErr
		}
		upd.Workflow = snap
	} else if cur.Workflow != nil {
		upd.Workflow = cur.Workflow
	} else {
		upd.UseLegacyOverrides = true
		upd.LegacyStepOverrides = cur.StepOverrides
	}

	out, err := s.repo.UpdateDraft(ctx, upd)
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
	// D1: every new proposal has exactly one pending state (pending_focal_approval) —
	// pending_admin_approval is no longer a valid submit target. reviewers are
	// validated as non-empty at create time (immutable after, A5); this is a
	// defensive re-check for rows that predate the reviewers table.
	reviewers, err := s.repo.ListReviewers(ctx, req.Subject.CompanyID, req.ProposalID)
	if err != nil {
		return nil, err
	}
	if len(reviewers) == 0 {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "proposal has no assigned reviewers", nil)
	}

	statusUpd := StatusUpdate{
		ProposalID:               req.ProposalID,
		CompanyID:                req.Subject.CompanyID,
		Status:                   StatusPendingFocalApproval,
		FromStatus:               cur.Status, // ADR-2: cur.Status == StatusDraft (checked above)
		ActorMembershipID:        req.Subject.MembershipID,
		ActorUserID:              req.Subject.UserID,
		SetFocalApprovalMetadata: false,
	}
	// Submit freeze foundation: mark schema v2 snapshot immutable. Runtime still late-resolves
	// template until Phase 3 — do not switch materialization here.
	if cur.Workflow != nil && cur.Workflow.SchemaVersion == ProposalWorkflowSchemaV2 {
		frozen := *cur.Workflow
		frozen.Frozen = true
		statusUpd.Workflow = &frozen
	}

	updated, applied, err := s.repo.UpdateStatus(ctx, statusUpd)
	if err != nil {
		return nil, err
	}
	if applied {
		s.metrics.RecordTransition(updated.CompanyID, cur.Status, StatusPendingFocalApproval)
	}
	snap := s.enrichProposalForNotification(ctx, *updated)
	if s.notifier != nil && s.membershipValidator != nil {
		// Phase 5: for v3 proposals (reviewer list already loaded above), send targeted
		// notifications to only the assigned reviewers — not a broadcast to all focal_review
		// holders (plan §6.8, NotifyReviewersForReview). Legacy proposals without an
		// explicit reviewers table fall back to the broadcast (NotifyFocalsForReview).
		safeNotify(snap.ProposalID, func() {
			if len(reviewers) > 0 {
				// v3 path: resolve UserID for each assigned reviewer then notify.
				var members []MemberInfo
				for _, r := range reviewers {
					if m, err := s.membershipValidator.ResolveMembership(ctx, snap.CompanyID, r.MembershipID); err == nil && m != nil {
						members = append(members, *m)
					}
				}
				if len(members) > 0 {
					s.notifier.NotifyReviewersForReview(ctx, snap, members)
				}
			} else {
				// Legacy fallback: broadcast to all focal_review holders.
				if focals, err := s.membershipValidator.ListMembersWithPermissionFull(ctx, snap.CompanyID, "ad_hoc_alert.focal_review"); err == nil {
					s.notifier.NotifyFocalsForReview(ctx, snap, focals)
				}
			}
		})
	}
	return updated, nil
}

// Approve casts one reviewer's vote (v3 D3/D4 — one round, unanimous, 1..N
// reviewers). Replaces FocalApprove. See §6.5 for the 3-phase concurrency design.
func (s *service) Approve(ctx context.Context, req ApproveRequest) (*ApproveResponse, error) {
	if err := s.authorize(ctx, req.Subject, "ad_hoc_alert.focal_review", authapp.ResourceRef{Type: "ad_hoc_proposal", ID: req.ProposalID}); err != nil {
		return nil, err
	}
	_, finalT0Date, err := parseOptionalISODate(req.FinalT0Date, "final_t0_date")
	if err != nil {
		return nil, err
	}
	_, finalDeadlineDate, err := parseOptionalISODate(req.FinalDeadlineDate, "final_deadline_date")
	if err != nil {
		return nil, err
	}

	reservation, err := s.repo.ReserveVote(ctx, ReserveVoteInput{
		CompanyID:         req.Subject.CompanyID,
		ProposalID:        req.ProposalID,
		ActorMembershipID: req.Subject.MembershipID,
		ActorUserID:       req.Subject.UserID,
		FinalT0Date:       finalT0Date,
		FinalDeadlineDate: finalDeadlineDate,
		AdjustmentNote:    strings.TrimSpace(req.AdjustmentNote),
		Comment:           strings.TrimSpace(req.Comment),
	})
	if err != nil {
		return nil, err
	}

	progress := ApprovalProgressDTO{Required: reservation.Required, Completed: reservation.Completed}

	if reservation.Proposal.Status == StatusApproved {
		// Already finalized by an earlier vote/retry — idempotent replay (EV-2).
		return &ApproveResponse{
			Proposal:           *reservation.Proposal,
			ApprovalProgress:   progress,
			Finalized:          true,
			RecordID:           reservation.Proposal.RecordID,
			WorkflowInstanceID: reservation.Proposal.WorkflowInstanceID,
		}, nil
	}

	if !reservation.IsLastVote {
		return &ApproveResponse{
			Proposal:         *reservation.Proposal,
			ApprovalProgress: progress,
			Finalized:        false,
		}, nil
	}

	// Last vote (or a safe retry of one — §6.5 test #3): attempt finalize. On
	// error the proposal remains pending_focal_approval; retrying is safe because
	// finalize uses a deterministic record ID and a guarded completion UPDATE.
	result, err := s.finalizeApprovedProposal(ctx, reservation)
	if err != nil {
		return nil, err
	}
	updated, err := s.repo.FindByID(ctx, req.Subject.CompanyID, req.ProposalID)
	if err != nil {
		return nil, err
	}
	return &ApproveResponse{
		Proposal:           *updated,
		ApprovalProgress:   progress,
		Finalized:          true,
		RecordID:           result.RecordID,
		WorkflowInstanceID: result.WorkflowInstanceID,
	}, nil
}

// finalizeApprovedProposal is Phase B+C of §6.5: create the disclosure record
// (idempotent via deterministic RecordID, ADR-1B) then guarded-UPDATE the
// proposal to approved. Must only be called when reservation.IsLastVote is true.
func (s *service) finalizeApprovedProposal(ctx context.Context, r *VoteReservation) (*FinalizeResult, error) {
	cur := r.Proposal
	typeDisplayName, _ := s.typeCatalog.GetTypeDisplayName(ctx, cur.CompanyID, cur.TypeID)
	title := ResolveAdHocRecordTitle(cur.ChangeNote, typeDisplayName, cur.TypeID)

	finalT0 := r.LastFinalT0Date
	if finalT0 == nil {
		finalT0 = cur.ProposedT0Date
	}
	var t0Time *time.Time
	if finalT0 != nil {
		if parsed, err := time.Parse("2006-01-02", *finalT0); err == nil {
			t0Time = &parsed
		}
	}

	// ADR-1B: deterministic RecordID so a retried finalize (after a prior
	// CreateAndSubmitRecordWithOpts failure) is recognized as a replay instead of
	// creating an orphaned second disclosure_records row (CF-01).
	deterministicRecordID := uuid.NewSHA1(
		adhocRecordIDNamespace,
		[]byte(fmt.Sprintf("%s:%s", cur.CompanyID, cur.ProposalID)),
	).String()
	recordID, workflowInstanceID, err := s.recordCreator.CreateAndSubmitRecordWithOpts(ctx, cur.CompanyID, cur.TypeID, cur.CreatedBy, title, t0Time, CreateRecordOpts{
		RecordID:      deterministicRecordID,
		StepOverrides: cur.StepOverrides,
	})
	if err != nil {
		if httpErr, ok := perr.AsHTTPError(err); ok {
			return nil, httpErr
		}
		return nil, perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "failed to create disclosure record", err)
	}

	updated, applied, err := s.repo.CompleteFinalize(ctx, StatusUpdate{
		ProposalID:         cur.ProposalID,
		CompanyID:          cur.CompanyID,
		Status:             StatusApproved,
		FromStatus:         StatusPendingFocalApproval,
		ActorMembershipID:  r.ActorMembershipID,
		ActorUserID:        r.ActorUserID,
		RecordID:           recordID,
		WorkflowInstanceID: workflowInstanceID,
		FinalT0Date:        r.LastFinalT0Date,
		FinalDeadlineDate:  r.LastFinalDeadlineDate,
		AdjustmentNote:     r.LastAdjustmentNote,
	})
	if err != nil {
		return nil, err
	}
	if applied {
		s.metrics.RecordTransition(updated.CompanyID, StatusPendingFocalApproval, StatusApproved)
	}
	snap := s.enrichProposalForNotification(ctx, *updated)
	if s.notifier != nil && s.membershipValidator != nil {
		safeNotify(snap.ProposalID, func() {
			if creator, err := s.membershipValidator.ResolveMembership(ctx, snap.CompanyID, snap.CreatedBy); err == nil && creator != nil {
				s.notifier.NotifyCreatorApproved(ctx, snap, *creator)
			}
		})
	}
	return &FinalizeResult{RecordID: recordID, WorkflowInstanceID: workflowInstanceID}, nil
}

// AdminApprove is kept unchanged for the legacy two-round flow.
//
// @deprecated: serves only (1) legacy clients still calling POST .../admin-approve
// directly, and (2) FinalizeLegacyApproval's internal reuse for the one-time
// migration endpoint. Do not call from any new code path. Removed only once
// both conditions in the migration runbook (§12.5) are satisfied.
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
	// D1: pending_admin_approval is legacy-only and never reached by new proposals;
	// the one-round flow only rejects out of pending_focal_approval.
	if cur.Status != StatusPendingFocalApproval {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "proposal cannot be rejected in current state", nil)
	}
	if err := s.authorize(ctx, req.Subject, "ad_hoc_alert.focal_review", authapp.ResourceRef{Type: "ad_hoc_proposal", ID: req.ProposalID, Attributes: map[string]any{"workflow_state": cur.Status}}); err != nil {
		return nil, err
	}
	assigned, err := s.repo.IsAssignedReviewer(ctx, req.Subject.CompanyID, req.ProposalID, req.Subject.MembershipID)
	if err != nil {
		return nil, err
	}
	if !assigned {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "only an assigned reviewer can reject this proposal", nil)
	}
	reason := strings.TrimSpace(req.RejectReason)
	if reason == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "reject_reason is required", nil)
	}
	rejected, applied, err := s.repo.UpdateStatus(ctx, StatusUpdate{
		ProposalID: req.ProposalID,
		CompanyID:  req.Subject.CompanyID,
		Status:     StatusRejected,
		// ADR-2: cur.Status == StatusPendingFocalApproval (checked above).
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
	p, err := s.repo.FindByID(ctx, req.Subject.CompanyID, req.ProposalID)
	if err != nil {
		return nil, err
	}
	return s.embedReviewState(ctx, p), nil
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
	for i := range items {
		items[i] = *s.embedReviewState(ctx, &items[i])
	}
	return &ListProposalsResponse{Items: items, Page: req.Page, PageSize: req.PageSize, Total: total}, nil
}

func (s *service) ListEligibleReviewers(ctx context.Context, req ListEligibleReviewersRequest) ([]EligibleController, error) {
	if err := s.authorize(ctx, req.Subject, "ad_hoc_alert.propose", authapp.ResourceRef{Type: "ad_hoc_proposal"}); err != nil {
		return nil, err
	}
	if s.membershipValidator == nil {
		return []EligibleController{}, nil
	}
	excludeMembershipID := req.Subject.MembershipID
	allowSelf, err := s.creatorMaySelfAssignReviewer(ctx, req.Subject.CompanyID, req.Subject.MembershipID)
	if err != nil {
		return nil, fmt.Errorf("check creator reviewer eligibility: %w", err)
	}
	if allowSelf {
		excludeMembershipID = ""
	}
	return s.membershipValidator.ListMembersWithPermission(ctx, req.Subject.CompanyID, "ad_hoc_alert.focal_review", excludeMembershipID)
}

// creatorMaySelfAssignReviewer: the creator may self-assign as reviewer only when
// they hold ad_hoc_alert.focal_review. Sole self-reviewer is allowed.
func (s *service) creatorMaySelfAssignReviewer(ctx context.Context, companyID, membershipID string) (bool, error) {
	if s.membershipValidator == nil {
		return false, nil
	}
	return s.membershipValidator.HasPermission(ctx, companyID, membershipID, "ad_hoc_alert.focal_review")
}

// normalizeReviewerIDs trims whitespace and drops empty strings.
func normalizeReviewerIDs(ids []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		out = append(out, id)
	}
	return out
}

// embedReviewState populates Reviewers/Approvals/ApprovalProgress on a
// ProposalDTO copy for read paths (GetProposal/ListProposals). Errors are
// swallowed with safe fallbacks — enrichment must never block a read.
func (s *service) embedReviewState(ctx context.Context, p *ProposalDTO) *ProposalDTO {
	if p == nil {
		return p
	}
	out := *p
	reviewers, err := s.repo.ListReviewers(ctx, p.CompanyID, p.ProposalID)
	if err == nil {
		out.Reviewers = reviewers
	}
	approvals, err := s.repo.ListApprovals(ctx, p.CompanyID, p.ProposalID)
	if err == nil {
		out.Approvals = approvals
	}
	out.ApprovalProgress = &ApprovalProgressDTO{Required: len(out.Reviewers), Completed: len(out.Approvals)}
	return &out
}

// FinalizeLegacyApproval is a thin wrapper around AdminApprove (field-mapped
// per §5.3) for the one-time migration endpoint that auto-finalizes proposals
// stuck at pending_admin_approval. Gated on rbac.manage (platform admin only).
func (s *service) FinalizeLegacyApproval(ctx context.Context, sub Subject, companyID, proposalID string) error {
	if err := s.authorize(ctx, sub, "rbac.manage", authapp.ResourceRef{Type: "platform"}); err != nil {
		return err
	}
	cur, err := s.repo.FindByID(ctx, companyID, proposalID)
	if err != nil {
		return err
	}
	// §5.3 field mapping: use proposed dates as final dates (no human override
	// in migration path), fixed adjustment note for audit trail, and a
	// deterministic idempotency key so a repeated call is a safe no-op.
	var finalT0, finalDeadline string
	if cur.ProposedT0Date != nil {
		finalT0 = *cur.ProposedT0Date
	}
	if cur.ProposedDeadlineDate != nil {
		finalDeadline = *cur.ProposedDeadlineDate
	}
	// AdminApprove's identity check requires Subject.MembershipID == the
	// designated process controller; ActorUserID still records the real actor
	// (the platform admin running this migration) for audit purposes.
	_, err = s.AdminApprove(ctx, AdminApproveRequest{
		Subject:           Subject{UserID: sub.UserID, MembershipID: cur.ProcessControllerID, CompanyID: companyID},
		ProposalID:        proposalID,
		IdempotencyKey:    "migration-0098:" + proposalID,
		FinalT0Date:       finalT0,
		FinalDeadlineDate: finalDeadline,
		AdjustmentNote:    "Auto-approved by migration 0098 (D9)",
	})
	return err
}

// ListPendingLegacyApprovals is gated on rbac.manage (platform admin only)
// since it scans across all companies (§6.7/A1).
func (s *service) ListPendingLegacyApprovals(ctx context.Context, sub Subject) ([]PendingApprovalRow, error) {
	if err := s.authorize(ctx, sub, "rbac.manage", authapp.ResourceRef{Type: "platform"}); err != nil {
		return nil, err
	}
	return s.repo.ListPendingAdminApproval(ctx)
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

func (s *service) seedWorkflowInputs(ctx context.Context, companyID, typeID string) ([]ProposalWorkflowStepInput, error) {
	if s.seeder == nil {
		return nil, perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "workflow template seeder is unavailable", nil)
	}
	inputs, err := s.seeder.SeedFromDisclosureType(ctx, companyID, typeID)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	if len(inputs) == 0 {
		return nil, newAdHocFieldError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, "type_id", "disclosure type has no effective workflow steps to seed")
	}
	return inputs, nil
}

func (s *service) normalizeAndValidateWorkflow(
	ctx context.Context,
	companyID, typeID string,
	inputs []ProposalWorkflowStepInput,
	existingIDs map[string]struct{},
	frozen bool,
) (*ProposalWorkflowSnapshot, error) {
	newID := func() string {
		if s.idg != nil {
			return s.idg.NewUUID()
		}
		return uuid.NewString()
	}
	snap, err := NormalizeProposalWorkflowSteps(typeID, inputs, existingIDs, frozen, newID)
	if err != nil {
		return nil, err
	}
	needsOrg := false
	for _, st := range snap.Steps {
		if st.DepartmentID != "" || st.AssigneeMembershipID != "" {
			needsOrg = true
			break
		}
	}
	if needsOrg {
		if err := ValidateWorkflowStepOrgRefs(ctx, s.org, companyID, snap.Steps); err != nil {
			return nil, err
		}
	}
	return snap, nil
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
