package disclosure

import (
	"context"
	"errors"
	"testing"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
)

type duplicatePeriodicRecordService struct {
	createCalls int
	getCalls    int
	record      disclosureapp.RecordDTO
}

func (s *duplicatePeriodicRecordService) CreateRecord(context.Context, disclosureapp.CreateRecordRequest) (*disclosureapp.RecordDTO, error) {
	s.createCalls++
	return nil, disclosureapp.ErrDuplicateRecordID
}

func (s *duplicatePeriodicRecordService) GetRecord(context.Context, disclosureapp.GetRecordRequest) (*disclosureapp.RecordDTO, error) {
	s.getCalls++
	copy := s.record
	return &copy, nil
}

func (s *duplicatePeriodicRecordService) SubmitRecord(context.Context, disclosureapp.SubmitRecordRequest) (*disclosureapp.RecordDTO, error) {
	return nil, nil
}

func (s *duplicatePeriodicRecordService) GetEffectiveWorkflow(context.Context, disclosureapp.GetEffectiveWorkflowRequest) (*disclosureapp.GetEffectiveWorkflowResponse, error) {
	return nil, nil
}

type fakeWorkflowMaterializer struct {
	calls int
	req   workflowapp.CreateWorkflowInstanceRequest
}

func (w *fakeWorkflowMaterializer) CreateWorkflowInstanceInternal(_ context.Context, req workflowapp.CreateWorkflowInstanceRequest) (*workflowapp.WorkflowInstanceDTO, error) {
	w.calls++
	w.req = req
	return &workflowapp.WorkflowInstanceDTO{WorkflowInstanceID: "wf-resumed"}, nil
}

// TestMapWorkflowSource_PassesThroughAllThreeValues is the regression guard for Architecture
// Integrity Fix A: a global_workflow-sourced ad-hoc record must NOT be recorded as
// global_template (mirrors the identical fix in internal/disclosure/infra/workflow/bootstrap.go).
func TestMapWorkflowSource_PassesThroughAllThreeValues(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"global_workflow", "global_workflow"},
		{"global_template", "global_template"},
		{"company_override", "company_override"},
		{"", "global_template"}, // defensive default only for an unexpected empty value
	}
	for _, c := range cases {
		if got := mapWorkflowSource(c.input); got != c.want {
			t.Errorf("mapWorkflowSource(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestResolveWorkflowSnapshotForMaterialize_V2SkipsEffectiveWorkflow(t *testing.T) {
	snap := &adhocapp.ProposalWorkflowSnapshot{
		SchemaVersion: 2,
		Frozen:        true,
		Steps: []adhocapp.ProposalWorkflowStep{
			{ID: "ps-1", Order: 1, Name: "A", ProcessingDays: 3, DepartmentID: "d1", AssigneeMembershipID: "assignee-b"},
			{ID: "ps-2", Order: 2, Name: "B", ProcessingDays: 1, DepartmentID: "d1", AssigneeMembershipID: "assignee-c"},
		},
	}
	// nil disclosure service is intentional: v2 path must not call GetEffectiveWorkflow.
	got, err := resolveWorkflowSnapshotForMaterialize(context.Background(), nil, disclosureapp.Subject{CompanyID: "co"}, "type-ignored", adhocapp.CreateRecordOpts{
		ProposalWorkflow: snap,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.mode != adhocapp.MaterializationModeV2Snapshot || got.effectiveWorkflowN != 0 {
		t.Fatalf("%#v", got)
	}
	if got.workflowSource != workflowapp.WorkflowSourceProposalSnapshotV2 {
		t.Fatalf("source=%q", got.workflowSource)
	}
	if len(got.snapshot) != 2 || got.snapshot[0].StepID != "ps-1" || got.snapshot[0].ProcessingDays != 3 {
		t.Fatalf("%#v", got.snapshot)
	}
	if got.firstTaskAssignee != "assignee-b" {
		t.Fatalf("first assignee=%q", got.firstTaskAssignee)
	}
}

func TestResolveWorkflowSnapshotForMaterialize_V2UnfrozenFails(t *testing.T) {
	snap := &adhocapp.ProposalWorkflowSnapshot{
		SchemaVersion: 2,
		Frozen:        false,
		Steps: []adhocapp.ProposalWorkflowStep{
			{ID: "ps-1", Order: 1, Name: "A", ProcessingDays: 1, DepartmentID: "d1", AssigneeMembershipID: "m1"},
		},
	}
	_, err := resolveWorkflowSnapshotForMaterialize(context.Background(), nil, disclosureapp.Subject{}, "t", adhocapp.CreateRecordOpts{ProposalWorkflow: snap})
	if err == nil {
		t.Fatal("expected fail")
	}
}

func TestResolveWorkflowSnapshotForMaterialize_V3SkipsEffectiveWorkflow(t *testing.T) {
	snap := &adhocapp.ProposalWorkflowSnapshot{
		SchemaVersion: 3,
		Frozen:        true,
		Steps: []adhocapp.ProposalWorkflowStep{
			{ID: "ps-1", Order: 1, Name: "A", ProcessingDays: 3, DepartmentID: "d1", AssigneeMembershipIDs: []string{"m1", "m2"}},
			{ID: "ps-2", Order: 2, Name: "B", ProcessingDays: 1, DepartmentID: "d2", AssigneeMembershipIDs: []string{"m3"}},
		},
	}
	got, err := resolveWorkflowSnapshotForMaterialize(context.Background(), nil, disclosureapp.Subject{CompanyID: "co"}, "type-ignored", adhocapp.CreateRecordOpts{
		ProposalWorkflow: snap,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.mode != adhocapp.MaterializationModeV3Snapshot || got.effectiveWorkflowN != 0 {
		t.Fatalf("%#v", got)
	}
	if got.workflowSource != workflowapp.WorkflowSourceProposalSnapshotV3 {
		t.Fatalf("source=%q", got.workflowSource)
	}
	if got.firstTaskAssignee != "" {
		t.Fatalf("v3 must not set singular first assignee, got %q", got.firstTaskAssignee)
	}
	if len(got.firstTaskAssignees) != 2 || got.firstTaskAssignees[0] != "m1" {
		t.Fatalf("%#v", got.firstTaskAssignees)
	}
	if got.snapshot[0].AssigneeMembershipID != "" || len(got.snapshot[0].AssigneeMembershipIDs) != 2 {
		t.Fatalf("%#v", got.snapshot[0])
	}
}

func TestCreateAndSubmitRecordWithOpts_duplicatePeriodicRecordResumesWorkflow(t *testing.T) {
	records := &duplicatePeriodicRecordService{record: disclosureapp.RecordDTO{
		RecordID: "periodic-record", CompanyID: "co-1", TypeID: "type-1",
	}}
	workflow := &fakeWorkflowMaterializer{}
	adapter := &RecordCreatorAdapter{svc: records, workflow: workflow, workflowOn: true}
	snapshot := &adhocapp.ProposalWorkflowSnapshot{
		SchemaVersion: 2,
		Frozen:        true,
		Steps: []adhocapp.ProposalWorkflowStep{{
			ID: "step-1", Order: 1, Name: "Review", ProcessingDays: 1,
			DepartmentID: "d-1", AssigneeMembershipID: "m-1",
		}},
	}

	recordID, workflowID, err := adapter.CreateAndSubmitRecordWithOpts(context.Background(), "co-1", "type-1", "m-1", "title", nil, adhocapp.CreateRecordOpts{
		RecordID:                  "periodic-record",
		SkipCompanySubmit:         true,
		ResumeWorkflowOnDuplicate: true,
		ProposalWorkflow:          snapshot,
	})
	if err != nil {
		t.Fatal(err)
	}
	if recordID != "periodic-record" || workflowID != "wf-resumed" {
		t.Fatalf("record=%q workflow=%q", recordID, workflowID)
	}
	if records.createCalls != 1 || records.getCalls != 1 || workflow.calls != 1 {
		t.Fatalf("create=%d get=%d workflow=%d", records.createCalls, records.getCalls, workflow.calls)
	}
	if workflow.req.RecordID != "periodic-record" {
		t.Fatalf("workflow record=%q", workflow.req.RecordID)
	}
}

func TestCreateAndSubmitPeriodicRecord_requiresWorkflowBeforeRecordWrite(t *testing.T) {
	adapter := &RecordCreatorAdapter{}
	recordID, workflowID, err := adapter.CreateAndSubmitPeriodicRecord(context.Background(), "cycle-1", "co-1", "type-1", "m-1", "title", nil, "2026-10-03")
	if !errors.Is(err, errPeriodicWorkflowUnavailable) {
		t.Fatalf("err=%v", err)
	}
	if recordID != "" || workflowID != "" {
		t.Fatalf("record=%q workflow=%q", recordID, workflowID)
	}
}
