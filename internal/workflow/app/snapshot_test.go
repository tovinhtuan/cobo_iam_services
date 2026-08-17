package app

import (
	"errors"
	"testing"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

func TestMapProposalWorkflowToSnapshot_UsesProposalStepIDsAndDays(t *testing.T) {
	snap := &adhocapp.ProposalWorkflowSnapshot{
		SchemaVersion: 2,
		Frozen:        true,
		Steps: []adhocapp.ProposalWorkflowStep{
			{ID: "ps-c", SourceStepID: "tpl-c", Order: 2, Name: "C", ProcessingDays: 4, DepartmentID: "dep-c"},
			{ID: "ps-a", SourceStepID: "tpl-a", Order: 1, Name: "A", ProcessingDays: 2, DepartmentID: "dep-a"},
			{ID: "ps-x", SourceStepID: "", Order: 3, Name: "Custom", ProcessingDays: 0, DepartmentID: "dep-x"},
		},
	}
	got := MapProposalWorkflowToSnapshot(snap)
	if len(got) != 3 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].StepID != "ps-a" || got[0].StepCode != "ps-a" || got[0].DisplayOrder != 1 {
		t.Fatalf("first %#v", got[0])
	}
	if got[0].ProcessingDays != 2 || got[0].Department != "dep-a" || got[0].Stage != "A" {
		t.Fatalf("mapped fields %#v", got[0])
	}
	if got[1].StepID != "ps-c" || got[2].StepID != "ps-x" {
		t.Fatalf("order %#v", got)
	}
	// Custom step keeps proposal id; source_step_id is not used as StepID.
	if got[2].StepID != "ps-x" {
		t.Fatalf("custom step id %#v", got[2])
	}
	if FirstStepCode(got) != "ps-a" {
		t.Fatalf("FirstStepCode=%q", FirstStepCode(got))
	}
	if got[0].ReminderConfig != nil {
		t.Fatalf("proposal must not invent reminder_config, got %+v", got[0].ReminderConfig)
	}
	if len(got[0].EffectiveReminderDays) != 2 || got[0].EffectiveReminderDays[0] != 3 || got[0].EffectiveReminderDays[1] != 1 {
		t.Fatalf("ad-hoc must freeze DEFAULT [3,1], got %v", got[0].EffectiveReminderDays)
	}
	got[0].EffectiveReminderDays[0] = 99
	if disclosureapp.DefaultWorkflowStepReminderDays[0] != 3 {
		t.Fatal("must not mutate canonical default slice")
	}
}

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

func TestMapEffectiveWorkflowToSnapshot_FreezesConfiguredAndEffectiveReminder(t *testing.T) {
	steps := []disclosureapp.WorkflowStepDTO{
		{StepID: "s1", Stage: "A", DepartmentID: "d1", DueRule: "T+5", DisplayOrder: 1, ProcessingDays: 5,
			ReminderConfig: &disclosureapp.WorkflowStepReminderConfig{Enabled: true, Mode: disclosureapp.WorkflowStepReminderModeDaysBefore, DaysBefore: []int{7, 2}}},
		{StepID: "s2", Stage: "B", DepartmentID: "d1", DueRule: "T+3", DisplayOrder: 2, ProcessingDays: 3},
		{StepID: "s3", Stage: "C", DepartmentID: "d1", DueRule: "T+1", DisplayOrder: 3, ProcessingDays: 1,
			ReminderConfig: &disclosureapp.WorkflowStepReminderConfig{Enabled: false, Mode: disclosureapp.WorkflowStepReminderModeDaysBefore}},
	}
	got := MapEffectiveWorkflowToSnapshot(steps, "global_workflow")
	if len(got) != 3 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].ReminderConfig == nil || got[0].EffectiveReminderDays[0] != 7 || got[0].EffectiveReminderDays[1] != 2 {
		t.Fatalf("step1 %#v", got[0])
	}
	if got[1].ReminderConfig != nil {
		t.Fatalf("step2 configured should be absent")
	}
	if len(got[1].EffectiveReminderDays) != 2 || got[1].EffectiveReminderDays[0] != 3 || got[1].EffectiveReminderDays[1] != 1 {
		t.Fatalf("legacy/default effective=%v", got[1].EffectiveReminderDays)
	}
	if got[2].ReminderConfig == nil || got[2].ReminderConfig.Enabled {
		t.Fatalf("disabled config %#v", got[2].ReminderConfig)
	}
	if got[2].EffectiveReminderDays == nil || len(got[2].EffectiveReminderDays) != 0 {
		t.Fatalf("disabled effective=%v", got[2].EffectiveReminderDays)
	}

	got[1].EffectiveReminderDays[0] = 99
	steps[1].ReminderConfig = &disclosureapp.WorkflowStepReminderConfig{Enabled: true, Mode: disclosureapp.WorkflowStepReminderModeDaysBefore, DaysBefore: []int{5, 2}}
	if got[1].EffectiveReminderDays[0] != 99 {
		t.Fatal("snapshot days must be a copy")
	}
	again := MapEffectiveWorkflowToSnapshot(steps, "global_workflow")
	if again[1].EffectiveReminderDays[0] != 5 {
		t.Fatalf("new snapshot after config change=%v", again[1].EffectiveReminderDays)
	}
	if got[1].EffectiveReminderDays[0] != 99 {
		t.Fatal("version isolation: prior snapshot must not late-resolve")
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
