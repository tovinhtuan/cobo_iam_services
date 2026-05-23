package app

import "testing"

func TestExtractTemplateWorkflow_ReadsStructuredEnterpriseWorkflowBlock(t *testing.T) {
	blocks := []TemplateBlockDTO{
		{
			BlockID:   "block-workflow",
			BlockKey:  "enterprise_workflow",
			BlockType: "rich_text",
			Config: map[string]any{
				"steps": []any{
					map[string]any{
						"id":                "step-001",
						"stage":             "Review",
						"department_id":     "dept-finance",
						"assignee_role_ids": []any{"role-reviewer"},
						"processing_days":   float64(2),
						"display_order":     float64(1),
						"documents": []any{
							map[string]any{"id": "doc-001", "name": "Checklist", "required": true},
						},
					},
				},
			},
		},
	}

	steps := ExtractTemplateWorkflow(blocks)
	if len(steps) != 1 {
		t.Fatalf("steps=%d want 1", len(steps))
	}
	if steps[0].StepID != "step-001" {
		t.Fatalf("step_id=%q want step-001", steps[0].StepID)
	}
	if steps[0].DepartmentID != "dept-finance" {
		t.Fatalf("department_id=%q want dept-finance", steps[0].DepartmentID)
	}
	if len(steps[0].Documents) != 1 || steps[0].Documents[0].DocID != "doc-001" {
		t.Fatalf("documents=%#v want one document", steps[0].Documents)
	}
}

func TestValidateTemplateWorkflowForActivation_RejectsMissingRoles(t *testing.T) {
	err := ValidateTemplateWorkflowForActivation([]TemplateBlockDTO{
		{
			BlockID:   "block-workflow",
			BlockKey:  "enterprise_workflow",
			BlockType: "rich_text",
			Config: map[string]any{
				"steps": []any{
					map[string]any{
						"step_id":         "step-001",
						"stage":           "Review",
						"department_id":   "dept-finance",
						"processing_days": float64(2),
					},
				},
			},
		},
	})
	if err == nil {
		t.Fatal("expected workflow validation error")
	}
}
