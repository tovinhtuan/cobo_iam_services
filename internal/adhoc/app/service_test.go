package app

import (
	"context"
	"net/http"
	"testing"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type fakeRepository struct {
	proposal    *ProposalDTO
	lastUpdate  StatusUpdate
	updateCalls int
}

func (f *fakeRepository) Insert(ctx context.Context, p ProposalDTO) (*ProposalDTO, error) {
	cp := p
	return &cp, nil
}

func (f *fakeRepository) FindByID(ctx context.Context, companyID, proposalID string) (*ProposalDTO, error) {
	return f.proposal, nil
}

func (f *fakeRepository) UpdateStatus(ctx context.Context, upd StatusUpdate) (*ProposalDTO, error) {
	f.lastUpdate = upd
	f.updateCalls++
	cp := *f.proposal
	cp.Status = upd.Status
	cp.RecordID = upd.RecordID
	cp.WorkflowInstanceID = upd.WorkflowInstanceID
	cp.FinalT0Date = upd.FinalT0Date
	cp.FinalDeadlineDate = upd.FinalDeadlineDate
	cp.AdjustmentNote = upd.AdjustmentNote
	return &cp, nil
}

func (f *fakeRepository) List(ctx context.Context, companyID string, statusFilter []string, page, pageSize int) ([]ProposalDTO, int, error) {
	return nil, 0, nil
}

type fakeRecordCreator struct {
	t0Date     *time.Time
	callCount  int
	recordID   string
	workflowID string
}

func (f *fakeRecordCreator) CreateAndSubmitRecord(ctx context.Context, companyID, typeID, createdByMembershipID, title string, t0Date *time.Time) (string, string, error) {
	f.callCount++
	f.t0Date = t0Date
	return f.recordID, f.workflowID, nil
}

type fakeIDGen struct{}

func (fakeIDGen) NewUUID() string { return "uuid-test" }

type fakeAuthService struct {
	decision       authapp.Decision
	authorizeCalls int
	lastAction     string
}

func (f *fakeAuthService) Authorize(_ context.Context, req authapp.AuthorizeRequest) (*authapp.AuthorizeDecision, error) {
	f.authorizeCalls++
	f.lastAction = req.Action
	return &authapp.AuthorizeDecision{Decision: f.decision}, nil
}

func (f *fakeAuthService) AuthorizeBatch(_ context.Context, _ authapp.AuthorizeBatchRequest) (*authapp.AuthorizeBatchResponse, error) {
	return &authapp.AuthorizeBatchResponse{}, nil
}

func (f *fakeAuthService) GetEffectiveAccess(_ context.Context, _, _ string) (*authapp.EffectiveAccessSummary, error) {
	return &authapp.EffectiveAccessSummary{}, nil
}

func newTestService(repo *fakeRepository, recordCreator *fakeRecordCreator, auth *fakeAuthService) Service {
	return NewService(repo, recordCreator, fakeIDGen{}, false, auth)
}

func TestCreateProposalRequiresPermission(t *testing.T) {
	repo := &fakeRepository{}
	auth := &fakeAuthService{decision: authapp.DecisionDeny}
	svc := newTestService(repo, &fakeRecordCreator{}, auth)

	_, err := svc.CreateProposal(context.Background(), CreateProposalRequest{
		Subject: Subject{CompanyID: "company-001", MembershipID: "member-001", UserID: "user-001"},
		TypeID:  "dt-001",
	})
	if err == nil {
		t.Fatal("expected permission error")
	}
	httpErr, ok := err.(*perr.HTTPError)
	if !ok || httpErr.HTTPStatus != http.StatusForbidden {
		t.Fatalf("expected 403 HTTPError, got %#v", err)
	}
	if auth.lastAction != "ad_hoc_alert.propose" {
		t.Fatalf("expected propose action, got %q", auth.lastAction)
	}
}

func TestFocalApproveRequiresPermission(t *testing.T) {
	repo := &fakeRepository{proposal: &ProposalDTO{
		ProposalID: "prop-001", CompanyID: "company-001", TypeID: "dt-001", Status: StatusPendingFocalApproval,
	}}
	auth := &fakeAuthService{decision: authapp.DecisionDeny}
	svc := newTestService(repo, &fakeRecordCreator{}, auth)

	_, err := svc.FocalApprove(context.Background(), ProposalActionRequest{
		Subject:    Subject{CompanyID: "company-001", MembershipID: "member-001", UserID: "user-001"},
		ProposalID: "prop-001",
	})
	if err == nil {
		t.Fatal("expected permission error")
	}
	httpErr, ok := err.(*perr.HTTPError)
	if !ok || httpErr.HTTPStatus != http.StatusForbidden {
		t.Fatalf("expected 403 HTTPError, got %#v", err)
	}
	if auth.lastAction != "ad_hoc_alert.focal_review" {
		t.Fatalf("expected focal_review action, got %q", auth.lastAction)
	}
	if repo.updateCalls != 0 {
		t.Fatalf("expected no mutation, got %d updates", repo.updateCalls)
	}
}

func TestRejectUsesStatusBasedPermission(t *testing.T) {
	repo := &fakeRepository{proposal: &ProposalDTO{
		ProposalID: "prop-001", CompanyID: "company-001", TypeID: "dt-001", Status: StatusPendingAdminApproval,
	}}
	auth := &fakeAuthService{decision: authapp.DecisionDeny}
	svc := newTestService(repo, &fakeRecordCreator{}, auth)

	_, err := svc.Reject(context.Background(), RejectRequest{
		Subject:      Subject{CompanyID: "company-001", MembershipID: "member-001", UserID: "user-001"},
		ProposalID:   "prop-001",
		RejectReason: "Not valid",
	})
	if err == nil {
		t.Fatal("expected permission error")
	}
	if auth.lastAction != "ad_hoc_alert.admin_review" {
		t.Fatalf("expected admin_review action, got %q", auth.lastAction)
	}
	if repo.updateCalls != 0 {
		t.Fatalf("expected no mutation, got %d updates", repo.updateCalls)
	}
}

func TestAdminApprovePersistsFinalOverrideFields(t *testing.T) {
	repo := &fakeRepository{proposal: &ProposalDTO{
		ProposalID: "prop-001",
		CompanyID:  "company-001",
		TypeID:     "dt-001",
		Status:     StatusPendingAdminApproval,
		ChangeNote: "Urgent approval",
	}}
	recordCreator := &fakeRecordCreator{recordID: "record-001", workflowID: "wf-001"}
	auth := &fakeAuthService{decision: authapp.DecisionAllow}
	svc := newTestService(repo, recordCreator, auth)

	resp, err := svc.AdminApprove(context.Background(), AdminApproveRequest{
		Subject:           Subject{CompanyID: "company-001", MembershipID: "member-admin", UserID: "user-admin"},
		ProposalID:        "prop-001",
		FinalT0Date:       "2026-06-01",
		FinalDeadlineDate: "2026-06-30",
		AdjustmentNote:    "  Finalized by admin  ",
	})
	if err != nil {
		t.Fatalf("AdminApprove() error = %v", err)
	}
	if auth.lastAction != "ad_hoc_alert.admin_review" {
		t.Fatalf("expected admin_review action, got %q", auth.lastAction)
	}
	if recordCreator.callCount != 1 {
		t.Fatalf("expected record creator to be called once, got %d", recordCreator.callCount)
	}
	if recordCreator.t0Date == nil || recordCreator.t0Date.Format("2006-01-02") != "2026-06-01" {
		t.Fatalf("expected t0Date 2026-06-01, got %#v", recordCreator.t0Date)
	}
	if repo.updateCalls != 1 {
		t.Fatalf("expected repository update once, got %d", repo.updateCalls)
	}
	if repo.lastUpdate.FinalT0Date == nil || *repo.lastUpdate.FinalT0Date != "2026-06-01" {
		t.Fatalf("expected FinalT0Date to be persisted, got %#v", repo.lastUpdate.FinalT0Date)
	}
	if repo.lastUpdate.FinalDeadlineDate == nil || *repo.lastUpdate.FinalDeadlineDate != "2026-06-30" {
		t.Fatalf("expected FinalDeadlineDate to be persisted, got %#v", repo.lastUpdate.FinalDeadlineDate)
	}
	if repo.lastUpdate.AdjustmentNote != "Finalized by admin" {
		t.Fatalf("expected trimmed AdjustmentNote, got %q", repo.lastUpdate.AdjustmentNote)
	}
	if resp.Proposal.FinalDeadlineDate == nil || *resp.Proposal.FinalDeadlineDate != "2026-06-30" {
		t.Fatalf("expected response to include final deadline date, got %#v", resp.Proposal.FinalDeadlineDate)
	}
}

func TestAdminApproveRejectsInvalidFinalDate(t *testing.T) {
	repo := &fakeRepository{proposal: &ProposalDTO{
		ProposalID: "prop-001",
		CompanyID:  "company-001",
		TypeID:     "dt-001",
		Status:     StatusPendingAdminApproval,
	}}
	recordCreator := &fakeRecordCreator{recordID: "record-001", workflowID: "wf-001"}
	auth := &fakeAuthService{decision: authapp.DecisionAllow}
	svc := newTestService(repo, recordCreator, auth)

	_, err := svc.AdminApprove(context.Background(), AdminApproveRequest{
		Subject:           Subject{CompanyID: "company-001", MembershipID: "member-admin", UserID: "user-admin"},
		ProposalID:        "prop-001",
		FinalDeadlineDate: "06/30/2026",
	})
	if err == nil {
		t.Fatal("expected invalid final_deadline_date error")
	}
	if recordCreator.callCount != 0 {
		t.Fatalf("expected record creator to not be called, got %d", recordCreator.callCount)
	}
}
