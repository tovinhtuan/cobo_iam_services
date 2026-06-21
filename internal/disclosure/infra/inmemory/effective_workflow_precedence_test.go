package inmemory

import (
	"context"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

// TestGetEffectiveWorkflow_NoActiveGlobalWorkflow_FallsBackToLegacy is Batch R4 case #3:
// a type with no global_workflows row at all must still return the legacy enterprise_workflow
// content-block steps, source="global_template" — unchanged pre-R1 behavior.
func TestGetEffectiveWorkflow_NoActiveGlobalWorkflow_FallsBackToLegacy(t *testing.T) {
	r := NewRepository()
	ctx := context.Background()
	const typeID = "dt-legacy-only"
	const companyID = "company-test"

	r.catalog[typeID] = disclosureapp.DisclosureTypeDTO{
		TypeID:    typeID,
		VersionNo: 1,
		Blocks: []disclosureapp.TemplateBlockDTO{
			{
				BlockKey: "enterprise_workflow",
				Config: map[string]any{
					"steps": []any{
						map[string]any{"step_id": "leg-1", "stage": "Legacy Stage", "department_id": "d1",
							"assignee_role_ids": []any{"role-reviewer"}, "processing_days": 3, "display_order": 1},
					},
				},
			},
		},
	}

	dto, err := r.GetEffectiveWorkflow(ctx, companyID, typeID)
	if err != nil {
		t.Fatalf("GetEffectiveWorkflow: %v", err)
	}
	if dto.Source != "global_template" {
		t.Fatalf("source=%q want global_template", dto.Source)
	}
	if len(dto.Workflow) != 1 || dto.Workflow[0].Stage != "Legacy Stage" {
		t.Fatalf("unexpected workflow: %+v", dto.Workflow)
	}
}

// TestGetEffectiveWorkflow_ActiveGlobalWorkflow_WinsOverLegacy is Batch R4 case #1/#2 combined
// (the in-memory layer has no separate publish/activate version history — see
// global_workflow_read.go's MySQL implementation and r1-r2-live-integration-proof.json for the
// live-DEV proof of the real publish≠activate distinction). Here: once a global_workflows row is
// "active" with ActiveVersionNo set (the in-memory analogue of an activated version), it must win
// over a legacy block that also exists for the same type.
func TestGetEffectiveWorkflow_ActiveGlobalWorkflow_WinsOverLegacy(t *testing.T) {
	r := NewRepository()
	ctx := context.Background()
	const typeID = "dt-both"
	const companyID = "company-test"

	r.catalog[typeID] = disclosureapp.DisclosureTypeDTO{
		TypeID:    typeID,
		VersionNo: 1,
		Blocks: []disclosureapp.TemplateBlockDTO{
			{BlockKey: "enterprise_workflow", Config: map[string]any{
				"steps": []any{map[string]any{"step_id": "leg-1", "stage": "Legacy Stage", "department_id": "d1",
					"assignee_role_ids": []any{"role-reviewer"}, "processing_days": 3, "display_order": 1}},
			}},
		},
	}

	active := 1
	r.globalWorkflows = map[string]*disclosureapp.GlobalWorkflowDTO{
		typeID: {
			WorkflowID:      "wf-1",
			TypeID:          typeID,
			Status:          "active",
			ActiveVersionNo: &active,
			Steps: []disclosureapp.GlobalWorkflowStepInput{
				{StepID: "gw-1", Stage: "Governed Stage", DepartmentID: "d2",
					AssigneeRoleIds: []string{"reviewer"}, ProcessingDays: 5, DisplayOrder: 1},
			},
		},
	}

	dto, err := r.GetEffectiveWorkflow(ctx, companyID, typeID)
	if err != nil {
		t.Fatalf("GetEffectiveWorkflow: %v", err)
	}
	if dto.Source != "global_workflow" {
		t.Fatalf("source=%q want global_workflow", dto.Source)
	}
	if dto.VersionNo != active {
		t.Fatalf("version_no=%d want %d", dto.VersionNo, active)
	}
	if len(dto.Workflow) != 1 || dto.Workflow[0].Stage != "Governed Stage" {
		t.Fatalf("unexpected workflow (must come from global workflow, not legacy): %+v", dto.Workflow)
	}
	if dto.Workflow[0].DueRule != "T+5" {
		t.Fatalf("due_rule=%q want synthesized T+5 (processing_days=5, no explicit due_rule)", dto.Workflow[0].DueRule)
	}
}

// TestGetEffectiveWorkflow_GlobalWorkflowWithoutActiveVersionNo_FallsBackToLegacy guards the
// "draft-only, never activated" case: a global_workflows row exists but ActiveVersionNo is nil
// (no version has ever been activated for it) — must still fall back to legacy, not surface an
// unpublished/unactivated draft to the runtime.
func TestGetEffectiveWorkflow_GlobalWorkflowWithoutActiveVersionNo_FallsBackToLegacy(t *testing.T) {
	r := NewRepository()
	ctx := context.Background()
	const typeID = "dt-draft-only"
	const companyID = "company-test"

	r.catalog[typeID] = disclosureapp.DisclosureTypeDTO{
		TypeID: typeID, VersionNo: 1,
		Blocks: []disclosureapp.TemplateBlockDTO{
			{BlockKey: "enterprise_workflow", Config: map[string]any{
				"steps": []any{map[string]any{"step_id": "leg-1", "stage": "Legacy Stage", "department_id": "d1",
					"assignee_role_ids": []any{"role-reviewer"}, "processing_days": 3, "display_order": 1}},
			}},
		},
	}
	r.globalWorkflows = map[string]*disclosureapp.GlobalWorkflowDTO{
		typeID: {WorkflowID: "wf-1", TypeID: typeID, Status: "active", ActiveVersionNo: nil},
	}

	dto, err := r.GetEffectiveWorkflow(ctx, companyID, typeID)
	if err != nil {
		t.Fatalf("GetEffectiveWorkflow: %v", err)
	}
	if dto.Source != "global_template" {
		t.Fatalf("source=%q want global_template (no active version yet → legacy fallback)", dto.Source)
	}
}

// TestGetEffectiveWorkflow_CompanyOverride_StillWinsOverGlobalWorkflow is Batch R4 case #4: the
// company-override branch (pre-existing, untouched by Batch R1) must still take priority even
// when a global workflow is also active for the type.
func TestGetEffectiveWorkflow_CompanyOverride_StillWinsOverGlobalWorkflow(t *testing.T) {
	r := NewRepository()
	ctx := context.Background()
	const typeID = "dt-override-and-global"
	const companyID = "company-test"

	r.catalog[typeID] = disclosureapp.DisclosureTypeDTO{TypeID: typeID, VersionNo: 1}

	active := 1
	r.globalWorkflows = map[string]*disclosureapp.GlobalWorkflowDTO{
		typeID: {
			WorkflowID: "wf-1", TypeID: typeID, Status: "active", ActiveVersionNo: &active,
			Steps: []disclosureapp.GlobalWorkflowStepInput{
				{StepID: "gw-1", Stage: "Governed Stage", DepartmentID: "d2", ProcessingDays: 5, DisplayOrder: 1},
			},
		},
	}

	r.overrideByCompanyType[overrideKey(companyID, typeID)] = &overrideState{
		header: disclosureapp.CompanyWorkflowOverrideHeaderDTO{ActiveVersionNo: 1},
		versions: map[int]disclosureapp.CompanyWorkflowOverrideVersionDTO{
			1: {
				VersionNo: 1,
				State:     "approved",
				Workflow: []disclosureapp.WorkflowStepDTO{
					{StepID: "ov-1", Stage: "Override Stage", DepartmentID: "d3", DisplayOrder: 1},
				},
			},
		},
	}

	dto, err := r.GetEffectiveWorkflow(ctx, companyID, typeID)
	if err != nil {
		t.Fatalf("GetEffectiveWorkflow: %v", err)
	}
	if dto.Source != "company_override" {
		t.Fatalf("source=%q want company_override — override must win over an active global workflow", dto.Source)
	}
	if len(dto.Workflow) != 1 || dto.Workflow[0].Stage != "Override Stage" {
		t.Fatalf("unexpected workflow (must come from override, not global workflow): %+v", dto.Workflow)
	}
}
