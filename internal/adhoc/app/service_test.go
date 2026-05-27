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
	if upd.Status == StatusPendingAdminApproval && upd.SetFocalApprovalMetadata {
		now := time.Now().UTC()
		cp.FocalApprovedBy = upd.ActorMembershipID
		cp.FocalApprovedAt = &now
	}
	return &cp, nil
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

func (f *fakeRepository) CompleteAdminApproval(ctx context.Context, upd StatusUpdate, idemKey string) (*ProposalDTO, error) {
	f.completeCalls++
	f.lastUpdate = upd
	if f.completeErr != nil {
		return nil, f.completeErr
	}
	cp := *f.proposal
	cp.Status = upd.Status
	cp.RecordID = upd.RecordID
	cp.WorkflowInstanceID = upd.WorkflowInstanceID
	cp.FinalT0Date = upd.FinalT0Date
	cp.FinalDeadlineDate = upd.FinalDeadlineDate
	cp.AdjustmentNote = upd.AdjustmentNote
	f.proposal = &cp
	return &cp, nil
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
	err           error
}

func (f *fakeRecordCreator) CreateAndSubmitRecord(ctx context.Context, companyID, typeID, createdByMembershipID, title string, t0Date *time.Time) (string, string, error) {
	return f.CreateAndSubmitRecordWithOpts(ctx, companyID, typeID, createdByMembershipID, title, t0Date, CreateRecordOpts{})
}

func (f *fakeRecordCreator) CreateAndSubmitRecordWithOpts(ctx context.Context, companyID, typeID, createdByMembershipID, title string, t0Date *time.Time, opts CreateRecordOpts) (string, string, error) {
	f.callCount++
	f.t0Date = t0Date
	f.lastTitle = title
	f.lastOverrides = append([]WorkflowStepOverride(nil), opts.StepOverrides...)
	if f.err != nil {
		return "", "", f.err
	}
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

func newAllowValidator() *fakeMembershipValidator { return &fakeMembershipValidator{active: true, hasPerm: true} }

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

func newTestService(repo *fakeRepository, recordCreator *fakeRecordCreator, typeCatalog *fakeTypeCatalog, auth *fakeAuthService) Service {
	return NewService(repo, recordCreator, typeCatalog, fakeIDGen{}, false, auth, newAllowValidator())
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
		Subject:                        Subject{CompanyID: "company-001", MembershipID: "member-001", UserID: "user-001"},
		TypeID:                         "dt-irregular",
		ProcessControllerMembershipID:  "member-controller",
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
	svc := NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, true, auth, nil)

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
	// No permission check should be called — gate is purely by identity.
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
		ChangeNote:          "Tiêu đề cảnh báo bất thường\nMô tả chi tiết không vào title",
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
	if recordCreator.lastTitle != "Tiêu đề cảnh báo bất thường" {
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

// ─── Process Controller validation tests ─────────────────────────────────────

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
	svc := NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, auth, mv)

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
	svc := NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, auth, mv)

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
	svc := NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, auth, mv)

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
