package app_test

import (
	"context"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

// TestGlobalWorkflowChain verifies Model A: PUT workflow updates the template draft
// without publishing Portal; only template Activate opens the has_workflow gate.
func TestGlobalWorkflowChain(t *testing.T) {
	ctx := context.Background()
	repo := inmemory.NewRepository()
	const typeID = "dt-model-a-chain"
	seedTemplateDraft(t, repo, typeID)
	svc := disclosureapp.NewService(repo, nil, idgen.UUIDv7Generator{})

	cmsSubject := testSubjectWF
	portalSubject := disclosureapp.Subject{
		UserID:       "user-portal",
		MembershipID: "member-portal",
		CompanyID:    cmsSubject.CompanyID,
	}

	wf, err := svc.CmsUpsertGlobalWorkflow(ctx, disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject:    cmsSubject,
		TypeID:     typeID,
		ChangeNote: "initial workflow",
		Steps: []disclosureapp.GlobalWorkflowStepInput{
			{
				StepID:          "review",
				Stage:           "Review",
				DepartmentID:    "dept-compliance",
				AssigneeRoleIds: []string{"role-reviewer"},
				DueRule:         "T+5",
				ProcessingDays:  5,
				DisplayOrder:    1,
			},
		},
	})
	if err != nil {
		t.Fatalf("CmsUpsertGlobalWorkflow: %v", err)
	}
	if wf.WorkflowID == "" || wf.TypeID != typeID || len(wf.Steps) != 1 {
		t.Fatalf("unexpected workflow projection: %+v", wf)
	}

	count, err := repo.CountGlobalWorkflowsByTypeId(ctx, typeID)
	if err != nil {
		t.Fatalf("CountGlobalWorkflowsByTypeId: %v", err)
	}
	if count != 0 {
		t.Fatalf("PUT must not create global workflow runtime rows, count=%d", count)
	}

	resp, err := svc.CmsGetGlobalWorkflow(ctx, disclosureapp.CmsGetGlobalWorkflowRequest{
		Subject: cmsSubject,
		TypeID:  typeID,
	})
	if err != nil {
		t.Fatalf("CmsGetGlobalWorkflow: %v", err)
	}
	if resp.Data == nil || resp.Data.TypeID != typeID {
		t.Fatal("expected draft workflow projection")
	}

	_, err = svc.CreateRecord(ctx, disclosureapp.CreateRecordRequest{
		Subject: portalSubject,
		Payload: disclosureapp.RecordPayload{TypeID: typeID, Title: "Draft leak?", Content: "x"},
	})
	if err == nil {
		t.Fatal("SAVE_DRAFT_DOES_NOT_PUBLISH: portal create must fail before template activate")
	}
	if herr, ok := err.(*perr.HTTPError); !ok || herr.Code != "TEMPLATE_NO_WORKFLOW" {
		t.Fatalf("before activate want TEMPLATE_NO_WORKFLOW, got %v", err)
	}

	if _, err := svc.ActivateTypeVersion(ctx, disclosureapp.ActivateTypeVersionRequest{
		Subject: cmsSubject, TypeID: typeID, VersionNo: 1,
	}); err != nil {
		t.Fatalf("ActivateTypeVersion: %v", err)
	}

	has, err := repo.HasActiveEnterpriseWorkflow(ctx, portalSubject.CompanyID, typeID)
	if err != nil || !has {
		t.Fatalf("after activate has_workflow=%v err=%v", has, err)
	}
	if _, err := repo.GetTypeDetail(ctx, portalSubject.CompanyID, typeID); err != nil {
		t.Fatalf("portal GetTypeDetail after activate: %v", err)
	}
}
