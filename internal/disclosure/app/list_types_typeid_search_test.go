package app_test

import (
	"context"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

func TestListTypes_SearchMatchesTypeID(t *testing.T) {
	ctx := context.Background()
	repo := inmemory.NewRepository()
	seedTemplateDraft(t, repo, "bctc-quy-2026")
	svc := disclosureapp.NewService(repo, nil, idgen.UUIDv7Generator{})
	resp, err := svc.ListTypes(ctx, disclosureapp.ListTypesRequest{
		Subject:  testSubjectWF,
		Query:    "quy-2026",
		ListMode: "management",
	})
	if err != nil {
		t.Fatalf("ListTypes: %v", err)
	}
	found := false
	for _, item := range resp.Items {
		if item.TypeID == "bctc-quy-2026" {
			found = true
		}
	}
	if !found {
		t.Fatalf("search by type_id fragment must find the template, got %+v", resp.Items)
	}
}
