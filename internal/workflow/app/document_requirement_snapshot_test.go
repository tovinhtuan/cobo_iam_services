package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

func TestMigration0135_SourceShape_NoBackfill(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "../../.."))
	up := filepath.Join(root, "migrations/0135_workflow_step_document_requirement_snapshots.up.sql")
	down := filepath.Join(root, "migrations/0135_workflow_step_document_requirement_snapshots.down.sql")
	upBody, err := os.ReadFile(up)
	if err != nil {
		t.Fatal(err)
	}
	s := string(upBody)
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS workflow_step_document_requirement_snapshots",
		"uq_wsdrs_instance_step_doc",
		"idx_wsdrs_instance_step",
		"idx_wsdrs_company_record_step",
		"source_doc_id",
		"requirement_key",
		"template_file_id",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("up migration missing %q", want)
		}
	}
	if strings.Contains(strings.ToUpper(s), "INSERT INTO") {
		t.Fatal("LEGACY_SNAPSHOT_BACKFILL_ROWS_CREATED must be 0 — no INSERT in up migration")
	}
	downBody, err := os.ReadFile(down)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(downBody), "DROP TABLE IF EXISTS workflow_step_document_requirement_snapshots") {
		t.Fatal("down migration must drop snapshot table")
	}
}

func TestProjectDocumentRequirementSnapshots_BasicAndEmpty(t *testing.T) {
	steps := []disclosureapp.WorkflowStepDTO{
		{
			StepID: "prep",
			Documents: []disclosureapp.WorkflowDocumentDTO{
				{DocID: "doc-a", Name: "A", Required: true, TemplateFileID: "wdt_1", TemplateFileName: "a.pdf"},
				{DocID: "doc-b", Name: "B", Required: false},
			},
		},
	}
	got := ProjectDocumentRequirementSnapshots(steps)
	if len(got) != 2 {
		t.Fatalf("len=%d want 2", len(got))
	}
	if got[0].SourceDocID != "doc-a" || !got[0].Required || got[0].TemplateFileID != "wdt_1" || got[0].Ordinal != 0 {
		t.Fatalf("row0=%#v", got[0])
	}
	if got[1].SourceDocID != "doc-b" || got[1].Required || got[1].Ordinal != 1 {
		t.Fatalf("row1=%#v", got[1])
	}
	if len(ProjectDocumentRequirementSnapshots([]disclosureapp.WorkflowStepDTO{{StepID: "x", Documents: nil}})) != 0 {
		t.Fatal("empty documents must project zero rows")
	}
	if ProjectDocumentRequirementSnapshots(nil) != nil && len(ProjectDocumentRequirementSnapshots(nil)) != 0 {
		t.Fatal("nil steps")
	}
}

func TestProjectDocumentRequirementSnapshots_OverrideEmptyDoesNotInvent(t *testing.T) {
	// Effective override with documents=[] must snapshot zero — no template fallback at projection.
	overrideSteps := []disclosureapp.WorkflowStepDTO{
		{StepID: "s1", Documents: []disclosureapp.WorkflowDocumentDTO{}},
	}
	if n := ProjectDocumentRequirementSnapshots(overrideSteps); len(n) != 0 {
		t.Fatalf("empty override docs projected %d", len(n))
	}
}

func TestCreateWorkflowInstanceInternal_PersistsDocumentRequirements(t *testing.T) {
	repo := &fakeWorkflowRepository{}
	svc := NewService(repo, nil, &seqIDGen{}, WithFlags(Flags{SnapshotEnabled: true}))
	docs := ProjectDocumentRequirementSnapshots([]disclosureapp.WorkflowStepDTO{{
		StepID: "prep",
		Documents: []disclosureapp.WorkflowDocumentDTO{
			{DocID: "doc-a", Name: "Required Doc", Required: true, TemplateFileID: "wdt_x", TemplateFileName: "x.xlsx"},
			{DocID: "doc-b", Name: "Optional Doc", Required: false},
		},
	}})
	_, err := svc.CreateWorkflowInstanceInternal(context.Background(), CreateWorkflowInstanceRequest{
		Subject:              Subject{UserID: "u", MembershipID: "m", CompanyID: "c"},
		RecordID:             "rec-1",
		Snapshot:             []StepSnapshot{{StepID: "prep", StepCode: "prep", DisplayOrder: 1}},
		DocumentRequirements: docs,
		WorkflowSource:       "global_template",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.docReqs) != 2 {
		t.Fatalf("docReqs=%d want 2", len(repo.docReqs))
	}
	listed, err := repo.ListDocumentRequirementSnapshotsByInstanceStep(context.Background(), "c", repo.createdInstance.WorkflowInstanceID, "prep")
	if err != nil || len(listed) != 2 {
		t.Fatalf("list=%d err=%v", len(listed), err)
	}
	if listed[0].DisclosureRecordID != "rec-1" || listed[0].Name != "Required Doc" || !listed[0].Required {
		t.Fatalf("listed0=%#v", listed[0])
	}
	if listed[1].Required || listed[1].TemplateFileID != "" {
		t.Fatalf("listed1=%#v", listed[1])
	}
}

func TestCreateWorkflowInstanceInternal_DocumentRequirementInsertFailureRollsBack(t *testing.T) {
	repo := &fakeWorkflowRepository{failDocRequirementInsert: fmt.Errorf("doc snapshot failed")}
	svc := NewService(repo, nil, fakeWorkflowIDGen{}, WithFlags(Flags{SnapshotEnabled: true}))
	_, err := svc.CreateWorkflowInstanceInternal(context.Background(), CreateWorkflowInstanceRequest{
		Subject:  Subject{UserID: "u", MembershipID: "m", CompanyID: "c"},
		RecordID: "rec-fail",
		Snapshot: []StepSnapshot{{StepID: "s1", StepCode: "s1", DisplayOrder: 1}},
		DocumentRequirements: []DocumentRequirementSnapshot{
			{StepCode: "s1", SourceDocID: "d1", RequirementKey: "d1", Name: "N", Required: true},
		},
	})
	if err == nil {
		t.Fatal("expected failure")
	}
	if len(repo.instances) != 0 {
		t.Fatal("instance must not commit when document requirement insert fails")
	}
	if len(repo.docReqs) != 0 {
		t.Fatal("no orphan doc rows")
	}
}

func TestCreateWorkflowInstanceInternal_IdempotentDocReqsOnFake(t *testing.T) {
	repo := &fakeWorkflowRepository{}
	svc := NewService(repo, nil, &seqIDGen{}, WithFlags(Flags{SnapshotEnabled: true}))
	docs := []DocumentRequirementSnapshot{
		{StepCode: "s1", SourceDocID: "d1", RequirementKey: "d1", Name: "N", Required: true, Ordinal: 0},
	}
	req := CreateWorkflowInstanceRequest{
		Subject:              Subject{UserID: "u", MembershipID: "m", CompanyID: "c"},
		RecordID:             "rec-idem",
		Snapshot:             []StepSnapshot{{StepID: "s1", StepCode: "s1", DisplayOrder: 1}},
		DocumentRequirements: docs,
	}
	if _, err := svc.CreateWorkflowInstanceInternal(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	// Second create gets new instance id (new UUID) — separate snapshots; no duplicate within one instance.
	if _, err := svc.CreateWorkflowInstanceInternal(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	byInstance := map[string]int{}
	for _, row := range repo.docReqs {
		byInstance[row.WorkflowInstanceID]++
	}
	if len(byInstance) != 2 {
		t.Fatalf("want 2 instances with docs, got %#v", byInstance)
	}
	for id, n := range byInstance {
		if n != 1 {
			t.Fatalf("instance %s has %d docs", id, n)
		}
	}
}

func TestDocumentRequirements_ImmutabilityAfterProjectionChange(t *testing.T) {
	v1Steps := []disclosureapp.WorkflowStepDTO{{
		StepID: "s1",
		Documents: []disclosureapp.WorkflowDocumentDTO{
			{DocID: "doc-a", Name: "A", Required: true},
			{DocID: "doc-b", Name: "B", Required: true},
		},
	}}
	repo := &fakeWorkflowRepository{}
	svc := NewService(repo, nil, &seqIDGen{}, WithFlags(Flags{SnapshotEnabled: true}))
	w1Docs := ProjectDocumentRequirementSnapshots(v1Steps)
	created, err := svc.CreateWorkflowInstanceInternal(context.Background(), CreateWorkflowInstanceRequest{
		Subject:              Subject{UserID: "u", MembershipID: "m", CompanyID: "c"},
		RecordID:             "rec-w1",
		Snapshot:             []StepSnapshot{{StepID: "s1", StepCode: "s1", DisplayOrder: 1}},
		DocumentRequirements: w1Docs,
	})
	if err != nil {
		t.Fatal(err)
	}
	v2Steps := []disclosureapp.WorkflowStepDTO{{
		StepID: "s1",
		Documents: []disclosureapp.WorkflowDocumentDTO{
			{DocID: "doc-a", Name: "A", Required: true},
			{DocID: "doc-c", Name: "C", Required: true},
		},
	}}
	_ = ProjectDocumentRequirementSnapshots(v2Steps)
	listed, _ := repo.ListDocumentRequirementSnapshotsByInstanceStep(context.Background(), "c", created.WorkflowInstanceID, "s1")
	if len(listed) != 2 || listed[0].SourceDocID != "doc-a" || listed[1].SourceDocID != "doc-b" {
		t.Fatalf("W1 must remain A+B, got %#v", listed)
	}
	w2Docs := ProjectDocumentRequirementSnapshots(v2Steps)
	created2, err := svc.CreateWorkflowInstanceInternal(context.Background(), CreateWorkflowInstanceRequest{
		Subject:              Subject{UserID: "u", MembershipID: "m", CompanyID: "c"},
		RecordID:             "rec-w2",
		Snapshot:             []StepSnapshot{{StepID: "s1", StepCode: "s1", DisplayOrder: 1}},
		DocumentRequirements: w2Docs,
	})
	if err != nil {
		t.Fatal(err)
	}
	listed2, _ := repo.ListDocumentRequirementSnapshotsByInstanceStep(context.Background(), "c", created2.WorkflowInstanceID, "s1")
	if len(listed2) != 2 || listed2[1].SourceDocID != "doc-c" {
		t.Fatalf("W2 must be A+C, got %#v", listed2)
	}
}

func TestDocumentRequirements_CompanyOverrideProjectionUsesOverrideDocsOnly(t *testing.T) {
	override := []disclosureapp.WorkflowStepDTO{{
		StepID: "s1",
		Documents: []disclosureapp.WorkflowDocumentDTO{
			{DocID: "doc-b", Name: "Override B", Required: true},
		},
	}}
	got := ProjectDocumentRequirementSnapshots(override)
	if len(got) != 1 || got[0].SourceDocID != "doc-b" {
		t.Fatalf("override projection=%#v", got)
	}
}
