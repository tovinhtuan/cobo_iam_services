package app_test

import (
	"context"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

func TestCmsUpsertGlobalWorkflow_ReminderConfigPersistsAndClears(t *testing.T) {
	ctx := context.Background()
	const typeID = "dt-reminder-persist"
	svc, _ := newSeededWFService(t, typeID)
	base := disclosureapp.GlobalWorkflowStepInput{
		Stage: "Review", DepartmentID: "d1", AssigneeRoleIds: []string{"r1"},
		ProcessingDays: 3, DueRule: "T+3", DisplayOrder: 1,
	}

	custom := base
	custom.ReminderConfig = &disclosureapp.WorkflowStepReminderConfig{
		Enabled: true, Mode: disclosureapp.WorkflowStepReminderModeDaysBefore, DaysBefore: []int{7, 2},
	}
	wf, err := svc.CmsUpsertGlobalWorkflow(ctx, disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID, Steps: []disclosureapp.GlobalWorkflowStepInput{custom},
	})
	if err != nil {
		t.Fatalf("custom upsert: %v", err)
	}
	if wf.Steps[0].ReminderConfig == nil {
		t.Fatal("custom reminder_config dropped on persist")
	}
	if got := wf.Steps[0].ReminderConfig.DaysBefore; len(got) != 2 || got[0] != 7 || got[1] != 2 {
		t.Fatalf("readback days=%v", got)
	}

	disabled := base
	disabled.ReminderConfig = &disclosureapp.WorkflowStepReminderConfig{
		Enabled: false, Mode: disclosureapp.WorkflowStepReminderModeDaysBefore,
	}
	wf, err = svc.CmsUpsertGlobalWorkflow(ctx, disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID, Steps: []disclosureapp.GlobalWorkflowStepInput{disabled},
	})
	if err != nil {
		t.Fatalf("disabled upsert: %v", err)
	}
	if wf.Steps[0].ReminderConfig == nil || wf.Steps[0].ReminderConfig.Enabled {
		t.Fatalf("disabled must persist, got %+v", wf.Steps[0].ReminderConfig)
	}

	specific := base
	specific.ReminderConfig = &disclosureapp.WorkflowStepReminderConfig{
		Enabled: true, Mode: disclosureapp.WorkflowStepReminderModeSpecificDate,
	}
	wf, err = svc.CmsUpsertGlobalWorkflow(ctx, disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID, Steps: []disclosureapp.GlobalWorkflowStepInput{specific},
	})
	if err != nil {
		t.Fatalf("specific_date upsert: %v", err)
	}
	if wf.Steps[0].ReminderConfig == nil || wf.Steps[0].ReminderConfig.Mode != disclosureapp.WorkflowStepReminderModeSpecificDate {
		t.Fatalf("specific_date dropped: %+v", wf.Steps[0].ReminderConfig)
	}

	def := base
	wf, err = svc.CmsUpsertGlobalWorkflow(ctx, disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID, Steps: []disclosureapp.GlobalWorkflowStepInput{def},
	})
	if err != nil {
		t.Fatalf("default omit upsert: %v", err)
	}
	if wf.Steps[0].ReminderConfig != nil {
		t.Fatalf("DEFAULT omit must remove stale custom, got %+v", wf.Steps[0].ReminderConfig)
	}
}

func TestCmsUpsertGlobalWorkflow_RejectsEmptyCustomReminder(t *testing.T) {
	ctx := context.Background()
	svc := newWFService()
	_, err := svc.CmsUpsertGlobalWorkflow(ctx, disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF,
		TypeID:  "dt-reminder-invalid",
		Steps: []disclosureapp.GlobalWorkflowStepInput{{
			Stage: "Review", DepartmentID: "d1", AssigneeRoleIds: []string{"r1"},
			ProcessingDays: 3, DueRule: "T+3", DisplayOrder: 1,
			ReminderConfig: &disclosureapp.WorkflowStepReminderConfig{
				Enabled: true, Mode: disclosureapp.WorkflowStepReminderModeDaysBefore, DaysBefore: []int{},
			},
		}},
	})
	if err == nil {
		t.Fatal("empty custom must reject")
	}
}

func TestCmsUpsertGlobalWorkflow_DocumentsPersistAndClear(t *testing.T) {
	ctx := context.Background()
	const typeID = "dt-documents-persist"
	svc, _ := newSeededWFService(t, typeID)
	base := disclosureapp.GlobalWorkflowStepInput{
		Stage: "Review", DepartmentID: "d1", AssigneeRoleIds: []string{"r1"},
		ProcessingDays: 3, DueRule: "T+3", DisplayOrder: 1,
	}

	withDocs := base
	withDocs.Documents = []disclosureapp.WorkflowDocumentDTO{
		{DocID: "doc_a", Name: "Báo cáo tài chính Quý 3", Required: true, TemplateFileID: "wdt_cms_f1", TemplateFileName: "form.xlsx"},
		{DocID: "doc_b", Name: "Biên bản xác nhận số liệu", Required: true},
	}
	wf, err := svc.CmsUpsertGlobalWorkflow(ctx, disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID, Steps: []disclosureapp.GlobalWorkflowStepInput{withDocs},
	})
	if err != nil {
		t.Fatalf("upsert with documents: %v", err)
	}
	if len(wf.Steps) != 1 || len(wf.Steps[0].Documents) != 2 {
		t.Fatalf("documents dropped on persist: %#v", wf.Steps[0].Documents)
	}
	if wf.Steps[0].Documents[0].TemplateFileID != "wdt_cms_f1" || wf.Steps[0].Documents[1].Name != "Biên bản xác nhận số liệu" {
		t.Fatalf("document fields wrong: %#v", wf.Steps[0].Documents)
	}

	got, err := svc.CmsGetGlobalWorkflow(ctx, disclosureapp.CmsGetGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID,
	})
	if err != nil || got.Data == nil {
		t.Fatalf("reload: %v data=%v", err, got)
	}
	if len(got.Data.Steps[0].Documents) != 2 || got.Data.Steps[0].Documents[0].DocID != "doc_a" {
		t.Fatalf("reload lost documents: %#v", got.Data.Steps[0].Documents)
	}

	cleared := base
	cleared.Documents = []disclosureapp.WorkflowDocumentDTO{}
	wf, err = svc.CmsUpsertGlobalWorkflow(ctx, disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID, Steps: []disclosureapp.GlobalWorkflowStepInput{cleared},
	})
	if err != nil {
		t.Fatalf("clear documents: %v", err)
	}
	if len(wf.Steps[0].Documents) != 0 {
		t.Fatalf("empty documents must replace, got %#v", wf.Steps[0].Documents)
	}
}
