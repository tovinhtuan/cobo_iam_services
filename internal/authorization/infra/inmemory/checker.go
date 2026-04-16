package inmemory

import (
	"context"
	"strings"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type Checker struct{}

func NewChecker() *Checker { return &Checker{} }

func (c *Checker) Check(_ context.Context, req authapp.AuthorizeRequest, effective *authapp.EffectiveAccessSummary) (*authapp.AuthorizeDecision, error) {
	if req.Subject.CompanyID != effective.CompanyID || req.Subject.MembershipID != effective.MembershipID {
		code := perr.CodeCompanyScopeMismatch
		return &authapp.AuthorizeDecision{Decision: authapp.DecisionDeny, MatchedPermissions: []string{}, ScopeReasons: []string{}, ResponsibilityReasons: []string{}, DenyReasonCode: &code}, nil
	}

	mapped := mapActionToPermissions(req.Action)
	if len(mapped) == 0 || !containsAny(effective.Permissions, mapped) {
		code := perr.CodePermissionDenied
		return &authapp.AuthorizeDecision{Decision: authapp.DecisionDeny, MatchedPermissions: []string{}, ScopeReasons: []string{}, ResponsibilityReasons: []string{}, DenyReasonCode: &code}, nil
	}

	scopeReasons := []string{}
	if effective.DataScope.HasCompanyWideAccess {
		scopeReasons = append(scopeReasons, "company_wide_access")
	} else {
		if depID, _ := req.Resource.Attributes["department_id"].(string); depID != "" {
			if containsDepartment(effective.DataScope.Departments, depID) {
				scopeReasons = append(scopeReasons, "department_membership:"+depID)
			}
		}
		if containsAssignment(effective.DataScope.RecordAssignments, req.Resource.Type, req.Resource.ID) {
			scopeReasons = append(scopeReasons, "assignment:"+req.Resource.Type+":"+req.Resource.ID)
		}
	}
	if len(scopeReasons) == 0 {
		code := perr.CodeDataScopeDenied
		return &authapp.AuthorizeDecision{Decision: authapp.DecisionDeny, MatchedPermissions: mapped, ScopeReasons: []string{}, ResponsibilityReasons: []string{}, DenyReasonCode: &code}, nil
	}

	respReasons := []string{}
	if strings.Contains(req.Action, "approve") {
		if containsResponsibility(effective.Responsibilities, "workflow_approver:disclosure") {
			respReasons = append(respReasons, "workflow_assignee_rule:legal_approval")
		} else {
			code := perr.CodeResponsibilityRequired
			return &authapp.AuthorizeDecision{Decision: authapp.DecisionDeny, MatchedPermissions: mapped, ScopeReasons: scopeReasons, ResponsibilityReasons: []string{}, DenyReasonCode: &code}, nil
		}
	}

	return &authapp.AuthorizeDecision{
		Decision:              authapp.DecisionAllow,
		MatchedPermissions:    matchedPermissions(effective.Permissions, mapped),
		ScopeReasons:          scopeReasons,
		ResponsibilityReasons: respReasons,
		DenyReasonCode:        nil,
	}, nil
}

func mapActionToPermissions(action string) []string {
	switch strings.TrimSpace(action) {
	case "disclosure.approve":
		return []string{"disclosure.approve", "approve_disclosure"}
	case "disclosure.view":
		return []string{"disclosure.view", "view_disclosure"}
	case "disclosure.create":
		return []string{"disclosure.create", "create_disclosure"}
	case "disclosure.update", "disclosure.edit":
		return []string{"disclosure.edit", "update_disclosure"}
	case "disclosure.submit":
		return []string{"submit_disclosure"}
	case "workflow.create":
		return []string{"create_workflow"}
	case "workflow.review":
		return []string{"review_workflow_task"}
	case "workflow.confirm":
		return []string{"workflow.step.confirm", "confirm_workflow_task"}
	case "workflow.override":
		return []string{"workflow.step.override"}
	case "workflow.resolve_assignees":
		return []string{"create_workflow"}
	case "notification.enqueue":
		return []string{"enqueue_notification"}
	case "notification.dispatch":
		return []string{"dispatch_notification"}
	case "notification.resolve_recipients":
		return []string{"enqueue_notification"}
	case "dashboard.view":
		return []string{"view_dashboard"}
	case "admin.membership.create",
		"admin.membership.update",
		"admin.membership.delete",
		"admin.membership.list",
		"admin.membership.role.assign",
		"admin.membership.role.remove",
		"admin.membership.department.assign",
		"admin.membership.department.remove",
		"admin.membership.title.assign",
		"admin.membership.title.remove",
		"admin.permissions.list",
		"admin.roles.list",
		"admin.role.permission.assign",
		"admin.role.permission.remove",
		"admin.resource_scope_rule.create",
		"admin.workflow_assignee_rule.create",
		"admin.notification_rule.create":
		return []string{"admin_manage_access", "system.settings", "rbac.manage"}
	default:
		return nil
	}
}

func contains(items []string, target string) bool {
	for _, it := range items {
		if strings.EqualFold(it, target) {
			return true
		}
	}
	return false
}

func containsAny(items, targets []string) bool {
	for _, target := range targets {
		if contains(items, target) {
			return true
		}
	}
	return false
}

func matchedPermissions(items, targets []string) []string {
	out := make([]string, 0, len(targets))
	for _, target := range targets {
		if contains(items, target) {
			out = append(out, target)
		}
	}
	return out
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
