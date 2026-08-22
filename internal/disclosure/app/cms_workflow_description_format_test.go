package app_test

import (
	"context"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func TestUpsertGlobalWorkflow_DescriptionFormatRoundTrip(t *testing.T) {
	ctx := context.Background()
	svc, _ := newSeededWFService(t, "dt-desc-format-roundtrip")
	const typeID = "dt-desc-format-roundtrip"

	_, err := svc.CmsUpsertGlobalWorkflow(ctx, disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject:    testSubjectWF,
		TypeID:     typeID,
		ChangeNote: "safe html description",
		Steps: []disclosureapp.GlobalWorkflowStepInput{
			{
				Stage:             "Thu thập",
				Description:       "<p><strong>Safe</strong></p>",
				DescriptionFormat: "safe_html",
				Instructions:      "guide",
				DepartmentID:      "d1",
				AssigneeRoleIds:   []string{"role-reviewer"},
				ProcessingDays:    3,
				DueRule:           "T+3",
				DisplayOrder:      1,
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
	if got.Data.Steps[0].DescriptionFormat != "safe_html" {
		t.Fatalf("description_format=%q", got.Data.Steps[0].DescriptionFormat)
	}
	if got.Data.Steps[0].Description != "<p><strong>Safe</strong></p>" {
		t.Fatalf("description=%q", got.Data.Steps[0].Description)
	}
}

func TestUpsertGlobalWorkflow_UnknownDescriptionFormatRejected(t *testing.T) {
	ctx := context.Background()
	svc, _ := newSeededWFService(t, "dt-desc-format-bad")
	const typeID = "dt-desc-format-bad"

	_, err := svc.CmsUpsertGlobalWorkflow(ctx, disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF,
		TypeID:  typeID,
		Steps: []disclosureapp.GlobalWorkflowStepInput{
			{
				Stage:             "Review",
				DescriptionFormat: "raw_html",
				DepartmentID:      "d1",
				AssigneeRoleIds:   []string{"role-reviewer"},
				ProcessingDays:    3,
				DueRule:           "T+3",
				DisplayOrder:      1,
			},
		},
	})
	if err == nil {
		t.Fatal("expected validation error for unknown description_format")
	}
	he, ok := err.(*perr.HTTPError)
	if !ok || he.HTTPStatus != 400 {
		t.Fatalf("expected HTTP 400, got %v", err)
	}
}

func TestUpsertGlobalWorkflow_LegacyMissingDescriptionFormatAccepted(t *testing.T) {
	ctx := context.Background()
	svc, _ := newSeededWFService(t, "dt-desc-format-legacy")
	const typeID = "dt-desc-format-legacy"

	_, err := svc.CmsUpsertGlobalWorkflow(ctx, disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF,
		TypeID:  typeID,
		Steps: []disclosureapp.GlobalWorkflowStepInput{
			{
				Stage:           "Review",
				Description:     "BCTT",
				DepartmentID:    "d1",
				AssigneeRoleIds: []string{"role-reviewer"},
				ProcessingDays:  3,
				DueRule:         "T+3",
				DisplayOrder:    1,
			},
		},
	})
	if err != nil {
		t.Fatalf("legacy upsert: %v", err)
	}
	got, err := svc.CmsGetGlobalWorkflow(ctx, disclosureapp.CmsGetGlobalWorkflowRequest{
		Subject: testSubjectWF,
		TypeID:  typeID,
	})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Data.Steps[0].Description != "BCTT" {
		t.Fatalf("description=%q", got.Data.Steps[0].Description)
	}
	if got.Data.Steps[0].DescriptionFormat != "" {
		t.Fatalf("expected omitempty plain default, got %q", got.Data.Steps[0].DescriptionFormat)
	}
}
