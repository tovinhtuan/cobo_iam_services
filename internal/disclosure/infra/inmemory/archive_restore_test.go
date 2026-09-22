package inmemory

import (
	"context"
	"net/http"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func TestArchiveRestore_DraftRoundTrip(t *testing.T) {
	r := NewRepository()
	_, err := r.UpsertTypeVersion(context.Background(), disclosureapp.UpsertTypeVersionRequest{
		Subject: disclosureapp.Subject{CompanyID: "c1", UserID: "u1"},
		TypeID:  "dt-draft-only", Scope: "global", GroupID: "group-001",
		Name: "Draft Only", Category: "x", TemplateCategory: "irregular", Description: "d",
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	arc, err := r.ArchiveGlobalTemplate(context.Background(), disclosureapp.ArchiveGlobalTemplateParams{
		TypeID: "dt-draft-only", UpdatedBy: "u1", Reason: "cleanup",
	})
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if arc.ArchivedFromVersionNo != nil || arc.ArchiveReason != "cleanup" {
		t.Fatalf("archive %+v", arc)
	}
	arc2, err := r.ArchiveGlobalTemplate(context.Background(), disclosureapp.ArchiveGlobalTemplateParams{
		TypeID: "dt-draft-only", UpdatedBy: "u2", Reason: "again",
	})
	if err != nil || !arc2.AlreadyArchived || arc2.ArchiveReason != "cleanup" {
		t.Fatalf("idempotent %+v err=%v", arc2, err)
	}
	rest, err := r.RestoreGlobalTemplate(context.Background(), disclosureapp.RestoreGlobalTemplateParams{
		TypeID: "dt-draft-only", UpdatedBy: "u1",
	})
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if rest.RestoredMode != "draft" || rest.AfterActiveVersionNo != 0 {
		t.Fatalf("restore %+v", rest)
	}
	life, err := r.GetTypeLifecycle(context.Background(), "dt-draft-only")
	if err != nil || life.Status != "active" || life.ActiveVersionNo != 0 || life.ArchivedFromVersionNo != nil {
		t.Fatalf("lifecycle %+v err=%v", life, err)
	}
}

func TestArchiveRestore_ActiveRoundTripAndActivateGuard(t *testing.T) {
	r := NewRepository()
	typeID := "dt-periodic-financial"
	arc, err := r.ArchiveGlobalTemplate(context.Background(), disclosureapp.ArchiveGlobalTemplateParams{
		TypeID: typeID, UpdatedBy: "u1", Reason: "retire",
	})
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if arc.ArchivedFromVersionNo == nil || *arc.ArchivedFromVersionNo < 1 {
		t.Fatalf("want archived_from >=1 got %+v", arc)
	}
	_, err = r.ActivateTypeVersion(context.Background(), disclosureapp.ActivateTypeVersionRequest{
		Subject: disclosureapp.Subject{CompanyID: "c1", UserID: "u1"},
		TypeID:  typeID, VersionNo: *arc.ArchivedFromVersionNo,
	})
	if err == nil {
		t.Fatal("activate archived must fail")
	}
	if herr, ok := err.(*perr.HTTPError); !ok || herr.HTTPStatus != http.StatusConflict {
		t.Fatalf("want 409, got %v", err)
	}
	from := *arc.ArchivedFromVersionNo
	rest, err := r.RestoreGlobalTemplate(context.Background(), disclosureapp.RestoreGlobalTemplateParams{
		TypeID: typeID, UpdatedBy: "u1", ExpectedFromVersionNo: &from, RestoreActiveVersionNo: from,
	})
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if rest.RestoredMode != "active" || rest.AfterActiveVersionNo != from {
		t.Fatalf("restore %+v", rest)
	}
	life, err := r.GetTypeLifecycle(context.Background(), typeID)
	if err != nil || life.Status != "active" || life.ActiveVersionNo != from {
		t.Fatalf("lifecycle %+v err=%v", life, err)
	}
}

func TestRestore_DraftPathRejectedWhenMetadataPresent(t *testing.T) {
	r := NewRepository()
	typeID := "dt-event-major-change"
	if _, err := r.ArchiveGlobalTemplate(context.Background(), disclosureapp.ArchiveGlobalTemplateParams{TypeID: typeID, UpdatedBy: "u1"}); err != nil {
		t.Fatalf("archive: %v", err)
	}
	_, err := r.RestoreGlobalTemplate(context.Background(), disclosureapp.RestoreGlobalTemplateParams{TypeID: typeID, UpdatedBy: "u1"})
	if err == nil {
		t.Fatal("expected mismatch for draft restore on previously-active archive")
	}
	if herr, ok := err.(*perr.HTTPError); !ok || herr.HTTPStatus != http.StatusConflict {
		t.Fatalf("got %v", err)
	}
}

func TestRestore_MissingVersion(t *testing.T) {
	r := NewRepository()
	typeID := "dt-custom-obligation"
	arc, err := r.ArchiveGlobalTemplate(context.Background(), disclosureapp.ArchiveGlobalTemplateParams{TypeID: typeID, UpdatedBy: "u1"})
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	from := *arc.ArchivedFromVersionNo
	// Remove version snapshots to simulate missing version.
	delete(r.catalogByVer, typeID)
	r.versions[typeID] = nil
	_, err = r.RestoreGlobalTemplate(context.Background(), disclosureapp.RestoreGlobalTemplateParams{
		TypeID: typeID, UpdatedBy: "u1", ExpectedFromVersionNo: &from, RestoreActiveVersionNo: from,
	})
	if err == nil {
		t.Fatal("expected version missing")
	}
	if herr, ok := err.(*perr.HTTPError); !ok || string(herr.Code) != "TEMPLATE_RESTORE_VERSION_MISSING" {
		t.Fatalf("got %v", err)
	}
}
