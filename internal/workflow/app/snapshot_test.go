package app

import (
	"errors"
	"testing"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

func TestMapEffectiveWorkflowToSnapshotOrdersByDisplayOrder(t *testing.T) {
	steps := []disclosureapp.WorkflowStepDTO{
		{StepID: "s2", Stage: "Review", DepartmentID: "d2", DueRule: "T+3", DisplayOrder: 2, ProcessingDays: 3},
		{StepID: "s1", Stage: "Draft", DepartmentID: "d1", DueRule: "T+1", DisplayOrder: 1, ProcessingDays: 1},
	}
	got := MapEffectiveWorkflowToSnapshot(steps, "global_template")
	if len(got) != 2 {
		t.Fatalf("len(snapshot) = %d, want 2", len(got))
	}
	if got[0].StepID != "s1" || got[1].StepID != "s2" {
		t.Fatalf("order = %#v", got)
	}
	if got[0].StepCode != "s1" || got[0].Department != "d1" {
		t.Fatalf("first step = %#v", got[0])
	}
}

func TestApplyAdHocStepOverridesPatchesProcessingDays(t *testing.T) {
	base := []StepSnapshot{
		{StepID: "s1", StepCode: "s1", DisplayOrder: 1, ProcessingDays: 2},
		{StepID: "s2", StepCode: "s2", DisplayOrder: 2, ProcessingDays: 5},
	}
	got := ApplyAdHocStepOverrides(base, []adhocapp.WorkflowStepOverride{
		{StepID: "s2", ProcessingDays: 9},
	})
	if got[0].ProcessingDays != 2 || got[1].ProcessingDays != 9 {
		t.Fatalf("got %#v", got)
	}
	if base[1].ProcessingDays != 5 {
		t.Fatal("expected input snapshot unchanged")
	}
}

func TestFirstStepCodeUsesLowestDisplayOrder(t *testing.T) {
	code := FirstStepCode([]StepSnapshot{
		{StepID: "b", StepCode: "step_b", DisplayOrder: 2},
		{StepID: "a", StepCode: "step_a", DisplayOrder: 1},
	})
	if code != "step_a" {
		t.Fatalf("FirstStepCode() = %q, want step_a", code)
	}
}

func TestValidateSnapshotRejectsEmpty(t *testing.T) {
	err := ValidateSnapshot(nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrEmptyWorkflowSnapshot) {
		t.Fatalf("err = %v", err)
	}
}

func TestIsEmptyEffectiveWorkflow(t *testing.T) {
	if IsEmptyEffectiveWorkflow(nil) {
		t.Fatal("nil should be false")
	}
	if !IsEmptyEffectiveWorkflow(ValidateSnapshot(nil)) {
		t.Fatal("expected true for empty snapshot validate error")
	}
}
