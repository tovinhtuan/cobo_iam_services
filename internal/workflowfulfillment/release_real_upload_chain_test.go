package workflowfulfillment_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
	wff "github.com/cobo/cobo_iam_services/internal/workflowfulfillment"
	wffmemory "github.com/cobo/cobo_iam_services/internal/workflowfulfillment/memory"
)

// Release chain: authored documents[] → Project → List/Upload (no SQL B1 seed) → B3 evaluate clears.
func TestRelease_RealProjected_TenantUploadThenCompleteGate(t *testing.T) {
	projected := workflowapp.ProjectDocumentRequirementSnapshots([]disclosureapp.WorkflowStepDTO{{
		StepID: "step-001",
		Documents: []disclosureapp.WorkflowDocumentDTO{
			{DocID: "qa-ev-r1-obligation-memo", Name: "R1", Required: true},
			{DocID: "qa-ev-r2-draft-disclosure", Name: "R2", Required: true},
			{DocID: "qa-ev-r3-supporting-note", Name: "R3", Required: false},
		},
	}})
	if len(projected) != 3 {
		t.Fatalf("projected=%d", len(projected))
	}
	byID := map[string]workflowapp.DocumentRequirementSnapshot{}
	byStep := make([]workflowapp.DocumentRequirementSnapshot, 0, len(projected))
	for i, d := range projected {
		d.ID = "snap-" + d.SourceDocID
		d.CompanyID = "co-a"
		d.WorkflowInstanceID = "wi-1"
		d.DisclosureRecordID = "rec-1"
		d.Ordinal = i
		byID[d.ID] = d
		byStep = append(byStep, d)
	}

	today := time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC)
	deadline := &fakeDeadline{
		viewOK:   true,
		mutateOK: true,
		wf: wff.WorkflowContext{
			WorkflowInstanceID: "wi-1",
			CompanyID:          "co-a",
			RecordID:           "rec-1",
			T0Date:             today.AddDate(0, 0, -1),
			Timezone:           "Asia/Ho_Chi_Minh",
			SnapshotJSONSteps: []workflowapp.StepSnapshot{{
				StepID: "step-001", StepCode: "step-001", Stage: "S1", DueRule: "T+5",
			}},
		},
	}
	snaps := &fakeSnapshots{byID: byID, byStep: byStep}
	files := wffmemory.NewRepository()
	store := newMemStorage()
	svc := wff.NewService(deadline, snaps, files, store, nil).WithNow(func() time.Time { return today })
	sub := wff.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "co-a"}

	list, err := svc.ListRequirements(context.Background(), sub, "rec-1", "step-001")
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Requirements) != 3 {
		t.Fatalf("list=%d", len(list.Requirements))
	}
	if !list.Requirements[0].Capabilities.CanUpload {
		t.Fatal("expected can_upload")
	}

	r1 := byStep[0].ID
	r2 := byStep[1].ID
	up1, err := svc.Upload(context.Background(), sub, "rec-1", "step-001", r1, "r1.pdf", "application/pdf", bytes.NewReader([]byte("%PDF-1.4 r1")), 11)
	if err != nil {
		t.Fatalf("upload r1: %v", err)
	}
	if up1.File.FileID == "" || !store.Exists((func() string {
		got, _ := files.GetActiveByID(context.Background(), "co-a", up1.File.FileID)
		if got == nil {
			t.Fatal("no ACTIVE row")
		}
		return got.StorageKey
	})()) {
		t.Fatal("binary missing")
	}

	counts := map[string]int{r1: 1}
	if miss := wff.EvaluateMissingRequiredDocuments(byStep, counts); len(miss) != 1 {
		t.Fatalf("missing after r1=%#v", miss)
	}

	if _, err := svc.Upload(context.Background(), sub, "rec-1", "step-001", r2, "r2.pdf", "application/pdf", bytes.NewReader([]byte("%PDF-1.4 r2")), 11); err != nil {
		t.Fatalf("upload r2: %v", err)
	}
	counts[r2] = 1
	if miss := wff.EvaluateMissingRequiredDocuments(byStep, counts); len(miss) != 0 {
		t.Fatalf("optional missing must not block: %#v", miss)
	}
}
