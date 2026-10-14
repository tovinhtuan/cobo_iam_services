package app

import (
	"context"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

type visibleOptionsRepo struct {
	listTypesPaginationRepo
	optionTypeIDs []string
	optionCalls   int
}

func (r *visibleOptionsRepo) ListTypeFilterOptions(_ context.Context, _ string, typeIDs []string) (*ListTypeFilterOptionsResponse, error) {
	r.optionCalls++
	r.optionTypeIDs = append([]string(nil), typeIDs...)
	return &ListTypeFilterOptionsResponse{
		Departments: []TypeFilterOptionDTO{{ID: "d2", Name: "Phòng Pháp chế"}, {ID: "d1", Name: "Ban Tổng Giám đốc"}, {ID: "d3", Name: "Đội dự án"}},
	}, nil
}

// F8: options are built from the types the company actually sees in the list (applicability
// filter applied), so no option leads to an empty list.
func TestListTypeFilterOptions_UsesTypesVisibleToTheCompany(t *testing.T) {
	now := time.Now().UTC()
	notForThisCompany := &applicability.TemplateApplicabilityRules{
		ApplicableCompanyClasses: []applicability.CompanyClass{"listed"},
		ApplicableSectors:        []applicability.BusinessSector{"banking"},
	}
	repo := &visibleOptionsRepo{listTypesPaginationRepo: listTypesPaginationRepo{
		light: []DisclosureTypeSummaryDTO{
			{TypeID: "g-visible", Scope: "global", Name: "A", CreatedAt: now},
			{TypeID: "g-hidden", Scope: "global", Name: "B", CreatedAt: now, ApplicabilityRules: notForThisCompany},
			{TypeID: "c-own", Scope: "company", Name: "C", CreatedAt: now},
		},
	}}
	svc := NewService(repo, nil, idgen.UUIDv7Generator{})
	resp, err := svc.ListTypeFilterOptions(context.Background(), ListTypeFilterOptionsRequest{Subject: Subject{CompanyID: "c1"}})
	if err != nil {
		t.Fatal(err)
	}
	if repo.optionCalls != 1 {
		t.Fatalf("option calls = %d", repo.optionCalls)
	}
	got := append([]string(nil), repo.optionTypeIDs...)
	sort.Strings(got)
	if want := []string{"c-own", "g-visible"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("options type ids = %v, want %v (applicability-filtered list)", got, want)
	}
	names := make([]string, 0, len(resp.Departments))
	for _, d := range resp.Departments {
		names = append(names, d.Name)
	}
	// Vietnamese order: Đ sorts right after D, not after Z.
	if want := []string{"Ban Tổng Giám đốc", "Đội dự án", "Phòng Pháp chế"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("departments not sorted by Vietnamese name: %v", names)
	}
}

// F10: `tag` values are exact (a comma stays inside the tag); `tag_ids` keeps its CSV meaning.
func TestParseTagFilters_ExactTagsKeepCommas(t *testing.T) {
	got := ParseTagFilters("", []string{"Báo cáo, quý", " BCTC ", "bctc", ""})
	if want := []string{"Báo cáo, quý", "BCTC"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("exact tags = %v, want %v", got, want)
	}
	got = ParseTagFilters("Tài chính, Thuế", []string{"Thuế"})
	if want := []string{"Tài chính", "Thuế"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("csv + exact = %v, want %v", got, want)
	}
}
