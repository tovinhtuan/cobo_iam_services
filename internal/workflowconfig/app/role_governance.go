package app

import (
	"fmt"
	"strings"
)

// WorkflowStepLike is a minimal view of a workflow step for governance validation, so these helpers
// do not depend on any large domain type. Later batches can adapt their step types to it.
type WorkflowStepLike interface {
	// AssigneeRoleIDs returns the role ids/codes assigned to the step.
	AssigneeRoleIDs() []string
}

// roleSlice is a trivial adapter so callers can validate a plain [][]string of role ids if they have
// no concrete step type yet.
type roleSlice []string

func (r roleSlice) AssigneeRoleIDs() []string { return r }

// StepsFromRoleIDs adapts a list of per-step role-id slices into []WorkflowStepLike.
func StepsFromRoleIDs(perStepRoleIDs [][]string) []WorkflowStepLike {
	out := make([]WorkflowStepLike, 0, len(perStepRoleIDs))
	for _, ids := range perStepRoleIDs {
		out = append(out, roleSlice(ids))
	}
	return out
}

// ─── Governance validation helpers (pure; no runtime side effects) ──────────────

// ValidateRoleExists returns an error if the role is not in the registry.
func (r *RoleRegistry) ValidateRoleExists(roleID string) error {
	if _, ok := r.GetRole(roleID); !ok {
		return fmt.Errorf("unknown role %q", strings.TrimSpace(roleID))
	}
	return nil
}

// ValidateRoleBoundary enforces the override boundary for a role. If a tenant attempts to override a
// role (attemptedOverride == true), it must be overridable (not system_owned, not locked, override
// allowed). Unknown roles are rejected.
func (r *RoleRegistry) ValidateRoleBoundary(roleID string, attemptedOverride bool) error {
	d, ok := r.GetRole(roleID)
	if !ok {
		return fmt.Errorf("unknown role %q", strings.TrimSpace(roleID))
	}
	if !attemptedOverride {
		return nil
	}
	if d.Class == RoleClassSystemOwned {
		return fmt.Errorf("role %q is system-owned and cannot be overridden", d.Code)
	}
	if d.Locked {
		return fmt.Errorf("role %q is locked and cannot be overridden", d.Code)
	}
	if !d.AllowedOverride {
		return fmt.Errorf("role %q does not allow override", d.Code)
	}
	return nil
}

// ValidateRoleOverride checks both that the role may be overridden and that the target scope is
// allowed for that role.
func (r *RoleRegistry) ValidateRoleOverride(roleID, targetScope string) error {
	if err := r.ValidateRoleBoundary(roleID, true); err != nil {
		return err
	}
	if strings.TrimSpace(targetScope) == "" {
		return nil
	}
	if !r.CanAssignToScope(roleID, targetScope) {
		d, _ := r.GetRole(roleID)
		return fmt.Errorf("role %q cannot be assigned to scope %q", d.Code, strings.TrimSpace(targetScope))
	}
	return nil
}

// ValidateMandatoryApprovalRole ensures at least one step is assigned a role that IsApprovalRole.
// Enforces Phase 13A: every workflow must retain an approval role (mandatory, non-removable).
func (r *RoleRegistry) ValidateMandatoryApprovalRole(steps []WorkflowStepLike) error {
	for _, s := range steps {
		for _, roleID := range s.AssigneeRoleIDs() {
			if r.IsApprovalRole(roleID) {
				return nil
			}
		}
	}
	return fmt.Errorf("workflow must include at least one approval role")
}
