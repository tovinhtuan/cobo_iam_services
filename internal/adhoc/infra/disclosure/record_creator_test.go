package disclosure

import "testing"

// TestMapWorkflowSource_PassesThroughAllThreeValues is the regression guard for Architecture
// Integrity Fix A: a global_workflow-sourced ad-hoc record must NOT be recorded as
// global_template (mirrors the identical fix in internal/disclosure/infra/workflow/bootstrap.go).
func TestMapWorkflowSource_PassesThroughAllThreeValues(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"global_workflow", "global_workflow"},
		{"global_template", "global_template"},
		{"company_override", "company_override"},
		{"", "global_template"}, // defensive default only for an unexpected empty value
	}
	for _, c := range cases {
		if got := mapWorkflowSource(c.input); got != c.want {
			t.Errorf("mapWorkflowSource(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}
