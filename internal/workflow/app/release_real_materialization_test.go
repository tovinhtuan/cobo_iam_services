package app

import (
	"context"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

// Release-grade chain: authored documents[] → Project → CreateWorkflowInstanceInternal → persisted snaps.
// No SQL seed. Proves B1 real materialization path used by EnsureOnSubmit / periodic creator.
func TestRelease_RealAuthoringMaterializationPersistsSnapshots(t *testing.T) {
	repo := &fakeWorkflowRepository{}
	svc := NewService(repo, nil, &seqIDGen{}, WithFlags(Flags{SnapshotEnabled: true}))
	steps := []disclosureapp.WorkflowStepDTO{{
		StepID: "step-001",
		Documents: []disclosureapp.WorkflowDocumentDTO{
			{DocID: "qa-ev-r1-obligation-memo", Name: "R1", Required: true},
			{DocID: "qa-ev-r2-draft-disclosure", Name: "R2", Required: true},
			{DocID: "qa-ev-r3-supporting-note", Name: "R3", Required: false},
		},
	}}
	docs := ProjectDocumentRequirementSnapshots(steps)
	if len(docs) != 3 {
		t.Fatalf("project len=%d", len(docs))
	}
	created, err := svc.CreateWorkflowInstanceInternal(context.Background(), CreateWorkflowInstanceRequest{
		Subject:              Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c_001"},
		RecordID:             "rec-release-1",
		Snapshot:             []StepSnapshot{{StepID: "step-001", StepCode: "step-001", DisplayOrder: 1}},
		DocumentRequirements: docs,
		WorkflowSource:       "global_template",
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.ListDocumentRequirementSnapshotsByInstanceStep(context.Background(), "c_001", created.WorkflowInstanceID, "step-001")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("persisted=%d want 3", len(got))
	}
	if got[0].SourceDocID != "qa-ev-r1-obligation-memo" || !got[0].Required {
		t.Fatalf("row0=%#v", got[0])
	}
	if got[2].SourceDocID != "qa-ev-r3-supporting-note" || got[2].Required {
		t.Fatalf("row2=%#v", got[2])
	}
}

func TestRelease_EmptyAuthoringDocumentsProjectZeroSnapshots(t *testing.T) {
	docs := ProjectDocumentRequirementSnapshots([]disclosureapp.WorkflowStepDTO{
		{StepID: "step-001", Documents: []disclosureapp.WorkflowDocumentDTO{}},
	})
	if len(docs) != 0 {
		t.Fatalf("empty authoring must project 0, got %d", len(docs))
	}
}
