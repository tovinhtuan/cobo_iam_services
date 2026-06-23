package app

import (
	"testing"

	wfcapp "github.com/cobo/cobo_iam_services/internal/workflowconfig/app"
)

// TestConflictRule1_SameFieldChanged is already exercised by
// TestComputeRebaseDiff_UpdateStepField_Conflict in workflow_override_diff_test.go; this test
// additionally asserts the persisted ConflictType field Batch 4 adds.
func TestConflictRule1_SameFieldChanged_ConflictType(t *testing.T) {
	base := []DiffStepInput{{Key: "review", Step: step("s1", "Review", "d1", nil, "T+1", 1)}}
	target := []DiffStepInput{{Key: "review", Step: step("s1", "Review", "d-global-new", nil, "T+1", 1)}}
	company := []DiffStepInput{{Key: "review", Step: step("s1", "Review", "d-company-custom", nil, "T+1", 1)}}

	_, conflicts := ComputeRebaseDiff(base, target, company)
	conf := findConflict(conflicts, "review", "department_id")
	if conf == nil {
		t.Fatalf("expected a department_id conflict")
	}
	if conf.ConflictType != ConflictTypeSameFieldChanged {
		t.Errorf("ConflictType = %q, want %q", conf.ConflictType, ConflictTypeSameFieldChanged)
	}
	if conf.Severity != ConflictSeverityAdvisory {
		t.Errorf("Severity = %q, want %q (the persisted 'warning' value)", conf.Severity, ConflictSeverityAdvisory)
	}
}

// TestConflictRule2_GlobalDeletedCustomizedStep_ConflictType.
func TestConflictRule2_GlobalDeletedCustomizedStep_ConflictType(t *testing.T) {
	base := []DiffStepInput{{Key: "legacy", Step: step("s1", "Legacy", "d1", nil, "T+1", 1)}}
	target := []DiffStepInput{}
	company := []DiffStepInput{{Key: "legacy", Step: step("s1", "Legacy Customized", "d9", []string{"compliance"}, "T+9", 1)}}

	_, conflicts := ComputeRebaseDiff(base, target, company)
	conf := findConflict(conflicts, "legacy", "__step_existence__")
	if conf == nil {
		t.Fatalf("expected a step-existence conflict")
	}
	if conf.ConflictType != ConflictTypeGlobalDeletedCustomized {
		t.Errorf("ConflictType = %q, want %q", conf.ConflictType, ConflictTypeGlobalDeletedCustomized)
	}
	if conf.Severity != ConflictSeverityBlocking {
		t.Errorf("Severity = %q, want %q", conf.Severity, ConflictSeverityBlocking)
	}
}

// TestConflictRule3_GlobalRenamedCompanyReassigned: global renamed `stage`, company independently
// changed `assignee_role_ids` (a DIFFERENT field) on the SAME step -> informational, severity info.
func TestConflictRule3_GlobalRenamedCompanyReassigned(t *testing.T) {
	base := []DiffStepInput{{Key: "review", Step: step("s1", "Review", "d1", []string{"reviewer"}, "T+1", 1)}}
	target := []DiffStepInput{{Key: "review", Step: step("s1", "Review v2", "d1", []string{"reviewer"}, "T+1", 1)}}
	company := []DiffStepInput{{Key: "review", Step: step("s1", "Review", "d1", []string{"compliance"}, "T+1", 1)}}

	conflicts := DetectRule3And4Conflicts(base, target, company)
	if len(conflicts) != 1 {
		t.Fatalf("expected exactly 1 rule-3 conflict, got %+v", conflicts)
	}
	if conflicts[0].ConflictType != ConflictTypeGlobalRenamedCompanyReassigned {
		t.Errorf("ConflictType = %q, want %q", conflicts[0].ConflictType, ConflictTypeGlobalRenamedCompanyReassigned)
	}
	if conflicts[0].Severity != ConflictSeverityInfo {
		t.Errorf("Severity = %q, want %q (rule 3 is never blocking/warning per CONFLICT_DETECTION_MODEL.md)", conflicts[0].Severity, ConflictSeverityInfo)
	}
}

// TestConflictRule3_DoesNotFireWhenCompanyAlsoRenamed: if the company ALSO changed stage, this is
// rule 1 (same field changed) territory, not rule 3 — must not double-report.
func TestConflictRule3_DoesNotFireWhenCompanyAlsoRenamed(t *testing.T) {
	base := []DiffStepInput{{Key: "review", Step: step("s1", "Review", "d1", []string{"reviewer"}, "T+1", 1)}}
	target := []DiffStepInput{{Key: "review", Step: step("s1", "Review v2", "d1", []string{"reviewer"}, "T+1", 1)}}
	company := []DiffStepInput{{Key: "review", Step: step("s1", "Review Custom", "d1", []string{"compliance"}, "T+1", 1)}}

	conflicts := DetectRule3And4Conflicts(base, target, company)
	for _, c := range conflicts {
		if c.ConflictType == ConflictTypeGlobalRenamedCompanyReassigned {
			t.Errorf("rule 3 must not fire when company also changed stage, got %+v", c)
		}
	}
}

// TestConflictRule4_ReorderAndDueDateChanged: global reordered, company independently changed
// due_rule (not display_order) -> informational, severity info.
func TestConflictRule4_ReorderAndDueDateChanged(t *testing.T) {
	base := []DiffStepInput{{Key: "review", Step: step("s1", "Review", "d1", nil, "T+3", 1)}}
	target := []DiffStepInput{{Key: "review", Step: step("s1", "Review", "d1", nil, "T+3", 5)}}
	company := []DiffStepInput{{Key: "review", Step: step("s1", "Review", "d1", nil, "T+9", 1)}}

	conflicts := DetectRule3And4Conflicts(base, target, company)
	if len(conflicts) != 1 {
		t.Fatalf("expected exactly 1 rule-4 conflict, got %+v", conflicts)
	}
	if conflicts[0].ConflictType != ConflictTypeReorderAndDueDateChanged {
		t.Errorf("ConflictType = %q, want %q", conflicts[0].ConflictType, ConflictTypeReorderAndDueDateChanged)
	}
	if conflicts[0].Severity != ConflictSeverityInfo {
		t.Errorf("Severity = %q, want %q", conflicts[0].Severity, ConflictSeverityInfo)
	}
}

// TestConflictRule5_NewStepUnresolvedRole: a brand-new global step (add_step) whose role doesn't
// resolve -> blocking conflict. A new step with a VALID role is "not a conflict by default".
func TestConflictRule5_NewStepUnresolvedRole(t *testing.T) {
	registry := wfcapp.DefaultRoleRegistry()
	ops := []PatchOperation{
		{Op: OpAddStep, StepKey: "new-step", Origin: OriginGlobalChanged, NewStep: &WorkflowStepDTO{
			StepID: "s2", Stage: "New Step", AssigneeRoleIds: []string{"role-that-does-not-exist-xyz"},
		}},
	}
	conflicts := DetectRule5Conflicts(ops, registry)
	if len(conflicts) != 1 {
		t.Fatalf("expected exactly 1 rule-5 conflict, got %+v", conflicts)
	}
	if conflicts[0].ConflictType != ConflictTypeNewMandatoryStepUnresolvedRole {
		t.Errorf("ConflictType = %q, want %q", conflicts[0].ConflictType, ConflictTypeNewMandatoryStepUnresolvedRole)
	}
	if conflicts[0].Severity != ConflictSeverityBlocking {
		t.Errorf("Severity = %q, want %q", conflicts[0].Severity, ConflictSeverityBlocking)
	}
}

func TestConflictRule5_NewStepValidRole_NotAConflict(t *testing.T) {
	registry := wfcapp.DefaultRoleRegistry()
	allRoles := registry.ListRoles()
	if len(allRoles) == 0 {
		t.Skip("no roles in default registry to test against")
	}
	validRole := allRoles[0].RoleID
	ops := []PatchOperation{
		{Op: OpAddStep, StepKey: "new-step", Origin: OriginGlobalChanged, NewStep: &WorkflowStepDTO{
			StepID: "s2", Stage: "New Step", AssigneeRoleIds: []string{validRole},
		}},
	}
	conflicts := DetectRule5Conflicts(ops, registry)
	if len(conflicts) != 0 {
		t.Errorf("a new step with a valid role must not be a conflict (rule 5: 'not a conflict by default'), got %+v", conflicts)
	}
}

// TestConflictRule6_CompanyRemovedMandatoryStep: company removed a step both base and target
// still carry -> conflict, severity warning (not blocking — the removal may be intentional, but
// must never be silent). This is the bug fixed in this batch (PREFLIGHT_AUDIT.md §3).
func TestConflictRule6_CompanyRemovedMandatoryStep(t *testing.T) {
	base := []DiffStepInput{{Key: "mandatory", Step: step("s1", "Mandatory Step", "d1", nil, "T+1", 1)}}
	target := []DiffStepInput{{Key: "mandatory", Step: step("s1", "Mandatory Step", "d1", nil, "T+1", 1)}}
	company := []DiffStepInput{} // company dropped it entirely

	_, conflicts := ComputeRebaseDiff(base, target, company)
	conf := findConflict(conflicts, "mandatory", "__step_existence__")
	if conf == nil {
		t.Fatalf("expected rule-6 conflict for a company-removed step still present in base+target, got conflicts=%+v", conflicts)
	}
	if conf.ConflictType != ConflictTypeCompanyRemovedMandatoryStep {
		t.Errorf("ConflictType = %q, want %q", conf.ConflictType, ConflictTypeCompanyRemovedMandatoryStep)
	}
	if conf.Severity != ConflictSeverityAdvisory {
		t.Errorf("Severity = %q, want %q (warning — not blocking, the removal may be intentional)", conf.Severity, ConflictSeverityAdvisory)
	}
}

// TestConflictRule7_RoleNoLongerExists_Implemented: every step in the company's CURRENT snapshot
// is checked against the role registry, independent of any other diff signal.
func TestConflictRule7_RoleNoLongerExists_Implemented(t *testing.T) {
	registry := wfcapp.DefaultRoleRegistry()
	company := []DiffStepInput{
		{Key: "step-with-bad-role", Step: &WorkflowStepDTO{StepID: "s1", AssigneeRoleIds: []string{"this-role-was-deleted-123"}}},
	}
	conflicts := DetectRule7Conflicts(company, registry)
	if len(conflicts) != 1 {
		t.Fatalf("expected exactly 1 rule-7 conflict, got %+v", conflicts)
	}
	if conflicts[0].ConflictType != ConflictTypeRoleNoLongerExists {
		t.Errorf("ConflictType = %q, want %q", conflicts[0].ConflictType, ConflictTypeRoleNoLongerExists)
	}
	if conflicts[0].Severity != ConflictSeverityBlocking {
		t.Errorf("Severity = %q, want %q", conflicts[0].Severity, ConflictSeverityBlocking)
	}
}

func TestConflictRule7_ValidRole_NotAConflict(t *testing.T) {
	registry := wfcapp.DefaultRoleRegistry()
	allRoles := registry.ListRoles()
	if len(allRoles) == 0 {
		t.Skip("no roles in default registry to test against")
	}
	company := []DiffStepInput{
		{Key: "step-with-good-role", Step: &WorkflowStepDTO{StepID: "s1", AssigneeRoleIds: []string{allRoles[0].RoleID}}},
	}
	conflicts := DetectRule7Conflicts(company, registry)
	if len(conflicts) != 0 {
		t.Errorf("a valid role must not be a conflict, got %+v", conflicts)
	}
}

// TestConflictRule8_DepartmentExistence_DeferredNotEvaluated is the explicit, asserted proof
// that Rule 8 is deferred (PREFLIGHT_AUDIT.md §5) — not a silently-missing feature. There is no
// DetectRule8Conflicts function in this codebase at all; this test documents that fact by
// confirming that an obviously-invalid department id produces ZERO conflict anywhere in the
// pipeline (ComputeRebaseDiff + all 4 detector functions), so a future reader cannot mistake
// "no conflict" for "department was validated and found fine."
func TestConflictRule8_DepartmentExistence_DeferredNotEvaluated(t *testing.T) {
	base := []DiffStepInput{{Key: "review", Step: step("s1", "Review", "dept-deleted-999", nil, "T+1", 1)}}
	target := []DiffStepInput{{Key: "review", Step: step("s1", "Review", "dept-deleted-999", nil, "T+1", 1)}}
	company := []DiffStepInput{{Key: "review", Step: step("s1", "Review", "dept-deleted-999", nil, "T+1", 1)}}

	_, conflicts := ComputeRebaseDiff(base, target, company)
	conflicts = append(conflicts, DetectRule3And4Conflicts(base, target, company)...)
	registry := wfcapp.DefaultRoleRegistry()
	conflicts = append(conflicts, DetectRule7Conflicts(company, registry)...)

	for _, c := range conflicts {
		if c.FieldPath == "department_id" || c.ConflictType == "department_no_longer_valid" {
			t.Errorf("Rule 8 must be deferred (not implemented) in Batch 4 — got an unexpected department-related conflict %+v", c)
		}
	}
}
