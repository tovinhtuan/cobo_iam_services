package inmemory

import (
	"context"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

// Batch 3: save-draft (UpsertGlobalWorkflow) must NOT reset version pointers.
func TestUpsertGlobalWorkflow_PreservesVersionPointers(t *testing.T) {
	r := NewRepository()
	ctx := context.Background()
	const typeID = "dt-ptr"

	mkReq := func(stage string) disclosureapp.CmsUpsertGlobalWorkflowRequest {
		return disclosureapp.CmsUpsertGlobalWorkflowRequest{
			Subject: disclosureapp.Subject{UserID: "u1", CompanyID: "c1", MembershipID: "m1"},
			TypeID:  typeID,
			Steps: []disclosureapp.GlobalWorkflowStepInput{
				{Stage: stage, DepartmentID: "d1", AssigneeRoleIds: []string{"approver"}, ProcessingDays: 2, DueRule: "T+2", DisplayOrder: 1},
			},
		}
	}

	// initial save
	if _, err := r.UpsertGlobalWorkflow(ctx, mkReq("Review"), "wf-1"); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	// simulate publish/activate having set the pointers (version service writes these)
	pub, act := 6, 5
	r.globalWorkflows[typeID].PublishedVersionNo = &pub
	r.globalWorkflows[typeID].ActiveVersionNo = &act

	// save draft again (new workflow_id) — pointers must survive
	if _, err := r.UpsertGlobalWorkflow(ctx, mkReq("Review v2"), "wf-2"); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	got := r.globalWorkflows[typeID]
	if got.PublishedVersionNo == nil || *got.PublishedVersionNo != 6 {
		t.Fatalf("published_version_no not preserved: %v", got.PublishedVersionNo)
	}
	if got.ActiveVersionNo == nil || *got.ActiveVersionNo != 5 {
		t.Fatalf("active_version_no not preserved: %v", got.ActiveVersionNo)
	}
	// sanity: workflow_id did change (save-draft semantics), proving pointers survived a real re-create.
	if got.WorkflowID != "wf-2" {
		t.Fatalf("workflow_id = %q, want wf-2", got.WorkflowID)
	}
}
