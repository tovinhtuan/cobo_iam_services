package inmemory

import (
	"context"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

// F5 (in-memory parity): an approved company override replaces the template workflow for the
// department filter and options.
func TestInMemory_DepartmentFilterFollowsApprovedOverride(t *testing.T) {
	r := NewRepository()
	r.mu.Lock()
	r.catalog["dt-r2-ovr"] = disclosureapp.DisclosureTypeDTO{TypeID: "dt-r2-ovr", GroupID: "group-001", Name: "R2 override", VersionNo: 1}
	r.catalogScope["dt-r2-ovr"] = "global"
	if r.globalWorkflows == nil {
		r.globalWorkflows = map[string]*disclosureapp.GlobalWorkflowDTO{}
	}
	r.globalWorkflows["dt-r2-ovr"] = &disclosureapp.GlobalWorkflowDTO{
		TypeID: "dt-r2-ovr",
		Steps:  []disclosureapp.GlobalWorkflowStepInput{{DepartmentID: "dept-001"}},
	}
	r.overrideByCompanyType[overrideKey("c_001", "dt-r2-ovr")] = &overrideState{
		header: disclosureapp.CompanyWorkflowOverrideHeaderDTO{CompanyID: "c_001", TypeID: "dt-r2-ovr", ActiveVersionNo: 1},
		versions: map[int]disclosureapp.CompanyWorkflowOverrideVersionDTO{
			1: {VersionNo: 1, State: "approved", Workflow: []disclosureapp.WorkflowStepDTO{{DepartmentID: "dep-tenant-hr"}}},
		},
	}
	r.mu.Unlock()

	ids := func(companyID, dept string) map[string]bool {
		items, _, err := r.ListTypes(context.Background(), disclosureapp.ListTypesParams{CompanyID: companyID, DepartmentID: dept, Page: 1, PageSize: 100})
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]bool{}
		for _, it := range items {
			out[it.TypeID] = true
		}
		return out
	}
	if !ids("c_001", "dep-tenant-hr")["dt-r2-ovr"] {
		t.Fatal("override department must match for the overriding company")
	}
	if ids("c_001", "dept-001")["dt-r2-ovr"] {
		t.Fatal("template department is replaced by the approved override")
	}
	if !ids("c_002", "dept-001")["dt-r2-ovr"] {
		t.Fatal("another company still uses the template workflow")
	}

	resp, err := r.ListTypeFilterOptions(context.Background(), "c_001", []string{"dt-r2-ovr"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Departments) != 1 || resp.Departments[0].ID != "dep-tenant-hr" {
		t.Fatalf("options must follow the effective workflow: %+v", resp.Departments)
	}
	other, err := r.ListTypeFilterOptions(context.Background(), "c_002", []string{"dt-r2-ovr"})
	if err != nil {
		t.Fatal(err)
	}
	if len(other.Departments) != 1 || other.Departments[0].ID != "dept-001" || other.Departments[0].Name != "Phòng Pháp chế" {
		t.Fatalf("a global type keeps its template departments for other companies: %+v", other.Departments)
	}
}

// F13: filter options never expose another company's private types (tags) and only use the
// given visible types.
func TestInMemory_FilterOptionsScopedToCompanyAndVisibleTypes(t *testing.T) {
	r := NewRepository()
	r.mu.Lock()
	r.catalog["dt-r2-other-co"] = disclosureapp.DisclosureTypeDTO{TypeID: "dt-r2-other-co", Name: "Other company", VersionNo: 1, Tags: []string{"Bí mật công ty B"}}
	r.catalogScope["dt-r2-other-co"] = "c_002"
	r.catalog["dt-r2-own"] = disclosureapp.DisclosureTypeDTO{TypeID: "dt-r2-own", Name: "Own", VersionNo: 1, Tags: []string{"Riêng A"}}
	r.catalogScope["dt-r2-own"] = "c_001"
	r.mu.Unlock()

	resp, err := r.ListTypeFilterOptions(context.Background(), "c_001", []string{"dt-r2-own", "dt-r2-other-co"})
	if err != nil {
		t.Fatal(err)
	}
	tags := map[string]bool{}
	for _, tag := range resp.Tags {
		tags[tag.ID] = true
	}
	if tags["Bí mật công ty B"] {
		t.Fatalf("another company's private tag leaked: %v", tags)
	}
	if !tags["Riêng A"] || len(tags) != 1 {
		t.Fatalf("tags must come from the company's visible types only: %v", tags)
	}
}
