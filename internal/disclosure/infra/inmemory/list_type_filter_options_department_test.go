package inmemory

import (
	"context"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

// Template workflow steps store a platform department code (workflow_template_departments);
// the Portal filter must show its name, never the raw code.
func TestListTypeFilterOptions_DepartmentNameFromTemplateCatalog(t *testing.T) {
	r := NewRepository()
	r.mu.Lock()
	if r.globalWorkflows == nil {
		r.globalWorkflows = map[string]*disclosureapp.GlobalWorkflowDTO{}
	}
	r.globalWorkflows["dt-dept-name-test"] = &disclosureapp.GlobalWorkflowDTO{
		TypeID: "dt-dept-name-test",
		Steps: []disclosureapp.GlobalWorkflowStepInput{
			{DepartmentID: "dept-001"},
			{DepartmentID: "dept-unknown-code"},
		},
	}
	r.mu.Unlock()

	resp, err := r.ListTypeFilterOptions(context.Background(), "c_001", []string{"dt-dept-name-test"})
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]string{}
	for _, d := range resp.Departments {
		names[d.ID] = d.Name
	}
	if got := names["dept-001"]; got != "Phòng Pháp chế" {
		t.Fatalf("dept-001 name = %q, want catalog name %q", got, "Phòng Pháp chế")
	}
	// A code missing from the catalog still falls back to itself instead of disappearing.
	if got := names["dept-unknown-code"]; got != "dept-unknown-code" {
		t.Fatalf("unknown code name = %q, want fallback to the code", got)
	}
}
