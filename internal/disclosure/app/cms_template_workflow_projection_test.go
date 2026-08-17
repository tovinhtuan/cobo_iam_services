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

func TestExtractTemplateWorkflow_PersistsReminderConfigCustomAndOmit(t *testing.T) {
	blocks := []TemplateBlockDTO{
		{
			BlockKey: "enterprise_workflow",
			Config: map[string]any{
				"steps": []any{
					map[string]any{
						"step_id": "s1", "stage": "One", "department_id": "d1",
						"assignee_role_ids": []any{"r1"}, "processing_days": float64(2), "display_order": float64(1),
						"reminder_config": map[string]any{"enabled": true, "mode": "days_before", "days_before": []any{float64(7), float64(2)}},
					},
					map[string]any{
						"step_id": "s2", "stage": "Two", "department_id": "d1",
						"assignee_role_ids": []any{"r1"}, "processing_days": float64(2), "display_order": float64(2),
					},
					map[string]any{
						"step_id": "s3", "stage": "Three", "department_id": "d1",
						"assignee_role_ids": []any{"r1"}, "processing_days": float64(1), "display_order": float64(3),
						"reminder_config": map[string]any{"enabled": false, "mode": "days_before", "days_before": []any{}},
					},
					map[string]any{
						"step_id": "s4", "stage": "Four", "department_id": "d1",
						"assignee_role_ids": []any{"r1"}, "processing_days": float64(1), "display_order": float64(4),
						"reminder_config": map[string]any{"enabled": true, "mode": "specific_date"},
					},
				},
			},
		},
	}
	steps := ExtractTemplateWorkflow(blocks)
	if len(steps) != 4 {
		t.Fatalf("steps=%d", len(steps))
	}
	if steps[0].ReminderConfig == nil || !steps[0].ReminderConfig.Enabled {
		t.Fatalf("custom missing: %+v", steps[0].ReminderConfig)
	}
	assertDays(t, steps[0].ReminderConfig.DaysBefore, []int{7, 2})
	if steps[1].ReminderConfig != nil {
		t.Fatalf("DEFAULT omit must stay absent, got %+v", steps[1].ReminderConfig)
	}
	if steps[2].ReminderConfig == nil || steps[2].ReminderConfig.Enabled {
		t.Fatalf("disabled must persist, got %+v", steps[2].ReminderConfig)
	}
	if steps[3].ReminderConfig == nil || steps[3].ReminderConfig.Mode != WorkflowStepReminderModeSpecificDate {
		t.Fatalf("specific_date must persist, got %+v", steps[3].ReminderConfig)
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

func TestValidateTemplateWorkflowForActivation_RejectsEmptyCustomReminder(t *testing.T) {
	err := ValidateTemplateWorkflowForActivation([]TemplateBlockDTO{
		{
			BlockKey: "enterprise_workflow",
			Config: map[string]any{
				"steps": []any{
					map[string]any{
						"step_id": "step-001", "stage": "Review", "department_id": "dept-finance",
						"assignee_role_ids": []any{"role-reviewer"}, "processing_days": float64(2),
						"reminder_config": map[string]any{"enabled": true, "mode": "days_before", "days_before": []any{}},
					},
				},
			},
		},
	})
	if err == nil {
		t.Fatal("empty custom reminder_config must reject")
	}
}
