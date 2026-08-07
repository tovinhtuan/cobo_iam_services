package disclosure

import (
	"context"
	"testing"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
)

// TestMapWorkflowSource_PassesThroughAllThreeValues is the regression guard for Architecture
// Integrity Fix A: a global_workflow-sourced ad-hoc record must NOT be recorded as
// global_template (mirrors the identical fix in internal/disclosure/infra/workflow/bootstrap.go).
func TestMapWorkflowSource_PassesThroughAllThreeValues(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"global_workflow", "global_workflow"},
		{"global_template", "global_template"},
		{"company_override", "company_override"},
		{"", "global_template"}, // defensive default only for an unexpected empty value
	}
	for _, c := range cases {
		if got := mapWorkflowSource(c.input); got != c.want {
			t.Errorf("mapWorkflowSource(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestResolveWorkflowSnapshotForMaterialize_V2SkipsEffectiveWorkflow(t *testing.T) {
	snap := &adhocapp.ProposalWorkflowSnapshot{
		SchemaVersion: 2,
		Frozen:        true,
		Steps: []adhocapp.ProposalWorkflowStep{
			{ID: "ps-1", Order: 1, Name: "A", ProcessingDays: 3, DepartmentID: "d1", AssigneeMembershipID: "assignee-b"},
			{ID: "ps-2", Order: 2, Name: "B", ProcessingDays: 1, DepartmentID: "d1", AssigneeMembershipID: "assignee-c"},
		},
	}
	// nil disclosure service is intentional: v2 path must not call GetEffectiveWorkflow.
	got, err := resolveWorkflowSnapshotForMaterialize(context.Background(), nil, disclosureapp.Subject{CompanyID: "co"}, "type-ignored", adhocapp.CreateRecordOpts{
		ProposalWorkflow: snap,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.mode != adhocapp.MaterializationModeV2Snapshot || got.effectiveWorkflowN != 0 {
		t.Fatalf("%#v", got)
	}
	if got.workflowSource != workflowapp.WorkflowSourceProposalSnapshotV2 {
		t.Fatalf("source=%q", got.workflowSource)
	}
	if len(got.snapshot) != 2 || got.snapshot[0].StepID != "ps-1" || got.snapshot[0].ProcessingDays != 3 {
		t.Fatalf("%#v", got.snapshot)
	}
	if got.firstTaskAssignee != "assignee-b" {
		t.Fatalf("first assignee=%q", got.firstTaskAssignee)
	}
}

func TestResolveWorkflowSnapshotForMaterialize_V2UnfrozenFails(t *testing.T) {
	snap := &adhocapp.ProposalWorkflowSnapshot{
		SchemaVersion: 2,
		Frozen:        false,
		Steps: []adhocapp.ProposalWorkflowStep{
			{ID: "ps-1", Order: 1, Name: "A", ProcessingDays: 1, DepartmentID: "d1", AssigneeMembershipID: "m1"},
		},
	}
	_, err := resolveWorkflowSnapshotForMaterialize(context.Background(), nil, disclosureapp.Subject{}, "t", adhocapp.CreateRecordOpts{ProposalWorkflow: snap})
	if err == nil {
		t.Fatal("expected fail")
	}
}

func TestResolveWorkflowSnapshotForMaterialize_V2RejectsDualOverrides(t *testing.T) {
	snap := &adhocapp.ProposalWorkflowSnapshot{
		SchemaVersion: 2,
		Frozen:        true,
		Steps: []adhocapp.ProposalWorkflowStep{
			{ID: "ps-1", Order: 1, Name: "A", ProcessingDays: 1, DepartmentID: "d1", AssigneeMembershipID: "m1"},
		},
	}
	_, err := resolveWorkflowSnapshotForMaterialize(context.Background(), nil, disclosureapp.Subject{}, "t", adhocapp.CreateRecordOpts{
		ProposalWorkflow: snap,
		StepOverrides:    []adhocapp.WorkflowStepOverride{{StepID: "x", ProcessingDays: 1}},
	})
	if err == nil {
		t.Fatal("expected conflict")
	}
}
