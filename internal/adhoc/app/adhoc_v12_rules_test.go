package app

import (
	"context"
	"errors"
	"net/http"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// MR-2 (bug-adhoc-v12-2026-10-16): ad-hoc rules of business contract v1.2 §3.2.

var (
	v12Creator   = Subject{CompanyID: "company-001", MembershipID: "member-creator", UserID: "user-creator"}
	v12ReviewerA = Subject{CompanyID: "company-001", MembershipID: "member-a", UserID: "user-a"}
	v12ReviewerB = Subject{CompanyID: "company-001", MembershipID: "member-b", UserID: "user-b"}
)

func v12Repo(status string, approvedBy ...string) *fakeRepository {
	repo := &fakeRepository{
		proposal: &ProposalDTO{
			ProposalID: "prop-001", CompanyID: "company-001", TypeID: "dt-001",
			Status: status, CreatedBy: v12Creator.MembershipID,
		},
		reviewers: []ReviewerDTO{{MembershipID: v12ReviewerA.MembershipID}, {MembershipID: v12ReviewerB.MembershipID}},
	}
	for _, id := range approvedBy {
		repo.approvals = append(repo.approvals, ApprovalDTO{MembershipID: id})
	}
	return repo
}

func assertAllReviewersApprovedConflict(t *testing.T, err error) {
	t.Helper()
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusConflict || he.Code != perr.CodeStateConflict || he.Details["reason"] != "all_reviewers_approved" {
		t.Fatalf("err=%v, want 409 %s with reason all_reviewers_approved", err, perr.CodeStateConflict)
	}
}

// H2: once every reviewer has approved, the proposal can no longer be rejected or cancelled.
func TestReject_RefusedOnceEveryReviewerApproved(t *testing.T) {
	repo := v12Repo(StatusPendingFocalApproval, v12ReviewerA.MembershipID, v12ReviewerB.MembershipID)
	svc := newTestService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, &fakeAuthService{decision: authapp.DecisionAllow})

	_, err := svc.Reject(context.Background(), RejectRequest{Subject: v12ReviewerA, ProposalID: "prop-001", RejectReason: "changed my mind"})
	assertAllReviewersApprovedConflict(t, err)
	if repo.proposal.Status != StatusPendingFocalApproval {
		t.Fatalf("status=%q, want pending_focal_approval", repo.proposal.Status)
	}
}

func TestCancel_RefusedOnceEveryReviewerApproved(t *testing.T) {
	repo := v12Repo(StatusPendingFocalApproval, v12ReviewerA.MembershipID, v12ReviewerB.MembershipID)
	svc := newTestService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, &fakeAuthService{decision: authapp.DecisionAllow})

	_, err := svc.Cancel(context.Background(), ProposalActionRequest{Subject: v12Creator, ProposalID: "prop-001"})
	assertAllReviewersApprovedConflict(t, err)
	if repo.proposal.Status != StatusPendingFocalApproval {
		t.Fatalf("status=%q, want pending_focal_approval", repo.proposal.Status)
	}
}

func TestRejectAndCancel_AllowedWhileAVoteIsOutstanding(t *testing.T) {
	for name, act := range map[string]func(Service) error{
		"reject": func(s Service) error {
			_, err := s.Reject(context.Background(), RejectRequest{Subject: v12ReviewerB, ProposalID: "prop-001", RejectReason: "not needed"})
			return err
		},
		"cancel": func(s Service) error {
			_, err := s.Cancel(context.Background(), ProposalActionRequest{Subject: v12Creator, ProposalID: "prop-001"})
			return err
		},
	} {
		repo := v12Repo(StatusPendingFocalApproval, v12ReviewerA.MembershipID)
		svc := newTestService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, &fakeAuthService{decision: authapp.DecisionAllow})
		if err := act(svc); err != nil {
			t.Fatalf("%s with one vote outstanding: %v", name, err)
		}
	}
}

// H3: a finalize that fails after the last vote was recorded releases that vote, so the proposal is
// not left with every vote in and nothing anyone can do (no approve button, no reject/cancel).
func TestApprove_FinalizeFailureReleasesTheLastVote(t *testing.T) {
	ctx := context.Background()
	repo := v12Repo(StatusPendingFocalApproval, v12ReviewerA.MembershipID)
	recordCreator := &fakeRecordCreator{recordID: "record-001", workflowID: "wf-001", err: errors.New("create workflow instance: boom")}
	svc := newTestService(repo, recordCreator, &fakeTypeCatalog{category: "irregular"}, &fakeAuthService{decision: authapp.DecisionAllow})

	if _, err := svc.Approve(ctx, ApproveRequest{Subject: v12ReviewerB, ProposalID: "prop-001"}); err == nil {
		t.Fatal("expected the finalize failure to surface")
	}
	if len(repo.approvals) != 1 || repo.approvals[0].MembershipID != v12ReviewerA.MembershipID {
		t.Fatalf("approvals=%+v after a failed finalize, want only reviewer A's vote", repo.approvals)
	}
	if repo.proposal.Status != StatusPendingFocalApproval {
		t.Fatalf("status=%q, want pending_focal_approval", repo.proposal.Status)
	}

	recordCreator.err = nil
	resp, err := svc.Approve(ctx, ApproveRequest{Subject: v12ReviewerB, ProposalID: "prop-001"})
	if err != nil || !resp.Finalized {
		t.Fatalf("approve again after the failure: resp=%+v err=%v, want finalized", resp, err)
	}
}

// H3: a finalize retry after a record was created without its workflow must resume the workflow,
// not approve the proposal with an empty workflow_instance_id.
func TestBuildCreateRecordOptsForFinalize_ResumesWorkflowOnDuplicate(t *testing.T) {
	for name, p := range map[string]*ProposalDTO{
		"legacy": {ProposalID: "p"},
		"v2":     {ProposalID: "p", Workflow: &ProposalWorkflowSnapshot{SchemaVersion: ProposalWorkflowSchemaV2}},
		"v3":     {ProposalID: "p", Workflow: &ProposalWorkflowSnapshot{SchemaVersion: ProposalWorkflowSchemaV3}},
	} {
		opts, _, err := BuildCreateRecordOptsForFinalize("record-1", p)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !opts.ResumeWorkflowOnDuplicate {
			t.Fatalf("%s: ResumeWorkflowOnDuplicate=false, a retried finalize would approve without a workflow", name)
		}
	}
}

// XL-06: submit re-checks the reviewers (active, focal_review) and the proposer's right to create
// the record that finalize creates on their behalf.
func TestSubmitProposal_RechecksReviewers(t *testing.T) {
	for name, mv := range map[string]*fakeMembershipValidator{
		"inactive reviewer":      {active: false, hasPerm: true},
		"reviewer lost approval": {active: true, hasPerm: false},
	} {
		repo := v12Repo(StatusDraft)
		svc := NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false,
			&fakeAuthService{decision: authapp.DecisionAllow}, mv, nil, noopMetrics{})
		_, err := svc.SubmitProposal(context.Background(), ProposalActionRequest{Subject: v12Creator, ProposalID: "prop-001"})
		he, ok := perr.AsHTTPError(err)
		if !ok || he.HTTPStatus != http.StatusUnprocessableEntity || he.Details["field"] != "reviewer_membership_ids" {
			t.Fatalf("%s: err=%v, want 422 on reviewer_membership_ids", name, err)
		}
		if repo.proposal.Status != StatusDraft {
			t.Fatalf("%s: status=%q, want ad_hoc_draft", name, repo.proposal.Status)
		}
	}
}

func TestSubmitProposal_RequiresProposerRecordCreatePermission(t *testing.T) {
	repo := v12Repo(StatusDraft)
	auth := &fakeAuthService{allowByAction: map[string]authapp.Decision{"ad_hoc_alert.propose": authapp.DecisionAllow}}
	svc := newTestService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, auth)

	_, err := svc.SubmitProposal(context.Background(), ProposalActionRequest{Subject: v12Creator, ProposalID: "prop-001"})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusForbidden || he.Details["permission"] != "disclosure.create" {
		t.Fatalf("err=%v, want 403 for disclosure.create", err)
	}
	if repo.proposal.Status != StatusDraft {
		t.Fatalf("status=%q, want ad_hoc_draft", repo.proposal.Status)
	}
}

// Review DB-01/BES-02: finalize runs under a per-proposal lock and re-reads the votes there. A vote
// released by a failed finalize meanwhile means there is nothing to finalize yet.
func TestApprove_RechecksVotesUnderTheFinalizeLock(t *testing.T) {
	repo := v12Repo(StatusPendingFocalApproval, v12ReviewerA.MembershipID)
	repo.onLockFinalize = func() { repo.approvals = repo.approvals[1:] } // A's vote released meanwhile
	recordCreator := &fakeRecordCreator{recordID: "record-001", workflowID: "wf-001"}
	svc := newTestService(repo, recordCreator, &fakeTypeCatalog{category: "irregular"}, &fakeAuthService{decision: authapp.DecisionAllow})

	resp, err := svc.Approve(context.Background(), ApproveRequest{Subject: v12ReviewerB, ProposalID: "prop-001"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Finalized || repo.lockFinalizeCalls != 1 || recordCreator.callCount != 0 {
		t.Fatalf("finalized=%v locks=%d record creations=%d, want no finalize once a vote is missing", resp.Finalized, repo.lockFinalizeCalls, recordCreator.callCount)
	}
}

// Review BES-01/DB-02: once finalize created its record, a failure keeps the votes; reject stays
// closed and a retry resumes the record instead of leaving it behind a rejection.
func TestApprove_FinalizeFailureAfterTheRecordKeepsTheVotes(t *testing.T) {
	ctx := context.Background()
	repo := v12Repo(StatusPendingFocalApproval, v12ReviewerA.MembershipID)
	recordID := AdHocRecordID("company-001", "prop-001")
	repo.existingRecords = map[string]bool{recordID: true}
	recordCreator := &fakeRecordCreator{err: errors.New("create workflow instance: boom")}
	svc := newTestService(repo, recordCreator, &fakeTypeCatalog{category: "irregular"}, &fakeAuthService{decision: authapp.DecisionAllow})

	if _, err := svc.Approve(ctx, ApproveRequest{Subject: v12ReviewerB, ProposalID: "prop-001"}); err == nil {
		t.Fatal("expected the finalize failure to surface")
	}
	if repo.lastReleaseRecordID != recordID || len(repo.approvals) != 2 {
		t.Fatalf("release record=%q approvals=%d, want the deterministic record checked and both votes kept", repo.lastReleaseRecordID, len(repo.approvals))
	}
	_, err := svc.Reject(ctx, RejectRequest{Subject: v12ReviewerA, ProposalID: "prop-001", RejectReason: "late"})
	assertAllReviewersApprovedConflict(t, err)
}

// Review DB-05: a proposed T0 read from a DATE column is RFC 3339 (parseTime); finalize must still
// pass it to the workflow when no reviewer entered a final T0.
func TestApprove_UsesTheProposedT0ReadAsRFC3339(t *testing.T) {
	repo := v12Repo(StatusPendingFocalApproval, v12ReviewerA.MembershipID)
	proposed := "2026-11-01T00:00:00Z"
	repo.proposal.ProposedT0Date = &proposed
	recordCreator := &fakeRecordCreator{recordID: "record-001", workflowID: "wf-001"}
	svc := newTestService(repo, recordCreator, &fakeTypeCatalog{category: "irregular"}, &fakeAuthService{decision: authapp.DecisionAllow})

	if _, err := svc.Approve(context.Background(), ApproveRequest{Subject: v12ReviewerB, ProposalID: "prop-001"}); err != nil {
		t.Fatal(err)
	}
	if recordCreator.t0Date == nil || recordCreator.t0Date.Format("2006-01-02") != "2026-11-01" {
		t.Fatalf("workflow T0=%v, want 2026-11-01", recordCreator.t0Date)
	}
}

// API-01: the server tells the client when a finalize retry is available (every vote in, pending).
func TestGetProposal_FlagsAFinalizeRetry(t *testing.T) {
	for name, tc := range map[string]struct {
		approvedBy []string
		want       bool
	}{
		"every vote in": {[]string{v12ReviewerA.MembershipID, v12ReviewerB.MembershipID}, true},
		"a vote out":    {[]string{v12ReviewerA.MembershipID}, false},
	} {
		repo := v12Repo(StatusPendingFocalApproval, tc.approvedBy...)
		svc := newTestService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, &fakeAuthService{decision: authapp.DecisionAllow})
		p, err := svc.GetProposal(context.Background(), GetProposalRequest{Subject: v12ReviewerA, ProposalID: "prop-001"})
		if err != nil {
			t.Fatal(err)
		}
		if p.FinalizeRetryAvailable != tc.want {
			t.Fatalf("%s: finalize_retry_available=%v, want %v", name, p.FinalizeRetryAvailable, tc.want)
		}
	}
}

// Review BES-05/DB-06: an infrastructure error while re-checking reviewers is not "invalid reviewer".
func TestSubmitProposal_ReviewerCheckFailureIsNotAReviewerError(t *testing.T) {
	repo := v12Repo(StatusDraft)
	mv := &fakeMembershipValidator{active: true, hasPerm: true, activeErr: errors.New("db down")}
	svc := NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false,
		&fakeAuthService{decision: authapp.DecisionAllow}, mv, nil, noopMetrics{})
	_, err := svc.SubmitProposal(context.Background(), ProposalActionRequest{Subject: v12Creator, ProposalID: "prop-001"})
	if he, ok := perr.AsHTTPError(err); ok && he.HTTPStatus == http.StatusUnprocessableEntity {
		t.Fatalf("err=%v, a failed check must not read as an invalid reviewer", err)
	}
	if err == nil {
		t.Fatal("expected the check failure to surface")
	}
}

// Review BES-03: a retried finalize reads the record back as the proposer, so submit also needs
// disclosure.view.
func TestSubmitProposal_RequiresProposerRecordViewPermission(t *testing.T) {
	repo := v12Repo(StatusDraft)
	auth := &fakeAuthService{allowByAction: map[string]authapp.Decision{
		"ad_hoc_alert.propose": authapp.DecisionAllow,
		"disclosure.create":    authapp.DecisionAllow,
	}}
	svc := newTestService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, auth)
	_, err := svc.SubmitProposal(context.Background(), ProposalActionRequest{Subject: v12Creator, ProposalID: "prop-001"})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusForbidden || he.Details["permission"] != "disclosure.view" {
		t.Fatalf("err=%v, want 403 for disclosure.view", err)
	}
}
