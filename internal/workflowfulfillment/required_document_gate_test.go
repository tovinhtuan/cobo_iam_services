package workflowfulfillment_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
	wff "github.com/cobo/cobo_iam_services/internal/workflowfulfillment"
	wffmemory "github.com/cobo/cobo_iam_services/internal/workflowfulfillment/memory"
)

func snap(id, source, name string, required bool, ordinal int) workflowapp.DocumentRequirementSnapshot {
	return workflowapp.DocumentRequirementSnapshot{
		ID:                 id,
		CompanyID:          "co-a",
		DisclosureRecordID: "rec-1",
		WorkflowInstanceID: "wi-1",
		StepCode:           "prep",
		SourceDocID:        source,
		RequirementKey:     source,
		Name:               name,
		Required:           required,
		Ordinal:            ordinal,
	}
}

func activeFile(id, reqID string) wff.FulfillmentFile {
	return wff.FulfillmentFile{
		ID:                    id,
		CompanyID:             "co-a",
		DisclosureRecordID:    "rec-1",
		WorkflowInstanceID:    "wi-1",
		StepCode:              "prep",
		RequirementSnapshotID: reqID,
		StorageKey:            "key/" + id,
		OriginalFileName:      id + ".pdf",
		MimeType:              "application/pdf",
		FileSize:              10,
		UploadedBy:            "m1",
		UploadedAt:            time.Now().UTC(),
		LifecycleStatus:       wff.LifecycleActive,
	}
}

func TestB3_Evaluate_RequiredWithActiveFile(t *testing.T) {
	snaps := []workflowapp.DocumentRequirementSnapshot{snap("r1", "d1", "A", true, 0)}
	missing := wff.EvaluateMissingRequiredDocuments(snaps, map[string]int{"r1": 1})
	if len(missing) != 0 {
		t.Fatalf("expected none missing, got %+v", missing)
	}
}

func TestB3_Evaluate_RequiredWithoutFile(t *testing.T) {
	snaps := []workflowapp.DocumentRequirementSnapshot{snap("r1", "d1", "A", true, 0)}
	missing := wff.EvaluateMissingRequiredDocuments(snaps, map[string]int{})
	if len(missing) != 1 || missing[0].RequirementSnapshotID != "r1" {
		t.Fatalf("got %+v", missing)
	}
}

func TestB3_Evaluate_OptionalWithoutFile(t *testing.T) {
	snaps := []workflowapp.DocumentRequirementSnapshot{snap("r1", "d1", "Opt", false, 0)}
	missing := wff.EvaluateMissingRequiredDocuments(snaps, nil)
	if len(missing) != 0 {
		t.Fatalf("optional must not block, got %+v", missing)
	}
}

func TestB3_Evaluate_EmptyRequirements(t *testing.T) {
	missing := wff.EvaluateMissingRequiredDocuments(nil, map[string]int{"x": 1})
	if missing != nil {
		t.Fatalf("legacy empty snaps → nil, got %+v", missing)
	}
}

func TestB3_Evaluate_AllMissingReturned_OrderedByOrdinal(t *testing.T) {
	snaps := []workflowapp.DocumentRequirementSnapshot{
		snap("rC", "dc", "C", true, 2),
		snap("rA", "da", "A", true, 0),
		snap("rB", "db", "B", true, 1),
	}
	missing := wff.EvaluateMissingRequiredDocuments(snaps, map[string]int{"rA": 1})
	if len(missing) != 2 {
		t.Fatalf("want B+C, got %+v", missing)
	}
	if missing[0].RequirementSnapshotID != "rB" || missing[1].RequirementSnapshotID != "rC" {
		t.Fatalf("order want B then C: %+v", missing)
	}
}

func TestB3_Evaluate_SupersededAndDeletedNotCounted(t *testing.T) {
	// activeCounts only includes ACTIVE; SUPERSEDED/DELETED absent from map.
	snaps := []workflowapp.DocumentRequirementSnapshot{snap("r1", "d1", "A", true, 0)}
	missing := wff.EvaluateMissingRequiredDocuments(snaps, map[string]int{})
	if len(missing) != 1 {
		t.Fatalf("non-ACTIVE must not fulfill: %+v", missing)
	}
}

func TestB3_Evaluate_TemplateFileNotFulfillment(t *testing.T) {
	s := snap("r1", "d1", "A", true, 0)
	s.TemplateFileID = "tpl-1"
	s.TemplateFileName = "sample.pdf"
	missing := wff.EvaluateMissingRequiredDocuments([]workflowapp.DocumentRequirementSnapshot{s}, nil)
	if len(missing) != 1 {
		t.Fatalf("template must not fulfill: %+v", missing)
	}
}

func TestB3_Evaluate_MultipleActiveAllowed(t *testing.T) {
	snaps := []workflowapp.DocumentRequirementSnapshot{snap("r1", "d1", "A", true, 0)}
	missing := wff.EvaluateMissingRequiredDocuments(snaps, map[string]int{"r1": 2})
	if len(missing) != 0 {
		t.Fatalf("got %+v", missing)
	}
}

func TestB3_ErrorContract_CodeStatusPayloadNoLeak(t *testing.T) {
	missing := []wff.MissingRequirement{
		{RequirementSnapshotID: "r2", SourceDocID: "d2", Name: "B"},
		{RequirementSnapshotID: "r3", SourceDocID: "d3", Name: "C"},
	}
	err := wff.NewRequiredDocumentMissingError(missing)
	he, ok := perr.AsHTTPError(err)
	if !ok {
		t.Fatal("expected HTTPError")
	}
	if he.Code != perr.CodeWorkflowStepRequiredDocumentMissing {
		t.Fatalf("code=%s", he.Code)
	}
	if he.HTTPStatus != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d", he.HTTPStatus)
	}
	raw := he.Error() + he.Message
	for _, leak := range []string{"storage_key", "SELECT ", "/var/", "C:\\", "sql.Err"} {
		if strings.Contains(strings.ToLower(raw), strings.ToLower(leak)) {
			t.Fatalf("leak %q in %q", leak, raw)
		}
	}
	list, ok := he.Details["missing_requirements"].([]map[string]any)
	if !ok || len(list) != 2 {
		t.Fatalf("details %+v", he.Details)
	}
	for _, item := range list {
		if _, ok := item["requirement_snapshot_id"]; !ok {
			t.Fatalf("missing field %+v", item)
		}
		if _, ok := item["source_doc_id"]; !ok {
			t.Fatalf("missing field %+v", item)
		}
		if _, ok := item["name"]; !ok {
			t.Fatalf("missing field %+v", item)
		}
		if _, ok := item["storage_key"]; ok {
			t.Fatal("storage_key must not appear")
		}
	}
}

func TestB3_MemoryGate_LegacyNoSnapshotPreservesComplete(t *testing.T) {
	repo := wffmemory.NewRepository()
	if err := repo.CompleteStepWithRequiredDocuments("co-a", "wi-1", "prep", nil); err != nil {
		t.Fatal(err)
	}
	if !repo.IsStepCompleted("wi-1", "prep") {
		t.Fatal("legacy zero snaps must complete")
	}
}

func TestB3_MemoryGate_NoLiveFallback(t *testing.T) {
	// Empty snaps must not invent requirements from live template — gate sees only snaps argument.
	repo := wffmemory.NewRepository()
	if err := repo.CompleteStepWithRequiredDocuments("co-a", "wi-1", "prep", []workflowapp.DocumentRequirementSnapshot{}); err != nil {
		t.Fatal(err)
	}
}

func TestB3_MemoryGate_BlockedZeroMutation(t *testing.T) {
	repo := wffmemory.NewRepository()
	snaps := []workflowapp.DocumentRequirementSnapshot{snap("r1", "d1", "A", true, 0)}
	err := repo.CompleteStepWithRequiredDocuments("co-a", "wi-1", "prep", snaps)
	if err == nil {
		t.Fatal("expected block")
	}
	if repo.IsStepCompleted("wi-1", "prep") {
		t.Fatal("blocked complete must not mutate")
	}
}

func TestB3_MemoryGate_SuccessThenRetryIdempotent(t *testing.T) {
	repo := wffmemory.NewRepository()
	snaps := []workflowapp.DocumentRequirementSnapshot{snap("r1", "d1", "A", true, 0)}
	repo.Seed(activeFile("f1", "r1"))
	if err := repo.CompleteStepWithRequiredDocuments("co-a", "wi-1", "prep", snaps); err != nil {
		t.Fatal(err)
	}
	if err := repo.CompleteStepWithRequiredDocuments("co-a", "wi-1", "prep", snaps); err != nil {
		t.Fatal(err)
	}
}

func TestB3_MemoryGate_OptionalOnlyAllowed(t *testing.T) {
	repo := wffmemory.NewRepository()
	snaps := []workflowapp.DocumentRequirementSnapshot{snap("r1", "d1", "Opt", false, 0)}
	if err := repo.CompleteStepWithRequiredDocuments("co-a", "wi-1", "prep", snaps); err != nil {
		t.Fatal(err)
	}
}

func TestB3_MemoryGate_SupersededOnlyBlocked(t *testing.T) {
	repo := wffmemory.NewRepository()
	f := activeFile("f1", "r1")
	f.LifecycleStatus = wff.LifecycleSuperseded
	repo.Seed(f)
	snaps := []workflowapp.DocumentRequirementSnapshot{snap("r1", "d1", "A", true, 0)}
	if err := repo.CompleteStepWithRequiredDocuments("co-a", "wi-1", "prep", snaps); err == nil {
		t.Fatal("superseded-only must block")
	}
}

func TestB3_MemoryGate_DeletedOnlyBlocked(t *testing.T) {
	repo := wffmemory.NewRepository()
	f := activeFile("f1", "r1")
	f.LifecycleStatus = wff.LifecycleDeleted
	repo.Seed(f)
	snaps := []workflowapp.DocumentRequirementSnapshot{snap("r1", "d1", "A", true, 0)}
	if err := repo.CompleteStepWithRequiredDocuments("co-a", "wi-1", "prep", snaps); err == nil {
		t.Fatal("deleted-only must block")
	}
}

func TestB3_MemoryGate_CrossCompanyFileNotCounted(t *testing.T) {
	repo := wffmemory.NewRepository()
	f := activeFile("f1", "r1")
	f.CompanyID = "co-other"
	repo.Seed(f)
	snaps := []workflowapp.DocumentRequirementSnapshot{snap("r1", "d1", "A", true, 0)}
	if err := repo.CompleteStepWithRequiredDocuments("co-a", "wi-1", "prep", snaps); err == nil {
		t.Fatal("cross-company must not fulfill")
	}
}

func TestB3_MemoryGate_OtherStepFileNotCounted(t *testing.T) {
	repo := wffmemory.NewRepository()
	f := activeFile("f1", "r1")
	f.StepCode = "other"
	repo.Seed(f)
	snaps := []workflowapp.DocumentRequirementSnapshot{snap("r1", "d1", "A", true, 0)}
	if err := repo.CompleteStepWithRequiredDocuments("co-a", "wi-1", "prep", snaps); err == nil {
		t.Fatal("other step must not fulfill")
	}
}

func TestB3_MemoryGate_OtherInstanceFileNotCounted(t *testing.T) {
	repo := wffmemory.NewRepository()
	f := activeFile("f1", "r1")
	f.WorkflowInstanceID = "wi-other"
	repo.Seed(f)
	snaps := []workflowapp.DocumentRequirementSnapshot{snap("r1", "d1", "A", true, 0)}
	if err := repo.CompleteStepWithRequiredDocuments("co-a", "wi-1", "prep", snaps); err == nil {
		t.Fatal("other instance must not fulfill")
	}
}

func TestB3_Race_CompleteVsDeleteLastFile(t *testing.T) {
	snaps := []workflowapp.DocumentRequirementSnapshot{snap("r1", "d1", "A", true, 0)}
	var completeOK, deleteOK, completeBlocked, deleteDenied atomic.Int64
	for i := 0; i < 40; i++ {
		repo := wffmemory.NewRepository()
		repo.Seed(activeFile("f1", "r1"))
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			err := repo.CompleteStepWithRequiredDocuments("co-a", "wi-1", "prep", snaps)
			if err == nil {
				completeOK.Add(1)
			} else if _, ok := perr.AsHTTPError(err); ok {
				completeBlocked.Add(1)
			} else {
				t.Errorf("unexpected complete err: %v", err)
			}
		}()
		go func() {
			defer wg.Done()
			err := repo.DeleteInTx(context.Background(), wff.DeleteTxInput{
				CompanyID:           "co-a",
				WorkflowInstanceID:  "wi-1",
				StepCode:            "prep",
				FileID:              "f1",
				DeletedBy:           "m1",
				DeletedAt:           time.Now().UTC(),
				RequireNotCompleted: true,
			})
			if err == nil {
				deleteOK.Add(1)
			} else {
				switch err.(type) {
				case wff.ErrStepCompleted, wff.ErrNotActive:
					deleteDenied.Add(1)
				default:
					t.Errorf("unexpected delete err: %v", err)
				}
			}
		}()
		wg.Wait()
		completed := repo.IsStepCompleted("wi-1", "prep")
		active, _ := repo.CountActiveByRequirement(context.Background(), "r1")
		if completed && active == 0 {
			t.Fatalf("invalid race outcome: completed with 0 active")
		}
		if !completed && active != 0 && active != 1 {
			t.Fatalf("unexpected active=%d completed=%v", active, completed)
		}
	}
	if completeOK.Load()+completeBlocked.Load() == 0 {
		t.Fatal("no complete outcomes")
	}
	_ = deleteOK
	_ = deleteDenied
}

func TestB3_Race_CompleteVsUpload(t *testing.T) {
	snaps := []workflowapp.DocumentRequirementSnapshot{snap("r1", "d1", "A", true, 0)}
	var blocked, ok atomic.Int64
	for i := 0; i < 40; i++ {
		repo := wffmemory.NewRepository()
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			err := repo.CompleteStepWithRequiredDocuments("co-a", "wi-1", "prep", snaps)
			if err == nil {
				ok.Add(1)
			} else {
				blocked.Add(1)
			}
		}()
		go func(n int) {
			defer wg.Done()
			f := activeFile(fmt.Sprintf("up-%d", n), "r1")
			_ = repo.UploadInTx(context.Background(), wff.UploadTxInput{
				RequirementSnapshotID: "r1",
				WorkflowInstanceID:    "wi-1",
				StepCode:              "prep",
				File:                  f,
				MaxActive:             10,
				RequireNotCompleted:   true,
			})
		}(i)
		wg.Wait()
		completed := repo.IsStepCompleted("wi-1", "prep")
		active, _ := repo.CountActiveByRequirement(context.Background(), "r1")
		if completed && active < 1 {
			t.Fatalf("complete succeeded without ACTIVE file")
		}
	}
	if blocked.Load() == 0 && ok.Load() == 0 {
		t.Fatal("no outcomes")
	}
}

func TestB3_Race_CompleteVsReplace(t *testing.T) {
	snaps := []workflowapp.DocumentRequirementSnapshot{snap("r1", "d1", "A", true, 0)}
	for i := 0; i < 40; i++ {
		repo := wffmemory.NewRepository()
		repo.Seed(activeFile("old", "r1"))
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = repo.CompleteStepWithRequiredDocuments("co-a", "wi-1", "prep", snaps)
		}()
		go func() {
			defer wg.Done()
			_ = repo.ReplaceInTx(context.Background(), wff.ReplaceTxInput{
				CompanyID:             "co-a",
				WorkflowInstanceID:    "wi-1",
				StepCode:              "prep",
				RequirementSnapshotID: "r1",
				OldFileID:             "old",
				NewFile:               activeFile("new", "r1"),
				RequireNotCompleted:   true,
			})
		}()
		wg.Wait()
		if repo.IsStepCompleted("wi-1", "prep") {
			active, _ := repo.CountActiveByRequirement(context.Background(), "r1")
			if active < 1 {
				t.Fatal("replace must not leave zero ACTIVE visible after successful complete")
			}
		}
	}
}

func TestB3_RetryAfterUpload(t *testing.T) {
	repo := wffmemory.NewRepository()
	snaps := []workflowapp.DocumentRequirementSnapshot{snap("r1", "d1", "A", true, 0)}
	if err := repo.CompleteStepWithRequiredDocuments("co-a", "wi-1", "prep", snaps); err == nil {
		t.Fatal("expected block")
	}
	repo.Seed(activeFile("f1", "r1"))
	if err := repo.CompleteStepWithRequiredDocuments("co-a", "wi-1", "prep", snaps); err != nil {
		t.Fatal(err)
	}
}
