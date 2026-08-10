package app

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func TestNormalizeProposalWorkflowSteps_V3ArrayEmitsSchema3(t *testing.T) {
	ids := []string{"m1", "m2"}
	snap, err := NormalizeProposalWorkflowSteps("t", []ProposalWorkflowStepInput{
		{Name: "A", ProcessingDays: 1, DepartmentID: "d1", AssigneeMembershipIDs: &ids},
	}, nil, false, func() string { return "ps1" })
	if err != nil {
		t.Fatal(err)
	}
	if snap.SchemaVersion != ProposalWorkflowSchemaV3 {
		t.Fatalf("schema=%d", snap.SchemaVersion)
	}
	if len(snap.Steps[0].AssigneeMembershipIDs) != 2 || snap.Steps[0].AssigneeMembershipID != "" {
		t.Fatalf("%#v", snap.Steps[0])
	}
}

func TestNormalizeProposalWorkflowSteps_V3EmptyArrayDraftOK(t *testing.T) {
	empty := []string{}
	snap, err := NormalizeProposalWorkflowSteps("t", []ProposalWorkflowStepInput{
		{Name: "A", ProcessingDays: 1, DepartmentID: "d1", AssigneeMembershipIDs: &empty},
	}, nil, false, func() string { return "ps1" })
	if err != nil {
		t.Fatal(err)
	}
	if snap.SchemaVersion != ProposalWorkflowSchemaV3 {
		t.Fatalf("schema=%d", snap.SchemaVersion)
	}
	if len(snap.Steps[0].AssigneeMembershipIDs) != 0 {
		t.Fatalf("%#v", snap.Steps[0].AssigneeMembershipIDs)
	}
}

func TestNormalizeProposalWorkflowSteps_DualAuthorityConflict(t *testing.T) {
	ids := []string{"m1"}
	_, err := NormalizeProposalWorkflowSteps("t", []ProposalWorkflowStepInput{
		{Name: "A", ProcessingDays: 1, DepartmentID: "d1", AssigneeMembershipID: "m2", AssigneeMembershipIDs: &ids},
	}, nil, false, func() string { return "ps1" })
	if err == nil || !strings.Contains(err.Error(), "workflow_contract_conflict") {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestNormalizeProposalWorkflowSteps_DuplicateAssigneesRejected(t *testing.T) {
	ids := []string{"m1", "m1"}
	_, err := NormalizeProposalWorkflowSteps("t", []ProposalWorkflowStepInput{
		{Name: "A", ProcessingDays: 1, DepartmentID: "d1", AssigneeMembershipIDs: &ids},
	}, nil, false, func() string { return "ps1" })
	if err == nil || !strings.Contains(err.Error(), "duplicates") {
		t.Fatalf("expected duplicates error, got %v", err)
	}
}

func TestNormalizeAndValidateWorkflowForSubmitV3_EmptyResolvesHead(t *testing.T) {
	org := &fakeOrgDirectory{
		depts:   map[string]bool{"d1": true},
		members: map[string]bool{"head-1": true},
		belong:  map[string]bool{"head-1\x00d1": true},
		heads:   map[string]string{"d1": "head-1"},
	}
	snap := &ProposalWorkflowSnapshot{
		SchemaVersion: ProposalWorkflowSchemaV3,
		Steps: []ProposalWorkflowStep{
			{ID: "ps1", Order: 1, Name: "A", ProcessingDays: 1, DepartmentID: "d1"},
		},
	}
	got, err := NormalizeAndValidateWorkflowForSubmitV3(context.Background(), org, "co", snap)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Steps[0].AssigneeMembershipIDs) != 1 || got.Steps[0].AssigneeMembershipIDs[0] != "head-1" {
		t.Fatalf("%#v", got.Steps[0])
	}
	if got.Frozen {
		t.Fatal("normalize must not set frozen")
	}
}

func TestNormalizeAndValidateWorkflowForSubmitV3_HeadNotConfigured(t *testing.T) {
	org := &fakeOrgDirectory{
		depts: map[string]bool{"d1": true},
		heads: map[string]string{},
	}
	snap := &ProposalWorkflowSnapshot{
		SchemaVersion: ProposalWorkflowSchemaV3,
		Steps: []ProposalWorkflowStep{
			{ID: "ps1", Order: 1, Name: "A", ProcessingDays: 1, DepartmentID: "d1"},
		},
	}
	_, err := NormalizeAndValidateWorkflowForSubmitV3(context.Background(), org, "co", snap)
	if err == nil || !strings.Contains(err.Error(), "department_head_not_configured") {
		t.Fatalf("got %v", err)
	}
}

func TestNormalizeAndValidateWorkflowForSubmitV3_ExplicitPreserved(t *testing.T) {
	org := &fakeOrgDirectory{
		depts:   map[string]bool{"d1": true},
		members: map[string]bool{"m1": true, "m2": true},
		belong:  map[string]bool{"m1\x00d1": true, "m2\x00d1": true},
		heads:   map[string]string{"d1": "should-not-use"},
	}
	snap := &ProposalWorkflowSnapshot{
		SchemaVersion: ProposalWorkflowSchemaV3,
		Steps: []ProposalWorkflowStep{
			{ID: "ps1", Order: 1, Name: "A", ProcessingDays: 2, DepartmentID: "d1", AssigneeMembershipIDs: []string{"m1", "m2"}},
		},
	}
	got, err := NormalizeAndValidateWorkflowForSubmitV3(context.Background(), org, "co", snap)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Steps[0].AssigneeMembershipIDs) != 2 {
		t.Fatalf("%v", got.Steps[0].AssigneeMembershipIDs)
	}
}

func TestNormalizeAndValidateWorkflowForSubmitV3_AtomicMultiStepHeadFail(t *testing.T) {
	org := &fakeOrgDirectory{
		depts:   map[string]bool{"d1": true, "d2": true},
		members: map[string]bool{"h1": true},
		belong:  map[string]bool{"h1\x00d1": true},
		heads:   map[string]string{"d1": "h1"},
	}
	snap := &ProposalWorkflowSnapshot{
		SchemaVersion: ProposalWorkflowSchemaV3,
		Steps: []ProposalWorkflowStep{
			{ID: "ps1", Order: 1, Name: "A", ProcessingDays: 1, DepartmentID: "d1"},
			{ID: "ps2", Order: 2, Name: "B", ProcessingDays: 1, DepartmentID: "d2"},
		},
	}
	_, err := NormalizeAndValidateWorkflowForSubmitV3(context.Background(), org, "co", snap)
	if err == nil {
		t.Fatal("expected fail")
	}
}

func TestBuildCreateRecordOptsForFinalize_V3Accepted(t *testing.T) {
	opts, mode, err := BuildCreateRecordOptsForFinalize("r1", &ProposalDTO{
		Workflow: &ProposalWorkflowSnapshot{SchemaVersion: ProposalWorkflowSchemaV3, Frozen: true, Steps: []ProposalWorkflowStep{
			{ID: "ps1", Order: 1, Name: "A", ProcessingDays: 1, DepartmentID: "d1", AssigneeMembershipIDs: []string{"m1"}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if mode != MaterializationModeV3Snapshot {
		t.Fatalf("mode=%q", mode)
	}
	if opts.ProposalWorkflow == nil || opts.ProposalWorkflow.SchemaVersion != ProposalWorkflowSchemaV3 {
		t.Fatalf("%#v", opts.ProposalWorkflow)
	}
}

func TestValidateFrozenProposalWorkflowForRuntime_AcceptsV3(t *testing.T) {
	err := ValidateFrozenProposalWorkflowForRuntime(&ProposalWorkflowSnapshot{
		SchemaVersion: ProposalWorkflowSchemaV3,
		Frozen:        true,
		Steps:         []ProposalWorkflowStep{{ID: "ps1", Order: 1, Name: "A", ProcessingDays: 1}},
	})
	if err != nil {
		t.Fatalf("%v", err)
	}
}

func TestValidateV3AssigneesRequired(t *testing.T) {
	err := ValidateV3AssigneesRequired(&ProposalWorkflowSnapshot{
		SchemaVersion: ProposalWorkflowSchemaV3,
		Frozen:        true,
		Steps: []ProposalWorkflowStep{
			{ID: "ps1", Order: 1, Name: "A", ProcessingDays: 1, DepartmentID: "d1", AssigneeMembershipIDs: []string{"m1", "m2"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	err = ValidateV3AssigneesRequired(&ProposalWorkflowSnapshot{
		SchemaVersion: ProposalWorkflowSchemaV3,
		Frozen:        true,
		Steps: []ProposalWorkflowStep{
			{ID: "ps1", Order: 1, Name: "A", ProcessingDays: 1, DepartmentID: "d1"},
		},
	})
	if err == nil {
		t.Fatal("expected empty assignees rejected")
	}
}

func TestProposalWorkflowSnapshot_V3JSONRoundTrip(t *testing.T) {
	snap := &ProposalWorkflowSnapshot{
		SchemaVersion: ProposalWorkflowSchemaV3,
		Steps: []ProposalWorkflowStep{{
			ID: "ps-1", Order: 1, Name: "A", ProcessingDays: 2,
			DepartmentID: "d1", AssigneeMembershipIDs: []string{"m1", "m2"},
		}},
	}
	raw, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "assignee_membership_ids") {
		t.Fatalf("missing array field: %s", raw)
	}
	var got ProposalWorkflowSnapshot
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != 3 || len(got.Steps[0].AssigneeMembershipIDs) != 2 {
		t.Fatalf("%#v", got)
	}
	if ResolveProposalWorkflowContractVersion(&got, nil) != ProposalWorkflowSchemaV3 {
		t.Fatal("version")
	}
}

func TestSubmitProposal_V3EmptyNormalizesToHeadAtomic(t *testing.T) {
	repo := &fakeRepository{
		proposal: &ProposalDTO{
			ProposalID:              "p1",
			CompanyID:               "co",
			Status:                  StatusDraft,
			TypeID:                  "type-1",
			CreatedBy:               "creator",
			ProposedDeadlineDayType: func() *ProposalDeadlineDayType { v := ProposalDeadlineDayTypeCalendarDays; return &v }(),
			Workflow: &ProposalWorkflowSnapshot{
				SchemaVersion: ProposalWorkflowSchemaV3, Frozen: false,
				Steps: []ProposalWorkflowStep{
					{ID: "ps1", Order: 1, Name: "A", ProcessingDays: 1, DepartmentID: "dep-1"},
				},
			},
		},
		reviewers: []ReviewerDTO{{MembershipID: "r1"}},
	}
	org := &fakeOrgDirectory{
		depts:   map[string]bool{"dep-1": true},
		members: map[string]bool{"head-1": true},
		belong:  map[string]bool{"head-1\x00dep-1": true},
		heads:   map[string]string{"dep-1": "head-1"},
	}
	svc := AttachWorkflowDeps(
		NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, &fakeAuthService{decision: authapp.DecisionAllow}, newAllowValidator(), nil, noopMetrics{}),
		org, nil,
	)
	out, err := svc.SubmitProposal(context.Background(), ProposalActionRequest{
		Subject: Subject{UserID: "u", MembershipID: "creator", CompanyID: "co"}, ProposalID: "p1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Workflow == nil || !out.Workflow.Frozen || out.Workflow.SchemaVersion != 3 {
		t.Fatalf("%#v", out.Workflow)
	}
	if len(out.Workflow.Steps[0].AssigneeMembershipIDs) != 1 || out.Workflow.Steps[0].AssigneeMembershipIDs[0] != "head-1" {
		t.Fatalf("%#v", out.Workflow.Steps[0])
	}
}

func TestSubmitProposal_V3HeadMissingNoPartialFreeze(t *testing.T) {
	repo := &fakeRepository{
		proposal: &ProposalDTO{
			ProposalID:              "p1",
			CompanyID:               "co",
			Status:                  StatusDraft,
			TypeID:                  "type-1",
			CreatedBy:               "creator",
			ProposedDeadlineDayType: func() *ProposalDeadlineDayType { v := ProposalDeadlineDayTypeCalendarDays; return &v }(),
			Workflow: &ProposalWorkflowSnapshot{
				SchemaVersion: ProposalWorkflowSchemaV3, Frozen: false,
				Steps: []ProposalWorkflowStep{
					{ID: "ps1", Order: 1, Name: "A", ProcessingDays: 1, DepartmentID: "dep-1"},
				},
			},
		},
		reviewers: []ReviewerDTO{{MembershipID: "r1"}},
	}
	org := &fakeOrgDirectory{depts: map[string]bool{"dep-1": true}, heads: map[string]string{}}
	svc := AttachWorkflowDeps(
		NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, &fakeAuthService{decision: authapp.DecisionAllow}, newAllowValidator(), nil, noopMetrics{}),
		org, nil,
	)
	_, err := svc.SubmitProposal(context.Background(), ProposalActionRequest{
		Subject: Subject{UserID: "u", MembershipID: "creator", CompanyID: "co"}, ProposalID: "p1",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if repo.proposal.Status != StatusDraft || repo.proposal.Workflow.Frozen {
		t.Fatalf("partial submit leaked: status=%s frozen=%v", repo.proposal.Status, repo.proposal.Workflow.Frozen)
	}
	if repo.updateCalls != 0 {
		t.Fatalf("UpdateStatus must not run, calls=%d", repo.updateCalls)
	}
}

func TestCreateProposal_V3WorkflowSteps(t *testing.T) {
	repo := &fakeRepository{}
	ids := []string{"mem-1", "mem-2"}
	org := &fakeOrgDirectory{
		depts:   map[string]bool{"dep-1": true},
		members: map[string]bool{"mem-1": true, "mem-2": true},
		belong:  map[string]bool{"mem-1\x00dep-1": true, "mem-2\x00dep-1": true},
	}
	svc := AttachWorkflowDeps(
		NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, &fakeAuthService{decision: authapp.DecisionAllow}, newAllowValidator(), nil, noopMetrics{}),
		org, nil,
	)
	out, err := svc.CreateProposal(context.Background(), CreateProposalRequest{
		Subject:               Subject{UserID: "u", MembershipID: "creator", CompanyID: "co"},
		TypeID:                "type-1",
		ReviewerMembershipIDs: []string{"rev-1"},
		WorkflowSteps: []ProposalWorkflowStepInput{
			{Name: "Rà soát", ProcessingDays: 2, DepartmentID: "dep-1", AssigneeMembershipIDs: &ids},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Workflow == nil || out.Workflow.SchemaVersion != 3 {
		t.Fatalf("%#v", out.Workflow)
	}
	if len(out.Workflow.Steps[0].AssigneeMembershipIDs) != 2 {
		t.Fatalf("%v", out.Workflow.Steps[0].AssigneeMembershipIDs)
	}
}

func TestEffectiveAssigneeMembershipIDs_LegacySingular(t *testing.T) {
	ids := EffectiveAssigneeMembershipIDs(ProposalWorkflowStep{AssigneeMembershipID: "m1"}, ProposalWorkflowSchemaV2)
	if len(ids) != 1 || ids[0] != "m1" {
		t.Fatalf("%v", ids)
	}
	ids = EffectiveAssigneeMembershipIDs(ProposalWorkflowStep{AssigneeMembershipID: "m1"}, ProposalWorkflowSchemaV3)
	if len(ids) != 0 {
		t.Fatalf("v3 must not fall back to singular: %v", ids)
	}
}

func TestValidateWorkflowStepOrgRefs_MultiAssignees(t *testing.T) {
	org := &fakeOrgDirectory{
		depts:   map[string]bool{"dep-ok": true},
		members: map[string]bool{"m1": true, "m2": true},
		belong:  map[string]bool{"m1\x00dep-ok": true, "m2\x00dep-ok": true},
	}
	steps := []ProposalWorkflowStep{{
		ID: "1", Order: 1, Name: "A", ProcessingDays: 1,
		DepartmentID: "dep-ok", AssigneeMembershipIDs: []string{"m1", "m2"},
	}}
	if err := ValidateWorkflowStepOrgRefs(context.Background(), org, "co", steps); err != nil {
		t.Fatal(err)
	}
	steps[0].AssigneeMembershipIDs = []string{"m1", "foreign"}
	org.members["foreign"] = false
	err := ValidateWorkflowStepOrgRefs(context.Background(), org, "co", steps)
	if err == nil {
		t.Fatal("expected reject")
	}
	if he, ok := perr.AsHTTPError(err); !ok || he.HTTPStatus != http.StatusUnprocessableEntity {
		t.Fatalf("%v", err)
	}
}
