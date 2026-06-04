package app_test

import (
	"context"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

var testSubjectWF = disclosureapp.Subject{
	UserID:       "user-cms",
	MembershipID: "m-001",
	CompanyID:    "c-001",
}

func newWFService() disclosureapp.Service {
	repo := inmemory.NewRepository()
	return disclosureapp.NewService(repo, nil, idgen.UUIDv7Generator{})
}

// TestUpsertGlobalWorkflow_SecondUpsertWithSameStepIdSucceeds verifies that
// calling upsert twice with the same explicit step_id values does not error.
// This is the exact scenario that caused "Duplicate entry" in MySQL before the fix.
func TestUpsertGlobalWorkflow_SecondUpsertWithSameStepIdSucceeds(t *testing.T) {
	ctx := context.Background()
	svc := newWFService()
	const typeID = "dt-collision-test"

	steps := []disclosureapp.GlobalWorkflowStepInput{
		{StepID: "step-review", Stage: "Review", DepartmentID: "d1", AssigneeRoleIds: []string{"r1"}, ProcessingDays: 3, DueRule: "T+3", DisplayOrder: 1},
		{StepID: "step-approve", Stage: "Approve", DepartmentID: "d2", AssigneeRoleIds: []string{"r2"}, ProcessingDays: 2, DueRule: "T+5", DisplayOrder: 2},
	}

	// First upsert.
	wf1, err := svc.CmsUpsertGlobalWorkflow(ctx, disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID, ChangeNote: "v1", Steps: steps,
	})
	if err != nil {
		t.Fatalf("first upsert failed: %v", err)
	}
	if len(wf1.Steps) != 2 {
		t.Fatalf("first upsert steps=%d want 2", len(wf1.Steps))
	}

	// Second upsert with the same step_ids — must NOT return duplicate key error.
	wf2, err := svc.CmsUpsertGlobalWorkflow(ctx, disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID, ChangeNote: "v2", Steps: steps,
	})
	if err != nil {
		t.Fatalf("second upsert with same step_ids failed (collision!): %v", err)
	}
	if len(wf2.Steps) != 2 {
		t.Fatalf("second upsert steps=%d want 2", len(wf2.Steps))
	}
}

// TestUpsertGlobalWorkflow_OnlyOneActiveWorkflowExists verifies that after N upserts,
// CountGlobalWorkflowsByTypeId returns 1 (not N).
func TestUpsertGlobalWorkflow_OnlyOneActiveWorkflowExists(t *testing.T) {
	ctx := context.Background()
	repo := inmemory.NewRepository()
	svc := disclosureapp.NewService(repo, nil, idgen.UUIDv7Generator{})
	const typeID = "dt-count-test"

	step := []disclosureapp.GlobalWorkflowStepInput{
		{StepID: "s1", Stage: "Step 1", DepartmentID: "d1", AssigneeRoleIds: []string{"r1"}, ProcessingDays: 1, DueRule: "T+1", DisplayOrder: 1},
	}

	for i := 0; i < 3; i++ {
		if _, err := svc.CmsUpsertGlobalWorkflow(ctx, disclosureapp.CmsUpsertGlobalWorkflowRequest{
			Subject: testSubjectWF, TypeID: typeID, Steps: step,
		}); err != nil {
			t.Fatalf("upsert %d failed: %v", i+1, err)
		}
	}

	count, err := repo.CountGlobalWorkflowsByTypeId(ctx, typeID)
	if err != nil {
		t.Fatalf("CountGlobalWorkflowsByTypeId: %v", err)
	}
	if count != 1 {
		t.Errorf("count=%d want 1 — multiple upserts should not accumulate active workflows", count)
	}
}

// TestUpsertGlobalWorkflow_GetReturnsLatestSteps verifies that GetGlobalWorkflow
// returns the steps from the most recent upsert, not from a previous one.
func TestUpsertGlobalWorkflow_GetReturnsLatestSteps(t *testing.T) {
	ctx := context.Background()
	svc := newWFService()
	const typeID = "dt-latest-steps-test"

	// First upsert: 1 step.
	_, err := svc.CmsUpsertGlobalWorkflow(ctx, disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID,
		Steps: []disclosureapp.GlobalWorkflowStepInput{
			{StepID: "s1", Stage: "Old Step", DepartmentID: "d1", AssigneeRoleIds: []string{"r1"}, ProcessingDays: 5, DueRule: "T+5", DisplayOrder: 1},
		},
	})
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	// Second upsert: 2 steps with updated content.
	_, err = svc.CmsUpsertGlobalWorkflow(ctx, disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID,
		Steps: []disclosureapp.GlobalWorkflowStepInput{
			{StepID: "s1", Stage: "New Step 1", DepartmentID: "d1", AssigneeRoleIds: []string{"r1"}, ProcessingDays: 3, DueRule: "T+3", DisplayOrder: 1},
			{StepID: "s2", Stage: "New Step 2", DepartmentID: "d2", AssigneeRoleIds: []string{"r2"}, ProcessingDays: 2, DueRule: "T+5", DisplayOrder: 2},
		},
	})
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	// GetGlobalWorkflow must return 2 steps from the second upsert.
	resp, err := svc.CmsGetGlobalWorkflow(ctx, disclosureapp.CmsGetGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID,
	})
	if err != nil {
		t.Fatalf("GetGlobalWorkflow: %v", err)
	}
	if resp.Data == nil {
		t.Fatal("expected workflow data, got nil")
	}
	if len(resp.Data.Steps) != 2 {
		t.Errorf("steps=%d want 2 — GetGlobalWorkflow must return latest upsert steps, not accumulated ones", len(resp.Data.Steps))
	}
	if resp.Data.Steps[0].Stage != "New Step 1" {
		t.Errorf("step[0].stage=%q want 'New Step 1'", resp.Data.Steps[0].Stage)
	}
}

// TestUpsertGlobalWorkflow_NoOrphanedStepsAfterUpsert verifies that
// step count after N upserts equals the last request's step count.
func TestUpsertGlobalWorkflow_NoOrphanedStepsAfterUpsert(t *testing.T) {
	ctx := context.Background()
	repo := inmemory.NewRepository()
	svc := disclosureapp.NewService(repo, nil, idgen.UUIDv7Generator{})
	const typeID = "dt-orphan-test"

	for i := 0; i < 5; i++ {
		if _, err := svc.CmsUpsertGlobalWorkflow(ctx, disclosureapp.CmsUpsertGlobalWorkflowRequest{
			Subject: testSubjectWF, TypeID: typeID,
			Steps: []disclosureapp.GlobalWorkflowStepInput{
				{StepID: "s1", Stage: "S1", DepartmentID: "d1", AssigneeRoleIds: []string{"r1"}, ProcessingDays: 1, DueRule: "T+1", DisplayOrder: 1},
				{StepID: "s2", Stage: "S2", DepartmentID: "d2", AssigneeRoleIds: []string{"r2"}, ProcessingDays: 2, DueRule: "T+3", DisplayOrder: 2},
			},
		}); err != nil {
			t.Fatalf("upsert %d: %v", i+1, err)
		}
	}

	resp, err := svc.CmsGetGlobalWorkflow(ctx, disclosureapp.CmsGetGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID,
	})
	if err != nil {
		t.Fatalf("GetGlobalWorkflow: %v", err)
	}
	if resp.Data == nil {
		t.Fatal("expected workflow")
	}
	// After 5 upserts each with 2 steps, must have exactly 2 steps (not 10).
	if len(resp.Data.Steps) != 2 {
		t.Errorf("steps=%d want 2 — orphaned steps from previous upserts must not accumulate", len(resp.Data.Steps))
	}
}

// TestUpsertGlobalWorkflow_UpsertAfterDeleteSucceeds verifies that upsert
// works correctly after a delete (regression: delete then re-create).
func TestUpsertGlobalWorkflow_UpsertAfterDeleteSucceeds(t *testing.T) {
	ctx := context.Background()
	svc := newWFService()
	const typeID = "dt-delete-then-upsert"

	step := []disclosureapp.GlobalWorkflowStepInput{
		{StepID: "s1", Stage: "Step", DepartmentID: "d1", AssigneeRoleIds: []string{"r1"}, ProcessingDays: 2, DueRule: "T+2", DisplayOrder: 1},
	}

	if _, err := svc.CmsUpsertGlobalWorkflow(ctx, disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID, Steps: step,
	}); err != nil {
		t.Fatalf("initial upsert: %v", err)
	}

	if err := svc.CmsDeleteGlobalWorkflow(ctx, disclosureapp.CmsDeleteGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID,
	}); err != nil {
		t.Fatalf("delete: %v", err)
	}

	wf, err := svc.CmsUpsertGlobalWorkflow(ctx, disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID, Steps: step,
	})
	if err != nil {
		t.Fatalf("upsert after delete: %v", err)
	}
	if len(wf.Steps) != 1 {
		t.Errorf("steps=%d want 1", len(wf.Steps))
	}
}
