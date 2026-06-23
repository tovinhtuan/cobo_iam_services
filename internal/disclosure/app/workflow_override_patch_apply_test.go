package app_test

import (
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

// Sprint 3 / Batch 5 — Patch Application Engine unit tests. These exercise ApplyPatchOperations
// directly (no service/repo/HTTP layer) for the operation families the service-level integration
// tests in workflow_override_apply_service_test.go don't individually isolate.

func keyOfStepID(s disclosureapp.WorkflowStepDTO) string { return s.StepID }

func TestApplyPatchOperations_AddStep_AppendsNewStepUnchanged(t *testing.T) {
	current := []disclosureapp.WorkflowStepDTO{
		{StepID: "s1", Stage: "Existing", DisplayOrder: 1},
	}
	newStep := disclosureapp.WorkflowStepDTO{StepID: "s2", Stage: "From Global", DepartmentID: "d1", AssigneeRoleIds: []string{"reviewer"}, DisplayOrder: 2}
	ops := []disclosureapp.PatchOperation{
		{Op: disclosureapp.OpAddStep, StepKey: "s2", NewStep: &newStep},
	}
	out, err := disclosureapp.ApplyPatchOperations(current, ops, nil, keyOfStepID)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 steps, got %d: %+v", len(out), out)
	}
	if out[0].StepID != "s1" || out[1].StepID != "s2" {
		t.Errorf("unexpected step order: %+v", out)
	}
	if out[1].Stage != "From Global" || out[1].DepartmentID != "d1" {
		t.Errorf("new step fields not preserved: %+v", out[1])
	}
}

func TestApplyPatchOperations_RemoveStep_DeletesOnlyTargetedStep(t *testing.T) {
	current := []disclosureapp.WorkflowStepDTO{
		{StepID: "s1", Stage: "Keep Me", DisplayOrder: 1},
		{StepID: "s2", Stage: "Remove Me", DisplayOrder: 2},
	}
	ops := []disclosureapp.PatchOperation{
		{Op: disclosureapp.OpRemoveStep, StepKey: "s2"},
	}
	out, err := disclosureapp.ApplyPatchOperations(current, ops, nil, keyOfStepID)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(out) != 1 || out[0].StepID != "s1" {
		t.Fatalf("expected only s1 to remain, got %+v", out)
	}
}

func TestApplyPatchOperations_RemoveStep_AbsentStepIsHarmlessNoOp(t *testing.T) {
	current := []disclosureapp.WorkflowStepDTO{{StepID: "s1", Stage: "Only Step", DisplayOrder: 1}}
	ops := []disclosureapp.PatchOperation{
		{Op: disclosureapp.OpRemoveStep, StepKey: "does-not-exist-in-company-snapshot"},
	}
	out, err := disclosureapp.ApplyPatchOperations(current, ops, nil, keyOfStepID)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(out) != 1 || out[0].StepID != "s1" {
		t.Fatalf("removing an absent step must be a no-op, got %+v", out)
	}
}

func TestApplyPatchOperations_UpdateStepField_TouchesOnlyNamedField(t *testing.T) {
	current := []disclosureapp.WorkflowStepDTO{
		{StepID: "s1", Stage: "Old Stage", DepartmentID: "d1", AssigneeRoleIds: []string{"reviewer"}, DueRule: "T+5", DisplayOrder: 1},
	}
	ops := []disclosureapp.PatchOperation{
		{Op: disclosureapp.OpUpdateStepField, StepKey: "s1", FieldPath: "stage", To: "New Stage"},
	}
	out, err := disclosureapp.ApplyPatchOperations(current, ops, nil, keyOfStepID)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if out[0].Stage != "New Stage" {
		t.Errorf("Stage = %q, want %q", out[0].Stage, "New Stage")
	}
	if out[0].DepartmentID != "d1" || out[0].DueRule != "T+5" || len(out[0].AssigneeRoleIds) != 1 || out[0].AssigneeRoleIds[0] != "reviewer" {
		t.Errorf("untouched fields changed: %+v", out[0])
	}
}

func TestApplyPatchOperations_ReplaceAssignees_ReplacesOnlyThatField(t *testing.T) {
	current := []disclosureapp.WorkflowStepDTO{
		{StepID: "s1", Stage: "Stage", AssigneeRoleIds: []string{"reviewer"}, DisplayOrder: 1},
	}
	ops := []disclosureapp.PatchOperation{
		{Op: disclosureapp.OpReplaceAssignees, StepKey: "s1", NewAssigneeRoleIds: []string{"approver", "legal"}},
	}
	out, err := disclosureapp.ApplyPatchOperations(current, ops, nil, keyOfStepID)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(out[0].AssigneeRoleIds) != 2 || out[0].AssigneeRoleIds[0] != "approver" {
		t.Errorf("AssigneeRoleIds = %v, want [approver legal]", out[0].AssigneeRoleIds)
	}
	if out[0].Stage != "Stage" {
		t.Errorf("Stage changed unexpectedly: %q", out[0].Stage)
	}
}

func TestApplyPatchOperations_ReplaceDueRule_ReplacesOnlyThatField(t *testing.T) {
	current := []disclosureapp.WorkflowStepDTO{{StepID: "s1", DueRule: "T+5", Stage: "Stage", DisplayOrder: 1}}
	ops := []disclosureapp.PatchOperation{
		{Op: disclosureapp.OpReplaceDueRule, StepKey: "s1", NewDueRule: "T+10"},
	}
	out, err := disclosureapp.ApplyPatchOperations(current, ops, nil, keyOfStepID)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if out[0].DueRule != "T+10" {
		t.Errorf("DueRule = %q, want T+10", out[0].DueRule)
	}
}

func TestApplyPatchOperations_ReorderStep_ChangesOnlyOrderNotContent(t *testing.T) {
	current := []disclosureapp.WorkflowStepDTO{
		{StepID: "s1", Stage: "First", DisplayOrder: 1},
		{StepID: "s2", Stage: "Second", DisplayOrder: 2},
	}
	newOrder := 5
	ops := []disclosureapp.PatchOperation{
		{Op: disclosureapp.OpReorderStep, StepKey: "s1", NewDisplayOrder: &newOrder},
	}
	out, err := disclosureapp.ApplyPatchOperations(current, ops, nil, keyOfStepID)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	// Re-sorted by display_order — s1 (now 5) should come after s2 (still 2).
	if out[0].StepID != "s2" || out[1].StepID != "s1" {
		t.Fatalf("expected reorder to resort steps, got %+v", out)
	}
	if out[1].Stage != "First" {
		t.Errorf("reorder must not change step content: %+v", out[1])
	}
}

func TestApplyPatchOperations_KeepCompanyResolution_SkipsTheOperation(t *testing.T) {
	current := []disclosureapp.WorkflowStepDTO{{StepID: "s1", Stage: "Company's Own Stage", DisplayOrder: 1}}
	ops := []disclosureapp.PatchOperation{
		{Op: disclosureapp.OpUpdateStepField, StepKey: "s1", FieldPath: "stage", To: "Global Wants This"},
	}
	resolutions := map[string]disclosureapp.ConflictResolutionLookup{
		"s1|stage": {Resolution: disclosureapp.ResolutionKeepCompany},
	}
	out, err := disclosureapp.ApplyPatchOperations(current, ops, resolutions, keyOfStepID)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if out[0].Stage != "Company's Own Stage" {
		t.Errorf("keep_company resolution must leave the company's value untouched, got %q", out[0].Stage)
	}
}

func TestApplyPatchOperations_AcceptGlobalResolution_AppliesTheOperation(t *testing.T) {
	current := []disclosureapp.WorkflowStepDTO{{StepID: "s1", Stage: "Company's Own Stage", DisplayOrder: 1}}
	ops := []disclosureapp.PatchOperation{
		{Op: disclosureapp.OpUpdateStepField, StepKey: "s1", FieldPath: "stage", To: "Global Wants This"},
	}
	resolutions := map[string]disclosureapp.ConflictResolutionLookup{
		"s1|stage": {Resolution: disclosureapp.ResolutionAcceptGlobal},
	}
	out, err := disclosureapp.ApplyPatchOperations(current, ops, resolutions, keyOfStepID)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if out[0].Stage != "Global Wants This" {
		t.Errorf("accept_global resolution must apply the global value, got %q", out[0].Stage)
	}
}

func TestApplyPatchOperations_MergeManualResolution_UsesResolutionValue(t *testing.T) {
	current := []disclosureapp.WorkflowStepDTO{{StepID: "s1", Stage: "Old", DisplayOrder: 1}}
	ops := []disclosureapp.PatchOperation{
		{Op: disclosureapp.OpUpdateStepField, StepKey: "s1", FieldPath: "stage", To: "Global's Proposal"},
	}
	resolutions := map[string]disclosureapp.ConflictResolutionLookup{
		"s1|stage": {Resolution: disclosureapp.ResolutionMergeManual, ResolutionValue: "Admin's Manual Value"},
	}
	out, err := disclosureapp.ApplyPatchOperations(current, ops, resolutions, keyOfStepID)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if out[0].Stage != "Admin's Manual Value" {
		t.Errorf("merge_manual resolution must use the admin's own value, got %q", out[0].Stage)
	}
}

func TestApplyPatchOperations_UnknownOp_ReturnsInvalidPatchError(t *testing.T) {
	current := []disclosureapp.WorkflowStepDTO{{StepID: "s1", DisplayOrder: 1}}
	ops := []disclosureapp.PatchOperation{{Op: "not_a_real_operation", StepKey: "s1"}}
	_, err := disclosureapp.ApplyPatchOperations(current, ops, nil, keyOfStepID)
	if err == nil {
		t.Fatalf("expected an error for an unknown operation")
	}
}
