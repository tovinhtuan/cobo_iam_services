package app_test

import (
	"context"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func TestApproveCompanyWorkflowOverride_RejectsEmptyDraft(t *testing.T) {
	svc, repo := newStalenessTestService(t, true)
	subject := disclosureapp.Subject{UserID: "u1", CompanyID: "c1"}
	typeID := staleTestTypeID

	if err := repo.SeedCompanyWorkflowOverrideDraftForTest("c1", typeID, 1, []disclosureapp.WorkflowStepDTO{}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	_, err := svc.ApproveCompanyWorkflowOverride(context.Background(), disclosureapp.ApproveCompanyWorkflowOverrideRequest{
		Subject:               subject,
		TypeID:                typeID,
		VersionNo:             1,
		Reason:                "test",
		SkipSelfApprovalCheck: true,
	})
	if err == nil {
		t.Fatal("expected activation to fail for empty override")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.Code != perr.CodeWorkflowOverrideEmpty {
		t.Fatalf("code = %v, want WORKFLOW_OVERRIDE_EMPTY", he)
	}
}

func TestUpsertCompanyWorkflowOverrideDraft_PublishRejectsEmpty(t *testing.T) {
	svc, _ := newStalenessTestService(t, true)
	subject := disclosureapp.Subject{UserID: "u1", CompanyID: "c1"}
	_, err := svc.UpsertCompanyWorkflowOverrideDraft(context.Background(), disclosureapp.UpsertCompanyWorkflowOverrideDraftRequest{
		Subject:  subject,
		TypeID:   staleTestTypeID,
		Workflow: []disclosureapp.WorkflowStepDTO{},
		Publish:  true,
	})
	if err == nil {
		t.Fatal("expected publish to fail for empty workflow")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.Code != perr.CodeWorkflowOverrideEmpty {
		t.Fatalf("code = %v, want WORKFLOW_OVERRIDE_EMPTY", he)
	}
}
