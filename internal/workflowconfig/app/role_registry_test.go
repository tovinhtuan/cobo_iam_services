package app_test

import (
	"testing"

	wfc "github.com/cobo/cobo_iam_services/internal/workflowconfig/app"
)

func reg() *wfc.RoleRegistry { return wfc.DefaultRoleRegistry() }

// 1. Registry contains expected existing roles (by code and by FE alias).
func TestRegistry_ContainsExpectedRoles(t *testing.T) {
	r := reg()
	for _, key := range []string{
		"reviewer", "approver", "confirmer", "legal_reviewer", "finance_reviewer",
		"disclosure_owner", "system_reviewer", "compliance_officer",
		"role-reviewer", "role-approver", "role-legal-reviewer", "disclosure_reviewer", // aliases
	} {
		if _, ok := r.GetRole(key); !ok {
			t.Errorf("expected role/alias %q in registry", key)
		}
	}
	if len(r.ListRoles()) == 0 {
		t.Fatal("ListRoles empty")
	}
}

// 2. system_owned cannot override.
func TestRegistry_SystemOwnedCannotOverride(t *testing.T) {
	r := reg()
	if !r.IsSystemOwned("disclosure_owner") {
		t.Fatal("disclosure_owner should be system_owned")
	}
	if r.CanOverride("disclosure_owner") {
		t.Fatal("system_owned role must not be overridable")
	}
	if err := r.ValidateRoleBoundary("disclosure_owner", true); err == nil {
		t.Fatal("expected boundary error overriding system_owned role")
	}
}

// 3. protected + locked cannot override.
func TestRegistry_ProtectedLockedCannotOverride(t *testing.T) {
	r := reg()
	if !r.IsProtected("compliance_officer") || !r.IsLocked("compliance_officer") {
		t.Fatal("compliance_officer should be protected + locked")
	}
	if r.CanOverride("compliance_officer") {
		t.Fatal("locked protected role must not be overridable")
	}
	if err := r.ValidateRoleBoundary("compliance_officer", true); err == nil {
		t.Fatal("expected boundary error overriding locked role")
	}
}

// 4. standard can override where allowed.
func TestRegistry_StandardCanOverride(t *testing.T) {
	r := reg()
	if !r.CanOverride("reviewer") {
		t.Fatal("standard reviewer should be overridable")
	}
	if err := r.ValidateRoleOverride("reviewer", wfc.ScopeDepartment); err != nil {
		t.Fatalf("reviewer override to department should pass: %v", err)
	}
}

// 5. approval role is detected.
func TestRegistry_ApprovalRoleDetected(t *testing.T) {
	r := reg()
	if !r.IsApprovalRole("approver") {
		t.Fatal("approver should be an approval role")
	}
	if r.IsApprovalRole("reviewer") {
		t.Fatal("reviewer should not be an approval role")
	}
}

// 6. mandatory approval role validation fails if missing, passes if present.
func TestRegistry_MandatoryApprovalValidation(t *testing.T) {
	r := reg()
	missing := wfc.StepsFromRoleIDs([][]string{{"reviewer"}, {"legal_reviewer"}})
	if err := r.ValidateMandatoryApprovalRole(missing); err == nil {
		t.Fatal("expected error when no approval role present")
	}
	present := wfc.StepsFromRoleIDs([][]string{{"reviewer"}, {"approver"}})
	if err := r.ValidateMandatoryApprovalRole(present); err != nil {
		t.Fatalf("expected pass when approval role present: %v", err)
	}
	// alias form also works.
	aliasPresent := wfc.StepsFromRoleIDs([][]string{{"role-approver"}})
	if err := r.ValidateMandatoryApprovalRole(aliasPresent); err != nil {
		t.Fatalf("alias approval role should be detected: %v", err)
	}
}

// 7. unknown role returns validation error.
func TestRegistry_UnknownRoleErrors(t *testing.T) {
	r := reg()
	if err := r.ValidateRoleExists("nope_role"); err == nil {
		t.Fatal("expected error for unknown role")
	}
	if _, err := r.MustGetRole("nope_role"); err == nil {
		t.Fatal("MustGetRole should error on unknown")
	}
	if err := r.ValidateRoleBoundary("nope_role", false); err == nil {
		t.Fatal("ValidateRoleBoundary should error on unknown role")
	}
}

// 8. assignment scope validation works.
func TestRegistry_AssignmentScopeValidation(t *testing.T) {
	r := reg()
	if !r.CanAssignToScope("reviewer", wfc.ScopeGroup) {
		t.Fatal("reviewer should allow group scope")
	}
	if r.CanAssignToScope("system_reviewer", wfc.ScopeDepartment) {
		t.Fatal("system_reviewer should not allow department scope")
	}
	if err := r.ValidateRoleOverride("reviewer", "nonexistent_scope"); err == nil {
		t.Fatal("expected error for invalid scope")
	}
}
