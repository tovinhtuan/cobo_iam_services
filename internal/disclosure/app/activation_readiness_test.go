package app

import (
	"testing"
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
	applyActivationReadiness(item)
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
	applyActivationReadiness(item)
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
	applyActivationReadiness(item)
	if item.ActivationReady || len(item.ActivationBlockers) == 0 {
		t.Fatalf("want not pinned blocker, got ready=%v blockers=%v", item.ActivationReady, item.ActivationBlockers)
	}
}
