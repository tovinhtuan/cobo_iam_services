package app

import "testing"

func TestValidateFrozenProposalWorkflowForRuntime_DuplicateStepID(t *testing.T) {
	snap := frozenV2Snap([]ProposalWorkflowStep{
		{ID: "ps-1", Order: 1, Name: "A", ProcessingDays: 1, DepartmentID: "d1", AssigneeMembershipID: "m1"},
		{ID: "ps-1", Order: 2, Name: "B", ProcessingDays: 1, DepartmentID: "d1", AssigneeMembershipID: "m1"},
	})
	if err := ValidateFrozenProposalWorkflowForRuntime(snap); err == nil {
		t.Fatal("duplicate proposal step id must be rejected before materialization")
	}
}
