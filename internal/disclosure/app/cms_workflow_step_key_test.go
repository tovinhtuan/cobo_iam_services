package app_test

import (
	"context"
	"strings"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

// reuses newWFService() and testSubjectWF from cms_workflow_upsert_idempotent_test.go.

func upsertWF(t *testing.T, svc disclosureapp.Service, typeID, note string, steps []disclosureapp.GlobalWorkflowStepInput) *disclosureapp.GlobalWorkflowDTO {
	t.Helper()
	wf, err := svc.CmsUpsertGlobalWorkflow(context.Background(), disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID, ChangeNote: note, Steps: steps,
	})
	if err != nil {
		t.Fatalf("upsert (%s) failed: %v", note, err)
	}
	return wf
}

func stepKeyByStage(wf *disclosureapp.GlobalWorkflowDTO, stage string) string {
	for _, s := range wf.Steps {
		if s.Stage == stage {
			return s.StepKey
		}
	}
	return ""
}

// 1. Existing/new steps get a stable, server-minted step_key.
func TestStepKey_MintedOnFirstUpsert(t *testing.T) {
	svc := newWFService()
	wf := upsertWF(t, svc, "dt-sk-1", "v1", []disclosureapp.GlobalWorkflowStepInput{
		{Stage: "Review", DepartmentID: "d1", AssigneeRoleIds: []string{"r1"}, ProcessingDays: 3, DueRule: "T+3", DisplayOrder: 1},
		{Stage: "Approve", DepartmentID: "d2", AssigneeRoleIds: []string{"r2"}, ProcessingDays: 2, DueRule: "T+5", DisplayOrder: 2},
	})
	if len(wf.Steps) != 2 {
		t.Fatalf("steps=%d want 2", len(wf.Steps))
	}
	seen := map[string]bool{}
	for _, s := range wf.Steps {
		if strings.TrimSpace(s.StepKey) == "" {
			t.Fatalf("step %q has empty step_key", s.Stage)
		}
		if !strings.HasPrefix(s.StepKey, "stp_") || len(s.StepKey) != 16 {
			t.Fatalf("step_key %q not in expected format stp_+12", s.StepKey)
		}
		if seen[s.StepKey] {
			t.Fatalf("duplicate step_key %q", s.StepKey)
		}
		seen[s.StepKey] = true
	}
}

// 2. Reorder keeps step_key.
func TestStepKey_ReorderPreservesKey(t *testing.T) {
	svc := newWFService()
	wf1 := upsertWF(t, svc, "dt-sk-2", "v1", []disclosureapp.GlobalWorkflowStepInput{
		{Stage: "Review", DepartmentID: "d1", AssigneeRoleIds: []string{"r1"}, ProcessingDays: 3, DueRule: "T+3", DisplayOrder: 1},
		{Stage: "Approve", DepartmentID: "d2", AssigneeRoleIds: []string{"r2"}, ProcessingDays: 2, DueRule: "T+5", DisplayOrder: 2},
	})
	reviewKey := stepKeyByStage(wf1, "Review")
	approveKey := stepKeyByStage(wf1, "Approve")

	// Round-trip the returned steps, swapped order.
	swapped := []disclosureapp.GlobalWorkflowStepInput{wf1.Steps[1], wf1.Steps[0]}
	swapped[0].DisplayOrder = 1
	swapped[1].DisplayOrder = 2
	wf2 := upsertWF(t, svc, "dt-sk-2", "v2", swapped)

	if got := stepKeyByStage(wf2, "Review"); got != reviewKey {
		t.Fatalf("Review key changed on reorder: %q -> %q", reviewKey, got)
	}
	if got := stepKeyByStage(wf2, "Approve"); got != approveKey {
		t.Fatalf("Approve key changed on reorder: %q -> %q", approveKey, got)
	}
}

// 3. Rename keeps step_key.
func TestStepKey_RenamePreservesKey(t *testing.T) {
	svc := newWFService()
	wf1 := upsertWF(t, svc, "dt-sk-3", "v1", []disclosureapp.GlobalWorkflowStepInput{
		{Stage: "Review", DepartmentID: "d1", AssigneeRoleIds: []string{"r1"}, ProcessingDays: 3, DueRule: "T+3", DisplayOrder: 1},
	})
	origKey := wf1.Steps[0].StepKey

	renamed := []disclosureapp.GlobalWorkflowStepInput{wf1.Steps[0]}
	renamed[0].Stage = "Business Review"
	wf2 := upsertWF(t, svc, "dt-sk-3", "v2", renamed)

	if len(wf2.Steps) != 1 {
		t.Fatalf("steps=%d want 1", len(wf2.Steps))
	}
	if wf2.Steps[0].StepKey != origKey {
		t.Fatalf("rename changed step_key: %q -> %q", origKey, wf2.Steps[0].StepKey)
	}
	if wf2.Steps[0].Stage != "Business Review" {
		t.Fatalf("rename not applied: %q", wf2.Steps[0].Stage)
	}
}

// 4. Add new step creates a new step_key (existing preserved).
func TestStepKey_AddStepGetsNewKey(t *testing.T) {
	svc := newWFService()
	wf1 := upsertWF(t, svc, "dt-sk-4", "v1", []disclosureapp.GlobalWorkflowStepInput{
		{Stage: "Review", DepartmentID: "d1", AssigneeRoleIds: []string{"r1"}, ProcessingDays: 3, DueRule: "T+3", DisplayOrder: 1},
	})
	reviewKey := wf1.Steps[0].StepKey

	withNew := []disclosureapp.GlobalWorkflowStepInput{
		wf1.Steps[0],
		{Stage: "Approve", DepartmentID: "d2", AssigneeRoleIds: []string{"r2"}, ProcessingDays: 2, DueRule: "T+5", DisplayOrder: 2},
	}
	wf2 := upsertWF(t, svc, "dt-sk-4", "v2", withNew)

	if got := stepKeyByStage(wf2, "Review"); got != reviewKey {
		t.Fatalf("Review key changed: %q -> %q", reviewKey, got)
	}
	newKey := stepKeyByStage(wf2, "Approve")
	if newKey == "" || newKey == reviewKey {
		t.Fatalf("new step did not get a distinct new key: %q", newKey)
	}
}

// 5 + 6. Delete retires a step_key; re-adding mints a fresh one (never reused).
func TestStepKey_DeleteRetiresAndReAddMintsNew(t *testing.T) {
	svc := newWFService()
	wf1 := upsertWF(t, svc, "dt-sk-5", "v1", []disclosureapp.GlobalWorkflowStepInput{
		{Stage: "Review", DepartmentID: "d1", AssigneeRoleIds: []string{"r1"}, ProcessingDays: 3, DueRule: "T+3", DisplayOrder: 1},
		{Stage: "Approve", DepartmentID: "d2", AssigneeRoleIds: []string{"r2"}, ProcessingDays: 2, DueRule: "T+5", DisplayOrder: 2},
	})
	approveKey := stepKeyByStage(wf1, "Approve")

	// Delete "Approve" (keep only Review).
	wf2 := upsertWF(t, svc, "dt-sk-5", "v2", []disclosureapp.GlobalWorkflowStepInput{wf1.Steps[0]})
	if len(wf2.Steps) != 1 || stepKeyByStage(wf2, "Approve") != "" {
		t.Fatalf("delete did not retire Approve step")
	}

	// Re-add a step named "Approve" with no key -> must mint a NEW key, never reuse the retired one.
	wf3 := upsertWF(t, svc, "dt-sk-5", "v3", []disclosureapp.GlobalWorkflowStepInput{
		wf2.Steps[0],
		{Stage: "Approve", DepartmentID: "d2", AssigneeRoleIds: []string{"r2"}, ProcessingDays: 2, DueRule: "T+5", DisplayOrder: 2},
	})
	reAddedKey := stepKeyByStage(wf3, "Approve")
	if reAddedKey == "" {
		t.Fatalf("re-added step has no key")
	}
	if reAddedKey == approveKey {
		t.Fatalf("re-added step reused retired key %q (must never reuse)", approveKey)
	}
}

// 7. Backward compatibility: legacy client sending neither step_key nor step_id still works.
func TestStepKey_BackwardCompatNoIdentitySupplied(t *testing.T) {
	svc := newWFService()
	wf := upsertWF(t, svc, "dt-sk-7", "v1", []disclosureapp.GlobalWorkflowStepInput{
		{Stage: "Review", DepartmentID: "d1", AssigneeRoleIds: []string{"r1"}, ProcessingDays: 3, DueRule: "T+3"},
		{Stage: "Approve", DepartmentID: "d2", AssigneeRoleIds: []string{"r2"}, ProcessingDays: 2, DueRule: "T+5"},
	})
	for _, s := range wf.Steps {
		if strings.TrimSpace(s.StepKey) == "" {
			t.Fatalf("legacy upsert produced empty step_key for %q", s.Stage)
		}
	}
}
