// Package app hosts the workflow-configuration design-time services (Phase 11–15 baseline).
//
// Batch 3 — Role Registry: a canonical, in-code registry of workflow assignee roles and their
// governance classes. It is the single source of truth for later batches (ResolveAssignees, routing
// governance, override boundary, protected/system-owned enforcement, mandatory approval validation).
//
// This batch ONLY provides the registry + pure validation helpers. It does NOT enforce anything on
// existing APIs and does NOT change runtime behavior, so no feature flag is required here.
package app

import (
	"fmt"
	"sort"
	"strings"
)

// RoleClass is the governance class of a role (Phase 13A ASSIGNMENT_GOVERNANCE_RULES).
type RoleClass string

const (
	// RoleClassSystemOwned: system-managed; tenant cannot override, remove, or remap.
	RoleClassSystemOwned RoleClass = "system_owned"
	// RoleClassProtected: business-critical; visible to tenant; overridable only if policy allows
	// and Locked == false.
	RoleClassProtected RoleClass = "protected"
	// RoleClassStandard: normal role; tenant may map/override within the allowed boundary.
	RoleClassStandard RoleClass = "standard"
)

// Assignment scopes a role may be bound to (kept minimal — not over-engineered).
const (
	ScopeUser           = "user"
	ScopeDepartment     = "department"
	ScopeGroup          = "group"
	ScopeCompanyDefault = "company_default"
	ScopeSystem         = "system"
)

// RoleDefinition is one canonical role entry.
type RoleDefinition struct {
	RoleID                  string
	Code                    string
	Name                    string
	Class                   RoleClass
	Locked                  bool // role_locked: tenant cannot change the role on a locked binding
	Mandatory               bool // mandatory role that cannot be removed (e.g. the approval role)
	Description             string
	AllowedOverride         bool // tenant may override/replace this role (within boundary)
	AllowedAssignmentScopes []string
	IsApprovalRole          bool
	// Aliases are alternative identifiers seen in source (e.g. FE ids like "role-reviewer").
	Aliases []string
}

// RoleRegistry is an immutable lookup over the seeded role definitions.
type RoleRegistry struct {
	byKey map[string]RoleDefinition // normalized RoleID/Code/Alias -> definition
	all   []RoleDefinition
}

func normKey(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// NewRoleRegistry builds the registry from the given definitions, indexing RoleID, Code, and aliases.
func NewRoleRegistry(defs []RoleDefinition) *RoleRegistry {
	r := &RoleRegistry{byKey: make(map[string]RoleDefinition), all: make([]RoleDefinition, 0, len(defs))}
	for _, d := range defs {
		r.all = append(r.all, d)
		for _, k := range append([]string{d.RoleID, d.Code}, d.Aliases...) {
			if nk := normKey(k); nk != "" {
				r.byKey[nk] = d
			}
		}
	}
	sort.Slice(r.all, func(i, j int) bool { return r.all[i].RoleID < r.all[j].RoleID })
	return r
}

// DefaultRoleRegistry returns the canonical seed (see CanonicalRoleSeed).
func DefaultRoleRegistry() *RoleRegistry { return NewRoleRegistry(CanonicalRoleSeed()) }

// ─── Lookup API ───────────────────────────────────────────────────────────────

func (r *RoleRegistry) GetRole(roleIDOrCode string) (RoleDefinition, bool) {
	d, ok := r.byKey[normKey(roleIDOrCode)]
	return d, ok
}

func (r *RoleRegistry) MustGetRole(roleIDOrCode string) (RoleDefinition, error) {
	if d, ok := r.GetRole(roleIDOrCode); ok {
		return d, nil
	}
	return RoleDefinition{}, fmt.Errorf("role not found in registry: %q", roleIDOrCode)
}

func (r *RoleRegistry) ListRoles() []RoleDefinition {
	out := make([]RoleDefinition, len(r.all))
	copy(out, r.all)
	return out
}

func (r *RoleRegistry) IsSystemOwned(roleIDOrCode string) bool {
	d, ok := r.GetRole(roleIDOrCode)
	return ok && d.Class == RoleClassSystemOwned
}

func (r *RoleRegistry) IsProtected(roleIDOrCode string) bool {
	d, ok := r.GetRole(roleIDOrCode)
	return ok && d.Class == RoleClassProtected
}

func (r *RoleRegistry) IsLocked(roleIDOrCode string) bool {
	d, ok := r.GetRole(roleIDOrCode)
	return ok && d.Locked
}

func (r *RoleRegistry) IsApprovalRole(roleIDOrCode string) bool {
	d, ok := r.GetRole(roleIDOrCode)
	return ok && d.IsApprovalRole
}

// CanOverride reports whether a tenant may override/replace this role. system_owned roles and locked
// roles can never be overridden, regardless of AllowedOverride.
func (r *RoleRegistry) CanOverride(roleIDOrCode string) bool {
	d, ok := r.GetRole(roleIDOrCode)
	if !ok {
		return false
	}
	if d.Class == RoleClassSystemOwned || d.Locked {
		return false
	}
	return d.AllowedOverride
}

func (r *RoleRegistry) CanAssignToScope(roleIDOrCode string, scope string) bool {
	d, ok := r.GetRole(roleIDOrCode)
	if !ok {
		return false
	}
	scope = normKey(scope)
	for _, s := range d.AllowedAssignmentScopes {
		if normKey(s) == scope {
			return true
		}
	}
	return false
}
