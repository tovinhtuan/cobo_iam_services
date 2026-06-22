package workflow

import "testing"

// TestWorkflowSourceLabel_PassesThroughAllThreeValues is the regression guard for Architecture
// Integrity Fix A: a global_workflow-sourced disclosure must NOT be recorded as global_template.
func TestWorkflowSourceLabel_PassesThroughAllThreeValues(t *testing.T) {
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
		if got := workflowSourceLabel(c.input); got != c.want {
			t.Errorf("workflowSourceLabel(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}
