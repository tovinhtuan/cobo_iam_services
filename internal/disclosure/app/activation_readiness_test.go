package app

import (
	"testing"
	"time"
)

func TestApplyActivationReadiness_PinnedValid(t *testing.T) {
	item := &DisclosureTypeDTO{
		TypeID:                "dt-ready",
		VersionNo:             1,
		WorkflowAuthorityMode: WorkflowAuthorityTemplatePinned,
		WorkflowManifest: &WorkflowPublicationManifest{
			SchemaVersion: WorkflowManifestSchemaVersion,
			Steps: []WorkflowPublicationStep{
				{WorkflowStepDTO: WorkflowStepDTO{StepID: "s1", Stage: "A", DepartmentID: "d1", AssigneeRoleIds: []string{"r1"}, ProcessingDays: 1, DueRule: "T+1"}},
				{WorkflowStepDTO: WorkflowStepDTO{StepID: "s2", Stage: "B", DepartmentID: "d2", AssigneeRoleIds: []string{"r1"}, ProcessingDays: 1, DueRule: "T+1"}},
			},
		},
	}
	applyActivationReadiness(item, time.Now().UTC(), nil)
	if !item.ActivationReady {
		t.Fatalf("want ready, blockers=%v", item.ActivationBlockers)
	}
}

func TestApplyActivationReadiness_MissingDepartment(t *testing.T) {
	item := &DisclosureTypeDTO{
		TypeID:                "dt-bad-dept",
		VersionNo:             1,
		WorkflowAuthorityMode: WorkflowAuthorityTemplatePinned,
		WorkflowManifest: &WorkflowPublicationManifest{
			SchemaVersion: WorkflowManifestSchemaVersion,
			Steps: []WorkflowPublicationStep{
				{WorkflowStepDTO: WorkflowStepDTO{StepID: "s1", Stage: "Thu thập", DepartmentID: "d1", AssigneeRoleIds: []string{"r1"}, ProcessingDays: 1}},
				{WorkflowStepDTO: WorkflowStepDTO{StepID: "s2", Stage: "Rà soát", DepartmentID: "", AssigneeRoleIds: []string{"r1"}, ProcessingDays: 1}},
			},
		},
	}
	applyActivationReadiness(item, time.Now().UTC(), nil)
	if item.ActivationReady {
		t.Fatal("want not ready")
	}
	found := false
	for _, b := range item.ActivationBlockers {
		if b.Code == "WORKFLOW_STEP_DEPARTMENT_REQUIRED" {
			found = true
		}
	}
	if !found {
		t.Fatalf("blockers=%v", item.ActivationBlockers)
	}
}

func TestApplyActivationReadiness_NotPinned(t *testing.T) {
	item := &DisclosureTypeDTO{TypeID: "dt-x", VersionNo: 1}
	applyActivationReadiness(item, time.Now().UTC(), nil)
	if item.ActivationReady || len(item.ActivationBlockers) == 0 {
		t.Fatalf("want not pinned blocker, got ready=%v blockers=%v", item.ActivationReady, item.ActivationBlockers)
	}
}

func TestApplyActivationReadiness_OverdueWarningDoesNotBlock(t *testing.T) {
	day := 5
	item := &DisclosureTypeDTO{
		TypeID:                "dt-overdue-preview",
		VersionNo:             1,
		WorkflowAuthorityMode: WorkflowAuthorityTemplatePinned,
		WorkflowManifest: &WorkflowPublicationManifest{
			SchemaVersion: WorkflowManifestSchemaVersion,
			Steps: []WorkflowPublicationStep{
				{WorkflowStepDTO: WorkflowStepDTO{StepID: "s1", Stage: "A", DepartmentID: "d1", AssigneeRoleIds: []string{"r1"}, ProcessingDays: 1, DueRule: "T+1"}},
			},
		},
		DeadlineConfig: &TemplateDeadlineConfig{
			FrequencyUnit:        "monthly",
			CycleAnchorDay:       day,
			DeadlineDays:         20,
			DeadlineDurationType: DurationTypeCalendarDays,
			ApplicableFromMode:   ApplicableFromModeCurrent,
			OpenDaysBeforeT:      0,
		},
	}
	loc := asiaHoChiMinh()
	eval := time.Date(2026, 9, 26, 10, 0, 0, 0, loc)
	applyActivationReadiness(item, eval, nil)
	if !item.ActivationReady {
		t.Fatalf("overdue must not block activation, blockers=%v", item.ActivationBlockers)
	}
	if item.FirstOccurrencePreview == nil || item.FirstOccurrencePreview.Status != FirstOccurrenceStatusOverdue {
		t.Fatalf("preview=%+v", item.FirstOccurrencePreview)
	}
	if len(item.ActivationWarnings) == 0 || item.ActivationWarnings[0].Blocking {
		t.Fatalf("warnings=%v", item.ActivationWarnings)
	}
	if item.ActivationWarnings[0].Code != ActivationWarningFirstOccurrenceOverdue {
		t.Fatalf("code=%s", item.ActivationWarnings[0].Code)
	}
}
