package inmemory

import (
	"context"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

func TestListTypes_ScopeGlobalExcludesCompanyTemplates(t *testing.T) {
	r := NewRepository()
	r.mu.Lock()
	r.catalog["dt-company-custom"] = disclosureapp.DisclosureTypeDTO{
		TypeID:           "dt-company-custom",
		GroupID:          "group-006",
		Name:             "Tenant custom",
		TemplateCategory: "custom",
		VersionNo:        1,
	}
	r.catalogScope["dt-company-custom"] = "c_001"
	r.mu.Unlock()

	items, total, err := r.ListTypes(context.Background(), disclosureapp.ListTypesParams{
		CompanyID: "c_001",
		Scope:     "global",
		Page:      1,
		PageSize:  100,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.TypeID == "dt-company-custom" {
			t.Fatalf("company template leaked into scope=global list")
		}
		if item.Scope != "global" {
			t.Fatalf("non-global scope in result: %s %s", item.TypeID, item.Scope)
		}
	}
	if total != len(items) {
		t.Fatalf("total=%d len=%d", total, len(items))
	}
	if total == 0 {
		t.Fatal("expected at least one seeded global template")
	}
}

func TestListTypes_DefaultIncludesGlobalAndSubjectCompany(t *testing.T) {
	r := NewRepository()
	r.mu.Lock()
	r.catalog["dt-company-custom"] = disclosureapp.DisclosureTypeDTO{
		TypeID:           "dt-company-custom",
		GroupID:          "group-006",
		Name:             "Tenant custom",
		TemplateCategory: "custom",
		VersionNo:        1,
	}
	r.catalogScope["dt-company-custom"] = "c_001"
	r.catalog["dt-other-company"] = disclosureapp.DisclosureTypeDTO{
		TypeID:           "dt-other-company",
		GroupID:          "group-006",
		Name:             "Other tenant",
		TemplateCategory: "custom",
		VersionNo:        1,
	}
	r.catalogScope["dt-other-company"] = "c_other"
	r.mu.Unlock()

	items, _, err := r.ListTypes(context.Background(), disclosureapp.ListTypesParams{
		CompanyID: "c_001",
		Page:      1,
		PageSize:  100,
	})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, item := range items {
		seen[item.TypeID] = true
	}
	if !seen["dt-company-custom"] {
		t.Fatal("expected subject company template in default catalog")
	}
	if seen["dt-other-company"] {
		t.Fatal("other-tenant template must stay isolated")
	}
}
