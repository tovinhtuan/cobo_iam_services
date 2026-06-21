package app

import (
	"context"
	"errors"
	"strings"
)

// Batch 4 — Assignee Resolution.
//
// Resolves the assignee(s) of a workflow step to concrete memberships, using the Role Registry for
// governance and an AssigneeDirectory for the actual lookups. There is NO silent fallback to the
// current user: when nothing maps, the result is an explicit, controlled "unresolved".
//
// The resolved unit is membership_id (the workflow assignee unit — Task.AssigneeMembershipID).

// Resolution reasons (stable strings for logs/metrics/UI later).
const (
	ReasonResolvedExplicitUser  = "resolved_explicit_user"
	ReasonResolvedRole          = "resolved_role"
	ReasonResolvedDepartment    = "resolved_department"
	ReasonUnresolvedNoMapping   = "unresolved_no_mapping"
	ReasonUnresolvedUnknownRole = "unresolved_unknown_role"
	ReasonUnresolvedProtected   = "unresolved_protected_role"
	ReasonUnresolvedEmpty       = "unresolved_empty_result"
	ReasonUnsupportedScope      = "unsupported_scope"
)

// Resolution sources.
const (
	SourceExplicitUser   = "explicit_user"
	SourceRole           = "role"
	SourceDepartment     = "department"
	SourceCompanyDefault = "company_default"
	SourceNone           = "none"
)

// ErrUnsupportedScope is returned by an AssigneeDirectory for a scope it cannot serve.
var ErrUnsupportedScope = errors.New("unsupported assignee scope")

// AssigneeResolutionResult is the controlled outcome of a resolution attempt.
// MembershipIDs are the resolved assignee memberships (the system's assignee unit).
type AssigneeResolutionResult struct {
	Resolved      bool
	MembershipIDs []string
	Reason        string
	Source        string
	RoleID        string
	Scope         string
	Blocking      bool // true when this unresolved state must block publish/activate downstream
}

// AssigneeResolutionInput describes one step's assignee configuration to resolve.
type AssigneeResolutionInput struct {
	CompanyID             string
	ExplicitMembershipIDs []string // explicit assignees take precedence
	RoleID                string   // role id/code (registry-resolvable)
	DepartmentID          string
	GroupID               string
	AttemptedOverride     bool // true when this is a tenant override path
}

// AssigneeDirectory looks up memberships by route. Implementations return membership_ids.
// Unsupported routes return ErrUnsupportedScope (never fabricate data).
type AssigneeDirectory interface {
	FindMembershipsByRole(ctx context.Context, companyID, roleIDOrCode string) ([]string, error)
	FindMembershipsByDepartment(ctx context.Context, companyID, departmentID string) ([]string, error)
	FindMembershipsByGroup(ctx context.Context, companyID, groupID string) ([]string, error)
	FindCompanyDefaultAssignees(ctx context.Context, companyID string) ([]string, error)
}

// AssigneeResolutionService resolves step assignees with governance from the Role Registry.
type AssigneeResolutionService struct {
	registry *RoleRegistry
	dir      AssigneeDirectory
}

func NewAssigneeResolutionService(registry *RoleRegistry, dir AssigneeDirectory) *AssigneeResolutionService {
	if registry == nil {
		registry = DefaultRoleRegistry()
	}
	return &AssigneeResolutionService{registry: registry, dir: dir}
}

// Resolve applies the precedence: explicit user → role → department → group → company default →
// unresolved. It NEVER returns the current user as a fallback. The error return is only for infra
// failures; "unresolved" is a normal (non-error) result.
func (s *AssigneeResolutionService) Resolve(ctx context.Context, in AssigneeResolutionInput) (AssigneeResolutionResult, error) {
	// 1. explicit user(s)
	if ids := nonEmpty(in.ExplicitMembershipIDs); len(ids) > 0 {
		return AssigneeResolutionResult{Resolved: true, MembershipIDs: ids, Reason: ReasonResolvedExplicitUser, Source: SourceExplicitUser, Scope: ScopeUser}, nil
	}

	// 2. role
	if roleID := strings.TrimSpace(in.RoleID); roleID != "" {
		def, ok := s.registry.GetRole(roleID)
		if !ok {
			return AssigneeResolutionResult{Reason: ReasonUnresolvedUnknownRole, Source: SourceRole, RoleID: roleID, Blocking: true}, nil
		}
		// system-owned roles are resolved by the system, never via a tenant override path.
		if def.Class == RoleClassSystemOwned && in.AttemptedOverride {
			return AssigneeResolutionResult{Reason: ReasonUnresolvedProtected, Source: SourceRole, RoleID: def.Code, Scope: ScopeSystem, Blocking: true}, nil
		}
		if s.dir == nil {
			return AssigneeResolutionResult{Reason: ReasonUnresolvedEmpty, Source: SourceRole, RoleID: def.Code, Blocking: true}, nil
		}
		ids, err := s.dir.FindMembershipsByRole(ctx, in.CompanyID, def.Code)
		if errors.Is(err, ErrUnsupportedScope) {
			return AssigneeResolutionResult{Reason: ReasonUnsupportedScope, Source: SourceRole, RoleID: def.Code, Blocking: true}, nil
		}
		if err != nil {
			return AssigneeResolutionResult{}, err
		}
		if ids = nonEmpty(ids); len(ids) == 0 {
			return AssigneeResolutionResult{Reason: ReasonUnresolvedEmpty, Source: SourceRole, RoleID: def.Code, Blocking: true}, nil
		}
		return AssigneeResolutionResult{Resolved: true, MembershipIDs: ids, Reason: ReasonResolvedRole, Source: SourceRole, RoleID: def.Code, Scope: ScopeCompanyDefault}, nil
	}

	// 3. department
	if dept := strings.TrimSpace(in.DepartmentID); dept != "" {
		return s.resolveViaDirectory(ctx, in.CompanyID, SourceDepartment, ScopeDepartment, func() ([]string, error) {
			if s.dir == nil {
				return nil, ErrUnsupportedScope
			}
			return s.dir.FindMembershipsByDepartment(ctx, in.CompanyID, dept)
		})
	}

	// 4. group
	if grp := strings.TrimSpace(in.GroupID); grp != "" {
		return s.resolveViaDirectory(ctx, in.CompanyID, SourceRole, ScopeGroup, func() ([]string, error) {
			if s.dir == nil {
				return nil, ErrUnsupportedScope
			}
			return s.dir.FindMembershipsByGroup(ctx, in.CompanyID, grp)
		})
	}

	// 5. no mapping
	return AssigneeResolutionResult{Reason: ReasonUnresolvedNoMapping, Source: SourceNone, Blocking: true}, nil
}

func (s *AssigneeResolutionService) resolveViaDirectory(_ context.Context, _ string, source, scope string, find func() ([]string, error)) (AssigneeResolutionResult, error) {
	ids, err := find()
	if errors.Is(err, ErrUnsupportedScope) {
		return AssigneeResolutionResult{Reason: ReasonUnsupportedScope, Source: source, Scope: scope, Blocking: true}, nil
	}
	if err != nil {
		return AssigneeResolutionResult{}, err
	}
	if ids = nonEmpty(ids); len(ids) == 0 {
		return AssigneeResolutionResult{Reason: ReasonUnresolvedEmpty, Source: source, Scope: scope, Blocking: true}, nil
	}
	reason := ReasonResolvedDepartment
	if scope != ScopeDepartment {
		reason = ReasonResolvedRole
	}
	return AssigneeResolutionResult{Resolved: true, MembershipIDs: ids, Reason: reason, Source: source, Scope: scope}, nil
}

func nonEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// WorkflowAssigneeResolverAdapter adapts this service to the narrow resolver interface the workflow
// runtime depends on (structural match; no import of the workflow package, so no import cycle).
type WorkflowAssigneeResolverAdapter struct {
	Service *AssigneeResolutionService
}

// ResolveStepAssignees resolves a step's assignees and returns (membershipIDs, resolved).
// Never returns the current user.
func (a WorkflowAssigneeResolverAdapter) ResolveStepAssignees(ctx context.Context, companyID, _ string, roleID string, explicitMembershipIDs []string) ([]string, bool) {
	if a.Service == nil {
		return nil, false
	}
	res, err := a.Service.Resolve(ctx, AssigneeResolutionInput{
		CompanyID:             companyID,
		RoleID:                roleID,
		ExplicitMembershipIDs: explicitMembershipIDs,
	})
	if err != nil || !res.Resolved {
		return nil, false
	}
	return res.MembershipIDs, true
}
