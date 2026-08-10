package app

import "testing"

func TestBuildProposalTracking_DraftNoRuntime_AllFuture(t *testing.T) {
	snap := &ProposalWorkflowSnapshot{
		SchemaVersion: ProposalWorkflowSchemaV3,
		Frozen:        false,
		Steps: []ProposalWorkflowStep{
			{ID: "s1", Order: 1, Name: "B1", DepartmentID: "d1", ProcessingDays: 2, AssigneeMembershipIDs: []string{"a", "b"}},
			{ID: "s2", Order: 2, Name: "B2", DepartmentID: "d2", ProcessingDays: 3, AssigneeMembershipIDs: []string{"c"}},
		},
	}
	got := BuildProposalTracking(StatusDraft, snap, false, "", "", nil)
	if got == nil {
		t.Fatal("expected tracking")
	}
	if got.HasRuntime || got.CurrentStep != nil || got.CompletedSteps != 0 || got.TotalSteps != 2 {
		t.Fatalf("unexpected draft tracking: %+v", got)
	}
	for _, step := range got.Steps {
		if step.Status != TrackingStepFuture {
			t.Fatalf("draft step %s status=%s want FUTURE", step.StepID, step.Status)
		}
		if step.Status == TrackingStepActive {
			t.Fatal("draft must not invent ACTIVE")
		}
		for _, a := range step.Assignees {
			if a.Source != TrackingAssigneeSourceFrozenPlan {
				t.Fatalf("future assignee source=%s", a.Source)
			}
		}
	}
}

func TestBuildProposalTracking_PendingNoRuntime_NoActive(t *testing.T) {
	snap := &ProposalWorkflowSnapshot{
		SchemaVersion: ProposalWorkflowSchemaV3,
		Frozen:        true,
		Steps: []ProposalWorkflowStep{
			{ID: "s1", Order: 1, Name: "B1", AssigneeMembershipIDs: []string{"a"}},
		},
	}
	got := BuildProposalTracking(StatusPendingFocalApproval, snap, false, "", "", nil)
	if got == nil || got.CurrentStep != nil || got.HasRuntime {
		t.Fatalf("pending must not show active runtime: %+v", got)
	}
	if got.Steps[0].Status != TrackingStepFuture {
		t.Fatalf("status=%s", got.Steps[0].Status)
	}
}

func TestBuildProposalTracking_ActiveV3_ShowsAllAssignees(t *testing.T) {
	snap := &ProposalWorkflowSnapshot{
		SchemaVersion: ProposalWorkflowSchemaV3,
		Frozen:        true,
		Steps: []ProposalWorkflowStep{
			{ID: "s1", Order: 1, Name: "B1", DepartmentID: "d1", ProcessingDays: 1, AssigneeMembershipIDs: []string{"plan-a"}},
			{ID: "s2", Order: 2, Name: "B2", DepartmentID: "d2", ProcessingDays: 2, AssigneeMembershipIDs: []string{"plan-b", "plan-c"}},
			{ID: "s3", Order: 3, Name: "B3", DepartmentID: "d3", ProcessingDays: 3, AssigneeMembershipIDs: []string{"plan-d"}},
		},
	}
	tasks := []RuntimeTaskView{
		{TaskID: "t1", StepCode: "s1", Status: "approved", AssigneeMembershipIDs: []string{"done-a"}},
		{TaskID: "t2", StepCode: "s2", Status: "pending", AssigneeMembershipIDs: []string{"m1", "m2", "m3"}},
	}
	got := BuildProposalTracking(StatusApproved, snap, true, "s2", "in_progress", tasks)
	if got == nil || !got.HasRuntime || got.CurrentStep == nil {
		t.Fatalf("expected active tracking: %+v", got)
	}
	if got.TotalSteps != 3 || got.CompletedSteps != 1 {
		t.Fatalf("progress total=%d completed=%d", got.TotalSteps, got.CompletedSteps)
	}
	if got.CurrentStep.StepID != "s2" || got.CurrentStep.Status != TrackingStepActive {
		t.Fatalf("current=%+v", got.CurrentStep)
	}
	if len(got.CurrentStep.Assignees) != 3 {
		t.Fatalf("expected all 3 runtime assignees, got %d", len(got.CurrentStep.Assignees))
	}
	for _, a := range got.CurrentStep.Assignees {
		if a.Source != TrackingAssigneeSourceRuntime {
			t.Fatalf("active assignee source=%s", a.Source)
		}
	}
	if got.Steps[0].Status != TrackingStepCompleted || got.Steps[2].Status != TrackingStepFuture {
		t.Fatalf("mapping=%v/%v/%v", got.Steps[0].Status, got.Steps[1].Status, got.Steps[2].Status)
	}
	if got.Steps[2].Assignees[0].MembershipID != "plan-d" || got.Steps[2].Assignees[0].Source != TrackingAssigneeSourceFrozenPlan {
		t.Fatalf("future must use frozen plan: %+v", got.Steps[2].Assignees)
	}
	if got.CurrentStepOrder == nil || *got.CurrentStepOrder != 2 {
		t.Fatalf("current_step_order=%v", got.CurrentStepOrder)
	}
}

func TestBuildProposalTracking_ActiveV2_Singleton(t *testing.T) {
	snap := &ProposalWorkflowSnapshot{
		SchemaVersion: ProposalWorkflowSchemaV2,
		Frozen:        true,
		Steps: []ProposalWorkflowStep{
			{ID: "s1", Order: 1, Name: "B1", AssigneeMembershipID: "only-one"},
		},
	}
	tasks := []RuntimeTaskView{
		{TaskID: "t1", StepCode: "s1", Status: "pending", AssigneeMembershipID: "only-one"},
	}
	got := BuildProposalTracking(StatusApproved, snap, true, "s1", "in_progress", tasks)
	if got == nil || got.CurrentStep == nil || len(got.CurrentStep.Assignees) != 1 {
		t.Fatalf("v2 singleton: %+v", got)
	}
	if got.CurrentStep.Assignees[0].MembershipID != "only-one" {
		t.Fatalf("assignee=%+v", got.CurrentStep.Assignees[0])
	}
}

func TestBuildProposalTracking_Completed_NoCurrentHandler(t *testing.T) {
	snap := &ProposalWorkflowSnapshot{
		SchemaVersion: ProposalWorkflowSchemaV3,
		Frozen:        true,
		Steps: []ProposalWorkflowStep{
			{ID: "s1", Order: 1, Name: "B1", AssigneeMembershipIDs: []string{"a"}},
			{ID: "s2", Order: 2, Name: "B2", AssigneeMembershipIDs: []string{"b"}},
		},
	}
	tasks := []RuntimeTaskView{
		{TaskID: "t1", StepCode: "s1", Status: "approved", AssigneeMembershipIDs: []string{"a"}},
		{TaskID: "t2", StepCode: "s2", Status: "approved", AssigneeMembershipIDs: []string{"b"}},
	}
	got := BuildProposalTracking(StatusApproved, snap, true, "s2", "approved", tasks)
	if got == nil || got.CurrentStep != nil {
		t.Fatalf("completed must clear current: %+v", got)
	}
	if got.CompletedSteps != 2 {
		t.Fatalf("completed=%d", got.CompletedSteps)
	}
	for _, step := range got.Steps {
		if step.Status != TrackingStepCompleted {
			t.Fatalf("step %s=%s", step.StepID, step.Status)
		}
	}
}

func TestBuildProposalTracking_NilSnap(t *testing.T) {
	if BuildProposalTracking(StatusApproved, nil, true, "s1", "in_progress", nil) != nil {
		t.Fatal("nil snap → nil tracking")
	}
}
