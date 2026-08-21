package app_test

import (
	"context"
	"strings"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

func TestUpsertGlobalWorkflow_InstructionsRoundTrip(t *testing.T) {
	ctx := context.Background()
	svc, _ := newSeededWFService(t, "dt-instructions-roundtrip")
	const typeID = "dt-instructions-roundtrip"

	_, err := svc.CmsUpsertGlobalWorkflow(ctx, disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject:    testSubjectWF,
		TypeID:     typeID,
		ChangeNote: "with instructions",
		Steps: []disclosureapp.GlobalWorkflowStepInput{
			{
				Stage:           "Thu thập",
				Instructions:    "Chuẩn bị hồ sơ QA 😀",
				DepartmentID:    "d1",
				AssigneeRoleIds: []string{"role-reviewer"},
				ProcessingDays:  3,
				DueRule:         "T+3",
				DisplayOrder:    1,
			},
		},
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := svc.CmsGetGlobalWorkflow(ctx, disclosureapp.CmsGetGlobalWorkflowRequest{
		Subject: testSubjectWF,
		TypeID:  typeID,
	})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Data == nil || len(got.Data.Steps) != 1 {
		t.Fatalf("expected 1 step, got %+v", got.Data)
	}
	if got.Data.Steps[0].Instructions != "Chuẩn bị hồ sơ QA 😀" {
		t.Fatalf("instructions mismatch: %q", got.Data.Steps[0].Instructions)
	}
}

func TestUpsertGlobalWorkflow_InstructionsTooLongRejected(t *testing.T) {
	ctx := context.Background()
	svc := newWFService()

	_, err := svc.CmsUpsertGlobalWorkflow(ctx, disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF,
		TypeID:  "dt-instructions-too-long",
		Steps: []disclosureapp.GlobalWorkflowStepInput{
			{
				Stage:           "Review",
				Instructions:    strings.Repeat("x", 2001),
				DepartmentID:    "d1",
				AssigneeRoleIds: []string{"role-reviewer"},
				ProcessingDays:  3,
				DueRule:         "T+3",
				DisplayOrder:    1,
			},
		},
	})
	if err == nil {
		t.Fatal("expected validation error for instructions > 2000")
	}
}
