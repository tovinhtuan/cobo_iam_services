package app_test

import (
	"context"
	"strings"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

type submitAllowAuth struct{}

func (submitAllowAuth) GetEffectiveAccess(_ context.Context, _, _ string) (*authapp.EffectiveAccessSummary, error) {
	return &authapp.EffectiveAccessSummary{
		MembershipID: "m1",
		CompanyID:    "c1",
		Permissions:  []string{"disclosure.submit", "disclosure.create", "disclosure.view", "disclosure.edit"},
	}, nil
}

func (submitAllowAuth) Authorize(_ context.Context, _ authapp.AuthorizeRequest) (*authapp.AuthorizeDecision, error) {
	return &authapp.AuthorizeDecision{Decision: authapp.DecisionAllow}, nil
}

func (submitAllowAuth) AuthorizeBatch(_ context.Context, _ authapp.AuthorizeBatchRequest) (*authapp.AuthorizeBatchResponse, error) {
	return &authapp.AuthorizeBatchResponse{}, nil
}

// Tenant "Hoàn tất cảnh báo" must not call a legal publish path.
// SubmitRecord is internal processing start only — never sets Published.
func TestSubmitRecord_DoesNotSetPublished(t *testing.T) {
	repo := inmemory.NewRepository()
	svc := disclosureapp.NewService(repo, submitAllowAuth{}, idgen.UUIDv7Generator{})
	sub := disclosureapp.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"}

	created, err := svc.CreateRecord(context.Background(), disclosureapp.CreateRecordRequest{
		Subject: sub,
		Payload: disclosureapp.RecordPayload{
			TypeID:      "dt-periodic-financial",
			Title:       "Tenant complete semantics",
			Summary:     "s",
			Content:     "c",
			PlannedDate: "2026-09-20",
		},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !strings.EqualFold(created.Status, "Draft") {
		t.Fatalf("create status=%q want Draft", created.Status)
	}

	submitted, err := svc.SubmitRecord(context.Background(), disclosureapp.SubmitRecordRequest{
		Subject:  sub,
		RecordID: created.RecordID,
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if strings.EqualFold(submitted.Status, "Published") {
		t.Fatalf("submit must not set Published; got %q", submitted.Status)
	}
	if !strings.EqualFold(submitted.Status, "PendingReview") && !strings.EqualFold(submitted.Status, "In Progress") {
		t.Fatalf("submit status=%q want PendingReview or In Progress", submitted.Status)
	}

	again, err := svc.SubmitRecord(context.Background(), disclosureapp.SubmitRecordRequest{
		Subject:  sub,
		RecordID: created.RecordID,
	})
	if err != nil {
		t.Fatalf("resubmit: %v", err)
	}
	if strings.EqualFold(again.Status, "Published") {
		t.Fatalf("resubmit must not set Published; got %q", again.Status)
	}
}
