package app

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

type listTypesPaginationRepo struct {
	Repository
	calls []ListTypesParams
	light []DisclosureTypeSummaryDTO
	full  map[string]DisclosureTypeSummaryDTO
}

func (r *listTypesPaginationRepo) ListTypes(_ context.Context, params ListTypesParams) ([]DisclosureTypeSummaryDTO, int, error) {
	r.calls = append(r.calls, params)
	if params.LightweightOnly {
		out := make([]DisclosureTypeSummaryDTO, len(r.light))
		copy(out, r.light)
		return out, len(out), nil
	}
	if len(params.TypeIDs) > 0 {
		out := make([]DisclosureTypeSummaryDTO, 0, len(params.TypeIDs))
		for _, id := range params.TypeIDs {
			if item, ok := r.full[id]; ok {
				out = append(out, item)
			}
		}
		return out, len(out), nil
	}
	return nil, 0, nil
}

func (r *listTypesPaginationRepo) GetCompanyApplicabilityProfile(_ context.Context, _ string) (applicability.CompanyApplicabilityProfile, error) {
	return applicability.CompanyApplicabilityProfile{}, nil
}

func (r *listTypesPaginationRepo) ListActiveDeadlineRuleCatalog(_ context.Context) ([]DeadlineRuleCatalogDTO, error) {
	return nil, nil
}

func newListTypesPaginationService(repo *listTypesPaginationRepo) Service {
	return NewService(repo, nil, idgen.UUIDv7Generator{})
}

func TestListTypes_PaginatesAfterLightweightApplicabilityPass(t *testing.T) {
	now := time.Now().UTC()
	repo := &listTypesPaginationRepo{
		light: []DisclosureTypeSummaryDTO{
			{TypeID: "t-3", Scope: "global", Name: "C", CreatedAt: now.Add(-1 * time.Hour)},
			{TypeID: "t-2", Scope: "global", Name: "B", CreatedAt: now.Add(-2 * time.Hour)},
			{TypeID: "t-1", Scope: "global", Name: "A", CreatedAt: now.Add(-3 * time.Hour)},
		},
		full: map[string]DisclosureTypeSummaryDTO{
			"t-3": {TypeID: "t-3", Name: "C", Description: "full-3"},
			"t-2": {TypeID: "t-2", Name: "B", Description: "full-2"},
		},
	}
	svc := newListTypesPaginationService(repo)
	resp, err := svc.ListTypes(context.Background(), ListTypesRequest{
		Subject:          Subject{CompanyID: "company-001"},
		Page:             1,
		PageSize:         2,
		PageProvided:     true,
		PageSizeProvided: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Total != 3 {
		t.Fatalf("total=%d want 3", resp.Total)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("items=%d want 2", len(resp.Items))
	}
	if resp.Items[0].TypeID != "t-3" || resp.Items[1].TypeID != "t-2" {
		t.Fatalf("order=%v,%v want t-3,t-2", resp.Items[0].TypeID, resp.Items[1].TypeID)
	}
	if resp.Items[0].Description != "full-3" {
		t.Fatalf("expected phase-2 payload for t-3")
	}
	if len(repo.calls) != 2 || !repo.calls[0].LightweightOnly || len(repo.calls[1].TypeIDs) != 2 {
		t.Fatalf("unexpected repo calls: %+v", repo.calls)
	}
}

func TestListTypes_InvalidPaginationReturns400(t *testing.T) {
	svc := newListTypesPaginationService(&listTypesPaginationRepo{})
	cases := []struct {
		name string
		req  ListTypesRequest
	}{
		{"page zero", ListTypesRequest{Subject: Subject{CompanyID: "c1"}, Page: 0, PageProvided: true}},
		{"page negative", ListTypesRequest{Subject: Subject{CompanyID: "c1"}, Page: -1, PageProvided: true}},
		{"page size zero", ListTypesRequest{Subject: Subject{CompanyID: "c1"}, PageSize: 0, PageSizeProvided: true}},
		{"page size too large", ListTypesRequest{Subject: Subject{CompanyID: "c1"}, PageSize: 9999, PageSizeProvided: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.ListTypes(context.Background(), tc.req)
			if err == nil {
				t.Fatal("expected error")
			}
			he, ok := err.(*perr.HTTPError)
			if !ok || he.HTTPStatus != http.StatusBadRequest {
				t.Fatalf("err=%v want 400", err)
			}
		})
	}
}

func TestListTypes_DefaultsPageAndPageSize(t *testing.T) {
	repo := &listTypesPaginationRepo{
		light: []DisclosureTypeSummaryDTO{{TypeID: "t-1", Scope: "global", Name: "A", CreatedAt: time.Now().UTC()}},
		full:  map[string]DisclosureTypeSummaryDTO{"t-1": {TypeID: "t-1", Name: "A"}},
	}
	svc := newListTypesPaginationService(repo)
	resp, err := svc.ListTypes(context.Background(), ListTypesRequest{Subject: Subject{CompanyID: "c1"}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Page != 1 || resp.PageSize != 20 {
		t.Fatalf("page=%d page_size=%d", resp.Page, resp.PageSize)
	}
}
