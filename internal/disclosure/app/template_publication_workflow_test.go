package app

import "testing"

func TestPublicationCandidateHashIncludesAndNormalizesTemplateBlocks(t *testing.T) {
	workflow := TemplateBlockDTO{
		BlockKey: "enterprise_workflow", BlockType: "workflow", Title: "Workflow",
		DisplayOrder: 2, Enabled: true,
		Config: map[string]any{"steps": []any{
			map[string]any{
				"step_id": "review", "stage": "Review", "department_id": "legal",
				"assignee_role_ids": []any{"reviewer"}, "processing_days": 2, "display_order": 1,
			},
		}},
		Validation: map[string]any{},
	}
	policy := TemplateBlockDTO{
		BlockKey: "risk_policy", BlockType: "policy", Title: "Risk",
		DisplayOrder: 1, Enabled: true,
		Config: map[string]any{"approval_threshold": 2},
		Validation: map[string]any{"required": true},
	}
	req := UpsertTypeVersionRequest{
		TypeID: "type-a", Scope: "global", GroupID: "group-001", Name: "A",
		Blocks: []TemplateBlockDTO{workflow, policy},
	}

	first, err := BuildTemplatePublicationCandidate(req)
	if err != nil {
		t.Fatal(err)
	}
	req.Blocks = []TemplateBlockDTO{policy, workflow}
	reordered, err := BuildTemplatePublicationCandidate(req)
	if err != nil {
		t.Fatal(err)
	}
	if first.CandidateHash != reordered.CandidateHash {
		t.Fatal("candidate hash must be independent of request block order")
	}

	req.Blocks[0].Config = map[string]any{"approval_threshold": 3}
	mutated, err := BuildTemplatePublicationCandidate(req)
	if err != nil {
		t.Fatal(err)
	}
	if first.CandidateHash == mutated.CandidateHash {
		t.Fatal("publication-significant policy mutation must change candidate hash")
	}
}

func TestCanonicalWorkflowPublication_IsDeterministic(t *testing.T) {
	steps := []WorkflowStepDTO{{
		StepID: "review", Stage: "Review", DepartmentID: "legal",
		AssigneeRoleIds: []string{"reviewer"}, ProcessingDays: 2, DisplayOrder: 1,
		Documents: []WorkflowDocumentDTO{},
	}}
	_, raw1, hash1, err := CanonicalWorkflowPublication(steps)
	if err != nil {
		t.Fatal(err)
	}
	_, raw2, hash2, err := CanonicalWorkflowPublication(steps)
	if err != nil {
		t.Fatal(err)
	}
	if hash1 != hash2 || string(raw1) != string(raw2) {
		t.Fatal("WORKFLOW_MANIFEST_HASH_DETERMINISTIC must PASS")
	}
}
