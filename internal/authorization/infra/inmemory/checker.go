package inmemory

import (
	"context"
	"strings"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type Checker struct{}

func NewChecker() *Checker { return &Checker{} }

func (c *Checker) Check(_ context.Context, req authapp.AuthorizeRequest, effective *authapp.EffectiveAccessSummary, policy *authapp.ActionPolicy) (*authapp.AuthorizeDecision, error) {
	if req.Subject.CompanyID != effective.CompanyID || req.Subject.MembershipID != effective.MembershipID {
		code := perr.CodeCompanyScopeMismatch
		return &authapp.AuthorizeDecision{Decision: authapp.DecisionDeny, MatchedPermissions: []string{}, ScopeReasons: []string{}, ResponsibilityReasons: []string{}, DenyReasonCode: &code}, nil
	}

	required := strings.TrimSpace(policy.RequiredPermission)
	if required == "" || !contains(effective.Permissions, required) {
		code := perr.CodePermissionDenied
		return &authapp.AuthorizeDecision{Decision: authapp.DecisionDeny, MatchedPermissions: []string{}, ScopeReasons: []string{}, ResponsibilityReasons: []string{}, DenyReasonCode: &code}, nil
	}

	scopeReasons, scopeAllowed := evalScope(req, effective, policy)
	if !scopeAllowed {
		code := perr.CodeDataScopeDenied
		return &authapp.AuthorizeDecision{
			Decision:              authapp.DecisionDeny,
			MatchedPermissions:    []string{required},
			ScopeReasons:          []string{},
			ResponsibilityReasons: []string{},
			DenyReasonCode:        &code,
		}, nil
	}

	if !evalWorkflowState(req, policy) {
		code := perr.CodeStateConflict
		return &authapp.AuthorizeDecision{
			Decision:              authapp.DecisionDeny,
			MatchedPermissions:    []string{required},
			ScopeReasons:          scopeReasons,
			ResponsibilityReasons: []string{},
			DenyReasonCode:        &code,
		}, nil
	}

	respReasons := []string{}
	if !evalEligibleActor(effective, policy) {
		code := perr.CodeResponsibilityRequired
		return &authapp.AuthorizeDecision{
			Decision:              authapp.DecisionDeny,
			MatchedPermissions:    []string{required},
			ScopeReasons:          scopeReasons,
			ResponsibilityReasons: []string{},
			DenyReasonCode:        &code,
		}, nil
	}
	if strings.Contains(strings.ToLower(req.Action), "approve") {
		if containsResponsibility(effective.Responsibilities, "workflow_approver:disclosure") {
			respReasons = append(respReasons, "workflow_assignee_rule:legal_approval")
		} else {
			code := perr.CodeResponsibilityRequired
			return &authapp.AuthorizeDecision{
				Decision:              authapp.DecisionDeny,
				MatchedPermissions:    []string{required},
				ScopeReasons:          scopeReasons,
				ResponsibilityReasons: []string{},
				DenyReasonCode:        &code,
			}, nil
		}
	}

	return &authapp.AuthorizeDecision{
		Decision:              authapp.DecisionAllow,
		MatchedPermissions:    []string{required},
		ScopeReasons:          scopeReasons,
		ResponsibilityReasons: respReasons,
		DenyReasonCode:        nil,
	}, nil
}

func evalScope(req authapp.AuthorizeRequest, effective *authapp.EffectiveAccessSummary, policy *authapp.ActionPolicy) ([]string, bool) {
	if effective.DataScope.HasCompanyWideAccess {
		return []string{"company_wide_access"}, true
	}
	parts := splitOrStar(policy.ScopeType)
	if len(parts) == 0 {
		return []string{"scope:any"}, true
	}
	scopeReasons := []string{}
	for _, p := range parts {
		switch p {
		case "assigned_only":
			if containsAssignment(effective.DataScope.RecordAssignments, req.Resource.Type, req.Resource.ID) {
				scopeReasons = append(scopeReasons, "assignment:"+req.Resource.Type+":"+req.Resource.ID)
			}
		case "owner_only":
			if ownerID, _ := req.Resource.Attributes["owner_membership_id"].(string); ownerID != "" && strings.EqualFold(ownerID, effective.MembershipID) {
				scopeReasons = append(scopeReasons, "owner_only")
			}
		case "org_unit_self", "org_unit_subtree":
			if ouID, _ := req.Resource.Attributes["org_unit_id"].(string); ouID != "" {
				if p == "org_unit_self" && contains(effective.DataScope.OrgUnitIDs, ouID) {
					scopeReasons = append(scopeReasons, "org_unit_self:"+ouID)
				}
				if p == "org_unit_subtree" && contains(effective.DataScope.OrgSubtreeUnitIDs, ouID) {
					scopeReasons = append(scopeReasons, "org_unit_subtree:"+ouID)
				}
			}
			if depID, _ := req.Resource.Attributes["department_id"].(string); depID != "" && containsDepartment(effective.DataScope.Departments, depID) {
				scopeReasons = append(scopeReasons, "department_membership:"+depID)
			}
		case "company_wide":
			// already checked at top; keep deny if not present.
		case "*":
			scopeReasons = append(scopeReasons, "scope:any")
		}
	}
	return scopeReasons, len(scopeReasons) > 0
}

func evalWorkflowState(req authapp.AuthorizeRequest, policy *authapp.ActionPolicy) bool {
	parts := splitOrStar(policy.WorkflowState)
	if len(parts) == 0 {
		return true
	}
	if contains(parts, "*") {
		return true
	}
	state, _ := req.Resource.Attributes["workflow_state"].(string)
	state = strings.TrimSpace(state)
	if state == "" {
		return true
	}
	return contains(parts, state)
}

func evalEligibleActor(effective *authapp.EffectiveAccessSummary, policy *authapp.ActionPolicy) bool {
	parts := splitOrStar(policy.EligibleActor)
	if len(parts) == 0 || contains(parts, "*") {
		return true
	}
	for _, p := range parts {
		if contains(effective.DataScope.PositionCodes, strings.ToLower(p)) {
			return true
		}
		if contains(effective.Responsibilities, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

func splitOrStar(s string) []string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" || s == "*" {
		return []string{"*"}
	}
	raw := strings.Split(s, "|")
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func contains(items []string, target string) bool {
	for _, it := range items {
		if strings.EqualFold(it, target) {
			return true
		}
	}
	return false
}

func containsDepartment(items []authapp.DepartmentScope, depID string) bool {
	for _, it := range items {
		if it.DepartmentID == depID {
			return true
		}
	}
	return false
}

func containsAssignment(items []authapp.ResourceAssignment, resourceType, resourceID string) bool {
	for _, it := range items {
		if it.ResourceType == resourceType && it.ResourceID == resourceID {
			return true
		}
	}
	return false
}

func containsResponsibility(items []string, target string) bool {
	for _, it := range items {
		if strings.EqualFold(it, target) {
			return true
		}
	}
	return false
}
