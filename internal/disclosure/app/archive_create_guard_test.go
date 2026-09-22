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
