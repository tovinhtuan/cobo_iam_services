package app

import (
	"context"
	"testing"
	"time"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

func TestListTypes_PassesTagPeriodicityDepartmentFilters(t *testing.T) {
	now := time.Now().UTC()
	repo := &listTypesPaginationRepo{
		light: []DisclosureTypeSummaryDTO{
			{TypeID: "t-1", Scope: "global", Name: "A", CreatedAt: now},
		},
		full: map[string]DisclosureTypeSummaryDTO{
			"t-1": {TypeID: "t-1", Name: "A", Tags: []string{"Tài chính"}},
		},
	}
	svc := newListTypesPaginationService(repo)
	_, err := svc.ListTypes(context.Background(), ListTypesRequest{
		Subject:          Subject{CompanyID: "c1"},
		Tags:             []string{"Tài chính"},
		Periodicity:      "quarterly",
		DepartmentID:     "dept-finance",
		Page:             1,
		PageSize:         20,
		PageProvided:     true,
		PageSizeProvided: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.calls) == 0 {
		t.Fatal("expected ListTypes call")
	}
	first := repo.calls[0]
	if len(first.Tags) != 1 || first.Tags[0] != "Tài chính" {
		t.Fatalf("tags=%v", first.Tags)
	}
	if first.Periodicity != "quarterly" {
		t.Fatalf("periodicity=%q", first.Periodicity)
	}
	if first.DepartmentID != "dept-finance" {
		t.Fatalf("department=%q", first.DepartmentID)
	}
}

func TestListTypeFilterOptions_EmptyArrays(t *testing.T) {
	repo := &filterOptionsRepo{}
	svc := NewService(repo, nil, idgen.UUIDv7Generator{})
	resp, err := svc.ListTypeFilterOptions(context.Background(), ListTypeFilterOptionsRequest{
		Subject: Subject{CompanyID: "c1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Tags == nil || resp.Departments == nil || len(resp.Frequencies) == 0 {
		t.Fatalf("expected non-nil empty tags/departments and default frequencies: %#v", resp)
	}
}

type filterOptionsRepo struct {
	Repository
}

func (r *filterOptionsRepo) ListTypeFilterOptions(_ context.Context, _ string) (*ListTypeFilterOptionsResponse, error) {
	return &ListTypeFilterOptionsResponse{
		Tags:        nil,
		Departments: nil,
		Frequencies: nil,
	}, nil
}

func (r *filterOptionsRepo) GetCompanyApplicabilityProfile(_ context.Context, _ string) (applicability.CompanyApplicabilityProfile, error) {
	return applicability.CompanyApplicabilityProfile{}, nil
}
