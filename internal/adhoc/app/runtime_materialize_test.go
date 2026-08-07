package app

import (
	"context"
	"net/http"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func frozenV2Snap(steps []ProposalWorkflowStep) *ProposalWorkflowSnapshot {
	return &ProposalWorkflowSnapshot{
		SchemaVersion:    ProposalWorkflowSchemaV2,
		DisclosureTypeID: "type-1",
		Frozen:           true,
		Steps:            steps,
	}
}

func TestValidateFrozenProposalWorkflowForRuntime(t *testing.T) {
	ok := frozenV2Snap([]ProposalWorkflowStep{
		{ID: "ps-1", Order: 1, Name: "A", ProcessingDays: 1, DepartmentID: "d1", AssigneeMembershipID: "m1"},
	})
	if err := ValidateFrozenProposalWorkflowForRuntime(ok); err != nil {
		t.Fatal(err)
	}

	unfrozen := *ok
	unfrozen.Frozen = false
	if err := ValidateFrozenProposalWorkflowForRuntime(&unfrozen); err == nil {
		t.Fatal("expected unfrozen reject")
	}

	badOrder := frozenV2Snap([]ProposalWorkflowStep{
		{ID: "ps-1", Order: 2, Name: "A", ProcessingDays: 0},
	})
	if err := ValidateFrozenProposalWorkflowForRuntime(badOrder); err == nil {
		t.Fatal("expected order reject")
	}

	emptyID := frozenV2Snap([]ProposalWorkflowStep{
		{ID: "", Order: 1, Name: "A", ProcessingDays: 0},
	})
	if err := ValidateFrozenProposalWorkflowForRuntime(emptyID); err == nil {
		t.Fatal("expected id reject")
	}
}

func TestValidateDirectAssigneeRequired(t *testing.T) {
	err := ValidateDirectAssigneeRequired(frozenV2Snap([]ProposalWorkflowStep{
		{ID: "ps-1", Order: 1, Name: "A", ProcessingDays: 1, DepartmentID: "d1"},
	}))
	if err == nil {
		t.Fatal("expected assignee required")
	}
	err = ValidateDirectAssigneeRequired(frozenV2Snap([]ProposalWorkflowStep{
		{ID: "ps-1", Order: 1, Name: "A", ProcessingDays: 1, AssigneeMembershipID: "m1"},
	}))
	if err == nil {
		t.Fatal("expected department required")
	}
	if err := ValidateDirectAssigneeRequired(frozenV2Snap([]ProposalWorkflowStep{
		{ID: "ps-1", Order: 1, Name: "A", ProcessingDays: 1, DepartmentID: "d1", AssigneeMembershipID: "m1"},
	})); err != nil {
		t.Fatal(err)
	}
}

func TestBuildCreateRecordOptsForFinalize_routesV2VsLegacy(t *testing.T) {
	v2 := &ProposalDTO{
		Workflow: frozenV2Snap([]ProposalWorkflowStep{
			{ID: "ps-1", Order: 1, Name: "A", ProcessingDays: 2, DepartmentID: "d1", AssigneeMembershipID: "assignee-b"},
		}),
		StepOverrides: []WorkflowStepOverride{{StepID: "tpl", ProcessingDays: 9}},
	}
	opts, mode, err := BuildCreateRecordOptsForFinalize("rec-1", v2)
	if err != nil || mode != MaterializationModeV2Snapshot {
		t.Fatalf("mode=%s err=%v", mode, err)
	}
	if opts.ProposalWorkflow == nil || len(opts.StepOverrides) != 0 {
		t.Fatalf("v2 must not dual-send overrides %#v", opts)
	}

	legacy := &ProposalDTO{StepOverrides: []WorkflowStepOverride{{StepID: "s1", ProcessingDays: 3}}}
	opts, mode, err = BuildCreateRecordOptsForFinalize("rec-2", legacy)
	if err != nil || mode != MaterializationModeLegacy {
		t.Fatalf("mode=%s err=%v", mode, err)
	}
	if opts.ProposalWorkflow != nil || len(opts.StepOverrides) != 1 {
		t.Fatalf("%#v", opts)
	}
}

func TestApprove_V2UsesFrozenSnapshot_NoCreatorFallback(t *testing.T) {
	repo := &fakeRepository{
		proposal: &ProposalDTO{
			ProposalID: "prop-v2",
			CompanyID:  "company-001",
			TypeID:     "dt-001",
			Status:     StatusPendingFocalApproval,
			CreatedBy:  "member-creator",
			Workflow: frozenV2Snap([]ProposalWorkflowStep{
				{
					ID: "ps-custom", SourceStepID: "", Order: 1, Name: "Custom X", ProcessingDays: 5,
					DepartmentID: "dep-1", AssigneeMembershipID: "member-assignee-b",
				},
				{
					ID: "ps-c", SourceStepID: "tpl-c", Order: 2, Name: "C", ProcessingDays: 1,
					DepartmentID: "dep-1", AssigneeMembershipID: "member-assignee-c",
				},
			}),
			StepOverrides: []WorkflowStepOverride{{StepID: "tpl-c", ProcessingDays: 1}},
		},
		reviewers: []ReviewerDTO{{MembershipID: "member-focal"}},
	}
	recordCreator := &fakeRecordCreator{recordID: "record-v2", workflowID: "wf-v2"}
	auth := &fakeAuthService{decision: authapp.DecisionAllow}
	base := newTestService(repo, recordCreator, &fakeTypeCatalog{category: "irregular"}, auth)
	svc := AttachWorkflowDeps(base, &fakeOrgDirectory{
		depts:   map[string]bool{"dep-1": true},
		members: map[string]bool{"member-assignee-b": true, "member-assignee-c": true},
		belong: map[string]bool{
			"member-assignee-b\x00dep-1": true,
			"member-assignee-c\x00dep-1": true,
		},
	}, nil)

	resp, err := svc.Approve(context.Background(), ApproveRequest{
		Subject:    Subject{CompanyID: "company-001", MembershipID: "member-focal", UserID: "user-focal"},
		ProposalID: "prop-v2",
	})
	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}
	if !resp.Finalized || resp.Proposal.Status != StatusApproved {
		t.Fatalf("finalize %#v", resp)
	}
	if recordCreator.lastOpts.ProposalWorkflow == nil || recordCreator.lastOpts.ProposalWorkflow.SchemaVersion != 2 {
		t.Fatalf("expected ProposalWorkflow on create opts %#v", recordCreator.lastOpts)
	}
	if len(recordCreator.lastOpts.StepOverrides) != 0 {
		t.Fatalf("must not dual-send step_overrides %#v", recordCreator.lastOpts.StepOverrides)
	}
	if FirstStepAssigneeMembershipID(recordCreator.lastOpts.ProposalWorkflow) != "member-assignee-b" {
		t.Fatalf("first assignee = %q", FirstStepAssigneeMembershipID(recordCreator.lastOpts.ProposalWorkflow))
	}
	if recordCreator.lastCreatedBy != "member-creator" {
		t.Fatalf("CF-15 record creator must remain proposal creator, got %q", recordCreator.lastCreatedBy)
	}
	// Approver must not be used as task assignee authority.
	if FirstStepAssigneeMembershipID(recordCreator.lastOpts.ProposalWorkflow) == "member-focal" {
		t.Fatal("approver must not be first-step assignee")
	}
}

func TestApprove_V2MissingAssignee_FailsWithoutMaterialize(t *testing.T) {
	repo := &fakeRepository{
		proposal: &ProposalDTO{
			ProposalID: "prop-v2-bad",
			CompanyID:  "company-001",
			TypeID:     "dt-001",
			Status:     StatusPendingFocalApproval,
			CreatedBy:  "member-creator",
			Workflow: frozenV2Snap([]ProposalWorkflowStep{
				{ID: "ps-1", Order: 1, Name: "A", ProcessingDays: 1, DepartmentID: "dep-1"},
			}),
		},
		reviewers: []ReviewerDTO{{MembershipID: "member-focal"}},
	}
	recordCreator := &fakeRecordCreator{recordID: "record-x", workflowID: "wf-x"}
	base := newTestService(repo, recordCreator, &fakeTypeCatalog{category: "irregular"}, &fakeAuthService{decision: authapp.DecisionAllow})
	svc := AttachWorkflowDeps(base, &fakeOrgDirectory{depts: map[string]bool{"dep-1": true}}, nil)

	_, err := svc.Approve(context.Background(), ApproveRequest{
		Subject:    Subject{CompanyID: "company-001", MembershipID: "member-focal", UserID: "user-focal"},
		ProposalID: "prop-v2-bad",
	})
	if err == nil {
		t.Fatal("expected materialize validation failure")
	}
	if recordCreator.callCount != 0 {
		t.Fatalf("must not materialize on invalid v2 assignment, calls=%d", recordCreator.callCount)
	}
	he, _ := perr.AsHTTPError(err)
	if he == nil || he.HTTPStatus != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %v", err)
	}
}

func TestApprove_V2InactiveAssignee_Fails(t *testing.T) {
	repo := &fakeRepository{
		proposal: &ProposalDTO{
			ProposalID: "prop-v2-inactive",
			CompanyID:  "company-001",
			TypeID:     "dt-001",
			Status:     StatusPendingFocalApproval,
			CreatedBy:  "member-creator",
			Workflow: frozenV2Snap([]ProposalWorkflowStep{
				{ID: "ps-1", Order: 1, Name: "A", ProcessingDays: 1, DepartmentID: "dep-1", AssigneeMembershipID: "mem-gone"},
			}),
		},
		reviewers: []ReviewerDTO{{MembershipID: "member-focal"}},
	}
	recordCreator := &fakeRecordCreator{recordID: "r", workflowID: "w"}
	base := newTestService(repo, recordCreator, &fakeTypeCatalog{category: "irregular"}, &fakeAuthService{decision: authapp.DecisionAllow})
	svc := AttachWorkflowDeps(base, &fakeOrgDirectory{
		depts:   map[string]bool{"dep-1": true},
		members: map[string]bool{}, // inactive / missing
		belong:  map[string]bool{},
	}, nil)

	_, err := svc.Approve(context.Background(), ApproveRequest{
		Subject:    Subject{CompanyID: "company-001", MembershipID: "member-focal", UserID: "user-focal"},
		ProposalID: "prop-v2-inactive",
	})
	if err == nil || recordCreator.callCount != 0 {
		t.Fatalf("expected inactive assignee fail without materialize: err=%v calls=%d", err, recordCreator.callCount)
	}
}

func TestApprove_LegacyStillSendsStepOverrides(t *testing.T) {
	repo := &fakeRepository{
		proposal: &ProposalDTO{
			ProposalID:    "prop-legacy",
			CompanyID:     "company-001",
			TypeID:        "dt-001",
			Status:        StatusPendingFocalApproval,
			CreatedBy:     "member-creator",
			StepOverrides: []WorkflowStepOverride{{StepID: "step-1", ProcessingDays: 7}},
		},
		reviewers: []ReviewerDTO{{MembershipID: "member-focal"}},
	}
	recordCreator := &fakeRecordCreator{recordID: "record-l", workflowID: "wf-l"}
	svc := newTestService(repo, recordCreator, &fakeTypeCatalog{category: "irregular"}, &fakeAuthService{decision: authapp.DecisionAllow})

	_, err := svc.Approve(context.Background(), ApproveRequest{
		Subject:    Subject{CompanyID: "company-001", MembershipID: "member-focal", UserID: "user-focal"},
		ProposalID: "prop-legacy",
	})
	if err != nil {
		t.Fatal(err)
	}
	if recordCreator.lastOpts.ProposalWorkflow != nil {
		t.Fatal("legacy must not set ProposalWorkflow")
	}
	if len(recordCreator.lastOverrides) != 1 || recordCreator.lastOverrides[0].StepID != "step-1" {
		t.Fatalf("%#v", recordCreator.lastOverrides)
	}
}

func TestPrepareV2Materialization_CrossCompanyRejected(t *testing.T) {
	snap := frozenV2Snap([]ProposalWorkflowStep{
		{ID: "ps-1", Order: 1, Name: "A", ProcessingDays: 1, DepartmentID: "foreign", AssigneeMembershipID: "m1"},
	})
	err := PrepareV2Materialization(context.Background(), &fakeOrgDirectory{
		depts: map[string]bool{}, members: map[string]bool{"m1": true},
	}, "co", snap)
	if err == nil {
		t.Fatal("expected reject")
	}
}
