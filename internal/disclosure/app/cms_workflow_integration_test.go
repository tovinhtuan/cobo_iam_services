package app_test

import (
	"context"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

// TestGlobalWorkflowChain verifies the end-to-end chain required for Gate 4:
// CMS configures global workflow → TemplateHasWorkflow detects it → portal create is allowed.
//
// Steps:
//  1. CMS user upserts a global workflow for a known type.
//  2. CountGlobalWorkflowsByTypeId returns > 0 for that type.
//  3. CreateRecord succeeds for that type (enforceHasWorkflowGate passes).
func TestGlobalWorkflowChain(t *testing.T) {
	ctx := context.Background()
	repo := inmemory.NewRepository()
	svc := disclosureapp.NewService(repo, nil, idgen.UUIDv7Generator{})

	const typeID = "dt-periodic-financial"
	const cmsUser = "user-cms-admin"
	const memberID = "member-cms"
	const companyID = "company-test"

	cmsSubject := disclosureapp.Subject{
		UserID:       cmsUser,
		MembershipID: memberID,
		CompanyID:    companyID,
	}

	// Step 1: CMS upserts a global workflow.
	wf, err := svc.CmsUpsertGlobalWorkflow(ctx, disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject:    cmsSubject,
		TypeID:     typeID,
		ChangeNote: "initial workflow",
		Steps: []disclosureapp.GlobalWorkflowStepInput{
			{
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
	if wf.WorkflowID == "" {
		t.Fatal("expected non-empty workflow_id")
	}
	if wf.TypeID != typeID {
		t.Fatalf("type_id=%q want %q", wf.TypeID, typeID)
	}
	if len(wf.Steps) != 1 {
		t.Fatalf("steps=%d want 1", len(wf.Steps))
	}

	// Step 2: CountGlobalWorkflowsByTypeId detects the workflow.
	count, err := repo.CountGlobalWorkflowsByTypeId(ctx, typeID)
	if err != nil {
		t.Fatalf("CountGlobalWorkflowsByTypeId: %v", err)
	}
	if count == 0 {
		t.Fatal("expected count > 0 after upserting global workflow")
	}

	// Step 3: CmsGetGlobalWorkflow retrieves the saved workflow.
	resp, err := svc.CmsGetGlobalWorkflow(ctx, disclosureapp.CmsGetGlobalWorkflowRequest{
		Subject: cmsSubject,
		TypeID:  typeID,
	})
	if err != nil {
		t.Fatalf("CmsGetGlobalWorkflow: %v", err)
	}
	if resp.Data == nil {
		t.Fatal("expected workflow data, got nil")
	}
	if resp.Data.TypeID != typeID {
		t.Fatalf("type_id=%q want %q", resp.Data.TypeID, typeID)
	}

	// Step 4: CreateRecord with that type passes the has_workflow gate.
	// Auth is nil (worker mode), so authorization is bypassed.
	portalSubject := disclosureapp.Subject{
		UserID:       "user-portal",
		MembershipID: "member-portal",
		CompanyID:    companyID,
	}
	rec, err := svc.CreateRecord(ctx, disclosureapp.CreateRecordRequest{
		Subject: portalSubject,
		Payload: disclosureapp.RecordPayload{
			TypeID:  typeID,
			Title:   "Test Disclosure",
			Content: "Test content",
		},
	})
	if err != nil {
		t.Fatalf("CreateRecord after global workflow configured: %v", err)
	}
	if rec.RecordID == "" {
		t.Fatal("expected non-empty record_id")
	}

	// Step 5: Delete the global workflow.
	if err := svc.CmsDeleteGlobalWorkflow(ctx, disclosureapp.CmsDeleteGlobalWorkflowRequest{
		Subject: cmsSubject,
		TypeID:  typeID,
	}); err != nil {
		t.Fatalf("CmsDeleteGlobalWorkflow: %v", err)
	}

	// After deletion, CountGlobalWorkflowsByTypeId should return 0.
	count, err = repo.CountGlobalWorkflowsByTypeId(ctx, typeID)
	if err != nil {
		t.Fatalf("CountGlobalWorkflowsByTypeId after delete: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected count=0 after delete, got %d", count)
	}
}
