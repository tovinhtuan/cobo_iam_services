package app

import (
	"context"
	"net/http"
	"strings"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

type service struct {
	repo          Repository
	recordCreator RecordCreator
	idg           idgen.Generator
	autoApprove   bool // WORKFLOW_ADHOC_AUTOAPPROVE_ENABLED: skip focal step
}

func NewService(repo Repository, recordCreator RecordCreator, idg idgen.Generator, autoApprove bool) Service {
	return &service{repo: repo, recordCreator: recordCreator, idg: idg, autoApprove: autoApprove}
}

func (s *service) CreateProposal(ctx context.Context, req CreateProposalRequest) (*ProposalDTO, error) {
	req.TypeID = strings.TrimSpace(req.TypeID)
	if req.TypeID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id is required", nil)
	}
	if len(req.StepOverrides) == 0 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "step_overrides is required", nil)
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
		ProposalID:        req.ProposalID,
		CompanyID:         req.Subject.CompanyID,
		Status:            nextStatus,
		ActorMembershipID: req.Subject.MembershipID,
		ActorUserID:       req.Subject.UserID,
	})
}

func (s *service) FocalApprove(ctx context.Context, req ProposalActionRequest) (*ProposalDTO, error) {
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
		ProposalID:        req.ProposalID,
		CompanyID:         req.Subject.CompanyID,
		Status:            StatusPendingAdminApproval,
		ActorMembershipID: req.Subject.MembershipID,
		ActorUserID:       req.Subject.UserID,
	})
}

func (s *service) AdminApprove(ctx context.Context, req AdminApproveRequest) (*AdminApproveResponse, error) {
	cur, err := s.repo.FindByID(ctx, req.Subject.CompanyID, req.ProposalID)
	if err != nil {
		return nil, err
	}
	if cur.Status != StatusPendingAdminApproval {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "proposal is not pending admin approval", nil)
	}

	// Auto-create and submit the disclosure record synchronously.
	title := "Ad-hoc: " + cur.TypeID
	if cur.ChangeNote != "" {
		title = "Ad-hoc: " + cur.ChangeNote
	}
	recordID, workflowInstanceID, err := s.recordCreator.CreateAndSubmitRecord(ctx, cur.CompanyID, cur.TypeID, req.Subject.MembershipID, title)
	if err != nil {
		return nil, perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "failed to create disclosure record", err)
	}

	updated, err := s.repo.UpdateStatus(ctx, StatusUpdate{
		ProposalID:         req.ProposalID,
		CompanyID:          req.Subject.CompanyID,
		Status:             StatusApproved,
		ActorMembershipID:  req.Subject.MembershipID,
		ActorUserID:        req.Subject.UserID,
		RecordID:           recordID,
		WorkflowInstanceID: workflowInstanceID,
	})
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
	if strings.TrimSpace(req.ProposalID) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "proposal_id is required", nil)
	}
	return s.repo.FindByID(ctx, req.Subject.CompanyID, req.ProposalID)
}

func (s *service) ListProposals(ctx context.Context, req ListProposalsRequest) (*ListProposalsResponse, error) {
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
