package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

type service struct {
	repo          Repository
	recordCreator RecordCreator
	typeCatalog   TypeCatalog
	idg           idgen.Generator
	autoApprove   bool // WORKFLOW_ADHOC_AUTOAPPROVE_ENABLED: skip focal step
	auth          authapp.Service
}

func NewService(repo Repository, recordCreator RecordCreator, typeCatalog TypeCatalog, idg idgen.Generator, autoApprove bool, auth authapp.Service) Service {
	return &service{repo: repo, recordCreator: recordCreator, typeCatalog: typeCatalog, idg: idg, autoApprove: autoApprove, auth: auth}
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
		return nil, err
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
	t0 := strings.TrimSpace(req.ProposedT0Date)
	dl := strings.TrimSpace(req.ProposedDeadline)
	p := ProposalDTO{
		ProposalID:    s.idg.NewUUID(),
		CompanyID:     req.Subject.CompanyID,
		TypeID:        req.TypeID,
		Status:        StatusDraft,
		StepOverrides: req.StepOverrides,
		ChangeNote:    strings.TrimSpace(req.ChangeNote),
		CreatedBy:     req.Subject.MembershipID,
	}
	if t0 != "" {
		p.ProposedT0Date = &t0
	}
	if dl != "" {
		p.ProposedDeadlineDate = &dl
	}
	return s.repo.Insert(ctx, p)
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
	return s.repo.UpdateStatus(ctx, StatusUpdate{
		ProposalID:               req.ProposalID,
		CompanyID:                req.Subject.CompanyID,
		Status:                   nextStatus,
		ActorMembershipID:        req.Subject.MembershipID,
		ActorUserID:              req.Subject.UserID,
		SetFocalApprovalMetadata: false,
	})
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
	return s.repo.UpdateStatus(ctx, StatusUpdate{
		ProposalID:               req.ProposalID,
		CompanyID:                req.Subject.CompanyID,
		Status:                   StatusPendingAdminApproval,
		ActorMembershipID:        req.Subject.MembershipID,
		ActorUserID:              req.Subject.UserID,
		SetFocalApprovalMetadata: true,
	})
}

func (s *service) AdminApprove(ctx context.Context, req AdminApproveRequest) (*AdminApproveResponse, error) {
	if err := s.authorize(ctx, req.Subject, "ad_hoc_alert.admin_review", authapp.ResourceRef{Type: "ad_hoc_proposal", ID: req.ProposalID}); err != nil {
		return nil, err
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
	title := "Ad-hoc: " + cur.TypeID
	if cur.ChangeNote != "" {
		title = "Ad-hoc: " + cur.ChangeNote
	}
	recordID := reservation.ProgressRecordID
	workflowInstanceID := reservation.ProgressWorkflowID
	if strings.TrimSpace(recordID) == "" {
		recordID, workflowInstanceID, err = s.recordCreator.CreateAndSubmitRecord(ctx, cur.CompanyID, cur.TypeID, req.Subject.MembershipID, title, finalT0Time)
		if err != nil {
			return nil, perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "failed to create disclosure record", err)
		}
		if err := s.repo.SaveAdminApprovalProgress(ctx, req.Subject.CompanyID, req.ProposalID, strings.TrimSpace(req.IdempotencyKey), recordID, workflowInstanceID, ""); err != nil {
			return nil, err
		}
	}

	updated, err := s.repo.CompleteAdminApproval(ctx, StatusUpdate{
		ProposalID:         req.ProposalID,
		CompanyID:          req.Subject.CompanyID,
		Status:             StatusApproved,
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
	permission := "ad_hoc_alert.admin_review"
	if cur.Status == StatusPendingFocalApproval {
		permission = "ad_hoc_alert.focal_review"
	}
	if err := s.authorize(ctx, req.Subject, permission, authapp.ResourceRef{Type: "ad_hoc_proposal", ID: req.ProposalID, Attributes: map[string]any{"workflow_state": cur.Status}}); err != nil {
		return nil, err
	}
	reason := strings.TrimSpace(req.RejectReason)
	if reason == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "reject_reason is required", nil)
	}
	return s.repo.UpdateStatus(ctx, StatusUpdate{
		ProposalID:        req.ProposalID,
		CompanyID:         req.Subject.CompanyID,
		Status:            StatusRejected,
		ActorMembershipID: req.Subject.MembershipID,
		ActorUserID:       req.Subject.UserID,
		RejectReason:      reason,
	})
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
	return s.repo.UpdateStatus(ctx, StatusUpdate{
		ProposalID:        req.ProposalID,
		CompanyID:         req.Subject.CompanyID,
		Status:            StatusCancelled,
		ActorMembershipID: req.Subject.MembershipID,
		ActorUserID:       req.Subject.UserID,
	})
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

func (s *service) authorize(ctx context.Context, sub Subject, action string, resource authapp.ResourceRef) error {
	decision, err := s.auth.Authorize(ctx, authapp.AuthorizeRequest{
		Subject:  authapp.SubjectRef{UserID: sub.UserID, MembershipID: sub.MembershipID, CompanyID: sub.CompanyID},
		Action:   action,
		Resource: resource,
	})
	if err != nil {
		return fmt.Errorf("authorize adhoc action: %w", err)
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
