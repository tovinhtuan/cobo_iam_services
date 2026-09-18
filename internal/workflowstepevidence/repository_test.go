package workflowstepevidence_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
	wff "github.com/cobo/cobo_iam_services/internal/workflowfulfillment"
	wffmem "github.com/cobo/cobo_iam_services/internal/workflowfulfillment/memory"
	wse "github.com/cobo/cobo_iam_services/internal/workflowstepevidence"
	"github.com/cobo/cobo_iam_services/internal/workflowstepevidence/memory"
)

func owner() wse.OwnerContext {
	return wse.OwnerContext{
		CompanyID:          "c_001",
		DisclosureRecordID: "rec-1",
		WorkflowInstanceID: "wi-1",
		StepCode:           "step-001",
	}
}

func baseFile(id string, o wse.OwnerContext) wse.EvidenceFile {
	return wse.EvidenceFile{
		ID:                 id,
		CompanyID:          o.CompanyID,
		DisclosureRecordID: o.DisclosureRecordID,
		WorkflowInstanceID: o.WorkflowInstanceID,
		StepCode:           o.StepCode,
		StorageKey:         wse.EvidenceObjectKey(o.CompanyID, id, id+".pdf"),
		OriginalFileName:   id + ".pdf",
		MimeType:           "application/pdf",
		FileSize:           10,
		UploadedBy:         "u1",
		UploadedAt:         time.Now().UTC(),
		LifecycleStatus:    wse.LifecycleActive,
	}
}

type byteStore struct {
	mu sync.Mutex
	m  map[string][]byte
}

func newByteStore() *byteStore { return &byteStore{m: map[string][]byte{}} }

func (s *byteStore) Write(objectKey string, body io.Reader) (int64, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[objectKey] = data
	return int64(len(data)), nil
}

func (s *byteStore) Read(objectKey string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.m[objectKey]
	if !ok {
		return nil, os.ErrNotExist
	}
	return append([]byte(nil), b...), nil
}

func (s *byteStore) Delete(objectKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, objectKey)
	return nil
}

func (s *byteStore) Exists(objectKey string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.m[objectKey]
	return ok
}

func (s *byteStore) keys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.m))
	for k := range s.m {
		out = append(out, k)
	}
	return out
}

func TestMigration0137_SourceShape_NoBackfill(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "../.."))
	up := filepath.Join(root, "migrations/0137_workflow_step_evidence_files.up.sql")
	down := filepath.Join(root, "migrations/0137_workflow_step_evidence_files.down.sql")
	upBody, err := os.ReadFile(up)
	if err != nil {
		t.Fatal(err)
	}
	s := string(upBody)
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS workflow_step_evidence_files",
		"idx_wsef_instance_step_lifecycle",
		"idx_wsef_company_id",
		"idx_wsef_record_step",
		"idx_wsef_supersedes",
		"lifecycle_status",
		"supersedes_file_id",
		"storage_key",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("up migration missing %q", want)
		}
	}
	if strings.Contains(s, "requirement_snapshot_id") {
		t.Fatal("generic table must not have requirement_snapshot_id")
	}
	upper := strings.ToUpper(s)
	if strings.Contains(upper, "UNIQUE(") || strings.Contains(upper, "UNIQUE (") {
		t.Fatal("STEP_SINGLE_FILE_UNIQUE_CONSTRAINT must be false")
	}
	if strings.Contains(upper, "INSERT INTO") {
		t.Fatal("GENERIC_BACKFILL_ROW_COUNT must be 0 — no INSERT in up migration")
	}
	downBody, err := os.ReadFile(down)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(downBody), "DROP TABLE IF EXISTS workflow_step_evidence_files") {
		t.Fatal("down migration must drop evidence table")
	}
}

func TestLifecycleTransitions(t *testing.T) {
	if err := wse.ValidateInitialLifecycle(wse.LifecycleActive); err != nil {
		t.Fatal(err)
	}
	if err := wse.ValidateInitialLifecycle(wse.LifecycleDeleted); err == nil {
		t.Fatal("expected invalid initial")
	}
	if err := wse.ValidateTransition(wse.LifecycleActive, wse.LifecycleDeleted); err != nil {
		t.Fatal(err)
	}
	if err := wse.ValidateTransition(wse.LifecycleActive, wse.LifecycleSuperseded); err != nil {
		t.Fatal(err)
	}
	if err := wse.ValidateTransition(wse.LifecycleDeleted, wse.LifecycleActive); err == nil {
		t.Fatal("expected forbidden")
	}
	if err := wse.ValidateTransition(wse.LifecycleSuperseded, wse.LifecycleDeleted); err == nil {
		t.Fatal("expected forbidden")
	}
}

func TestStorageNamespaceDistinctFromFulfillment(t *testing.T) {
	if wse.StorageNamespace != "workflow-step-evidence" {
		t.Fatalf("namespace=%s", wse.StorageNamespace)
	}
	key := wse.EvidenceObjectKey("c", "wse_1", "a.pdf")
	if !strings.HasPrefix(key, "workflow-step-evidence/") {
		t.Fatalf("key=%s", key)
	}
	if strings.Contains(key, "workflow-step-document-fulfillments") {
		t.Fatal("must not collide with B2 namespace")
	}
}

func TestRepository_CreateListOrderContextQuota(t *testing.T) {
	repo := memory.NewRepository()
	ctx := context.Background()
	o := owner()
	t0 := time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC)
	f1 := baseFile("wse_a", o)
	f1.UploadedAt = t0.Add(time.Second)
	f2 := baseFile("wse_b", o)
	f2.UploadedAt = t0
	if err := repo.CreateActiveInTx(ctx, wse.CreateTxInput{File: f2, Owner: o, RequireNotCompleted: true}); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateActiveInTx(ctx, wse.CreateTxInput{File: f1, Owner: o, RequireNotCompleted: true}); err != nil {
		t.Fatal(err)
	}
	list, err := repo.ListActiveByStep(ctx, o)
	if err != nil || len(list) != 2 {
		t.Fatalf("list=%v err=%v", list, err)
	}
	if list[0].ID != "wse_b" || list[1].ID != "wse_a" {
		t.Fatalf("order=%v %v", list[0].ID, list[1].ID)
	}

	for _, bad := range []wse.OwnerContext{
		{CompanyID: "other", DisclosureRecordID: o.DisclosureRecordID, WorkflowInstanceID: o.WorkflowInstanceID, StepCode: o.StepCode},
		{CompanyID: o.CompanyID, DisclosureRecordID: "other", WorkflowInstanceID: o.WorkflowInstanceID, StepCode: o.StepCode},
		{CompanyID: o.CompanyID, DisclosureRecordID: o.DisclosureRecordID, WorkflowInstanceID: "other", StepCode: o.StepCode},
		{CompanyID: o.CompanyID, DisclosureRecordID: o.DisclosureRecordID, WorkflowInstanceID: o.WorkflowInstanceID, StepCode: "other"},
	} {
		got, _ := repo.GetByIDInContext(ctx, bad, "wse_a")
		if got != nil {
			t.Fatalf("expected nil for bad context %#v", bad)
		}
	}

	del := baseFile("wse_del", o)
	repo.Seed(del)
	_ = repo.DeleteInTx(ctx, wse.DeleteTxInput{FileID: "wse_del", Owner: o, DeletedBy: "u1", DeletedAt: time.Now().UTC(), RequireNotCompleted: true})
	supOld := baseFile("wse_old", o)
	repo.Seed(supOld)
	newF := baseFile("wse_new", o)
	_ = repo.ReplaceInTx(ctx, wse.ReplaceTxInput{OldFileID: "wse_old", NewFile: newF, Owner: o, RequireNotCompleted: true})
	list, _ = repo.ListActiveByStep(ctx, o)
	for _, f := range list {
		if f.ID == "wse_del" || f.ID == "wse_old" {
			t.Fatalf("non-active listed: %s", f.ID)
		}
	}
	oldRow, _ := repo.GetByIDInContext(ctx, o, "wse_old")
	if oldRow == nil || oldRow.LifecycleStatus != wse.LifecycleSuperseded || oldRow.SupersededByFileID != "wse_new" {
		t.Fatalf("lineage %#v", oldRow)
	}
	newRow, _ := repo.GetByIDInContext(ctx, o, "wse_new")
	if newRow == nil || newRow.SupersedesFileID != "wse_old" {
		t.Fatalf("new lineage %#v", newRow)
	}
}

func TestRepository_ActiveLimitAndReplaceAtLimit(t *testing.T) {
	repo := memory.NewRepository()
	ctx := context.Background()
	o := owner()
	for i := 0; i < wse.MaxActiveFilesPerStep; i++ {
		f := baseFile("wse_lim_"+string(rune('a'+i)), o)
		f.ID = "wse_lim_" + string(rune('a'+i))
		if err := repo.CreateActiveInTx(ctx, wse.CreateTxInput{File: f, Owner: o, RequireNotCompleted: true}); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}
	n, _ := repo.CountActiveByStep(ctx, o)
	if n != 10 {
		t.Fatalf("n=%d", n)
	}
	if err := repo.CreateActiveInTx(ctx, wse.CreateTxInput{File: baseFile("wse_overflow", o), Owner: o, RequireNotCompleted: true}); !errors.As(err, &wse.ErrActiveLimitReached{}) {
		t.Fatalf("11th want limit, got %v", err)
	}
	list, _ := repo.ListActiveByStep(ctx, o)
	oldID := list[0].ID
	repl := baseFile("wse_repl", o)
	if err := repo.ReplaceInTx(ctx, wse.ReplaceTxInput{OldFileID: oldID, NewFile: repl, Owner: o, RequireNotCompleted: true}); err != nil {
		t.Fatalf("replace at limit: %v", err)
	}
	n, _ = repo.CountActiveByStep(ctx, o)
	if n != 10 {
		t.Fatalf("after replace n=%d", n)
	}
}

func TestRepository_CompletedBlocksMutation(t *testing.T) {
	repo := memory.NewRepository()
	ctx := context.Background()
	o := owner()
	repo.SetStepCompleted(o.WorkflowInstanceID, o.StepCode, true)
	if err := repo.CreateActiveInTx(ctx, wse.CreateTxInput{File: baseFile("wse_1", o), Owner: o, RequireNotCompleted: true}); !errors.As(err, &wse.ErrStepCompleted{}) {
		t.Fatalf("got %v", err)
	}
}

func TestRepository_ZeroSnapshotCompatible(t *testing.T) {
	repo := memory.NewRepository()
	o := owner()
	if err := repo.CreateActiveInTx(context.Background(), wse.CreateTxInput{
		File: baseFile("wse_z", o), Owner: o, RequireNotCompleted: true,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestB3_GenericDoesNotSatisfyRequirement(t *testing.T) {
	snaps := []workflowapp.DocumentRequirementSnapshot{
		{ID: "req-1", SourceDocID: "d1", Name: "Required", Required: true, Ordinal: 0},
	}
	missing := wff.EvaluateMissingRequiredDocuments(snaps, map[string]int{})
	if len(missing) != 1 {
		t.Fatalf("missing=%v", missing)
	}
	o := owner()
	generic := memory.NewRepository()
	_ = generic.CreateActiveInTx(context.Background(), wse.CreateTxInput{File: baseFile("wse_g", o), Owner: o, RequireNotCompleted: true})
	fulfillEmpty := wffmem.NewRepository()
	err := fulfillEmpty.CompleteStepWithRequiredDocuments(o.CompanyID, o.WorkflowInstanceID, o.StepCode, snaps)
	if err == nil {
		t.Fatal("expected B3 missing; generic must not satisfy requirement")
	}
}

func TestB3_ZeroRequirementCompleteUnchanged(t *testing.T) {
	fulfill := wffmem.NewRepository()
	generic := memory.NewRepository()
	o := owner()
	_ = generic.CreateActiveInTx(context.Background(), wse.CreateTxInput{File: baseFile("wse_g", o), Owner: o, RequireNotCompleted: true})
	err := fulfill.CompleteStepWithRequiredDocuments(o.CompanyID, o.WorkflowInstanceID, o.StepCode, nil)
	if err != nil {
		t.Fatalf("zero snaps Complete must allow: %v", err)
	}
}

func TestB3_SourceHasNoGenericTableReference(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "../.."))
	for _, rel := range []string{
		"internal/workflowfulfillment/required_document_gate.go",
		"internal/workflowfulfillment/mysql/required_document_gate.go",
	} {
		b, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "workflow_step_evidence_files") {
			t.Fatalf("B3 file %s references generic table", rel)
		}
	}
}

func TestFoundation_CreateCompensateLogicalDeleteReplace(t *testing.T) {
	store := newByteStore()
	repo := memory.NewRepository()
	fnd := wse.NewFoundation(repo, store, nil)
	o := owner()
	row, err := fnd.CreateActive(context.Background(), o, "u1", "a.pdf", "application/pdf", bytes.NewReader([]byte("%PDF")), 4)
	if err != nil || row == nil {
		t.Fatalf("create: %v", err)
	}
	if !store.Exists(row.StorageKey) {
		t.Fatal("disk missing")
	}

	repo.SetStepCompleted(o.WorkflowInstanceID, o.StepCode, true)
	_, err = fnd.CreateActive(context.Background(), o, "u1", "b.pdf", "application/pdf", bytes.NewReader([]byte("%PDF2")), 5)
	if !errors.As(err, &wse.ErrStepCompleted{}) {
		t.Fatalf("want completed, got %v", err)
	}
	for _, k := range store.keys() {
		if strings.HasSuffix(k, "/b.pdf") {
			t.Fatalf("orphan after compensate: %s", k)
		}
	}

	repo.SetStepCompleted(o.WorkflowInstanceID, o.StepCode, false)
	repl, err := fnd.ReplaceAppendOnly(context.Background(), o, row.ID, "u1", "c.pdf", "application/pdf", bytes.NewReader([]byte("%PDF3")), 5)
	if err != nil {
		t.Fatal(err)
	}
	old, _ := repo.GetByIDInContext(context.Background(), o, row.ID)
	if old.LifecycleStatus != wse.LifecycleSuperseded || !store.Exists(row.StorageKey) {
		t.Fatalf("old must remain SUPERSEDED with binary: %#v", old)
	}
	if repl.SupersedesFileID != row.ID {
		t.Fatalf("lineage %s", repl.SupersedesFileID)
	}

	if err := fnd.DeleteLogical(context.Background(), o, repl.ID, "u1"); err != nil {
		t.Fatal(err)
	}
	if !store.Exists(repl.StorageKey) {
		t.Fatal("logical delete must retain binary")
	}
}
