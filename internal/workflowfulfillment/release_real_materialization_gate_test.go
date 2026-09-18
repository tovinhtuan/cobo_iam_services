package workflowfulfillment_test

import (
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
	wff "github.com/cobo/cobo_iam_services/internal/workflowfulfillment"
)

// Release invariant: start from authored documents[] projection (no SQL B1 seed),
// then prove B3 gate blocks until required ACTIVE fulfillments exist.
func TestRelease_RealProjectedSnapshots_CompleteGateAndOptional(t *testing.T) {
	docs := workflowapp.ProjectDocumentRequirementSnapshots([]disclosureapp.WorkflowStepDTO{{
		StepID: "step-001",
		Documents: []disclosureapp.WorkflowDocumentDTO{
			{DocID: "qa-ev-r1-obligation-memo", Name: "R1", Required: true},
			{DocID: "qa-ev-r2-draft-disclosure", Name: "R2", Required: true},
			{DocID: "qa-ev-r3-supporting-note", Name: "R3", Required: false},
		},
	}})
	if len(docs) != 3 {
		t.Fatalf("projected=%d", len(docs))
	}
	for i := range docs {
		docs[i].ID = "snap-" + docs[i].SourceDocID
		docs[i].CompanyID = "c_001"
		docs[i].WorkflowInstanceID = "wi-rel"
		docs[i].DisclosureRecordID = "rec-rel"
	}

	// Missing all required → block
	missing := wff.EvaluateMissingRequiredDocuments(docs, map[string]int{})
	if len(missing) != 2 {
		t.Fatalf("missing=%d want 2", len(missing))
	}

	// Only R1 ACTIVE → still block on R2
	missing = wff.EvaluateMissingRequiredDocuments(docs, map[string]int{docs[0].ID: 1})
	if len(missing) != 1 || missing[0].SourceDocID != "qa-ev-r2-draft-disclosure" {
		t.Fatalf("missing after R1=%#v", missing)
	}

	// R1+R2 ACTIVE, optional R3 missing → Complete allowed
	missing = wff.EvaluateMissingRequiredDocuments(docs, map[string]int{
		docs[0].ID: 1,
		docs[1].ID: 1,
	})
	if len(missing) != 0 {
		t.Fatalf("optional missing must not block: %#v", missing)
	}
}

func TestRelease_EmptyAuthoring_NoGateBlock(t *testing.T) {
	docs := workflowapp.ProjectDocumentRequirementSnapshots([]disclosureapp.WorkflowStepDTO{
		{StepID: "step-001", Documents: []disclosureapp.WorkflowDocumentDTO{}},
	})
	missing := wff.EvaluateMissingRequiredDocuments(docs, nil)
	if len(missing) != 0 {
		t.Fatalf("empty authoring legacy: missing=%#v", missing)
	}
}
