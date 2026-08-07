package app

import (
	"context"
	"net/http"
	"strings"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type fakeSeeder struct {
	inputs []ProposalWorkflowStepInput
	err    error
}

func (f *fakeSeeder) SeedFromDisclosureType(context.Context, string, string) ([]ProposalWorkflowStepInput, error) {
	return f.inputs, f.err
}

func TestCreateProposal_V2WorkflowSteps(t *testing.T) {
	repo := &fakeRepository{}
	auth := &fakeAuthService{decision: authapp.DecisionAllow}
	svc := AttachWorkflowDeps(
		NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, auth, newAllowValidator(), nil, noopMetrics{}),
		&fakeOrgDirectory{
			depts:   map[string]bool{"dep-1": true},
			members: map[string]bool{"mem-1": true},
			belong:  map[string]bool{"mem-1\x00dep-1": true},
		},
		nil,
	)
	out, err := svc.CreateProposal(context.Background(), CreateProposalRequest{
		Subject:               Subject{UserID: "u", MembershipID: "creator", CompanyID: "co"},
		TypeID:                "type-1",
		ReviewerMembershipIDs: []string{"rev-1"},
		WorkflowSteps: []ProposalWorkflowStepInput{
			{Name: "Rà soát", ProcessingDays: 2, SourceStepID: "tpl-1", DepartmentID: "dep-1", AssigneeMembershipID: "mem-1"},
			{Name: "Phê duyệt", ProcessingDays: 1},
		},
	})
	if err != nil {
		t.Fatalf("CreateProposal: %v", err)
	}
	if out.Workflow == nil || out.Workflow.SchemaVersion != 2 || len(out.Workflow.Steps) != 2 {
		t.Fatalf("%#v", out.Workflow)
	}
	if out.Workflow.Steps[0].ID == "" || out.Workflow.Steps[0].Order != 1 {
		t.Fatalf("%#v", out.Workflow.Steps[0])
	}
	if out.Workflow.Steps[0].AssigneeMembershipID != "mem-1" {
		t.Fatalf("assignee %#v", out.Workflow.Steps[0])
	}
}

func TestCreateProposal_WorkflowStepsAndOverridesConflict(t *testing.T) {
	svc := NewService(&fakeRepository{}, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, &fakeAuthService{decision: authapp.DecisionAllow}, newAllowValidator(), nil, noopMetrics{})
	_, err := svc.CreateProposal(context.Background(), CreateProposalRequest{
		Subject:               Subject{UserID: "u", MembershipID: "creator", CompanyID: "co"},
		TypeID:                "type-1",
		ReviewerMembershipIDs: []string{"rev-1"},
		StepOverrides:         []WorkflowStepOverride{{StepID: "s1", ProcessingDays: 1}},
		WorkflowSteps:         []ProposalWorkflowStepInput{{Name: "A", ProcessingDays: 1}},
	})
	if err == nil || !strings.Contains(err.Error(), "workflow_contract_conflict") {
		t.Fatalf("got %v", err)
	}
}

func TestCreateProposal_LegacyOverridesStillWorks(t *testing.T) {
	svc := NewService(&fakeRepository{}, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, &fakeAuthService{decision: authapp.DecisionAllow}, newAllowValidator(), nil, noopMetrics{})
	out, err := svc.CreateProposal(context.Background(), CreateProposalRequest{
		Subject:               Subject{UserID: "u", MembershipID: "creator", CompanyID: "co"},
		TypeID:                "type-1",
		ReviewerMembershipIDs: []string{"rev-1"},
		StepOverrides:         []WorkflowStepOverride{{StepID: "s1", ProcessingDays: 3}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Workflow != nil {
		t.Fatalf("legacy must not set workflow snapshot: %#v", out.Workflow)
	}
	if len(out.StepOverrides) != 1 || out.StepOverrides[0].ProcessingDays != 3 {
		t.Fatalf("%#v", out.StepOverrides)
	}
}

func TestCreateProposal_UseTemplateWorkflowSeeds(t *testing.T) {
	repo := &fakeRepository{}
	seeder := &fakeSeeder{inputs: []ProposalWorkflowStepInput{
		{Name: "Seeded", ProcessingDays: 5, SourceStepID: "tpl-a", DepartmentID: "dep-1"},
	}}
	svc := AttachWorkflowDeps(
		NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, &fakeAuthService{decision: authapp.DecisionAllow}, newAllowValidator(), nil, noopMetrics{}),
		&fakeOrgDirectory{depts: map[string]bool{"dep-1": true}},
		seeder,
	)
	out, err := svc.CreateProposal(context.Background(), CreateProposalRequest{
		Subject:               Subject{UserID: "u", MembershipID: "creator", CompanyID: "co"},
		TypeID:                "type-1",
		ReviewerMembershipIDs: []string{"rev-1"},
		UseTemplateWorkflow:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Workflow == nil || out.Workflow.Steps[0].SourceStepID != "tpl-a" || out.Workflow.Steps[0].AssigneeMembershipID != "" {
		t.Fatalf("%#v", out.Workflow)
	}
}

func TestPatchDraftProposal_AddRemoveReorder(t *testing.T) {
	repo := &fakeRepository{proposal: &ProposalDTO{
		ProposalID: "p1", CompanyID: "co", TypeID: "type-1", Status: StatusDraft, CreatedBy: "creator",
		Workflow: &ProposalWorkflowSnapshot{
			SchemaVersion: 2, DisclosureTypeID: "type-1",
			Steps: []ProposalWorkflowStep{
				{ID: "ps-1", Order: 1, Name: "One", ProcessingDays: 1},
				{ID: "ps-2", Order: 2, Name: "Two", ProcessingDays: 2},
			},
		},
	}}
	svc := NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, &fakeAuthService{decision: authapp.DecisionAllow}, newAllowValidator(), nil, noopMetrics{})
	steps := []ProposalWorkflowStepInput{
		{ID: "ps-2", Name: "Two", ProcessingDays: 9},
		{Name: "Three", ProcessingDays: 3},
	}
	out, err := svc.PatchDraftProposal(context.Background(), PatchDraftProposalRequest{
		Subject:       Subject{UserID: "u", MembershipID: "creator", CompanyID: "co"},
		ProposalID:    "p1",
		WorkflowSteps: &steps,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Workflow.Steps) != 2 {
		t.Fatalf("%#v", out.Workflow.Steps)
	}
	if out.Workflow.Steps[0].ID != "ps-2" || out.Workflow.Steps[0].Order != 1 || out.Workflow.Steps[0].ProcessingDays != 9 {
		t.Fatalf("reorder %#v", out.Workflow.Steps[0])
	}
	if out.Workflow.Steps[1].Name != "Three" || out.Workflow.Steps[1].ID == "ps-1" {
		t.Fatalf("add %#v", out.Workflow.Steps[1])
	}
}

func TestPatchDraftProposal_RejectsSubmitted(t *testing.T) {
	repo := &fakeRepository{proposal: &ProposalDTO{
		ProposalID: "p1", CompanyID: "co", TypeID: "type-1", Status: StatusPendingFocalApproval, CreatedBy: "creator",
		Workflow: &ProposalWorkflowSnapshot{SchemaVersion: 2, Frozen: true, Steps: []ProposalWorkflowStep{{ID: "a", Order: 1, Name: "A", ProcessingDays: 1}}},
	}}
	svc := NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, &fakeAuthService{decision: authapp.DecisionAllow}, newAllowValidator(), nil, noopMetrics{})
	steps := []ProposalWorkflowStepInput{{Name: "X", ProcessingDays: 1}}
	_, err := svc.PatchDraftProposal(context.Background(), PatchDraftProposalRequest{
		Subject: Subject{UserID: "u", MembershipID: "creator", CompanyID: "co"}, ProposalID: "p1", WorkflowSteps: &steps,
	})
	if err == nil {
		t.Fatal("expected reject")
	}
	he, _ := perr.AsHTTPError(err)
	if he == nil || he.HTTPStatus != http.StatusConflict {
		t.Fatalf("%v", err)
	}
}

func TestPatchDraftProposal_RejectsNonCreator(t *testing.T) {
	repo := &fakeRepository{proposal: &ProposalDTO{
		ProposalID: "p1", CompanyID: "co", TypeID: "type-1", Status: StatusDraft, CreatedBy: "creator",
	}}
	svc := NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, &fakeAuthService{decision: authapp.DecisionAllow}, newAllowValidator(), nil, noopMetrics{})
	steps := []ProposalWorkflowStepInput{{Name: "X", ProcessingDays: 1}}
	_, err := svc.PatchDraftProposal(context.Background(), PatchDraftProposalRequest{
		Subject: Subject{UserID: "u", MembershipID: "other", CompanyID: "co"}, ProposalID: "p1", WorkflowSteps: &steps,
	})
	if err == nil {
		t.Fatal("expected forbid")
	}
}

func TestSubmitProposal_FreezesV2Snapshot_RuntimeNotSwitched(t *testing.T) {
	repo := &fakeRepository{
		proposal: &ProposalDTO{
			ProposalID: "p1", CompanyID: "co", TypeID: "type-1", Status: StatusDraft, CreatedBy: "creator",
			Workflow: &ProposalWorkflowSnapshot{
				SchemaVersion: 2, Frozen: false,
				Steps: []ProposalWorkflowStep{{ID: "ps-1", Order: 1, Name: "A", ProcessingDays: 1, SourceStepID: "tpl"}},
			},
			StepOverrides: []WorkflowStepOverride{{StepID: "tpl", ProcessingDays: 1}},
		},
		reviewers: []ReviewerDTO{{MembershipID: "rev-1"}},
	}
	svc := NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, &fakeAuthService{decision: authapp.DecisionAllow}, newAllowValidator(), nil, noopMetrics{})
	out, err := svc.SubmitProposal(context.Background(), ProposalActionRequest{
		Subject: Subject{UserID: "u", MembershipID: "creator", CompanyID: "co"}, ProposalID: "p1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != StatusPendingFocalApproval {
		t.Fatalf("status %s", out.Status)
	}
	if out.Workflow == nil || !out.Workflow.Frozen {
		t.Fatalf("expected frozen snapshot %#v", out.Workflow)
	}
	// Runtime consumption remains legacy path in finalize — StepOverrides still present for dual-read.
	if len(out.StepOverrides) == 0 {
		t.Fatal("legacy derived overrides should remain for dual-read")
	}
}

func TestCreateProposal_CrossCompanyDepartmentRejected(t *testing.T) {
	svc := AttachWorkflowDeps(
		NewService(&fakeRepository{}, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, &fakeAuthService{decision: authapp.DecisionAllow}, newAllowValidator(), nil, noopMetrics{}),
		&fakeOrgDirectory{depts: map[string]bool{}},
		nil,
	)
	_, err := svc.CreateProposal(context.Background(), CreateProposalRequest{
		Subject:               Subject{UserID: "u", MembershipID: "creator", CompanyID: "co"},
		TypeID:                "type-1",
		ReviewerMembershipIDs: []string{"rev-1"},
		WorkflowSteps:         []ProposalWorkflowStepInput{{Name: "A", ProcessingDays: 1, DepartmentID: "foreign-dep"}},
	})
	if err == nil {
		t.Fatal("expected reject")
	}
}

func TestCreateProposal_MembershipDepartmentMismatch(t *testing.T) {
	svc := AttachWorkflowDeps(
		NewService(&fakeRepository{}, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, &fakeAuthService{decision: authapp.DecisionAllow}, newAllowValidator(), nil, noopMetrics{}),
		&fakeOrgDirectory{
			depts:   map[string]bool{"dep-1": true},
			members: map[string]bool{"mem-1": true},
			belong:  map[string]bool{}, // not in department
		},
		nil,
	)
	_, err := svc.CreateProposal(context.Background(), CreateProposalRequest{
		Subject:               Subject{UserID: "u", MembershipID: "creator", CompanyID: "co"},
		TypeID:                "type-1",
		ReviewerMembershipIDs: []string{"rev-1"},
		WorkflowSteps: []ProposalWorkflowStepInput{
			{Name: "A", ProcessingDays: 1, DepartmentID: "dep-1", AssigneeMembershipID: "mem-1"},
		},
	})
	if err == nil {
		t.Fatal("expected mismatch reject")
	}
}

func TestPatchDraft_TypeChangeReseeds(t *testing.T) {
	repo := &fakeRepository{proposal: &ProposalDTO{
		ProposalID: "p1", CompanyID: "co", TypeID: "type-old", Status: StatusDraft, CreatedBy: "creator",
		Workflow: &ProposalWorkflowSnapshot{
			SchemaVersion: 2, DisclosureTypeID: "type-old",
			Steps: []ProposalWorkflowStep{{ID: "old-1", Order: 1, Name: "Old", ProcessingDays: 1, SourceStepID: "old-src"}},
		},
	}}
	newType := "type-new"
	svc := AttachWorkflowDeps(
		NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, &fakeAuthService{decision: authapp.DecisionAllow}, newAllowValidator(), nil, noopMetrics{}),
		nil,
		&fakeSeeder{inputs: []ProposalWorkflowStepInput{{Name: "NewSeed", ProcessingDays: 4, SourceStepID: "new-src"}}},
	)
	out, err := svc.PatchDraftProposal(context.Background(), PatchDraftProposalRequest{
		Subject:    Subject{UserID: "u", MembershipID: "creator", CompanyID: "co"},
		ProposalID: "p1",
		TypeID:     &newType,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.TypeID != "type-new" || out.Workflow.Steps[0].SourceStepID != "new-src" || out.Workflow.Steps[0].ID == "old-1" {
		t.Fatalf("expected reset seed %#v", out.Workflow)
	}
}
