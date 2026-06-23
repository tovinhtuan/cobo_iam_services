package app

import (
	"reflect"
	"testing"
)

func step(stepID, stage, dept string, roles []string, dueRule string, displayOrder int) *WorkflowStepDTO {
	return &WorkflowStepDTO{
		StepID: stepID, Stage: stage, DepartmentID: dept, AssigneeRoleIds: roles,
		DueRule: dueRule, DisplayOrder: displayOrder,
	}
}

func findOp(ops []PatchOperation, stepKey, op string) *PatchOperation {
	for i := range ops {
		if ops[i].StepKey == stepKey && ops[i].Op == op {
			return &ops[i]
		}
	}
	return nil
}

func findConflict(conflicts []PreviewConflict, stepKey, fieldPath string) *PreviewConflict {
	for i := range conflicts {
		if conflicts[i].StepKey == stepKey && conflicts[i].FieldPath == fieldPath {
			return &conflicts[i]
		}
	}
	return nil
}

// TestComputeRebaseDiff_AddStep: target has a step the base/company never had -> add_step,
// origin global_changed.
func TestComputeRebaseDiff_AddStep(t *testing.T) {
	base := []DiffStepInput{{Key: "review", Step: step("s1", "Review", "d1", []string{"r1"}, "T+1", 1)}}
	target := []DiffStepInput{
		{Key: "review", Step: step("s1", "Review", "d1", []string{"r1"}, "T+1", 1)},
		{Key: "approval", Step: step("s2", "Approval", "d2", []string{"r2"}, "T+2", 2)},
	}
	company := []DiffStepInput{{Key: "review", Step: step("s1", "Review", "d1", []string{"r1"}, "T+1", 1)}}

	ops, conflicts := ComputeRebaseDiff(base, target, company)
	op := findOp(ops, "approval", OpAddStep)
	if op == nil {
		t.Fatalf("expected add_step for 'approval', got ops=%+v", ops)
	}
	if op.Origin != OriginGlobalChanged {
		t.Errorf("origin = %q, want %q", op.Origin, OriginGlobalChanged)
	}
	if op.NewStep == nil || op.NewStep.Stage != "Approval" {
		t.Errorf("NewStep not populated correctly: %+v", op.NewStep)
	}
	if len(conflicts) != 0 {
		t.Errorf("expected no conflicts, got %+v", conflicts)
	}
}

// TestComputeRebaseDiff_RemoveStep_Clean: global deleted a step the company also no longer has
// (company already dropped it too) -> remove_step, no conflict (rule 6's clean case).
func TestComputeRebaseDiff_RemoveStep_Clean(t *testing.T) {
	base := []DiffStepInput{{Key: "legacy", Step: step("s1", "Legacy", "d1", nil, "T+1", 1)}}
	target := []DiffStepInput{}
	company := []DiffStepInput{}

	ops, conflicts := ComputeRebaseDiff(base, target, company)
	op := findOp(ops, "legacy", OpRemoveStep)
	if op == nil {
		t.Fatalf("expected remove_step for 'legacy', got ops=%+v", ops)
	}
	if len(conflicts) != 0 {
		t.Errorf("expected no conflicts, got %+v", conflicts)
	}
}

// TestComputeRebaseDiff_RemoveStep_Conflict: global deleted a step the company customized
// (company still carries it) -> CONFLICT (rule 2/6), severity blocking.
func TestComputeRebaseDiff_RemoveStep_Conflict(t *testing.T) {
	base := []DiffStepInput{{Key: "legacy", Step: step("s1", "Legacy", "d1", nil, "T+1", 1)}}
	target := []DiffStepInput{}
	company := []DiffStepInput{{Key: "legacy", Step: step("s1", "Legacy Customized", "d9", []string{"compliance"}, "T+9", 1)}}

	ops, conflicts := ComputeRebaseDiff(base, target, company)
	if op := findOp(ops, "legacy", OpRemoveStep); op != nil {
		t.Errorf("expected NO clean remove_step op when company customized the step, got %+v", op)
	}
	conf := findConflict(conflicts, "legacy", "__step_existence__")
	if conf == nil {
		t.Fatalf("expected a step-existence conflict, got conflicts=%+v", conflicts)
	}
	if conf.Severity != ConflictSeverityBlocking {
		t.Errorf("severity = %q, want %q", conf.Severity, ConflictSeverityBlocking)
	}
}

// TestComputeRebaseDiff_UpdateStepField_GlobalOnly: only global changed 'stage' -> clean op.
func TestComputeRebaseDiff_UpdateStepField_GlobalOnly(t *testing.T) {
	base := []DiffStepInput{{Key: "review", Step: step("s1", "Review", "d1", nil, "T+1", 1)}}
	target := []DiffStepInput{{Key: "review", Step: step("s1", "Review v2", "d1", nil, "T+1", 1)}}
	company := []DiffStepInput{{Key: "review", Step: step("s1", "Review", "d1", nil, "T+1", 1)}}

	ops, conflicts := ComputeRebaseDiff(base, target, company)
	op := findOp(ops, "review", OpUpdateStepField)
	if op == nil || op.FieldPath != "stage" {
		t.Fatalf("expected update_step_field on 'stage', got ops=%+v", ops)
	}
	if op.Origin != OriginGlobalChanged {
		t.Errorf("origin = %q, want %q", op.Origin, OriginGlobalChanged)
	}
	if len(conflicts) != 0 {
		t.Errorf("expected no conflicts, got %+v", conflicts)
	}
}

// TestComputeRebaseDiff_UpdateStepField_CompanyOnly: only company changed 'department_id' ->
// nothing to rebase (company's customization stands untouched).
func TestComputeRebaseDiff_UpdateStepField_CompanyOnly(t *testing.T) {
	base := []DiffStepInput{{Key: "review", Step: step("s1", "Review", "d1", nil, "T+1", 1)}}
	target := []DiffStepInput{{Key: "review", Step: step("s1", "Review", "d1", nil, "T+1", 1)}}
	company := []DiffStepInput{{Key: "review", Step: step("s1", "Review", "d-custom", nil, "T+1", 1)}}

	ops, conflicts := ComputeRebaseDiff(base, target, company)
	if len(ops) != 0 {
		t.Errorf("expected no operations (company-only change needs no rebase action), got %+v", ops)
	}
	if len(conflicts) != 0 {
		t.Errorf("expected no conflicts, got %+v", conflicts)
	}
}

// TestComputeRebaseDiff_UpdateStepField_Conflict: both sides changed 'department_id' to
// DIFFERENT values -> conflict (rule 1).
func TestComputeRebaseDiff_UpdateStepField_Conflict(t *testing.T) {
	base := []DiffStepInput{{Key: "review", Step: step("s1", "Review", "d1", nil, "T+1", 1)}}
	target := []DiffStepInput{{Key: "review", Step: step("s1", "Review", "d-global-new", nil, "T+1", 1)}}
	company := []DiffStepInput{{Key: "review", Step: step("s1", "Review", "d-company-custom", nil, "T+1", 1)}}

	ops, conflicts := ComputeRebaseDiff(base, target, company)
	if op := findOp(ops, "review", OpUpdateStepField); op != nil {
		t.Errorf("expected NO clean op when both sides disagree, got %+v", op)
	}
	conf := findConflict(conflicts, "review", "department_id")
	if conf == nil {
		t.Fatalf("expected a department_id conflict, got conflicts=%+v", conflicts)
	}
	if conf.GlobalOld != "d1" || conf.GlobalNew != "d-global-new" || conf.CompanyValue != "d-company-custom" {
		t.Errorf("conflict values wrong: %+v", conf)
	}
}

// TestComputeRebaseDiff_BothChangedSameValue: both sides coincidentally converge -> clean op,
// origin both_changed, NOT a conflict.
func TestComputeRebaseDiff_BothChangedSameValue(t *testing.T) {
	base := []DiffStepInput{{Key: "review", Step: step("s1", "Review", "d1", nil, "T+1", 1)}}
	target := []DiffStepInput{{Key: "review", Step: step("s1", "Review", "d-same", nil, "T+1", 1)}}
	company := []DiffStepInput{{Key: "review", Step: step("s1", "Review", "d-same", nil, "T+1", 1)}}

	ops, conflicts := ComputeRebaseDiff(base, target, company)
	op := findOp(ops, "review", OpUpdateStepField)
	if op == nil || op.Origin != OriginBothChanged {
		t.Fatalf("expected update_step_field origin=both_changed, got ops=%+v", ops)
	}
	if len(conflicts) != 0 {
		t.Errorf("expected no conflicts when both sides converge, got %+v", conflicts)
	}
}

// TestComputeRebaseDiff_ReorderStep: display_order changed -> reorder_step, never a conflict
// (rule 4 — order has no field overlap with anything else).
func TestComputeRebaseDiff_ReorderStep(t *testing.T) {
	base := []DiffStepInput{{Key: "review", Step: step("s1", "Review", "d1", nil, "T+1", 1)}}
	target := []DiffStepInput{{Key: "review", Step: step("s1", "Review", "d1", nil, "T+1", 3)}}
	company := []DiffStepInput{{Key: "review", Step: step("s1", "Review", "d1", nil, "T+1", 1)}}

	ops, conflicts := ComputeRebaseDiff(base, target, company)
	op := findOp(ops, "review", OpReorderStep)
	if op == nil {
		t.Fatalf("expected reorder_step, got ops=%+v", ops)
	}
	if op.OldDisplayOrder == nil || *op.OldDisplayOrder != 1 || op.NewDisplayOrder == nil || *op.NewDisplayOrder != 3 {
		t.Errorf("reorder values wrong: %+v", op)
	}
	if len(conflicts) != 0 {
		t.Errorf("reorder must never be a conflict, got %+v", conflicts)
	}
}

// TestComputeRebaseDiff_ReplaceAssignees: compared as a SET, order-independent.
func TestComputeRebaseDiff_ReplaceAssignees(t *testing.T) {
	base := []DiffStepInput{{Key: "legal", Step: step("s1", "Legal", "d1", []string{"legal"}, "T+1", 1)}}
	target := []DiffStepInput{{Key: "legal", Step: step("s1", "Legal", "d1", []string{"risk", "legal"}, "T+1", 1)}}
	company := []DiffStepInput{{Key: "legal", Step: step("s1", "Legal", "d1", []string{"legal"}, "T+1", 1)}}

	ops, conflicts := ComputeRebaseDiff(base, target, company)
	op := findOp(ops, "legal", OpReplaceAssignees)
	if op == nil {
		t.Fatalf("expected replace_assignees, got ops=%+v", ops)
	}
	if len(op.NewAssigneeRoleIds) != 2 {
		t.Errorf("new assignees = %+v, want 2 roles", op.NewAssigneeRoleIds)
	}
	if len(conflicts) != 0 {
		t.Errorf("expected no conflicts, got %+v", conflicts)
	}
}

// TestComputeRebaseDiff_ReplaceAssignees_OrderInvariant: same roles, different order on each
// side -> must NOT register as changed (PATCH_MODEL_OPTIONS.md §3: compared as a set).
func TestComputeRebaseDiff_ReplaceAssignees_OrderInvariant(t *testing.T) {
	base := []DiffStepInput{{Key: "legal", Step: step("s1", "Legal", "d1", []string{"legal", "risk"}, "T+1", 1)}}
	target := []DiffStepInput{{Key: "legal", Step: step("s1", "Legal", "d1", []string{"risk", "legal"}, "T+1", 1)}}
	company := []DiffStepInput{{Key: "legal", Step: step("s1", "Legal", "d1", []string{"legal", "risk"}, "T+1", 1)}}

	ops, _ := ComputeRebaseDiff(base, target, company)
	if op := findOp(ops, "legal", OpReplaceAssignees); op != nil {
		t.Errorf("reordering the same set must not register as a change, got %+v", op)
	}
}

// TestComputeRebaseDiff_ReplaceDueRule.
func TestComputeRebaseDiff_ReplaceDueRule(t *testing.T) {
	base := []DiffStepInput{{Key: "legal", Step: step("s1", "Legal", "d1", nil, "T+3", 1)}}
	target := []DiffStepInput{{Key: "legal", Step: step("s1", "Legal", "d1", nil, "T+5", 1)}}
	company := []DiffStepInput{{Key: "legal", Step: step("s1", "Legal", "d1", nil, "T+3", 1)}}

	ops, _ := ComputeRebaseDiff(base, target, company)
	op := findOp(ops, "legal", OpReplaceDueRule)
	if op == nil || op.OldDueRule != "T+3" || op.NewDueRule != "T+5" {
		t.Fatalf("expected replace_due_rule T+3->T+5, got ops=%+v", ops)
	}
}

// TestComputeRebaseDiff_ReplaceReminders.
func TestComputeRebaseDiff_ReplaceReminders(t *testing.T) {
	baseStep := step("s1", "Legal", "d1", nil, "T+1", 1)
	targetStep := step("s1", "Legal", "d1", nil, "T+1", 1)
	targetStep.ReminderConfig = &WorkflowStepReminderConfig{Enabled: true, Mode: "days_before", DaysBefore: []int{1, 3}}
	companyStep := step("s1", "Legal", "d1", nil, "T+1", 1)

	base := []DiffStepInput{{Key: "legal", Step: baseStep}}
	target := []DiffStepInput{{Key: "legal", Step: targetStep}}
	company := []DiffStepInput{{Key: "legal", Step: companyStep}}

	ops, _ := ComputeRebaseDiff(base, target, company)
	op := findOp(ops, "legal", OpReplaceReminders)
	if op == nil {
		t.Fatalf("expected replace_reminders, got ops=%+v", ops)
	}
	if op.NewReminderConfig == nil || !op.NewReminderConfig.Enabled {
		t.Errorf("new reminder config wrong: %+v", op.NewReminderConfig)
	}
}

// TestComputeRebaseDiff_NilVsEmptyNormalization: a nil ReminderConfig on one side and an
// explicit "disabled, no days" on the other must be treated as EQUAL (no operation, no conflict)
// — mirrors HASH_CONTRACT.md §5's nil-vs-empty principle.
func TestComputeRebaseDiff_NilVsEmptyNormalization(t *testing.T) {
	baseStep := step("s1", "Legal", "d1", nil, "T+1", 1)
	baseStep.ReminderConfig = nil
	baseStep.Documents = nil
	baseStep.Groups = nil

	targetStep := step("s1", "Legal", "d1", nil, "T+1", 1)
	targetStep.ReminderConfig = &WorkflowStepReminderConfig{Enabled: false, DaysBefore: []int{}}
	targetStep.Documents = []WorkflowDocumentDTO{}
	targetStep.Groups = []WorkflowStepGroupDTO{}

	companyStep := step("s1", "Legal", "d1", nil, "T+1", 1)

	base := []DiffStepInput{{Key: "legal", Step: baseStep}}
	target := []DiffStepInput{{Key: "legal", Step: targetStep}}
	company := []DiffStepInput{{Key: "legal", Step: companyStep}}

	ops, conflicts := ComputeRebaseDiff(base, target, company)
	if len(ops) != 0 {
		t.Errorf("nil vs empty must not register as a change, got ops=%+v", ops)
	}
	if len(conflicts) != 0 {
		t.Errorf("nil vs empty must not register as a conflict, got conflicts=%+v", conflicts)
	}
}

// TestComputeRebaseDiff_CompanyOnlyStep_NeverDiffed: a step the company added that has no base
// OR target counterpart survives untouched — not an operation, not a conflict.
func TestComputeRebaseDiff_CompanyOnlyStep_NeverDiffed(t *testing.T) {
	base := []DiffStepInput{}
	target := []DiffStepInput{}
	company := []DiffStepInput{{Key: "company-extra", Step: step("custom-1", "Company Extra Step", "d1", nil, "T+1", 1)}}

	ops, conflicts := ComputeRebaseDiff(base, target, company)
	if len(ops) != 0 {
		t.Errorf("company-only step must produce no operation, got %+v", ops)
	}
	if len(conflicts) != 0 {
		t.Errorf("company-only step must produce no conflict, got %+v", conflicts)
	}
}

// TestComputeRebaseDiff_MissingStepKey: an empty Key on any input side triggers a
// STEP_IDENTITY_UNCLEAR, severity blocking conflict (Patch Engine Requirement #3) — never
// silently dropped, never guessed into a match.
func TestComputeRebaseDiff_MissingStepKey(t *testing.T) {
	base := []DiffStepInput{{Key: "", Step: step("", "Unclear", "d1", nil, "T+1", 1)}}
	target := []DiffStepInput{}
	company := []DiffStepInput{}

	_, conflicts := ComputeRebaseDiff(base, target, company)
	if len(conflicts) != 1 {
		t.Fatalf("expected exactly 1 STEP_IDENTITY_UNCLEAR conflict, got %+v", conflicts)
	}
	if conflicts[0].Reason != ConflictReasonStepIdentityUnclear {
		t.Errorf("reason = %q, want %q", conflicts[0].Reason, ConflictReasonStepIdentityUnclear)
	}
	if conflicts[0].Severity != ConflictSeverityBlocking {
		t.Errorf("severity = %q, want %q", conflicts[0].Severity, ConflictSeverityBlocking)
	}
}

// TestComputeRebaseDiff_Deterministic: calling ComputeRebaseDiff twice with the same inputs (in
// a different slice order) must produce byte-for-byte identical, sorted output — Patch Engine
// Requirement #8 ("Patch output must be deterministic").
func TestComputeRebaseDiff_Deterministic(t *testing.T) {
	base := []DiffStepInput{
		{Key: "b-step", Step: step("s2", "B", "d2", nil, "T+2", 2)},
		{Key: "a-step", Step: step("s1", "A", "d1", nil, "T+1", 1)},
	}
	target := []DiffStepInput{
		{Key: "b-step", Step: step("s2", "B2", "d2", nil, "T+2", 2)},
		{Key: "a-step", Step: step("s1", "A2", "d1", nil, "T+1", 1)},
	}
	company := []DiffStepInput{
		{Key: "b-step", Step: step("s2", "B", "d2", nil, "T+2", 2)},
		{Key: "a-step", Step: step("s1", "A", "d1", nil, "T+1", 1)},
	}

	ops1, _ := ComputeRebaseDiff(base, target, company)
	ops2, _ := ComputeRebaseDiff(base, target, company)

	if len(ops1) != len(ops2) {
		t.Fatalf("non-deterministic op count: %d vs %d", len(ops1), len(ops2))
	}
	for i := range ops1 {
		if !reflect.DeepEqual(ops1[i], ops2[i]) {
			t.Errorf("non-deterministic output at index %d: %+v vs %+v", i, ops1[i], ops2[i])
		}
	}
	// Sorted by step_key then op (a-step before b-step).
	if ops1[0].StepKey != "a-step" {
		t.Errorf("expected sorted output starting with 'a-step', got %q first", ops1[0].StepKey)
	}
}

// TestComputeRebaseDiff_DocumentsAndGroupsFieldsCovered proves the "every field survives" rule
// (PATCH_MODEL_OPTIONS.md §5) for the two fields easiest to silently forget: documents and
// groups (the exact Finding-3 risk class, relocated to a new code path).
func TestComputeRebaseDiff_DocumentsAndGroupsFieldsCovered(t *testing.T) {
	baseStep := step("s1", "Legal", "d1", nil, "T+1", 1)
	targetStep := step("s1", "Legal", "d1", nil, "T+1", 1)
	targetStep.Documents = []WorkflowDocumentDTO{{DocID: "doc-1", Name: "Bien ban", Required: true}}
	targetStep.Groups = []WorkflowStepGroupDTO{{GroupID: "g1", GroupName: "To 1", DepartmentID: "d1"}}
	companyStep := step("s1", "Legal", "d1", nil, "T+1", 1)

	base := []DiffStepInput{{Key: "legal", Step: baseStep}}
	target := []DiffStepInput{{Key: "legal", Step: targetStep}}
	company := []DiffStepInput{{Key: "legal", Step: companyStep}}

	ops, _ := ComputeRebaseDiff(base, target, company)
	if findOp(ops, "legal", OpUpdateStepField) == nil {
		t.Fatalf("expected at least one update_step_field for documents/groups, got %+v", ops)
	}
	docOp := false
	groupOp := false
	for _, op := range ops {
		if op.StepKey == "legal" && op.Op == OpUpdateStepField && op.FieldPath == "documents" {
			docOp = true
		}
		if op.StepKey == "legal" && op.Op == OpUpdateStepField && op.FieldPath == "groups" {
			groupOp = true
		}
	}
	if !docOp {
		t.Errorf("documents field change was not detected: %+v", ops)
	}
	if !groupOp {
		t.Errorf("groups field change was not detected: %+v", ops)
	}
}
