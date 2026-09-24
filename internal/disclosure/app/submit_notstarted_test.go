package app_test

import (
	"context"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

func TestSubmitRecord_NotStartedGoesToPendingReview(t *testing.T) {
	repo := inmemory.NewRepository()
	svc := disclosureapp.NewService(repo, submitAllowAuth{}, idgen.UUIDv7Generator{})
	sub := disclosureapp.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"}
	created, err := repo.Create(context.Background(), disclosureapp.RecordDTO{
		RecordID:  "rec_ns_1",
		CompanyID: "c1",
		TypeID:    "t1",
		Title:     "NS",
		Content:   "c",
		Status:    "NotStarted",
		CreatedBy: "u1",
		UpdatedBy: "u1",
	})
	if err != nil {
		t.Fatal(err)
	}
	submitted, err := svc.SubmitRecord(context.Background(), disclosureapp.SubmitRecordRequest{
		Subject:  sub,
		RecordID: created.RecordID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if submitted.Status != "PendingReview" {
		t.Fatalf("status=%q want PendingReview", submitted.Status)
	}
}
