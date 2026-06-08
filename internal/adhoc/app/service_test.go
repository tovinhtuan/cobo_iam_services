package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type fakeRepository struct {
	proposal           *ProposalDTO
	lastUpdate         StatusUpdate
	insertCalls        int
	updateCalls        int
	reserveCalls       int
	progressCalls      int
	completeCalls      int
	progressRecordID   string
	progressWorkflowID string
	completeErr        error
}

func (f *fakeRepository) Insert(ctx context.Context, p ProposalDTO) (*ProposalDTO, error) {
	f.insertCalls++
	cp := p
	return &cp, nil
}

func (f *fakeRepository) FindByID(ctx context.Context, companyID, proposalID string) (*ProposalDTO, error) {
	return f.proposal, nil
}

func (f *fakeRepository) UpdateStatus(ctx context.Context, upd StatusUpdate) (*ProposalDTO, bool, error) {
	f.lastUpdate = upd
	f.updateCalls++
	cp := *f.proposal
	cp.Status = upd.Status
	cp.RecordID = upd.RecordID
	cp.WorkflowInstanceID = upd.WorkflowInstanceID
	cp.FinalT0Date = upd.FinalT0Date
	cp.FinalDeadlineDate = upd.FinalDeadlineDate
	cp.AdjustmentNote = upd.AdjustmentNote
	if upd.Status == StatusPendingAdminApproval && upd.SetFocalApprovalMetadata {
		now := time.Now().UTC()
		cp.FocalApprovedBy = upd.ActorMembershipID
		cp.FocalApprovedAt = &now
	}
	return &cp, true, nil
}

func (f *fakeRepository) ReserveAdminApproval(ctx context.Context, in ReserveAdminApprovalInput) (*AdminApprovalReservation, error) {
	f.reserveCalls++
	return &AdminApprovalReservation{
		Proposal:           f.proposal,
		ProgressRecordID:   f.progressRecordID,
		ProgressWorkflowID: f.progressWorkflowID,
	}, nil
}

func (f *fakeRepository) SaveAdminApprovalProgress(ctx context.Context, companyID, proposalID, idemKey, recordID, workflowID, lastError string) error {
	f.progressCalls++
	f.progressRecordID = recordID
	f.progressWorkflowID = workflowID
	return nil
}

func (f *fakeRepository) CompleteAdminApproval(ctx context.Context, upd StatusUpdate, idemKey string) (*ProposalDTO, bool, error) {
	f.completeCalls++
	f.lastUpdate = upd
	if f.completeErr != nil {
		return nil, false, f.completeErr
	}
	cp := *f.proposal
	cp.Status = upd.Status
	cp.RecordID = upd.RecordID
	cp.WorkflowInstanceID = upd.WorkflowInstanceID
	cp.FinalT0Date = upd.FinalT0Date
	cp.FinalDeadlineDate = upd.FinalDeadlineDate
	cp.AdjustmentNote = upd.AdjustmentNote
	f.proposal = &cp
	return &cp, true, nil
}

func (f *fakeRepository) List(ctx context.Context, companyID string, statusFilter []string, page, pageSize int) ([]ProposalDTO, int, error) {
	return nil, 0, nil
}

type fakeRecordCreator struct {
	t0Date        *time.Time
	callCount     int
	recordID      string
	workflowID    string
	lastTitle     string
	lastOverrides []WorkflowStepOverride
	lastCreatedBy string // CF-15: captures the createdByMembershipID the service passed through
	err           error
}

func (f *fakeRecordCreator) CreateAndSubmitRecord(ctx context.Context, companyID, typeID, createdByMembershipID, title string, t0Date *time.Time) (string, string, error) {
	return f.CreateAndSubmitRecordWithOpts(ctx, companyID, typeID, createdByMembershipID, title, t0Date, CreateRecordOpts{})
}

func (f *fakeRecordCreator) CreateAndSubmitRecordWithOpts(ctx context.Context, companyID, typeID, createdByMembershipID, title string, t0Date *time.Time, opts CreateRecordOpts) (string, string, error) {
	f.callCount++
	f.t0Date = t0Date
	f.lastTitle = title
	f.lastCreatedBy = createdByMembershipID
	f.lastOverrides = append([]WorkflowStepOverride(nil), opts.StepOverrides...)
	if f.err != nil {
		return "", "", f.err
	}
	return f.recordID, f.workflowID, nil
}

type fakeIDGen struct{}

func (fakeIDGen) NewUUID() string { return "uuid-test" }

type fakeAuthService struct {
	mu             sync.Mutex // guards authorizeCalls/lastAction for concurrency tests (Cancel/FocalApprove races)
	decision       authapp.Decision
	authorizeCalls int
	lastAction     string
}

func (f *fakeAuthService) Authorize(_ context.Context, req authapp.AuthorizeRequest) (*authapp.AuthorizeDecision, error) {
	f.mu.Lock()
	f.authorizeCalls++
	f.lastAction = req.Action
	f.mu.Unlock()
	return &authapp.AuthorizeDecision{Decision: f.decision}, nil
}

func (f *fakeAuthService) AuthorizeBatch(_ context.Context, _ authapp.AuthorizeBatchRequest) (*authapp.AuthorizeBatchResponse, error) {
	return &authapp.AuthorizeBatchResponse{}, nil
}

func (f *fakeAuthService) GetEffectiveAccess(_ context.Context, _, _ string) (*authapp.EffectiveAccessSummary, error) {
	return &authapp.EffectiveAccessSummary{}, nil
}

type fakeTypeCatalog struct {
	category    string
	displayName string
	err         error
	callCount   int
	lastType    string
}

func (f *fakeTypeCatalog) GetTemplateCategory(_ context.Context, _ string, typeID string) (string, error) {
	f.callCount++
	f.lastType = typeID
	if f.err != nil {
		return "", f.err
	}
	return f.category, nil
}

func (f *fakeTypeCatalog) GetTypeDisplayName(_ context.Context, _ string, _ string) (string, error) {
	return f.displayName, nil
}

// fakeMembershipValidator always returns active=true and hasPerm=true unless overridden.
type fakeMembershipValidator struct {
	active       bool
	hasPerm      bool
	hasAdminRole bool
}

func newAllowValidator() *fakeMembershipValidator {
	return &fakeMembershipValidator{active: true, hasPerm: true}
}

func (f *fakeMembershipValidator) IsActiveMembership(_ context.Context, _, _ string) (bool, error) {
	return f.active, nil
}
func (f *fakeMembershipValidator) HasPermission(_ context.Context, _, _, _ string) (bool, error) {
	return f.hasPerm, nil
}
func (f *fakeMembershipValidator) HasActiveRoleCode(_ context.Context, _, _, roleCode string) (bool, error) {
	return f.hasAdminRole && roleCode == RoleCodeAdminDoanhNghiep, nil
}
func (f *fakeMembershipValidator) ListMembersWithPermission(_ context.Context, _, _, _ string) ([]EligibleController, error) {
	return []EligibleController{}, nil
}
func (f *fakeMembershipValidator) ResolveMembership(_ context.Context, _, _ string) (*MemberInfo, error) {
	return nil, nil
}
func (f *fakeMembershipValidator) ListMembersWithPermissionFull(_ context.Context, _, _ string) ([]MemberInfo, error) {
	return []MemberInfo{}, nil
}

func newTestService(repo *fakeRepository, recordCreator *fakeRecordCreator, typeCatalog *fakeTypeCatalog, auth *fakeAuthService) Service {
	return NewService(repo, recordCreator, typeCatalog, fakeIDGen{}, false, auth, newAllowValidator(), nil, noopMetrics{})
}

func TestCreateProposalRequiresPermission(t *testing.T) {
	repo := &fakeRepository{}
	typeCatalog := &fakeTypeCatalog{category: "irregular"}
	auth := &fakeAuthService{decision: authapp.DecisionDeny}
	svc := newTestService(repo, &fakeRecordCreator{}, typeCatalog, auth)

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
	if typeCatalog.callCount != 0 {
		t.Fatalf("expected catalog lookup to be skipped on denied auth, got %d calls", typeCatalog.callCount)
	}
}

func TestCreateProposalRejectsNonIrregularTemplates(t *testing.T) {
	tests := []struct {
		name     string
		category string
	}{
		{name: "periodic", category: "periodic"},
		{name: "custom", category: "custom"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepository{}
			typeCatalog := &fakeTypeCatalog{category: tc.category}
			auth := &fakeAuthService{decision: authapp.DecisionAllow}
			svc := newTestService(repo, &fakeRecordCreator{}, typeCatalog, auth)

			_, err := svc.CreateProposal(context.Background(), CreateProposalRequest{
				Subject: Subject{CompanyID: "company-001", MembershipID: "member-001", UserID: "user-001"},
				TypeID:  "dt-001",
			})
			if err == nil {
				t.Fatal("expected non-irregular template to be rejected")
			}
			httpErr, ok := err.(*perr.HTTPError)
			if !ok || httpErr.HTTPStatus != http.StatusBadRequest {
				t.Fatalf("expected 400 HTTPError, got %#v", err)
			}
			if repo.insertCalls != 0 {
				t.Fatalf("expected no proposal insert for category %q, got %d inserts", tc.category, repo.insertCalls)
			}
		})
	}
}

func TestCreateProposalAllowsIrregularTemplate(t *testing.T) {
	repo := &fakeRepository{}
	typeCatalog := &fakeTypeCatalog{category: "irregular"}
	auth := &fakeAuthService{decision: authapp.DecisionAllow}
	svc := newTestService(repo, &fakeRecordCreator{}, typeCatalog, auth)

	resp, err := svc.CreateProposal(context.Background(), CreateProposalRequest{
		Subject:                       Subject{CompanyID: "company-001", MembershipID: "member-001", UserID: "user-001"},
		TypeID:                        "dt-irregular",
		ProcessControllerMembershipID: "member-controller",
	})
	if err != nil {
		t.Fatalf("CreateProposal() error = %v", err)
	}
	if typeCatalog.lastType != "dt-irregular" {
		t.Fatalf("expected catalog lookup for dt-irregular, got %q", typeCatalog.lastType)
	}
	if repo.insertCalls != 1 {
		t.Fatalf("expected one proposal insert, got %d", repo.insertCalls)
	}
	if resp.TypeID != "dt-irregular" {
		t.Fatalf("expected proposal TypeID dt-irregular, got %q", resp.TypeID)
	}
}

func TestFocalApproveRequiresPermission(t *testing.T) {
	repo := &fakeRepository{proposal: &ProposalDTO{
		ProposalID: "prop-001", CompanyID: "company-001", TypeID: "dt-001", Status: StatusPendingFocalApproval,
	}}
	auth := &fakeAuthService{decision: authapp.DecisionDeny}
	svc := newTestService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, auth)

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

func TestSubmitProposalAutoApproveSkipsFocalMetadata(t *testing.T) {
	repo := &fakeRepository{proposal: &ProposalDTO{
		ProposalID: "prop-001",
		CompanyID:  "company-001",
		TypeID:     "dt-001",
		Status:     StatusDraft,
		CreatedBy:  "member-creator",
	}}
	auth := &fakeAuthService{decision: authapp.DecisionAllow}
	svc := NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, true, auth, nil, nil, noopMetrics{})

	resp, err := svc.SubmitProposal(context.Background(), ProposalActionRequest{
		Subject:    Subject{CompanyID: "company-001", MembershipID: "member-creator", UserID: "user-001"},
		ProposalID: "prop-001",
	})
	if err != nil {
		t.Fatalf("SubmitProposal() error = %v", err)
	}
	if resp.Status != StatusPendingAdminApproval {
		t.Fatalf("expected status %q, got %q", StatusPendingAdminApproval, resp.Status)
	}
	if repo.lastUpdate.SetFocalApprovalMetadata {
		t.Fatal("expected auto-approve submit to skip focal approval metadata")
	}
	if resp.FocalApprovedBy != "" || resp.FocalApprovedAt != nil {
		t.Fatalf("expected no focal approval metadata, got by=%q at=%v", resp.FocalApprovedBy, resp.FocalApprovedAt)
	}
}

func TestFocalApproveSetsFocalMetadata(t *testing.T) {
	repo := &fakeRepository{proposal: &ProposalDTO{
		ProposalID: "prop-001",
		CompanyID:  "company-001",
		TypeID:     "dt-001",
		Status:     StatusPendingFocalApproval,
	}}
	auth := &fakeAuthService{decision: authapp.DecisionAllow}
	svc := newTestService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, auth)

	resp, err := svc.FocalApprove(context.Background(), ProposalActionRequest{
		Subject:    Subject{CompanyID: "company-001", MembershipID: "member-focal", UserID: "user-focal"},
		ProposalID: "prop-001",
	})
	if err != nil {
		t.Fatalf("FocalApprove() error = %v", err)
	}
	if resp.Status != StatusPendingAdminApproval {
		t.Fatalf("expected status %q, got %q", StatusPendingAdminApproval, resp.Status)
	}
	if !repo.lastUpdate.SetFocalApprovalMetadata {
		t.Fatal("expected focal approve transition to persist focal metadata")
	}
	if resp.FocalApprovedBy != "member-focal" || resp.FocalApprovedAt == nil {
		t.Fatalf("expected focal metadata to be set, got by=%q at=%v", resp.FocalApprovedBy, resp.FocalApprovedAt)
	}
}

func TestRejectUsesStatusBasedPermission(t *testing.T) {
	// At pending_admin_approval stage, Reject uses identity check (not permission check).
	repo := &fakeRepository{proposal: &ProposalDTO{
		ProposalID: "prop-001", CompanyID: "company-001", TypeID: "dt-001",
		Status: StatusPendingAdminApproval, ProcessControllerID: "member-controller",
	}}
	auth := &fakeAuthService{decision: authapp.DecisionAllow}
	svc := newTestService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, auth)

	_, err := svc.Reject(context.Background(), RejectRequest{
		Subject:      Subject{CompanyID: "company-001", MembershipID: "member-001", UserID: "user-001"},
		ProposalID:   "prop-001",
		RejectReason: "Not valid",
	})
	if err == nil {
		t.Fatal("expected identity check error")
	}
	// No permission check should be called â€” gate is purely by identity.
	if auth.authorizeCalls != 0 {
		t.Fatalf("expected no authorize calls for admin stage rejection, got %d", auth.authorizeCalls)
	}
	if repo.updateCalls != 0 {
		t.Fatalf("expected no mutation, got %d updates", repo.updateCalls)
	}
}

func TestAdminApprovePersistsFinalOverrideFields(t *testing.T) {
	repo := &fakeRepository{proposal: &ProposalDTO{
		ProposalID:          "prop-001",
		CompanyID:           "company-001",
		TypeID:              "dt-001",
		Status:              StatusPendingAdminApproval,
		ChangeNote:          "Urgent approval",
		ProcessControllerID: "member-admin",
		StepOverrides:       []WorkflowStepOverride{{StepID: "step-1", ProcessingDays: 7}},
	}}
	recordCreator := &fakeRecordCreator{recordID: "record-001", workflowID: "wf-001"}
	auth := &fakeAuthService{decision: authapp.DecisionAllow}
	svc := newTestService(repo, recordCreator, &fakeTypeCatalog{category: "irregular"}, auth)

	resp, err := svc.AdminApprove(context.Background(), AdminApproveRequest{
		Subject:           Subject{CompanyID: "company-001", MembershipID: "member-admin", UserID: "user-admin"},
		ProposalID:        "prop-001",
		IdempotencyKey:    "idem-001",
		FinalT0Date:       "2026-06-01",
		FinalDeadlineDate: "2026-06-30",
		AdjustmentNote:    "  Finalized by admin  ",
	})
	if err != nil {
		t.Fatalf("AdminApprove() error = %v", err)
	}
	if recordCreator.callCount != 1 {
		t.Fatalf("expected record creator to be called once, got %d", recordCreator.callCount)
	}
	if len(recordCreator.lastOverrides) != 1 || recordCreator.lastOverrides[0].StepID != "step-1" || recordCreator.lastOverrides[0].ProcessingDays != 7 {
		t.Fatalf("step overrides = %#v", recordCreator.lastOverrides)
	}
	if recordCreator.t0Date == nil || recordCreator.t0Date.Format("2006-01-02") != "2026-06-01" {
		t.Fatalf("expected t0Date 2026-06-01, got %#v", recordCreator.t0Date)
	}
	if repo.progressCalls != 1 {
		t.Fatalf("expected progress to be saved once, got %d", repo.progressCalls)
	}
	if repo.completeCalls != 1 {
		t.Fatalf("expected repository complete once, got %d", repo.completeCalls)
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

func TestAdminApproveUsesProposalTitleLineForRecord(t *testing.T) {
	repo := &fakeRepository{proposal: &ProposalDTO{
		ProposalID:          "prop-002",
		CompanyID:           "company-001",
		TypeID:              "dt-001",
		Status:              StatusPendingAdminApproval,
		ChangeNote:          "TiÃªu Ä‘á» cáº£nh bÃ¡o báº¥t thÆ°á»ng\nMÃ´ táº£ chi tiáº¿t khÃ´ng vÃ o title",
		ProcessControllerID: "member-admin",
	}}
	recordCreator := &fakeRecordCreator{recordID: "record-002", workflowID: "wf-002"}
	auth := &fakeAuthService{decision: authapp.DecisionAllow}
	svc := newTestService(repo, recordCreator, &fakeTypeCatalog{category: "irregular", displayName: "Template fallback"}, auth)

	_, err := svc.AdminApprove(context.Background(), AdminApproveRequest{
		Subject:        Subject{CompanyID: "company-001", MembershipID: "member-admin", UserID: "user-admin"},
		ProposalID:     "prop-002",
		IdempotencyKey: "idem-002",
	})
	if err != nil {
		t.Fatalf("AdminApprove() error = %v", err)
	}
	if recordCreator.lastTitle != "TiÃªu Ä‘á» cáº£nh bÃ¡o báº¥t thÆ°á»ng" {
		t.Fatalf("record title = %q want proposal title line only", recordCreator.lastTitle)
	}
}

func TestAdminApproveRejectsInvalidFinalDate(t *testing.T) {
	repo := &fakeRepository{proposal: &ProposalDTO{
		ProposalID:          "prop-001",
		CompanyID:           "company-001",
		TypeID:              "dt-001",
		Status:              StatusPendingAdminApproval,
		ProcessControllerID: "member-admin",
	}}
	recordCreator := &fakeRecordCreator{recordID: "record-001", workflowID: "wf-001"}
	auth := &fakeAuthService{decision: authapp.DecisionAllow}
	svc := newTestService(repo, recordCreator, &fakeTypeCatalog{category: "irregular"}, auth)

	_, err := svc.AdminApprove(context.Background(), AdminApproveRequest{
		Subject:           Subject{CompanyID: "company-001", MembershipID: "member-admin", UserID: "user-admin"},
		ProposalID:        "prop-001",
		IdempotencyKey:    "idem-001",
		FinalDeadlineDate: "06/30/2026",
	})
	if err == nil {
		t.Fatal("expected invalid final_deadline_date error")
	}
	if recordCreator.callCount != 0 {
		t.Fatalf("expected record creator to not be called, got %d", recordCreator.callCount)
	}
}

// ---- R-2: safeNotify recovers panics in all 4 Notify* call sites ---------------

// panicNotifier panics on every method, simulating a buggy notifier
// implementation. Used to prove safeNotify prevents the panic from propagating
// to the service caller (R-2 fix).
type panicNotifier struct{}

func (panicNotifier) NotifyFocalsForReview(_ context.Context, _ ProposalDTO, _ []MemberInfo) {
	panic("R-2 test: NotifyFocalsForReview panic")
}
func (panicNotifier) NotifyControllerForReview(_ context.Context, _ ProposalDTO, _ MemberInfo) {
	panic("R-2 test: NotifyControllerForReview panic")
}
func (panicNotifier) NotifyCreatorApproved(_ context.Context, _ ProposalDTO, _ MemberInfo) {
	panic("R-2 test: NotifyCreatorApproved panic")
}
func (panicNotifier) NotifyCreatorRejected(_ context.Context, _ ProposalDTO, _ MemberInfo) {
	panic("R-2 test: NotifyCreatorRejected panic")
}

// memberAwareFakeMembershipValidator overrides ResolveMembership and
// ListMembersWithPermissionFull so they return non-nil/non-empty results.
// This ensures the notifier IS actually called (triggering the R-2 panic),
// rather than being skipped by the `ctrl != nil` guard.
type memberAwareFakeMembershipValidator struct {
	*fakeMembershipValidator
	member MemberInfo
}

func (m *memberAwareFakeMembershipValidator) ResolveMembership(_ context.Context, _, _ string) (*MemberInfo, error) {
	return &m.member, nil
}

func (m *memberAwareFakeMembershipValidator) ListMembersWithPermissionFull(_ context.Context, _, _ string) ([]MemberInfo, error) {
	return []MemberInfo{m.member}, nil
}

func newMemberAwareValidator() *memberAwareFakeMembershipValidator {
	return &memberAwareFakeMembershipValidator{
		fakeMembershipValidator: newAllowValidator(),
		member: MemberInfo{
			UserID:       "user-r2",
			MembershipID: "member-r2",
			Email:        "r2@example.com",
			FullName:     "R2 Member",
		},
	}
}

func TestR2_NotifyPanicRecovered_SubmitProposal(t *testing.T) {
	repo := &fakeRepository{proposal: &ProposalDTO{
		ProposalID:          "prop-r2-submit",
		CompanyID:           "company-r2",
		TypeID:              "dt-001",
		Status:              StatusDraft,
		CreatedBy:           "member-creator",
		ProcessControllerID: "member-ctrl",
	}}
	auth := &fakeAuthService{decision: authapp.DecisionAllow}
	// autoApprove=false: calls NotifyFocalsForReview, which panics; safeNotify must recover.
	svc := NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, auth, newMemberAwareValidator(), panicNotifier{}, noopMetrics{})

	_, err := svc.SubmitProposal(context.Background(), ProposalActionRequest{
		Subject:    Subject{CompanyID: "company-r2", MembershipID: "member-creator", UserID: "user-001"},
		ProposalID: "prop-r2-submit",
	})
	if err != nil {
		t.Fatalf("SubmitProposal must succeed despite notifier panic (R-2 safeNotify); got %v", err)
	}
}

func TestR2_NotifyPanicRecovered_FocalApprove(t *testing.T) {
	repo := &fakeRepository{proposal: &ProposalDTO{
		ProposalID:          "prop-r2-focal",
		CompanyID:           "company-r2",
		TypeID:              "dt-001",
		Status:              StatusPendingFocalApproval,
		ProcessControllerID: "member-ctrl",
	}}
	auth := &fakeAuthService{decision: authapp.DecisionAllow}
	// autoApprove=false: calls NotifyControllerForReview (ctrl != nil guaranteed by
	// memberAwareValidator), which panics; safeNotify must recover.
	svc := NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, auth, newMemberAwareValidator(), panicNotifier{}, noopMetrics{})

	_, err := svc.FocalApprove(context.Background(), ProposalActionRequest{
		Subject:    Subject{CompanyID: "company-r2", MembershipID: "member-focal", UserID: "user-focal"},
		ProposalID: "prop-r2-focal",
	})
	if err != nil {
		t.Fatalf("FocalApprove must succeed despite notifier panic (R-2 safeNotify); got %v", err)
	}
}

func TestR2_NotifyPanicRecovered_AdminApprove(t *testing.T) {
	repo := &fakeRepository{
		proposal: &ProposalDTO{
			ProposalID:          "prop-r2-admin",
			CompanyID:           "company-r2",
			TypeID:              "dt-001",
			Status:              StatusPendingAdminApproval,
			ChangeNote:          "Admin approve panic test",
			ProcessControllerID: "member-ctrl",
			CreatedBy:           "member-creator",
		},
	}
	recordCreator := &fakeRecordCreator{recordID: "record-r2", workflowID: "wf-r2"}
	auth := &fakeAuthService{decision: authapp.DecisionAllow}
	// ProcessControllerID == Subject.MembershipID passes the identity gate; then
	// NotifyCreatorApproved panics (creator != nil via memberAwareValidator);
	// safeNotify must recover without rolling back the approved transition.
	svc := NewService(repo, recordCreator, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, auth, newMemberAwareValidator(), panicNotifier{}, noopMetrics{})

	_, err := svc.AdminApprove(context.Background(), AdminApproveRequest{
		Subject:        Subject{CompanyID: "company-r2", MembershipID: "member-ctrl", UserID: "user-ctrl"},
		ProposalID:     "prop-r2-admin",
		IdempotencyKey: "idem-r2-admin",
	})
	if err != nil {
		t.Fatalf("AdminApprove must succeed despite notifier panic (R-2 safeNotify); got %v", err)
	}
}

func TestR2_NotifyPanicRecovered_Reject(t *testing.T) {
	repo := &fakeRepository{proposal: &ProposalDTO{
		ProposalID:          "prop-r2-reject",
		CompanyID:           "company-r2",
		TypeID:              "dt-001",
		Status:              StatusPendingAdminApproval,
		ProcessControllerID: "member-ctrl",
		CreatedBy:           "member-creator",
	}}
	auth := &fakeAuthService{decision: authapp.DecisionAllow}
	// admin-stage reject uses identity check only (no s.authorize call);
	// NotifyCreatorRejected panics (creator != nil via memberAwareValidator);
	// safeNotify must recover.
	svc := NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, auth, newMemberAwareValidator(), panicNotifier{}, noopMetrics{})

	_, err := svc.Reject(context.Background(), RejectRequest{
		Subject:      Subject{CompanyID: "company-r2", MembershipID: "member-ctrl", UserID: "user-ctrl"},
		ProposalID:   "prop-r2-reject",
		RejectReason: "Rejected for R-2 panic test",
	})
	if err != nil {
		t.Fatalf("Reject must succeed despite notifier panic (R-2 safeNotify); got %v", err)
	}
}

func TestAdminApproveRetryReusesSavedProgressAfterFinalizeFailure(t *testing.T) {
	repo := &fakeRepository{proposal: &ProposalDTO{
		ProposalID:          "prop-001",
		CompanyID:           "company-001",
		TypeID:              "dt-001",
		Status:              StatusPendingAdminApproval,
		ChangeNote:          "Urgent approval",
		ProcessControllerID: "member-admin",
	}}
	recordCreator := &fakeRecordCreator{recordID: "record-001", workflowID: "wf-001"}
	auth := &fakeAuthService{decision: authapp.DecisionAllow}
	svc := newTestService(repo, recordCreator, &fakeTypeCatalog{category: "irregular"}, auth)

	repo.completeErr = perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "transient finalize failure", nil)
	_, err := svc.AdminApprove(context.Background(), AdminApproveRequest{
		Subject:        Subject{CompanyID: "company-001", MembershipID: "member-admin", UserID: "user-admin"},
		ProposalID:     "prop-001",
		IdempotencyKey: "idem-001",
	})
	if err == nil {
		t.Fatal("expected first admin approve to fail")
	}
	if recordCreator.callCount != 1 {
		t.Fatalf("expected record creator to be called once on first attempt, got %d", recordCreator.callCount)
	}
	if repo.progressRecordID != "record-001" {
		t.Fatalf("expected progress record to be saved, got %q", repo.progressRecordID)
	}

	repo.completeErr = nil
	resp, err := svc.AdminApprove(context.Background(), AdminApproveRequest{
		Subject:        Subject{CompanyID: "company-001", MembershipID: "member-admin", UserID: "user-admin"},
		ProposalID:     "prop-001",
		IdempotencyKey: "idem-001",
	})
	if err != nil {
		t.Fatalf("expected retry to succeed, got %v", err)
	}
	if recordCreator.callCount != 1 {
		t.Fatalf("expected retry to reuse saved progress without creating a second record, got %d calls", recordCreator.callCount)
	}
	if resp.RecordID != "record-001" || resp.WorkflowInstanceID != "wf-001" {
		t.Fatalf("expected saved progress ids to be reused, got record=%q workflow=%q", resp.RecordID, resp.WorkflowInstanceID)
	}
}

// â”€â”€â”€ Process Controller validation tests â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func TestCreateProposal_MissingController(t *testing.T) {
	repo := &fakeRepository{}
	auth := &fakeAuthService{decision: authapp.DecisionAllow}
	svc := newTestService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, auth)

	_, err := svc.CreateProposal(context.Background(), CreateProposalRequest{
		Subject: Subject{CompanyID: "company-001", MembershipID: "member-001", UserID: "user-001"},
		TypeID:  "dt-001",
		// ProcessControllerMembershipID intentionally omitted
	})
	if err == nil {
		t.Fatal("expected error for missing process controller")
	}
	httpErr, ok := err.(*perr.HTTPError)
	if !ok || httpErr.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("expected 400, got %#v", err)
	}
}

func TestCreateProposal_ControllerIsSelf(t *testing.T) {
	repo := &fakeRepository{}
	auth := &fakeAuthService{decision: authapp.DecisionAllow}
	svc := newTestService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, auth)

	_, err := svc.CreateProposal(context.Background(), CreateProposalRequest{
		Subject:                       Subject{CompanyID: "company-001", MembershipID: "member-001", UserID: "user-001"},
		TypeID:                        "dt-001",
		ProcessControllerMembershipID: "member-001", // same as creator
	})
	if err == nil {
		t.Fatal("expected error for controller == creator")
	}
	httpErr, ok := err.(*perr.HTTPError)
	if !ok || httpErr.HTTPStatus != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %#v", err)
	}
}

func TestCreateProposal_ControllerIsSelf_AdminDoanhNghiep_Allowed(t *testing.T) {
	repo := &fakeRepository{}
	auth := &fakeAuthService{decision: authapp.DecisionAllow}
	mv := &fakeMembershipValidator{active: true, hasPerm: true, hasAdminRole: true}
	svc := NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, auth, mv, nil, noopMetrics{})

	resp, err := svc.CreateProposal(context.Background(), CreateProposalRequest{
		Subject:                       Subject{CompanyID: "company-001", MembershipID: "member-001", UserID: "user-001"},
		TypeID:                        "dt-irregular",
		ProcessControllerMembershipID: "member-001",
	})
	if err != nil {
		t.Fatalf("CreateProposal() error = %v", err)
	}
	if resp.ProcessControllerID != "member-001" {
		t.Fatalf("expected ProcessControllerID member-001, got %q", resp.ProcessControllerID)
	}
}

func TestCreateProposal_ControllerNoPermission(t *testing.T) {
	repo := &fakeRepository{}
	auth := &fakeAuthService{decision: authapp.DecisionAllow}
	mv := &fakeMembershipValidator{active: true, hasPerm: false}
	svc := NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, auth, mv, nil, noopMetrics{})

	_, err := svc.CreateProposal(context.Background(), CreateProposalRequest{
		Subject:                       Subject{CompanyID: "company-001", MembershipID: "member-001", UserID: "user-001"},
		TypeID:                        "dt-001",
		ProcessControllerMembershipID: "member-controller",
	})
	if err == nil {
		t.Fatal("expected permission error for controller without process_control permission")
	}
	httpErr, ok := err.(*perr.HTTPError)
	if !ok || httpErr.HTTPStatus != http.StatusForbidden {
		t.Fatalf("expected 403, got %#v", err)
	}
}

func TestCreateProposal_ControllerInactive(t *testing.T) {
	repo := &fakeRepository{}
	auth := &fakeAuthService{decision: authapp.DecisionAllow}
	mv := &fakeMembershipValidator{active: false, hasPerm: true}
	svc := NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, auth, mv, nil, noopMetrics{})

	_, err := svc.CreateProposal(context.Background(), CreateProposalRequest{
		Subject:                       Subject{CompanyID: "company-001", MembershipID: "member-001", UserID: "user-001"},
		TypeID:                        "dt-001",
		ProcessControllerMembershipID: "member-inactive",
	})
	if err == nil {
		t.Fatal("expected error for inactive controller")
	}
	httpErr, ok := err.(*perr.HTTPError)
	if !ok || httpErr.HTTPStatus != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %#v", err)
	}
}

func TestCreateProposal_ValidController_StoresID(t *testing.T) {
	repo := &fakeRepository{}
	auth := &fakeAuthService{decision: authapp.DecisionAllow}
	svc := newTestService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, auth)

	resp, err := svc.CreateProposal(context.Background(), CreateProposalRequest{
		Subject:                       Subject{CompanyID: "company-001", MembershipID: "member-001", UserID: "user-001"},
		TypeID:                        "dt-irregular",
		ProcessControllerMembershipID: "member-controller",
	})
	if err != nil {
		t.Fatalf("CreateProposal() error = %v", err)
	}
	if resp.ProcessControllerID != "member-controller" {
		t.Fatalf("expected ProcessControllerID 'member-controller', got %q", resp.ProcessControllerID)
	}
	if repo.insertCalls != 1 {
		t.Fatalf("expected one insert, got %d", repo.insertCalls)
	}
}

func TestAdminApprove_ByNonController_Returns403(t *testing.T) {
	repo := &fakeRepository{proposal: &ProposalDTO{
		ProposalID:          "prop-001",
		CompanyID:           "company-001",
		TypeID:              "dt-001",
		Status:              StatusPendingAdminApproval,
		ProcessControllerID: "member-controller", // different from requester
	}}
	auth := &fakeAuthService{decision: authapp.DecisionAllow}
	svc := newTestService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, auth)

	_, err := svc.AdminApprove(context.Background(), AdminApproveRequest{
		Subject:        Subject{CompanyID: "company-001", MembershipID: "member-other", UserID: "user-other"},
		ProposalID:     "prop-001",
		IdempotencyKey: "idem-001",
	})
	if err == nil {
		t.Fatal("expected 403 for non-controller")
	}
	httpErr, ok := err.(*perr.HTTPError)
	if !ok || httpErr.HTTPStatus != http.StatusForbidden {
		t.Fatalf("expected 403, got %#v", err)
	}
}

// â”€â”€â”€ ADR-1B / CF-01: deterministic RecordID + idempotent replay under concurrency â”€â”€

// concurrencyFakeRecordCreator models the production
// MySQL-unique-constraint-on-record_id + ErrDuplicateRecordID idempotent-replay
// semantics implemented in RecordCreatorAdapter (internal/adhoc/infra/disclosure
// /record_creator.go): the FIRST concurrent caller to arrive with a given
// deterministic RecordID "creates" the underlying disclosure record + workflow
// instance; every other concurrent caller carrying the SAME RecordID is resolved
// to that SAME (recordID, workflowInstanceID) pair instead of inserting a second,
// orphaned record/workflow instance.
type concurrencyFakeRecordCreator struct {
	mu          sync.Mutex
	created     map[string]string // deterministic RecordID -> workflowInstanceID
	createCalls int
	wfSeq       int
}

func newConcurrencyFakeRecordCreator() *concurrencyFakeRecordCreator {
	return &concurrencyFakeRecordCreator{created: map[string]string{}}
}

func (f *concurrencyFakeRecordCreator) CreateAndSubmitRecord(ctx context.Context, companyID, typeID, createdByMembershipID, title string, t0Date *time.Time) (string, string, error) {
	return f.CreateAndSubmitRecordWithOpts(ctx, companyID, typeID, createdByMembershipID, title, t0Date, CreateRecordOpts{})
}

func (f *concurrencyFakeRecordCreator) CreateAndSubmitRecordWithOpts(ctx context.Context, companyID, typeID, createdByMembershipID, title string, t0Date *time.Time, opts CreateRecordOpts) (string, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if wfID, exists := f.created[opts.RecordID]; exists {
		// ADR-1B idempotent replay: a concurrent winner already created this
		// deterministic RecordID â€” return its existing linked workflow instance
		// rather than creating a second (orphaned) one.
		return opts.RecordID, wfID, nil
	}
	f.createCalls++
	f.wfSeq++
	wfID := fmt.Sprintf("wf-concurrent-%03d", f.wfSeq)
	f.created[opts.RecordID] = wfID
	return opts.RecordID, wfID, nil
}

// concurrencyFakeRepo is a thread-safe Repository fake for AdminApprove
// concurrency tests. It mirrors the production idempotency-key-aware reservation
// contract that ReserveAdminApproval/SaveAdminApprovalProgress/
// CompleteAdminApproval implement against real MySQL:
//   - once finalization has completed, later reservations observe
//     ReplayApproved=true and AdminApprove short-circuits to success without
//     touching the record creator again;
//   - before completion, a reservation surfaces whatever progress
//     (record_id/workflow_instance_id) the winning attempt already saved, so a
//     late-arriving concurrent attempt skips creation and reuses it;
//   - CompleteAdminApproval itself is idempotent â€” a retry after completion
//     returns the already-persisted result rather than erroring or double-writing.
//
// What it deliberately does NOT pre-serialize is the create-record step itself:
// concurrent attempts that all observe ProgressRecordID=="" before the winner
// saves its progress legitimately race into CreateAndSubmitRecordWithOpts â€” and
// it is exactly ADR-1B's deterministic RecordID + idempotent-replay-on-duplicate
// (modeled by concurrencyFakeRecordCreator) that must collapse that race to one
// record. That convergence is what TC-01/03/05 verify.
type concurrencyFakeRepo struct {
	mu                 sync.Mutex
	proposal           ProposalDTO
	progressRecordID   string
	progressWorkflowID string
	completed          bool
	reserveCalls       int
	completeCalls      int
}

func (f *concurrencyFakeRepo) Insert(ctx context.Context, p ProposalDTO) (*ProposalDTO, error) {
	cp := p
	return &cp, nil
}

func (f *concurrencyFakeRepo) FindByID(ctx context.Context, companyID, proposalID string) (*ProposalDTO, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := f.proposal
	return &cp, nil
}

func (f *concurrencyFakeRepo) UpdateStatus(ctx context.Context, upd StatusUpdate) (*ProposalDTO, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.proposal.Status = upd.Status
	cp := f.proposal
	return &cp, true, nil
}

func (f *concurrencyFakeRepo) ReserveAdminApproval(ctx context.Context, in ReserveAdminApprovalInput) (*AdminApprovalReservation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reserveCalls++
	cp := f.proposal
	if f.completed {
		// Idempotency-key replay: finalization already happened â€” every other
		// concurrent attempt for the same logical request must short-circuit to
		// success with the already-persisted ids, never re-run creation.
		return &AdminApprovalReservation{Proposal: &cp, ReplayApproved: true}, nil
	}
	return &AdminApprovalReservation{
		Proposal:           &cp,
		ProgressRecordID:   f.progressRecordID,
		ProgressWorkflowID: f.progressWorkflowID,
	}, nil
}

func (f *concurrencyFakeRepo) SaveAdminApprovalProgress(ctx context.Context, companyID, proposalID, idemKey, recordID, workflowID, lastError string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.progressRecordID = recordID
	f.progressWorkflowID = workflowID
	return nil
}

func (f *concurrencyFakeRepo) CompleteAdminApproval(ctx context.Context, upd StatusUpdate, idemKey string) (*ProposalDTO, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.completeCalls++
	if f.completed {
		// CompleteAdminApproval's idempotency-key-aware guard makes a retried
		// completion a no-op replay â€” return the already-persisted result with
		// applied=false so callers (and metrics emission) never double-count it.
		cp := f.proposal
		return &cp, false, nil
	}
	f.completed = true
	f.proposal.Status = upd.Status
	f.proposal.RecordID = upd.RecordID
	f.proposal.WorkflowInstanceID = upd.WorkflowInstanceID
	cp := f.proposal
	return &cp, true, nil
}

func (f *concurrencyFakeRepo) List(ctx context.Context, companyID string, statusFilter []string, page, pageSize int) ([]ProposalDTO, int, error) {
	return nil, 0, nil
}

// TestAdminApprove_ConcurrentRetries_CreateExactlyOneRecordWithSameDeterministicID
// covers TC-01/TC-03/TC-05: N concurrent AdminApprove calls (modeling concurrent
// retries / double-submits by the same process controller) must converge on
// EXACTLY ONE created disclosure record sharing ONE deterministic RecordID â€” no
// orphan records, no duplicate workflow instances (CF-01, closed by ADR-1B's
// uuid.NewSHA1(namespace, companyID:proposalID) + idempotent-replay-on-duplicate).
func TestAdminApprove_ConcurrentRetries_CreateExactlyOneRecordWithSameDeterministicID(t *testing.T) {
	const companyID = "company-001"
	const proposalID = "prop-concurrent-001"

	repo := &concurrencyFakeRepo{proposal: ProposalDTO{
		ProposalID:          proposalID,
		CompanyID:           companyID,
		TypeID:              "dt-001",
		Status:              StatusPendingAdminApproval,
		ChangeNote:          "Concurrent approval race",
		CreatedBy:           "member-creator",
		ProcessControllerID: "member-admin",
	}}
	recordCreator := newConcurrencyFakeRecordCreator()
	auth := &fakeAuthService{decision: authapp.DecisionAllow}
	svc := NewService(repo, recordCreator, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, auth, newAllowValidator(), nil, noopMetrics{})

	const attempts = 8
	var wg sync.WaitGroup
	respRecordIDs := make([]string, attempts)
	respWorkflowIDs := make([]string, attempts)
	errs := make([]error, attempts)

	wg.Add(attempts)
	for i := 0; i < attempts; i++ {
		i := i
		go func() {
			defer wg.Done()
			resp, err := svc.AdminApprove(context.Background(), AdminApproveRequest{
				Subject:        Subject{CompanyID: companyID, MembershipID: "member-admin", UserID: "user-admin"},
				ProposalID:     proposalID,
				IdempotencyKey: fmt.Sprintf("idem-concurrent-%d", i),
			})
			errs[i] = err
			if resp != nil {
				respRecordIDs[i] = resp.RecordID
				respWorkflowIDs[i] = resp.WorkflowInstanceID
			}
		}()
	}
	wg.Wait()

	expectedRecordID := uuid.NewSHA1(adhocRecordIDNamespace, []byte(fmt.Sprintf("%s:%s", companyID, proposalID))).String()

	for i := 0; i < attempts; i++ {
		if errs[i] != nil {
			t.Fatalf("attempt %d: AdminApprove returned error %v", i, errs[i])
		}
		if respRecordIDs[i] != expectedRecordID {
			t.Fatalf("attempt %d: RecordID = %q, want deterministic %q", i, respRecordIDs[i], expectedRecordID)
		}
	}

	// CF-01: exactly one underlying record/workflow instance must be created â€”
	// no orphans regardless of how many concurrent attempts raced to create it.
	recordCreator.mu.Lock()
	createCalls := recordCreator.createCalls
	createdCount := len(recordCreator.created)
	recordCreator.mu.Unlock()
	if createCalls != 1 {
		t.Fatalf("expected exactly one record-creation call across %d concurrent attempts, got %d (orphan risk)", attempts, createCalls)
	}
	if createdCount != 1 {
		t.Fatalf("expected exactly one distinct created record, got %d", createdCount)
	}

	// All N callers must observe the SAME workflow instance ID â€” proving the
	// idempotent-replay path returned the existing linkage rather than minting
	// a fresh (orphaned) workflow instance per retry.
	firstWFID := respWorkflowIDs[0]
	for i := 1; i < attempts; i++ {
		if respWorkflowIDs[i] != firstWFID {
			t.Fatalf("attempt %d: WorkflowInstanceID = %q, want %q (all concurrent callers must converge on the same linkage)", i, respWorkflowIDs[i], firstWFID)
		}
	}
}

// â”€â”€â”€ ADR-1B determinism across faulted retries (TC-02) â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

// faultInjectingRecordCreator fails the first call to
// CreateAndSubmitRecordWithOpts (simulating a transient failure such as a
// workflow-instance-creation network blip AFTER the underlying disclosure
// record row was already inserted), then succeeds on the next call. It records
// the deterministic RecordID it was handed on every call.
type faultInjectingRecordCreator struct {
	mu            sync.Mutex
	failNext      bool
	recordID      string
	workflowID    string
	seenRecordIDs []string
}

func (f *faultInjectingRecordCreator) CreateAndSubmitRecord(ctx context.Context, companyID, typeID, createdByMembershipID, title string, t0Date *time.Time) (string, string, error) {
	return f.CreateAndSubmitRecordWithOpts(ctx, companyID, typeID, createdByMembershipID, title, t0Date, CreateRecordOpts{})
}

func (f *faultInjectingRecordCreator) CreateAndSubmitRecordWithOpts(ctx context.Context, companyID, typeID, createdByMembershipID, title string, t0Date *time.Time, opts CreateRecordOpts) (string, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seenRecordIDs = append(f.seenRecordIDs, opts.RecordID)
	if f.failNext {
		f.failNext = false
		return "", "", errors.New("simulated transient failure creating workflow instance")
	}
	return f.recordID, f.workflowID, nil
}

// TestAdminApprove_RetryAfterTransientFailure_ComputesSameDeterministicRecordID
// covers TC-02: a fault-injected first attempt fails (no progress is persisted â€”
// SaveAdminApprovalProgress is only reached on success), and a second attempt by
// the same process controller must compute and pass the EXACT SAME deterministic
// RecordID (ADR-1B: uuid.NewSHA1(namespace, companyID:proposalID) is a pure
// function of (company, proposal) â€” not random per attempt). This is what lets
// the disclosure layer recognize a retry as a replay (CF-01) instead of minting
// a second, orphaned record when the first attempt's INSERT actually landed.
func TestAdminApprove_RetryAfterTransientFailure_ComputesSameDeterministicRecordID(t *testing.T) {
	const companyID = "company-001"
	const proposalID = "prop-faulted-001"

	repo := &fakeRepository{proposal: &ProposalDTO{
		ProposalID:          proposalID,
		CompanyID:           companyID,
		TypeID:              "dt-001",
		Status:              StatusPendingAdminApproval,
		ChangeNote:          "Faulted approval retry",
		CreatedBy:           "member-creator",
		ProcessControllerID: "member-admin",
	}}
	recordCreator := &faultInjectingRecordCreator{failNext: true, recordID: "record-faulted-001", workflowID: "wf-faulted-001"}
	auth := &fakeAuthService{decision: authapp.DecisionAllow}
	svc := NewService(repo, recordCreator, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, auth, newAllowValidator(), nil, noopMetrics{})

	req := AdminApproveRequest{
		Subject:        Subject{CompanyID: companyID, MembershipID: "member-admin", UserID: "user-admin"},
		ProposalID:     proposalID,
		IdempotencyKey: "idem-faulted-001",
	}

	if _, err := svc.AdminApprove(context.Background(), req); err == nil {
		t.Fatal("expected first (fault-injected) AdminApprove attempt to fail")
	}
	if repo.progressCalls != 0 {
		t.Fatalf("expected no progress to be saved after a failed creation attempt, got %d", repo.progressCalls)
	}

	resp, err := svc.AdminApprove(context.Background(), req)
	if err != nil {
		t.Fatalf("expected retry to succeed, got %v", err)
	}

	recordCreator.mu.Lock()
	seen := append([]string(nil), recordCreator.seenRecordIDs...)
	recordCreator.mu.Unlock()

	if len(seen) != 2 {
		t.Fatalf("expected exactly two creation attempts (fault + retry), got %d: %v", len(seen), seen)
	}
	expectedRecordID := uuid.NewSHA1(adhocRecordIDNamespace, []byte(fmt.Sprintf("%s:%s", companyID, proposalID))).String()
	if seen[0] == "" || seen[1] == "" {
		t.Fatalf("expected both attempts to carry a non-empty deterministic RecordID, got %v", seen)
	}
	if seen[0] != seen[1] {
		t.Fatalf("ADR-1B violation: retry computed a DIFFERENT RecordID (%q != %q) â€” this would make a successful first INSERT look like a fresh record on retry, creating an orphan", seen[0], seen[1])
	}
	if seen[0] != expectedRecordID {
		t.Fatalf("RecordID = %q, want deterministic uuid.NewSHA1(namespace, %q:%q) = %q", seen[0], companyID, proposalID, expectedRecordID)
	}
	if resp.RecordID != "record-faulted-001" {
		t.Fatalf("expected response RecordID from successful retry, got %q", resp.RecordID)
	}
}

// â”€â”€â”€ CF-15: AdminApprove must create the disclosure record as the proposal's â”€â”€
// â”€â”€â”€ original creator, not the approving process controller â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

// TestAdminApprove_CreatesRecordAsProposalCreator_NotApprovingController covers
// the CF-15 regression: CreateAndSubmitRecordWithOpts must receive
// cur.CreatedBy as createdByMembershipID â€” NOT req.Subject.MembershipID (the
// approving process controller's identity). Mixing these up would attribute
// authorship of the disclosure record to the wrong membership.
func TestAdminApprove_CreatesRecordAsProposalCreator_NotApprovingController(t *testing.T) {
	repo := &fakeRepository{proposal: &ProposalDTO{
		ProposalID:          "prop-cf15-001",
		CompanyID:           "company-001",
		TypeID:              "dt-001",
		Status:              StatusPendingAdminApproval,
		ChangeNote:          "CF-15 attribution check",
		CreatedBy:           "member-original-creator",
		ProcessControllerID: "member-admin-approver",
	}}
	recordCreator := &fakeRecordCreator{recordID: "record-cf15-001", workflowID: "wf-cf15-001"}
	auth := &fakeAuthService{decision: authapp.DecisionAllow}
	svc := newTestService(repo, recordCreator, &fakeTypeCatalog{category: "irregular"}, auth)

	_, err := svc.AdminApprove(context.Background(), AdminApproveRequest{
		Subject:        Subject{CompanyID: "company-001", MembershipID: "member-admin-approver", UserID: "user-admin-approver"},
		ProposalID:     "prop-cf15-001",
		IdempotencyKey: "idem-cf15-001",
	})
	if err != nil {
		t.Fatalf("AdminApprove() error = %v", err)
	}
	if recordCreator.lastCreatedBy != "member-original-creator" {
		t.Fatalf("CF-15: createdByMembershipID = %q, want proposal creator %q (must not be the approving controller %q)",
			recordCreator.lastCreatedBy, "member-original-creator", "member-admin-approver")
	}
}

// â”€â”€â”€ ADR-2: lost-update guard under concurrent transitions (TC-12) â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

// raceFakeRepo is a thread-safe Repository fake whose UpdateStatus mirrors the
// production `UPDATE ... SET status = ? WHERE status = ?` lost-update guard plus
// the RowsAffected decision tree (ADR-2): a transition only commits when the
// caller's FromStatus still matches the currently-persisted status; otherwise
// the caller is told the proposal moved out from under it via 409
// CodeStateConflict carrying current_status/expected_from_status/
// attempted_target_status/proposal_id â€” exactly the contract
// internal/adhoc/infra/mysql/repository.go's UpdateStatus implements against
// real MySQL via RowsAffected==0 + re-read.
//
// FindByID additionally rendezvouses exactly two concurrent readers before
// returning. Without this, goroutine scheduling could let one caller's entire
// FindByID-then-UpdateStatus sequence finish before the other even reads â€”
// which is a harmless sequential pair, not the lost-update race ADR-2 exists
// for. The gate deterministically reproduces the real-world TOCTOU window
// (SELECT status, ...; ...; UPDATE ... WHERE status = <stale value>) by forcing
// both readers to observe the SAME pre-write snapshot before either writes.
type raceFakeRepo struct {
	mu       sync.Mutex
	proposal ProposalDTO

	gateMu    sync.Mutex
	gateCount int
	gateCh    chan struct{}
}

func newRaceFakeRepo(p ProposalDTO) *raceFakeRepo {
	return &raceFakeRepo{proposal: p, gateCh: make(chan struct{})}
}

func (f *raceFakeRepo) Insert(ctx context.Context, p ProposalDTO) (*ProposalDTO, error) {
	cp := p
	return &cp, nil
}

func (f *raceFakeRepo) FindByID(ctx context.Context, companyID, proposalID string) (*ProposalDTO, error) {
	f.mu.Lock()
	cp := f.proposal
	f.mu.Unlock()

	f.gateMu.Lock()
	f.gateCount++
	if f.gateCount == 1 {
		ch := f.gateCh
		f.gateMu.Unlock()
		<-ch // block until the second concurrent reader arrives and observes the same snapshot
	} else {
		close(f.gateCh)
		f.gateMu.Unlock()
	}
	return &cp, nil
}

func (f *raceFakeRepo) UpdateStatus(ctx context.Context, upd StatusUpdate) (*ProposalDTO, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.proposal.Status != upd.FromStatus {
		conflictErr := perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "proposal status changed concurrently", nil)
		conflictErr.Details = map[string]any{
			"current_status":          f.proposal.Status,
			"expected_from_status":    upd.FromStatus,
			"attempted_target_status": upd.Status,
			"proposal_id":             upd.ProposalID,
		}
		return nil, false, conflictErr
	}
	f.proposal.Status = upd.Status
	if upd.SetFocalApprovalMetadata {
		now := time.Now().UTC()
		f.proposal.FocalApprovedBy = upd.ActorMembershipID
		f.proposal.FocalApprovedAt = &now
	}
	cp := f.proposal
	return &cp, true, nil
}

func (f *raceFakeRepo) ReserveAdminApproval(ctx context.Context, in ReserveAdminApprovalInput) (*AdminApprovalReservation, error) {
	return nil, errors.New("not used in TC-12")
}

func (f *raceFakeRepo) SaveAdminApprovalProgress(ctx context.Context, companyID, proposalID, idemKey, recordID, workflowID, lastError string) error {
	return errors.New("not used in TC-12")
}

func (f *raceFakeRepo) CompleteAdminApproval(ctx context.Context, upd StatusUpdate, idemKey string) (*ProposalDTO, bool, error) {
	return nil, false, errors.New("not used in TC-12")
}

func (f *raceFakeRepo) List(ctx context.Context, companyID string, statusFilter []string, page, pageSize int) ([]ProposalDTO, int, error) {
	return nil, 0, nil
}

// TestConcurrentCancelAndFocalApprove_OneWinsOneGets409Conflict covers TC-12: a
// proposal sitting in pending_focal_approval is simultaneously raced by its
// creator calling Cancel and a focal reviewer calling FocalApprove. Both read
// the same starting status (cur.Status == pending_focal_approval) before either
// writes â€” the classic lost-update window ADR-2 closes with `WHERE status = ?`.
// Exactly one transition must win; the other must observe the status changed
// out from under it and receive 409 CodeStateConflict (never silently overwrite
// or silently no-op).
func TestConcurrentCancelAndFocalApprove_OneWinsOneGets409Conflict(t *testing.T) {
	const companyID = "company-001"
	const proposalID = "prop-race-001"

	repo := newRaceFakeRepo(ProposalDTO{
		ProposalID:          proposalID,
		CompanyID:           companyID,
		TypeID:              "dt-001",
		Status:              StatusPendingFocalApproval,
		CreatedBy:           "member-creator",
		ProcessControllerID: "member-admin",
	})
	auth := &fakeAuthService{decision: authapp.DecisionAllow}
	svc := NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, auth, newAllowValidator(), nil, noopMetrics{})

	var wg sync.WaitGroup
	var cancelResp, focalResp *ProposalDTO
	var cancelErr, focalErr error

	wg.Add(2)
	go func() {
		defer wg.Done()
		cancelResp, cancelErr = svc.Cancel(context.Background(), ProposalActionRequest{
			Subject:    Subject{CompanyID: companyID, MembershipID: "member-creator", UserID: "user-creator"},
			ProposalID: proposalID,
		})
	}()
	go func() {
		defer wg.Done()
		focalResp, focalErr = svc.FocalApprove(context.Background(), ProposalActionRequest{
			Subject:    Subject{CompanyID: companyID, MembershipID: "member-focal", UserID: "user-focal"},
			ProposalID: proposalID,
		})
	}()
	wg.Wait()

	type outcome struct {
		name string
		resp *ProposalDTO
		err  error
	}
	outcomes := []outcome{
		{name: "Cancel", resp: cancelResp, err: cancelErr},
		{name: "FocalApprove", resp: focalResp, err: focalErr},
	}

	successes, conflicts := 0, 0
	for _, o := range outcomes {
		switch {
		case o.err == nil:
			successes++
			if o.resp == nil {
				t.Fatalf("%s: winner must return the updated proposal", o.name)
			}
		case o.err != nil:
			httpErr, ok := o.err.(*perr.HTTPError)
			if !ok || httpErr.HTTPStatus != http.StatusConflict || httpErr.Code != perr.CodeStateConflict {
				t.Fatalf("%s: loser must receive 409 CodeStateConflict, got %#v", o.name, o.err)
			}
			details := httpErr.Details
			for _, key := range []string{"current_status", "expected_from_status", "attempted_target_status", "proposal_id"} {
				if _, ok := details[key]; !ok {
					t.Fatalf("%s: 409 conflict details missing %q, got %#v", o.name, key, details)
				}
			}
			conflicts++
		}
	}

	if successes != 1 || conflicts != 1 {
		t.Fatalf("expected exactly one winner and one 409 conflict, got %d successes / %d conflicts (cancelErr=%v, focalErr=%v)", successes, conflicts, cancelErr, focalErr)
	}

	// The persisted status must reflect whichever transition actually won â€”
	// never left in a state neither caller intended (no silent corruption).
	repo.mu.Lock()
	finalStatus := repo.proposal.Status
	repo.mu.Unlock()
	if finalStatus != StatusCancelled && finalStatus != StatusPendingAdminApproval {
		t.Fatalf("final status = %q, want either %q (Cancel won) or %q (FocalApprove won)", finalStatus, StatusCancelled, StatusPendingAdminApproval)
	}
}

// ─── Batch 5(a): cobo_adhoc_proposal_transition_total emission correctness ───

// metricsCall captures one RecordTransition invocation for assertion.
type metricsCall struct {
	companyID  string
	fromStatus string
	toStatus   string
}

// spyMetrics is a concurrency-safe Metrics implementation that records every
// RecordTransition call it observes, for use in tests asserting both
// idempotent-replay correctness (exactly-once emission) and label correctness
// (company_id, from_status, to_status — and nothing else, per AK.3).
type spyMetrics struct {
	mu    sync.Mutex
	calls []metricsCall
}

func (s *spyMetrics) RecordTransition(companyID, fromStatus, toStatus string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, metricsCall{companyID: companyID, fromStatus: fromStatus, toStatus: toStatus})
}

func (s *spyMetrics) RecordEmailShadowOutcome(string, string) {}

func (s *spyMetrics) snapshot() []metricsCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]metricsCall, len(s.calls))
	copy(cp, s.calls)
	return cp
}

// TestAdminApprove_ConcurrentRetries_EmitsExactlyOneTransitionMetric covers the
// Batch 5(a) emission rule (replay correctness): cobo_adhoc_proposal_transition_total
// must be recorded only for the ONE concurrent AdminApprove attempt whose own
// guarded completion actually applied the pending_admin_approval -> approved
// transition (ADR-2 EV-1, applied=true). Every other concurrent retry observes
// an idempotent replay (EV-2, applied=false via concurrencyFakeRepo's
// first-caller-wins f.completed gate) and must NOT emit — double-emission would
// corrupt rate()-based alerting in Batch 6.
func TestAdminApprove_ConcurrentRetries_EmitsExactlyOneTransitionMetric(t *testing.T) {
	const companyID = "company-001"
	const proposalID = "prop-concurrent-metrics-001"

	repo := &concurrencyFakeRepo{proposal: ProposalDTO{
		ProposalID:          proposalID,
		CompanyID:           companyID,
		TypeID:              "dt-001",
		Status:              StatusPendingAdminApproval,
		ChangeNote:          "Concurrent metrics emission race",
		CreatedBy:           "member-creator",
		ProcessControllerID: "member-admin",
	}}
	recordCreator := newConcurrencyFakeRecordCreator()
	auth := &fakeAuthService{decision: authapp.DecisionAllow}
	spy := &spyMetrics{}
	svc := NewService(repo, recordCreator, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, auth, newAllowValidator(), nil, spy)

	const attempts = 8
	var wg sync.WaitGroup
	wg.Add(attempts)
	for i := 0; i < attempts; i++ {
		i := i
		go func() {
			defer wg.Done()
			_, err := svc.AdminApprove(context.Background(), AdminApproveRequest{
				Subject:        Subject{CompanyID: companyID, MembershipID: "member-admin", UserID: "user-admin"},
				ProposalID:     proposalID,
				IdempotencyKey: fmt.Sprintf("idem-metrics-%d", i),
			})
			if err != nil {
				t.Errorf("attempt %d: AdminApprove returned error %v", i, err)
			}
		}()
	}
	wg.Wait()

	calls := spy.snapshot()
	if len(calls) != 1 {
		t.Fatalf("expected exactly one transition metric across %d concurrent retries (no double-count of idempotent replays), got %d: %#v", attempts, len(calls), calls)
	}
	got := calls[0]
	if got.companyID != companyID || got.fromStatus != StatusPendingAdminApproval || got.toStatus != StatusApproved {
		t.Fatalf("unexpected transition metric labels: %#v", got)
	}
}

// TestProposalActions_EmitTransitionMetricWithExactLabels covers the Batch 5(a)
// label contract (AK.3): every one of the five lifecycle transitions must emit
// cobo_adhoc_proposal_transition_total exactly once, with exactly the labels
// (company_id, from_status, to_status) — using the service's own status
// constants as the source of truth, never proposal_id / membership_id /
// actor_id / user_id (those are explicitly forbidden as unbounded-cardinality
// label values that would blow up the metric's series count).
func TestProposalActions_EmitTransitionMetricWithExactLabels(t *testing.T) {
	const companyID = "company-001"
	const proposalID = "prop-001"

	tests := []struct {
		name        string
		startStatus string
		wantFrom    string
		wantTo      string
		run         func(t *testing.T, svc Service)
	}{
		{
			name:        "SubmitProposal: draft -> pending_focal_approval",
			startStatus: StatusDraft,
			wantFrom:    StatusDraft,
			wantTo:      StatusPendingFocalApproval,
			run: func(t *testing.T, svc Service) {
				if _, err := svc.SubmitProposal(context.Background(), ProposalActionRequest{
					Subject:    Subject{CompanyID: companyID, MembershipID: "member-creator", UserID: "user-creator"},
					ProposalID: proposalID,
				}); err != nil {
					t.Fatalf("SubmitProposal() error = %v", err)
				}
			},
		},
		{
			name:        "FocalApprove: pending_focal_approval -> pending_admin_approval",
			startStatus: StatusPendingFocalApproval,
			wantFrom:    StatusPendingFocalApproval,
			wantTo:      StatusPendingAdminApproval,
			run: func(t *testing.T, svc Service) {
				if _, err := svc.FocalApprove(context.Background(), ProposalActionRequest{
					Subject:    Subject{CompanyID: companyID, MembershipID: "member-focal", UserID: "user-focal"},
					ProposalID: proposalID,
				}); err != nil {
					t.Fatalf("FocalApprove() error = %v", err)
				}
			},
		},
		{
			name:        "AdminApprove: pending_admin_approval -> approved",
			startStatus: StatusPendingAdminApproval,
			wantFrom:    StatusPendingAdminApproval,
			wantTo:      StatusApproved,
			run: func(t *testing.T, svc Service) {
				if _, err := svc.AdminApprove(context.Background(), AdminApproveRequest{
					Subject:        Subject{CompanyID: companyID, MembershipID: "member-admin", UserID: "user-admin"},
					ProposalID:     proposalID,
					IdempotencyKey: "idem-label-admin-approve",
				}); err != nil {
					t.Fatalf("AdminApprove() error = %v", err)
				}
			},
		},
		{
			name:        "Reject: pending_focal_approval -> rejected",
			startStatus: StatusPendingFocalApproval,
			wantFrom:    StatusPendingFocalApproval,
			wantTo:      StatusRejected,
			run: func(t *testing.T, svc Service) {
				if _, err := svc.Reject(context.Background(), RejectRequest{
					Subject:      Subject{CompanyID: companyID, MembershipID: "member-focal", UserID: "user-focal"},
					ProposalID:   proposalID,
					RejectReason: "Not valid",
				}); err != nil {
					t.Fatalf("Reject() error = %v", err)
				}
			},
		},
		{
			name:        "Cancel: draft -> cancelled",
			startStatus: StatusDraft,
			wantFrom:    StatusDraft,
			wantTo:      StatusCancelled,
			run: func(t *testing.T, svc Service) {
				if _, err := svc.Cancel(context.Background(), ProposalActionRequest{
					Subject:    Subject{CompanyID: companyID, MembershipID: "member-creator", UserID: "user-creator"},
					ProposalID: proposalID,
				}); err != nil {
					t.Fatalf("Cancel() error = %v", err)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepository{proposal: &ProposalDTO{
				ProposalID:          proposalID,
				CompanyID:           companyID,
				TypeID:              "dt-001",
				Status:              tc.startStatus,
				CreatedBy:           "member-creator",
				ProcessControllerID: "member-admin",
			}}
			auth := &fakeAuthService{decision: authapp.DecisionAllow}
			spy := &spyMetrics{}
			svc := NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, auth, newAllowValidator(), nil, spy)

			tc.run(t, svc)

			calls := spy.snapshot()
			if len(calls) != 1 {
				t.Fatalf("%s: expected exactly one transition metric emission, got %d: %#v", tc.name, len(calls), calls)
			}
			got := calls[0]
			if got.companyID != companyID {
				t.Fatalf("%s: company_id label = %q, want %q", tc.name, got.companyID, companyID)
			}
			if got.fromStatus != tc.wantFrom {
				t.Fatalf("%s: from_status label = %q, want %q", tc.name, got.fromStatus, tc.wantFrom)
			}
			if got.toStatus != tc.wantTo {
				t.Fatalf("%s: to_status label = %q, want %q", tc.name, got.toStatus, tc.wantTo)
			}
			// Cardinality guard: none of the three labels may carry an identifier
			// value (proposal_id / membership_id / actor_id / user_id) — they must
			// be drawn exclusively from the bounded status-constant vocabulary.
			for _, label := range []string{got.companyID, got.fromStatus, got.toStatus} {
				if label == proposalID || label == "member-creator" || label == "member-focal" || label == "member-admin" || label == "user-creator" || label == "user-focal" || label == "user-admin" {
					t.Fatalf("%s: label value %q leaked an identifier — forbidden per AK.3 (proposal_id/membership_id/actor_id/user_id must never be label values)", tc.name, label)
				}
			}
		})
	}
}
