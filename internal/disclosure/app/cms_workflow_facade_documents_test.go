package app

import (
	"testing"
)

// Regression: CMS facade must keep documents[] through input→DTO
// (E2E P0: reminder persisted while documents were dropped).
func TestWorkflowStepFromGlobalInput_DocumentsPreserved(t *testing.T) {
	in := GlobalWorkflowStepInput{
		Stage: "Review", DepartmentID: "d1", AssigneeRoleIds: []string{"r1"},
		ProcessingDays: 3, DueRule: "T+3", DisplayOrder: 1,
		Documents: []WorkflowDocumentDTO{
			{DocID: "doc-a", Name: "Biên bản", Required: true, TemplateFileID: "wdt_f1", TemplateFileName: "a.xlsx"},
		},
		ReminderConfig: &WorkflowStepReminderConfig{
			Enabled: true, Mode: "days_before", DaysBefore: []int{1, 3},
		},
	}
	dto := workflowStepFromGlobalInput(in, WorkflowStepDTO{})
	if len(dto.Documents) != 1 || dto.Documents[0].DocID != "doc-a" || dto.Documents[0].TemplateFileID != "wdt_f1" {
		t.Fatalf("workflowStepFromGlobalInput dropped documents: %#v", dto.Documents)
	}
	if dto.ReminderConfig == nil || !dto.ReminderConfig.Enabled {
		t.Fatalf("reminder must survive alongside documents")
	}
	if dto.DepartmentID != "d1" || dto.ProcessingDays != 3 || len(dto.AssigneeRoleIds) != 1 {
		t.Fatalf("role/department/SLA dropped: %#v", dto)
	}

	merged := mergeIncomingWorkflowSteps(nil, []GlobalWorkflowStepInput{in})
	if len(merged) != 1 || len(merged[0].Documents) != 1 || merged[0].Documents[0].Name != "Biên bản" {
		t.Fatalf("mergeIncomingWorkflowSteps dropped documents: %#v", merged)
	}
}

func TestProjectGlobalStepsFromDetail_DocumentsPreserved(t *testing.T) {
	detail := &DisclosureTypeDTO{
		WorkflowManifest: &WorkflowPublicationManifest{
			Steps: []WorkflowPublicationStep{{
				StepKey: "step-1",
				WorkflowStepDTO: WorkflowStepDTO{
					StepID: "step-1", Stage: "Review", DepartmentID: "d1",
					AssigneeRoleIds: []string{"r1"}, DueRule: "T+3", ProcessingDays: 3, DisplayOrder: 1,
					Documents: []WorkflowDocumentDTO{
						{DocID: "doc-a", Name: "Biên bản", Required: true, TemplateFileID: "wdt_f1", TemplateFileName: "a.xlsx"},
					},
					ReminderConfig: &WorkflowStepReminderConfig{Enabled: true, Mode: "days_before", DaysBefore: []int{1}},
				},
			}},
		},
	}
	projected := projectGlobalStepsFromDetail(detail)
	if len(projected) != 1 || len(projected[0].Documents) != 1 {
		t.Fatalf("projectGlobalStepsFromDetail dropped documents: %#v", projected)
	}
	d := projected[0].Documents[0]
	if d.DocID != "doc-a" || d.Name != "Biên bản" || d.TemplateFileID != "wdt_f1" {
		t.Fatalf("projection fields wrong: %#v", d)
	}
	if projected[0].ReminderConfig == nil || projected[0].DepartmentID != "d1" {
		t.Fatalf("projection lost reminder/department: %#v", projected[0])
	}
}
