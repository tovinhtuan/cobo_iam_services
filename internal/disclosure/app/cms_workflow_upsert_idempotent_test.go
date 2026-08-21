package app_test

import (
	"context"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
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

func seedTemplateDraft(t *testing.T, repo *inmemory.Repository, typeID string) {
	t.Helper()
	blocks := []disclosureapp.TemplateBlockDTO{
		{BlockID: "tid-m1", BlockKey: "legal_basis", BlockType: "rich_text", Title: "LB", Config: map[string]any{"max_length": 8000, "allow_html": false}, Validation: map[string]any{}, DisplayOrder: 1, Enabled: true},
		{BlockID: "tid-m2", BlockKey: "disclosure_content", BlockType: "rich_text", Title: "DC", Config: map[string]any{"max_length": 10000, "allow_html": true}, Validation: map[string]any{}, DisplayOrder: 2, Enabled: true},
		{BlockID: "tid-m3", BlockKey: "deadline", BlockType: "text", Title: "DL", Config: map[string]any{"max_length": 4000}, Validation: map[string]any{}, DisplayOrder: 3, Enabled: true},
		{BlockID: "tid-m4", BlockKey: "channels_and_format", BlockType: "rich_text", Title: "CF", Config: map[string]any{"max_length": 12000, "allow_html": false, "channels": []any{map[string]any{"id": "ch-001", "name": "Website", "file_types": []any{"PDF"}}}, "file_types": []any{"PDF", "XML"}}, Validation: map[string]any{}, DisplayOrder: 4, Enabled: true},
		{BlockID: "tid-m5", BlockKey: "legal_risks", BlockType: "rich_text", Title: "LR", Config: map[string]any{"max_length": 8000, "allow_html": false}, Validation: map[string]any{}, DisplayOrder: 5, Enabled: true},
		{BlockID: "tid-m6", BlockKey: "enterprise_workflow", BlockType: "rich_text", Title: "EW", Config: map[string]any{"max_length": 12000, "allow_html": true}, Validation: map[string]any{}, DisplayOrder: 6, Enabled: true},
	}
	if _, err := repo.UpsertTypeVersion(context.Background(), disclosureapp.UpsertTypeVersionRequest{
		Subject:           testSubjectWF,
		TypeID:            typeID,
		Scope:             "global",
		GroupID:           "group-001",
		Name:              "WF Test",
		Category:          "periodic",
		TemplateCategory:  "periodic",
		DeadlineStrategy:  "fixed",
		DeadlineRule:      "T+5",
		Periodicity:       "quarterly",
		DisplayGroupCodes:  []string{"display_groups_003"},
		ApplicabilityRules: applicability.DefaultGlobalRules(true),
		Blocks:            blocks,
	}); err != nil {
		t.Fatalf("seed template: %v", err)
	}
}

func newSeededWFService(t *testing.T, typeID string) (disclosureapp.Service, *inmemory.Repository) {
	t.Helper()
	repo := inmemory.NewRepository()
	seedTemplateDraft(t, repo, typeID)
	return disclosureapp.NewService(repo, nil, idgen.UUIDv7Generator{}), repo
}

// TestUpsertGlobalWorkflow_SecondUpsertWithSameStepIdSucceeds verifies that
// calling upsert twice with the same explicit step_id values does not error.
// This is the exact scenario that caused "Duplicate entry" in MySQL before the fix.
func TestUpsertGlobalWorkflow_SecondUpsertWithSameStepIdSucceeds(t *testing.T) {
	ctx := context.Background()
	const typeID = "dt-collision-test"
	svc, _ := newSeededWFService(t, typeID)

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
	const typeID = "dt-count-test"
	seedTemplateDraft(t, repo, typeID)
	svc := disclosureapp.NewService(repo, nil, idgen.UUIDv7Generator{})

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
	if count != 0 {
		t.Errorf("count=%d want 0 — compatibility PUT must not create global workflow runtime rows", count)
	}
	resp, err := svc.CmsGetGlobalWorkflow(ctx, disclosureapp.CmsGetGlobalWorkflowRequest{Subject: testSubjectWF, TypeID: typeID})
	if err != nil {
		t.Fatalf("CmsGetGlobalWorkflow: %v", err)
	}
	if resp.Data == nil || len(resp.Data.Steps) != 1 {
		t.Errorf("template draft steps missing after repeated PUT")
	}
}

// TestUpsertGlobalWorkflow_GetReturnsLatestSteps verifies that GetGlobalWorkflow
// returns the steps from the most recent upsert, not from a previous one.
func TestUpsertGlobalWorkflow_GetReturnsLatestSteps(t *testing.T) {
	ctx := context.Background()
	const typeID = "dt-latest-steps-test"
	svc, _ := newSeededWFService(t, typeID)

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
	const typeID = "dt-orphan-test"
	seedTemplateDraft(t, repo, typeID)
	svc := disclosureapp.NewService(repo, nil, idgen.UUIDv7Generator{})

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
	const typeID = "dt-delete-then-upsert"
	svc, _ := newSeededWFService(t, typeID)

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
