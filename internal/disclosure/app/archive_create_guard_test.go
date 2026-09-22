package app_test

import (
	"context"
	"net/http"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

func TestCreateRecord_RejectsArchivedGlobalTemplate(t *testing.T) {
	repo := inmemory.NewRepository()
	typeID := "dt-periodic-financial"
	if _, err := repo.ArchiveGlobalTemplate(context.Background(), disclosureapp.ArchiveGlobalTemplateParams{
		TypeID: typeID, UpdatedBy: "u1", Reason: "stop",
	}); err != nil {
		t.Fatalf("archive: %v", err)
	}

	svc := disclosureapp.NewService(repo, nil, idgen.UUIDv7Generator{})
	_, err := svc.CreateRecord(context.Background(), disclosureapp.CreateRecordRequest{
		Subject: disclosureapp.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"},
		Payload: disclosureapp.RecordPayload{TypeID: typeID, Title: "t", Content: "c", PlannedDate: "2026-01-15"},
	})
	if err == nil {
		t.Fatal("expected archived reject")
	}
	herr, ok := err.(*perr.HTTPError)
	if !ok || herr.HTTPStatus != http.StatusConflict || string(herr.Code) != "TEMPLATE_ARCHIVED" {
		t.Fatalf("got %v", err)
	}
}

func TestCreateRecord_RejectsNotPublishedDraft(t *testing.T) {
	repo := inmemory.NewRepository()
	typeID := "dt-draft-only-guard"
	sub := disclosureapp.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"}
	if _, err := repo.UpsertTypeVersion(context.Background(), disclosureapp.UpsertTypeVersionRequest{
		Subject:          sub,
		TypeID:           typeID,
		Scope:            "global",
		GroupID:          "group-001",
		Name:             "Draft Only Guard",
		Category:         "periodic",
		TemplateCategory: "periodic",
		DeadlineStrategy: "fixed",
		DeadlineRule:     "T+5",
		Periodicity:      "monthly",
		ChangeNote:       "create draft",
		Blocks: []disclosureapp.TemplateBlockDTO{
			{
				BlockKey:  "enterprise_workflow",
				BlockType: "rich_text",
				Enabled:   true,
				Config: map[string]any{
					"steps": []any{
						map[string]any{
							"step_id": "s1", "stage": "Review", "department_id": "d1",
							"assignee_role_ids": []any{"r1"}, "processing_days": 1, "display_order": 1,
							"description": "desc",
						},
					},
				},
			},
		},
	}); err != nil {
		t.Fatalf("upsert draft: %v", err)
	}
	life, err := repo.GetTypeLifecycle(context.Background(), typeID)
	if err != nil {
		t.Fatalf("lifecycle: %v", err)
	}
	if life.ActiveVersionNo != 0 || stringsEqualFoldArchived(life.Status) {
		t.Fatalf("expected draft not archived, got %+v", life)
	}

	svc := disclosureapp.NewService(repo, nil, idgen.UUIDv7Generator{})
	_, err = svc.CreateRecord(context.Background(), disclosureapp.CreateRecordRequest{
		Subject: sub,
		Payload: disclosureapp.RecordPayload{TypeID: typeID, Title: "t", Content: "c", PlannedDate: "2026-01-15"},
	})
	if err == nil {
		t.Fatal("expected not-active reject")
	}
	herr, ok := err.(*perr.HTTPError)
	if !ok || herr.HTTPStatus != http.StatusConflict || string(herr.Code) != "TEMPLATE_NOT_ACTIVE" {
		t.Fatalf("got %v", err)
	}
}

func stringsEqualFoldArchived(status string) bool {
	return status == "archived" || status == "Archived"
}

func TestCreateRecord_AllowsActiveGlobalTemplate(t *testing.T) {
	repo := inmemory.NewRepository()
	typeID := "dt-periodic-financial"
	svc := disclosureapp.NewService(repo, nil, idgen.UUIDv7Generator{})
	got, err := svc.CreateRecord(context.Background(), disclosureapp.CreateRecordRequest{
		Subject: disclosureapp.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"},
		Payload: disclosureapp.RecordPayload{TypeID: typeID, Title: "t", Content: "c", PlannedDate: "2026-01-15"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if got == nil || got.TypeID != typeID {
		t.Fatalf("unexpected record: %+v", got)
	}
}

func TestCreateRecord_UpdateExistingNotBlockedByArchive(t *testing.T) {
	repo := inmemory.NewRepository()
	typeID := "dt-periodic-financial"
	svc := disclosureapp.NewService(repo, nil, idgen.UUIDv7Generator{})
	sub := disclosureapp.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"}
	created, err := svc.CreateRecord(context.Background(), disclosureapp.CreateRecordRequest{
		Subject: sub,
		Payload: disclosureapp.RecordPayload{TypeID: typeID, Title: "t", Content: "c", PlannedDate: "2026-01-15"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := repo.ArchiveGlobalTemplate(context.Background(), disclosureapp.ArchiveGlobalTemplateParams{
		TypeID: typeID, UpdatedBy: "u1", Reason: "stop",
	}); err != nil {
		t.Fatalf("archive: %v", err)
	}
	updated, err := svc.UpdateRecord(context.Background(), disclosureapp.UpdateRecordRequest{
		Subject:  sub,
		RecordID: created.RecordID,
		Payload:  disclosureapp.RecordPayload{Title: "t2", Content: "c2", PlannedDate: "2026-01-16"},
	})
	if err != nil {
		t.Fatalf("update existing after archive: %v", err)
	}
	if updated.Title != "t2" {
		t.Fatalf("title=%q", updated.Title)
	}
	_, err = svc.CreateRecord(context.Background(), disclosureapp.CreateRecordRequest{
		Subject: sub,
		Payload: disclosureapp.RecordPayload{TypeID: typeID, Title: "new", Content: "c", PlannedDate: "2026-01-20"},
	})
	if err == nil {
		t.Fatal("new create must fail while archived")
	}
	herr, ok := err.(*perr.HTTPError)
	if !ok || string(herr.Code) != "TEMPLATE_ARCHIVED" {
		t.Fatalf("got %v", err)
	}
}

func TestCreateRecord_AfterDraftArchiveRestore_NotTEMPLATE_ARCHIVED(t *testing.T) {
	repo := inmemory.NewRepository()
	typeID := "dt-draft-restore-guard"
	sub := disclosureapp.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"}
	if _, err := repo.UpsertTypeVersion(context.Background(), disclosureapp.UpsertTypeVersionRequest{
		Subject: sub, TypeID: typeID, Scope: "global", GroupID: "group-001",
		Name: "Draft Restore Guard", Category: "periodic", TemplateCategory: "periodic",
		DeadlineStrategy: "fixed", DeadlineRule: "T+5", Periodicity: "monthly", ChangeNote: "draft",
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if _, err := repo.ArchiveGlobalTemplate(context.Background(), disclosureapp.ArchiveGlobalTemplateParams{
		TypeID: typeID, UpdatedBy: "u1", Reason: "soft",
	}); err != nil {
		t.Fatalf("archive: %v", err)
	}
	if _, err := repo.RestoreGlobalTemplate(context.Background(), disclosureapp.RestoreGlobalTemplateParams{
		TypeID: typeID, UpdatedBy: "u1", ExpectedFromVersionNo: nil, RestoreActiveVersionNo: 0,
	}); err != nil {
		t.Fatalf("restore: %v", err)
	}
	life, _ := repo.GetTypeLifecycle(context.Background(), typeID)
	if life == nil || life.Status == "archived" || life.ActiveVersionNo != 0 {
		t.Fatalf("after restore draft want status!=archived av=0, got %+v", life)
	}
	svc := disclosureapp.NewService(repo, nil, idgen.UUIDv7Generator{})
	_, err := svc.CreateRecord(context.Background(), disclosureapp.CreateRecordRequest{
		Subject: sub,
		Payload: disclosureapp.RecordPayload{TypeID: typeID, Title: "t", Content: "c", PlannedDate: "2026-01-15"},
	})
	if err == nil {
		t.Fatal("expected reject")
	}
	herr, ok := err.(*perr.HTTPError)
	if !ok || string(herr.Code) != "TEMPLATE_NOT_ACTIVE" {
		t.Fatalf("after draft restore want TEMPLATE_NOT_ACTIVE, got %v", err)
	}
}
